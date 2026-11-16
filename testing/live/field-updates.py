#!/usr/bin/env python3
"""Persistence-only scalar mutations, one field at a time, always restoring.

Use Python 3 and the campaign's existing Ruby YAML/JSON prerequisite; no pip
packages are needed. Values/configs/state remain private.
No vendor tests, syncs, provisioning flags, auth-mode changes or destruction.
Each saved plan is inspected before apply: only an in-place update is allowed.
"""
import argparse
import collections
import concurrent.futures
import copy
import csv
import datetime
import json
import os
from pathlib import Path
import re
import subprocess
import threading
import stat
import sys
import uuid
from contextlib import contextmanager
from urllib.parse import urlsplit, urlunsplit


def private_path(run, path, missing=False):
    """lstat every component; never canonicalize away an unsafe link."""
    run, path = Path(run), Path(path)
    if (not run.is_absolute() or not path.is_absolute() or '..' in run.parts
            or '..' in path.parts or not path.is_relative_to(run)):
        raise ValueError('unsafe private run path')
    for node in reversed([path, *path.parents]):
        try:
            info = node.lstat()
        except FileNotFoundError:
            if missing and (node == run or run in node.parents):
                continue
            raise
        inside = node == run or run in node.parents
        directory = stat.S_ISDIR(info.st_mode)
        if (stat.S_ISLNK(info.st_mode) or not (directory or stat.S_ISREG(info.st_mode))
                or ((node != path or node == run) and not directory)
                or (inside and (info.st_uid != os.getuid() or info.st_mode & 0o077
                                or (directory and info.st_mode & 0o500 != 0o500)
                                or (not directory and info.st_nlink != 1)))
                or (not inside and info.st_mode & 0o022
                    and not (info.st_uid == 0 and info.st_mode & stat.S_ISVTX))):
            raise ValueError('unsafe private run path')
    return path


def source_file(path):
    """Repository inputs are public, but must be regular and not redirectable."""
    for node in [path, *path.parents]:
        info = node.lstat()
        expected = stat.S_ISREG(info.st_mode) if node == path else stat.S_ISDIR(info.st_mode)
        if (not expected or info.st_uid not in {0, os.getuid()}
                or (node == path and info.st_nlink != 1)
                or (info.st_mode & 0o022 and not (info.st_uid == 0 and info.st_mode & stat.S_ISVTX))):
            raise ValueError('unsafe repository input path')
    return path


def unreadable_tree(error):
    raise ValueError('unreadable private run directory') from None


def private_tree(run):
    private_path(run, run)
    roles = {'manifest.json': False, 'ledger.jsonl': False,
             'inventory/schema.json': False, 'inventory/integration-cases.json': False,
             'inventory': True, 'work': True, 'evidence': True,
             'field-updates': True, 'field-updates/attempts': True}
    for relative, directory in roles.items():
        path = Path(run) / relative
        try:
            info = private_path(run, path).lstat()
        except FileNotFoundError:
            continue
        if not (stat.S_ISDIR(info.st_mode) if directory else stat.S_ISREG(info.st_mode)):
            raise ValueError('unsafe private input or output type')
    for directory, dirs, files in os.walk(run, followlinks=False, onerror=unreadable_tree):
        for name in dirs + files:
            path = private_path(run, Path(directory) / name)
            if name in {'main.tf.json', 'terraform.tfstate'} and not stat.S_ISREG(path.lstat().st_mode):
                raise ValueError('unsafe private fixture type')


@contextmanager
def private_open(run, path, mode):
    private_tree(run)
    path = private_path(run, path, missing=True)
    # Walk with directory descriptors so no intermediate symlink is followed.
    parent = os.open('/', os.O_RDONLY | os.O_DIRECTORY)
    try:
        for component in path.parent.parts[1:]:
            child = os.open(component, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=parent)
            os.close(parent)
            parent = child
        flags = os.O_WRONLY | os.O_CREAT | os.O_NOFOLLOW
        flags |= os.O_APPEND if 'a' in mode else os.O_EXCL if 'x' in mode else 0
        fd = os.open(path.name, flags, 0o600, dir_fd=parent)
        info = os.fstat(fd)
        if (not stat.S_ISREG(info.st_mode) or info.st_uid != os.getuid()
                or info.st_mode & 0o077 or info.st_nlink != 1):
            os.close(fd)
            raise ValueError('unsafe private output file')
        if 'w' in mode:
            os.ftruncate(fd, 0)
        with os.fdopen(fd, mode) as stream:
            yield stream
    finally:
        os.close(parent)


def private_write(run, path, data):
    with private_open(run, path, 'wb' if isinstance(data, bytes) else 'w') as stream:
        stream.write(data)


class ExclusiveLock:
    """O_EXCL ownership; interrupted/stale locks require manual recovery."""
    def __init__(self, run, path):
        self.run, self.path = Path(run), Path(path)
        self.identity = None
        self.token = uuid.uuid4().hex + '\n'

    def __enter__(self):
        private_tree(self.run)
        try:
            with private_open(self.run, self.path, 'x') as stream:
                self.identity = os.fstat(stream.fileno())
                stream.write(self.token)
        except FileExistsError:
            raise ValueError('field execution locked; manual recovery required') from None
        return self

    def __exit__(self, *exc):
        # Never remove another owner's replacement (including a symlink).
        try:
            current = self.path.lstat()
        except FileNotFoundError:
            return
        if (self.identity is not None and stat.S_ISREG(current.st_mode)
                and (current.st_dev, current.st_ino) == (self.identity.st_dev, self.identity.st_ino)):
            private_path(self.run, self.path)
            if self.path.read_text() == self.token:
                self.path.unlink()


ROOT = Path(__file__).resolve().parents[2]
SAFETY = {'name', 'type', 'tags', 'create_if_not_exists', 'use_role_arn',
          'use_service_account', 'use_iam_auth', 'use_msk_iam', 'use_rds_iam',
          'use_irsa', 'auth_mode', 'use_connect_server'}


def mutate(typ, key, value, nested=False):
    if (key in SAFETY and not (nested and key == 'name')) or 'provision' in key:
        return None, 'identity/tags already tested, provisioning, or credential-mode transition excluded'
    if isinstance(value, bool):
        return not value, 'explicit boolean toggle; no vendor connection'
    if key == 'ssl_mode':
        mysql_mode = typ in {'mysql', 'proxysql'}
        mode = 'required' if mysql_mode else ('skip-verify' if typ in {'sql_server', 'azuresqlserver'} else 'verify-full')
        return ('preferred' if mysql_mode else 'require') if value == mode else mode, 'native driver TLS enum'
    if key == 'ldap_encryption_method':
        return ('starttls' if value != 'starttls' else 'ssl'), 'LDAP transport enum'
    if key == 'protocol':
        return ('udp' if value != 'udp' else 'tcp'), 'syslog transport enum'
    if key.endswith('port') or key == 'port':
        n = int(value)
        result = n + 1 if n < 65535 else n - 1
        return (str(result) if isinstance(value, str) else result), 'usable changed TCP port'
    if key == 'proxysql_hostgroup_id':
        return str(int(value) + 1), 'numeric hostgroup identifier'
    if not isinstance(value, str):
        return None, 'non-scalar input outside configured scalar cohort'
    if key in {'uri', 'url', 'webhook_url', 'api_server', 'urls'}:
        u = urlsplit(value)
        if u.hostname and '.invalid' in u.hostname:
            return urlunsplit((u.scheme, u.netloc.replace(u.hostname, 'field-update.invalid'), u.path, u.query, u.fragment)), 'retain scheme/path and synthetic credentials; change only .invalid host'
        return None, 'URL cannot safely vary only its synthetic .invalid host'
    if key in {'host', 'hostname', 'ldap_hostname', 'keyspaces_endpoint', 'domain', 'dd_site', 'machine'}:
        return ('field-update.invalid' if value != 'field-update.invalid' else 'field-update-2.invalid'), 'synthetic host/domain'
    if key in {'credential_json', 'key_file'}:
        try:
            v = json.loads(value)
        except ValueError:
            return None, 'structured credential is not JSON; avoid invalid synthetic key material'
        if not isinstance(v, dict):
            return None, 'non-object credential JSON'
        # Preserve structure and rotate a synthetic credential component only.
        v['private_key_id'] = 'field-update-synthetic-key-id'
        return json.dumps(v, separators=(',', ':')), 'rotate synthetic JSON credential key identifier'
    if key == 'client_configuration':
        return value + '\nclient.id=field-update\n', 'valid Kafka properties addition'
    if key == 'fields':
        return '[{"name":"field-update","selector":"#field-update","value":"synthetic"}]', 'synthetic browser field JSON'
    if key in {'cpu', 'memory', 'storage'}:
        return {'cpu': '2', 'memory': '4Gi', 'storage': '20Gi'}[key] if value != {'cpu': '2', 'memory': '4Gi', 'storage': '20Gi'}[key] else {'cpu': '3', 'memory': '6Gi', 'storage': '30Gi'}[key], 'synthetic desktop capacity string'
    if key == 'version':
        if typ == 'keyspaces':
            return None, 'Keyspaces fixed protocol version is not a meaningful configurable transition'
        return '6.0' if value != '6.0' else '7.0', 'Mongo version selector'
    if key in {'region', 'region_name', 'aws_region_name'}:
        return 'us-west-2' if value != 'us-west-2' else 'us-east-1', 'valid AWS region string'
    if key in {'arn', 'aws_arn'}:
        return value + '-field-update', 'synthetic ARN resource suffix'
    if key in {'ldap_search_bind_dn', 'ldap_user_base_dn'}:
        return ('cn=field-update,dc=example,dc=invalid' if key.endswith('bind_dn') else 'ou=field-update,dc=example,dc=invalid'), 'valid synthetic LDAP DN'
    if key == 'network_id':
        return '0123456789abcdef' if value != '0123456789abcdef' else '1123456789abcdef', 'hex ZeroTier network identifier'
    return value + '-field-update', 'nonempty synthetic string/credential rotation'


def leaves(attrs):
    for key, value in attrs.items():
        if key == 'targets':
            for i, target in enumerate(value):
                for k, v in target.items():
                    yield (key, i, k), v
                # Exercise optional RDP fields even if the baseline omitted them.
                for k, v in {'name': '', 'domain': '', 'record': False}.items():
                    if k not in target:
                        yield (key, i, k), v
        else:
            yield (key,), value


def put(attrs, path, value):
    node = attrs
    for part in path[:-1]:
        node = node[part]
    node[path[-1]] = value


def get(node, path):
    for part in path:
        node = node[part]
    return node


def attrs(config):
    return config['resource']['adaptive_resource']['test']


def compare_remote(observed, expected, sensitive):
    withheld = observed is None or (isinstance(observed, str) and ('redacted' in observed.lower() or observed in {'********', '*****', '***', '<sensitive>'}))
    if withheld and sensitive:
        return 'exact-state-and-redacted-remote-readback'
    if isinstance(observed, bool) != isinstance(expected, bool) or observed != expected:
        raise RuntimeError('remote field does not equal intended value')
    return 'exact-state-and-exact-remote-value'


def compare_config_field(integration_type, key, configuration, observed, expected, sensitive):
    # Both the provider's mongodb36 builder and backend mongo36 runtime use
    # bool `yaml:"useTLS,omitempty"`; runtime unmarshals into a zero-value
    # MongoDBIntegrationConfiguration. An ABSENT key therefore encodes false.
    # Explicit null, another integration/key, or true is never equivalent.
    # The caller still requires exact state and post-apply no-op corroboration.
    if (integration_type == 'mongodb36' and key == 'useTLS'
            and key not in configuration and observed is None
            and expected is False and not sensitive):
        return 'exact-state-and-source-defined-omitted-false'
    return compare_remote(observed, expected, sensitive)


class HarnessBlocker(RuntimeError):
    pass


class RubyYaml:
    """Use the campaign's existing Ruby YAML/JSON prerequisite, no pip install."""
    @staticmethod
    def safe_load(text):
        result = subprocess.run(['ruby', '-ryaml', '-rjson', '-e',
                                 'STDOUT.write(JSON.generate(YAML.safe_load(STDIN.read)))'],
                                input=text, text=True, capture_output=True)
        if result.returncode:
            raise RuntimeError('cannot parse private YAML export')
        return json.loads(result.stdout)


class Cohort:
    def __init__(self, run):
        self.yaml = RubyYaml()
        self.run = Path(run)
        private_tree(self.run)
        private_path(self.run, self.run / 'evidence')
        self.runner = source_file(ROOT / 'testing/live/run.sh')
        source_file(ROOT / 'testing/live/integration-cases.json')
        source_file(ROOT / 'docs/parity-current-integration-fields.csv')
        self.base = self.run / 'field-updates'
        self.lock = threading.Lock()
        self.maps = {}
        self.schema = json.loads((self.run / 'inventory/schema.json').read_text())['provider_schemas']['registry.terraform.io/adaptive-scale/adaptive']['resource_schemas']['adaptive_resource']['block']
        with (ROOT / 'docs/parity-current-integration-fields.csv').open() as h:
            for row in csv.DictReader(h):
                if row['Reverse']:
                    self.maps[(row['Integration'], row['Reverse'])] = row['Key']
        # Ground originals in the resource stage's run-pinned inventory, never
        # a historical cohort size or a broad ledger-name prefix alone.
        inventory = json.loads((self.run / 'inventory/integration-cases.json').read_text())
        source_inventory = json.loads((ROOT / 'testing/live/integration-cases.json').read_text())
        if inventory != source_inventory:
            raise ValueError('Run-pinned integration inventory differs from source inventory')
        types = [entry['type'] for entry in inventory['cases']]
        if (not types or len(types) != len(set(types))
                or any(not re.fullmatch(r'[a-z0-9_-]+', typ) for typ in types)):
            raise ValueError('Expected nonempty unique original integration types')
        originals = {'integration-' + typ: typ for typ in types}
        records = [json.loads(x) for x in (self.run / 'ledger.jsonl').read_text().splitlines()]
        latest = {}
        for row in records:
            if row['phase'] == 'updated-noop' and row['case'].startswith('integration-'):
                if row['case'] not in originals:
                    raise ValueError('Unknown or nonoriginal integration eligibility record')
                latest[row['case']] = row['status']
        self.cases = sorted(c for c, status in latest.items() if status == 'PASS')
        if not self.cases:
            raise ValueError('Expected nonempty original passing integrations')
        prefix = json.loads((self.run / 'manifest.json').read_text())['prefix']
        for case in self.cases:
            private_path(self.run, self.run / 'work' / case / 'terraform.tfstate')
            original = attrs(json.loads((self.run / 'work' / case / 'main.tf.json').read_text()))
            if original['type'] != originals[case] or original['name'] != prefix + '-' + case:
                raise ValueError('Eligible fixture does not match original owned name and type')
        self.stamp = datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ') + '-' + uuid.uuid4().hex
        private_path(self.run, self.base, missing=True)
        self.base.mkdir(mode=0o700, exist_ok=True)
        attempts = self.base / 'attempts'
        private_path(self.run, attempts, missing=True)
        attempts.mkdir(mode=0o700, exist_ok=True)
        self.out = attempts / self.stamp
        private_path(self.run, self.out, missing=True)
        self.out.mkdir(mode=0o700)

    def record(self, case, phase, status, evidence, **extra):
        private_tree(self.run)
        private_path(self.run, Path(evidence))
        row = dict(at=datetime.datetime.now(datetime.timezone.utc).isoformat(), case=case,
                   phase=phase, status=status, exit=0 if status == 'PASS' else 1,
                   evidence=str(evidence), **extra)
        with self.lock:
            with private_open(self.run, self.run / 'ledger.jsonl', 'a') as h:
                h.write(json.dumps(row) + '\n')
        return row

    def tf(self, case, phase, expected, *args):
        private_tree(self.run)
        source_file(self.runner)
        private_path(self.run, self.out / (case + '-runner.log'), missing=True)
        # Open/validate the diagnostic destination BEFORE invoking any command.
        with private_open(self.run, self.out / (case + '-runner.log'), 'ab') as h:
            p = subprocess.run(['bash', str(self.runner), 'tf', str(self.run), case,
                                phase, str(expected), *args], stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
            h.write(p.stdout)
        if b'fixture guard rejected' in p.stdout:
            raise HarnessBlocker('fixture guard rejected command; harness blocker, not provider failure')
        return p.returncode == 0

    def show(self, case, phase, file=None):
        private_tree(self.run)
        dest = self.out / (case + '-' + phase + '.json')
        command = ['terraform', '-chdir=' + str(self.run / 'work' / case), 'show', '-json']
        if file:
            private_path(self.run, self.run / 'work' / case / file)
            command.append(file)
        # Match run.sh/guard.py isolation: inherited logging/data paths must not
        # redirect this direct `show` subprocess outside the validated Run.
        env = {k: v for k, v in os.environ.items() if not k.startswith('TF_CLI_ARGS')
               and k not in {'ADAPTIVE_SVC_TOKEN', 'ADAPTIVE_URL', 'ADAPTIVE_PROFILE',
                             'TF_LOG', 'TF_LOG_PATH', 'TF_REATTACH_PROVIDERS', 'TF_DATA_DIR', 'TF_WORKSPACE'}}
        env.update(TF_CLI_CONFIG_FILE=str(self.run / 'terraform.rc'), TF_IN_AUTOMATION='1', CHECKPOINT_DISABLE='1')
        with private_open(self.run, dest, 'xb') as h:
            p = subprocess.run(command, env=env, stdout=h, stderr=subprocess.DEVNULL)
        if p.returncode:
            raise RuntimeError('terraform show failed')
        return json.loads(dest.read_text()), dest

    def readback(self, case, name, phase):
        private_tree(self.run)
        source_file(self.runner)
        before = set((self.run / 'evidence').glob(case + '-readback-*.yaml'))
        dest = self.out / (case + '-' + phase + '-remote.yaml')
        with private_open(self.run, dest, 'xb') as h:
            p = subprocess.run(['bash', str(self.runner), 'readback', str(self.run), case, 'resources', name], stdout=h, stderr=subprocess.STDOUT)
        # run.sh itself writes exact-name CLI exports; locate its private evidence.
        private_tree(self.run)
        logs = sorted(set((self.run / 'evidence').glob(case + '-readback-*.yaml')) - before)
        if p.returncode or len(logs) != 1:
            raise RuntimeError('exact-name CLI readback failed')
        export = self.out / (case + '-' + phase + '-remote-export.yaml')
        private_write(self.run, export, logs[0].read_bytes())
        data = self.yaml.safe_load(export.read_text())
        self.record(case, phase + '-readback', 'PASS', export)
        return data, export

    def verify(self, case, original, path, expected, phase):
        state, statepath = self.show(case, phase + '-state')
        resources = state['values']['root_module']['resources']
        resource = next(x for x in resources if x['address'] == 'adaptive_resource.test')
        if get(resource['values'], path) != expected:
            raise RuntimeError('state field does not equal intended value')
        remote, evidence = self.readback(case, original['name'], phase)
        config = remote.get('configuration', {})
        key = self.maps.get((original['type'], path[0]))
        observed = config.get(key) if key else None
        verification = 'exact-state-and-redacted-remote-readback'
        if path[0] == 'targets' and isinstance(observed, str):
            decoded = self.yaml.safe_load(observed)
            observed = get(decoded, path[1:])
        elif len(path) > 1:
            observed = None
        schema = (self.schema['block_types']['targets']['block']['attributes'][path[-1]]
                  if path[0] == 'targets' else self.schema['attributes'][path[0]])
        verification = compare_config_field(original['type'], key, config, observed, expected, schema.get('sensitive', False))
        self.record(case, phase + '-verify', 'PASS', evidence, verification=verification, state_evidence=str(statepath))
        return verification

    def plan_apply(self, case, phase, allow_noop=False):
        plan = phase + '-' + self.stamp + '.tfplan'
        planpath = private_path(self.run, self.run / 'work' / case / plan, missing=True)
        if planpath.exists():
            raise ValueError('saved plan already exists; refusing overwrite')
        # Restore can legitimately be a no-op after a rejected update. This is
        # an explicit separate expected condition, never a mutation expectation.
        if allow_noop:
            ok = self.tf(case, phase + '-plan', 0, 'plan', '-input=false', '-out=' + plan)
        else:
            ok = self.tf(case, phase + '-plan', 2, 'plan', '-input=false', '-detailed-exitcode', '-out=' + plan)
        if not ok:
            raise RuntimeError('saved plan failed')
        data, evidence = self.show(case, phase + '-plan', plan)
        changes = [x for x in data.get('resource_changes', []) if x['change']['actions'] != ['no-op']]
        if any(x['address'] != 'adaptive_resource.test' or x['change']['actions'] != ['update'] for x in changes):
            raise RuntimeError('refusing non-update saved plan')
        if not allow_noop and not changes:
            raise RuntimeError('mutation did not produce an update')
        self.record(case, phase + '-plan-safety', 'PASS', evidence)
        if not self.tf(case, phase + '-apply', 0, 'apply', '-input=false', '-auto-approve', plan):
            raise RuntimeError('saved apply failed')
        if not self.tf(case, phase + '-noop', 0, 'plan', '-input=false', '-detailed-exitcode'):
            raise RuntimeError('post-apply plan was not a no-op')

    def case(self, case):
        if case not in self.cases:
            raise ValueError('only original passing integrations may be selected')
        with ExclusiveLock(self.run, self.run / 'work' / case / '.field-updates.lock'):
            return self._case(case)

    def _case(self, case):
        private_tree(self.run)
        dest = self.out / (case + '-results.json')
        configpath = self.run / 'work' / case / 'main.tf.json'
        backup = self.out / (case + '-original.tf.json')
        if backup.exists():
            original_bytes = backup.read_bytes()
            if configpath.read_bytes() != original_bytes:
                raise RuntimeError('unrestored original exists; manual recovery required before rerun')
        else:
            original_bytes = configpath.read_bytes()
            # Include legacy and all earlier attempts; never bypass dirty recovery.
            previous = list(self.base.glob(case + '-original.tf.json'))
            previous += list((self.base / 'attempts').glob('*/' + case + '-original.tf.json'))
            if any(path.read_bytes() != original_bytes for path in previous):
                raise RuntimeError('unrestored original exists; manual recovery required before rerun')
            private_write(self.run, backup, original_bytes)
        baseline = json.loads(original_bytes)
        a = attrs(baseline)
        if not a['name'].startswith(json.loads((self.run / 'manifest.json').read_text())['prefix'] + '-integration-'):
            raise RuntimeError('not an original owned integration')
        results = []
        for path, value in leaves(a):
            field = '.'.join(map(str, path))
            changed, reason = mutate(a['type'], path[-1], value, nested=len(path) > 1)
            result = dict(case=case, field=field, reason=reason, status='SKIP')
            if changed is None:
                results.append(result)
                self.record(case, 'field-' + field.replace('.', '-') + '-skip', 'SKIP', backup, reason=reason)
                private_write(self.run, dest, json.dumps(results, indent=2))
                continue
            phase = 'field-' + re.sub('[^a-z0-9_-]', '-', field)
            result.update(status='FAIL', attempted=True, restored=False)
            candidate = copy.deepcopy(baseline)
            put(attrs(candidate), path, changed)
            try:
                private_tree(self.run)
                private_write(self.run, configpath, json.dumps(candidate, indent=2) + '\n')
                self.plan_apply(case, phase)
                result['verification'] = self.verify(case, a, path, changed, phase)
                result['status'] = 'PASS'
            except Exception as exc:
                result['error'] = 'private operation failed'
                result['failure_kind'] = 'harness-blocker' if isinstance(exc, HarnessBlocker) else 'persistence-verification-failure'
                self.record(case, phase + '-failure', 'FAIL', self.out, error='private operation failed', failure_kind=result['failure_kind'])
                # Observe both state and current remote before any recovery write.
                try:
                    self.show(case, phase + '-failure-observed-state')
                    self.readback(case, a['name'], phase + '-failure-observed')
                except Exception as observe:
                    result['observation_error'] = 'private observation failed'
            finally:
                private_write(self.run, configpath, original_bytes)
                try:
                    self.plan_apply(case, phase + '-restore', allow_noop=True)
                    # Optional RDP fields absent in baseline return to SDK defaults.
                    restored = value
                    self.verify(case, a, path, restored, phase + '-restore')
                    result['restored'] = True
                except Exception as exc:
                    result['status'] = 'FAIL'
                    result['restore_error'] = 'private operation failed'
                    self.record(case, phase + '-restore-failure', 'FAIL', backup, error='private operation failed')
            results.append(result)
            private_write(self.run, dest, json.dumps(results, indent=2))
            print(case, field, result['status'], 'restored=' + str(result['restored']), flush=True)
            if not result['restored']:
                # Remaining fields are blocked, never compound a dirty fixture.
                completed = {x['field'] for x in results}
                for pending, _ in leaves(a):
                    f = '.'.join(map(str, pending))
                    if f not in completed:
                        results.append(dict(case=case, field=f, status='SKIP', reason='blocked by unresolved restoration'))
                private_write(self.run, dest, json.dumps(results, indent=2))
                break
        return results

    def inventory(self):
        private_tree(self.run)
        rows = []
        configured = set()
        for case in self.cases:
            a = attrs(json.loads((self.run / 'work' / case / 'main.tf.json').read_text()))
            for p, v in leaves(a):
                changed, why = mutate(a['type'], p[-1], v, nested=len(p) > 1)
                rows.append(dict(case=case, field='.'.join(map(str, p)), selected=changed is not None, reason=why))
                configured.add((a['type'], p[0]))
        mapped = []
        for (typ, field), key in sorted(self.maps.items()):
            if (typ, field) not in configured:
                mapped.append(dict(type=typ, field=field, wire_key=key, reason='not configured in eligible original fixture' if 'integration-' + typ in self.cases else 'integration outside eligible passing originals'))
        private_write(self.run, self.out / 'selection.json', json.dumps(rows, indent=2))
        private_write(self.run, self.out / 'mapped-unexercised.json', json.dumps(mapped, indent=2))
        return rows


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('run')
    parser.add_argument('--execute', action='store_true')
    parser.add_argument('--workers', type=int, default=6)
    parser.add_argument('--case', action='append')
    args = parser.parse_args()
    if args.case and len(args.case) != len(set(args.case)):
        raise ValueError('duplicate case selection is forbidden')
    if args.workers < 1:
        raise ValueError('workers must be positive')
    run = Path(args.run)
    with ExclusiveLock(run, run / '.field-updates.lock'):
        return run_cohort(args)


def run_cohort(args):
    c = Cohort(args.run)
    selection = c.inventory()
    print(json.dumps(dict(eligible_fixtures=len(c.cases), selected_fields=sum(x['selected'] for x in selection), skipped_fields=sum(not x['selected'] for x in selection), evidence=str(c.out))))
    if not args.execute:
        return
    cases = args.case or c.cases
    if any(case not in c.cases for case in cases):
        raise ValueError('only original passing integrations may be selected')
    rows = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=args.workers) as pool:
        futures = {pool.submit(c.case, case): case for case in cases}
        for future in concurrent.futures.as_completed(futures):
            try:
                rows.extend(future.result())
            except Exception as exc:
                rows.append(dict(case=futures[future], status='FAIL', error='private operation failed'))
    summary = dict(fixtures=len(cases), attempted=sum(x.get('attempted', False) for x in rows), counts=dict(collections.Counter(x['status'] for x in rows)), restored=sum(x.get('restored', False) for x in rows), fields=rows)
    result = c.out / 'results.json'
    private_write(Path(args.run), result, json.dumps(summary, indent=2))
    # Compatibility discovery only. Ledger evidence always names immutable attempts.
    latest = Path(args.run) / 'field-updates/results.json'
    if latest.exists():
        private_write(Path(args.run), c.out / 'previous-latest-results.json', latest.read_bytes())
    private_write(Path(args.run), latest, json.dumps(dict(summary, mutable_latest=True, immutable_result=str(result)), indent=2))
    print(json.dumps({k: v for k, v in summary.items() if k != 'fields'}))
    return 1 if any(row['status'] == 'FAIL' for row in rows) else 0


if __name__ == '__main__':
    try:
        raise SystemExit(main())
    except Exception:
        print('Field cohort blocked: unsafe paths, selection, lock or prerequisites', file=sys.stderr)
        raise SystemExit(1)
