#!/bin/bash
# Owned synthetic 1Password omission probe; never contacts the vendor API.
set -euo pipefail
set +x
umask 077
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
exec python3 -I - "$ROOT" "${1:?absolute isolated run directory required}" <<'UNCLASSIFIED_PY'
import copy
import datetime
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import uuid
import stat
from contextlib import contextmanager


def private_path(run, path, missing=False):
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
    # Recheck record destinations as well as fixture inputs on every operation.
    for relative, directory in [('ledger.jsonl', False), ('manifest.json', False), ('evidence', True)]:
        path = private_path(run, Path(run)/relative)
        if not (stat.S_ISDIR(path.lstat().st_mode) if directory else stat.S_ISREG(path.lstat().st_mode)):
            raise ValueError('unsafe private input or record type')
    for directory, dirs, files in os.walk(run, followlinks=False, onerror=unreadable_tree):
        for name in dirs + files:
            private_path(run, Path(directory) / name)


@contextmanager
def private_open(run, path, mode):
    private_tree(run)
    path = private_path(run, path, missing=True)
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

CASE = 'import-1password'
LOCAL = 'http://localhost:8080'


def load(path):
    return json.loads(path.read_text())


def attributes(state):
    resources = state['resources']
    assert len(resources) == 1
    resource = resources[0]
    assert (resource['mode'], resource['type'], resource['name']) == ('managed', 'adaptive_resource', 'test')
    assert len(resource['instances']) == 1
    assert not resource['instances'][0].get('deposed')
    return resource['instances'][0]['attributes']


class Probe:
    def __init__(self, root, run):
        private_tree(run)
        source_file(root/'testing/live/integration-cases.json')
        source_file(root/'testing/live/run.sh')
        # All required record/input paths are checked before evidence creation.
        for relative in ['manifest.json', 'ledger.jsonl', 'evidence',
                         'inventory/integration-cases.json',
                         'work/import-1password/main.tf.json', 'work/import-1password/terraform.tfstate',
                         'work/integration-1password/main.tf.json', 'work/integration-1password/terraform.tfstate',
                         'cli-origin/import-1password.id', 'cli-origin/import-1password.json',
                         'cli-origin/ownership.jsonl']:
            path = private_path(run, run / relative)
            info = path.lstat()
            if not (stat.S_ISDIR(info.st_mode) if relative == 'evidence' else stat.S_ISREG(info.st_mode)):
                raise ValueError('unsafe private input type')
        self.root, self.run = root, run
        self.directory = run/'work'/CASE
        self.config = self.directory/'main.tf.json'
        self.state = self.directory/'terraform.tfstate'
        self.stamp = datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ')+'-'+uuid.uuid4().hex
        self.evidence = run/'evidence'/('unclassified-secret-'+self.stamp)
        private_path(run, self.evidence, missing=True)
        self.evidence.mkdir(mode=0o700)
        self.failed = False

    def record(self, phase, good, **checks):
        private_tree(self.run)
        document = dict(checks, passed=bool(good))
        path = self.evidence/(phase+'.json')
        with private_open(self.run, path, 'x') as out:
            json.dump(document, out, sort_keys=True)
        with private_open(self.run, self.run/'ledger.jsonl', 'a') as out:
            out.write(json.dumps(dict(at=datetime.datetime.now(datetime.timezone.utc).isoformat(),case=CASE,
                                     phase=phase,status='PASS' if good else 'FAIL',exit=0 if good else 1,
                                     evidence=str(path)))+'\n')
        self.failed |= not good
        return good

    def attempt(self, phase, action):
        try:
            return action()
        except Exception:
            # Never print exception details: parsers/process errors can contain credentials.
            self.record(phase, False, unreadable_or_invalid=True)
            return None

    def local(self, path):
        private_path(self.run, path)

    def ownership(self):
        # Reject links before reading any fixture input, not after comparing its value.
        for relative in ['manifest.json', 'inventory/integration-cases.json',
                         'work/import-1password/main.tf.json', 'work/import-1password/terraform.tfstate',
                         'work/integration-1password/main.tf.json', 'work/integration-1password/terraform.tfstate',
                         'cli-origin/import-1password.id', 'cli-origin/import-1password.json',
                         'cli-origin/ownership.jsonl']:
            self.local(self.run/relative)
        manifest = load(self.run/'manifest.json')
        assert manifest['url'] == LOCAL
        prefix = manifest['prefix']
        assert re.fullmatch(r'tflive-[a-zA-Z0-9_-]+', prefix)
        self.name = prefix+'-'+CASE
        cases = load(self.run/'inventory/integration-cases.json')['cases']
        source_cases = load(source_file(self.root/'testing/live/integration-cases.json'))['cases']
        def expected(items):
            matches = [c for c in items if c['type'] == '1password']
            assert len(matches) == 1
            value = matches[0]['attributes']['service_account']
            assert isinstance(value, str) and value and value != '<REDACTED>'
            return value
        self.expected = expected(cases)
        assert self.expected == expected(source_cases)
        current = attributes(load(self.state))
        ident = (self.run/'cli-origin'/ (CASE+'.id')).read_text().strip()
        assert re.fullmatch(r'[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}', ident)
        assert current['id'] == ident
        self.ident = ident
        def owned(attrs, name):
            assert attrs['name'] == name and attrs['type'] == '1password'
            assert attrs['service_account'] == self.expected
        owned(current, self.name)
        self.original = self.config.read_bytes()
        owned(load(self.config)['resource']['adaptive_resource']['test'], self.name)
        source = self.run/'work/integration-1password'
        source_attrs = attributes(load(source/'terraform.tfstate'))
        assert re.fullmatch(r'[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}', source_attrs['id'])
        assert source_attrs['id'] != self.ident
        owned(source_attrs, prefix+'-integration-1password')
        owned(load(source/'main.tf.json')['resource']['adaptive_resource']['test'], prefix+'-integration-1password')
        document = self.run/'cli-origin'/(CASE+'.json')
        clone = load(document)
        assert clone['type'] == 'resource'
        owned(clone['configuration'], self.name)
        owners = [json.loads(line) for line in (self.run/'cli-origin/ownership.jsonl').read_text().splitlines()]
        assert any(o['fixture'] == CASE and o['name'] == self.name and o['origin'] == 'adaptive-cli'
                   and o['document'] == str(document) for o in owners)
        assert not any(os.environ.get(k) for k in ['ADAPTIVE_SVC_TOKEN','ADAPTIVE_URL','ADAPTIVE_PROFILE'])
        self.identity()
        private_write(self.run, self.evidence/'original-config.json', self.original)
        self.record('unclassified-ownership', True, source_inventory_matches=True, state_api_id_matches=True)
        return True

    def identity(self):
        private_tree(self.run)
        result = subprocess.run(['adaptive','whoami','-o','json'], capture_output=True)
        assert result.returncode == 0
        identity = json.loads(result.stdout)
        assert identity['URL'] == LOCAL and identity['CredentialSource'] == 'saved context'

    def export(self, include):
        # Recheck exact state/API identity immediately before every privileged export.
        self.local(self.state)
        current = attributes(load(self.state))
        assert current['id'] == self.ident and current['name'] == self.name and current['type'] == '1password'
        self.identity()
        args = ['adaptive','admin','export','resources','--name',self.name]
        if include:
            args.append('--include-secrets')
        raw = subprocess.run(args, capture_output=True)
        assert raw.returncode == 0
        # Parse in memory, never persist or echo a raw include-secrets response.
        parsed = subprocess.run(['ruby','-ryaml','-rjson','-e',
                                 'puts JSON.generate(YAML.safe_load(STDIN.read))'],
                                input=raw.stdout, capture_output=True)
        assert parsed.returncode == 0
        doc = json.loads(parsed.stdout)
        assert isinstance(doc, dict) and doc['type'] == 'resource'
        c = doc['configuration']
        assert isinstance(c, dict) and c['name'] == self.name and c['type'] == '1password'
        assert 'service_account' in c and isinstance(c['service_account'], str)
        assert isinstance(doc['userTags'], list)
        return doc

    def remote(self, phase, baseline=None):
        # Redaction and exact remote retention are independent assertions.
        def redaction():
            doc = self.export(False)
            hidden = doc['configuration']['service_account'] == '<REDACTED>'
            absent = self.expected not in json.dumps(doc)
            self.record(phase+'-default-redaction', hidden and absent,
                        native_field_redacted=hidden, expected_credential_absent=absent)
        self.attempt(phase+'-default-invalid', redaction)
        def exact():
            doc = self.export(True)
            held = doc['configuration']['service_account'] == self.expected
            unchanged = baseline is None or doc['configuration'] == baseline['configuration']
            tags = baseline is None or doc['userTags'] == baseline['userTags']
            self.record(phase, held and unchanged and tags, exact_synthetic_credential=held,
                        configuration_unchanged=unchanged, tags_unchanged=tags,
                        explicit_remote_empty=doc['configuration']['service_account'] == '')
            return doc
        return self.attempt(phase+'-invalid', exact)

    def tf(self, phase, expected, *args):
        private_tree(self.run)
        source_file(self.root/'testing/live/run.sh')
        for argument in args:
            if argument.startswith('-out='):
                path = private_path(self.run, self.directory/argument[5:], missing=True)
                if path.exists():
                    raise ValueError('saved plan already exists')
        offset = (self.run/'ledger.jsonl').stat().st_size
        result = subprocess.run(['bash',str(self.root/'testing/live/run.sh'),'tf',str(self.run),CASE,
                                 phase,str(expected),*args], capture_output=True)
        with (self.run/'ledger.jsonl').open() as stream:
            stream.seek(offset)
            rows = [json.loads(line) for line in stream]
        rows = [r for r in rows if r.get('case') == CASE and r.get('phase') == phase and r.get('status') != 'STARTED']
        good = result.returncode == 0 and len(rows) == 1 and rows[0]['exit'] == expected and rows[0]['status'] == 'PASS'
        self.record(phase+'-checked', good, expected_exit_observed=good)
        return rows[0] if good else None

    def execute(self):
        if not self.attempt('unclassified-ownership-invalid', self.ownership):
            return 1
        baseline = self.remote('unclassified-baseline')
        # Never mutate after a failed baseline/security/identity assertion.
        if self.failed or baseline is None:
            return 1
        omit_plan = 'unclassified-omit-'+self.stamp+'.tfplan'
        restore_plan = 'unclassified-restore-'+self.stamp+'.tfplan'
        try:
            private_tree(self.run)
            altered = json.loads(self.original)
            attrs = altered['resource']['adaptive_resource']['test']
            del attrs['service_account']
            attrs['tags'] = attrs.get('tags', [])+['guard=must-not-drop-account']
            private_write(self.run, self.config, json.dumps(altered))
            planned = self.tf('unclassified-omit-plan', 2, 'plan','-input=false','-detailed-exitcode','-out='+omit_plan)
            if planned:
                rejected = self.tf('unclassified-omission-rejected', 1, 'apply','-input=false','-auto-approve',omit_plan)
                diagnostic = False
                if rejected:
                    log = private_path(self.run, Path(rejected['evidence'])).read_text()
                    diagnostic = 'server-held values for service_account' in log
                self.record('unclassified-guard-diagnostic', diagnostic, correct_guard_diagnostic=diagnostic)
                private_write(self.run, self.evidence/'post-error-state.json', private_path(self.run, self.state).read_bytes())
            self.remote('unclassified-secret-retained', baseline)
        except Exception:
            self.record('unclassified-probe-error', False, unreadable_or_invalid=True)
        finally:
            # No assertion failure may short-circuit restoration or the final no-op.
            private_write(self.run, self.config, self.original)
            planned = self.attempt('unclassified-restore-plan-error', lambda: self.tf(
                'unclassified-restore-plan', 0, 'plan','-input=false','-out='+restore_plan))
            if planned:
                self.attempt('unclassified-restore-apply-error', lambda: self.tf(
                    'unclassified-restore-apply', 0, 'apply','-input=false','-auto-approve',restore_plan))
            self.remote('unclassified-secret-restored', baseline)
            self.attempt('unclassified-noop-error', lambda: self.tf(
                'unclassified-restored-noop', 0, 'plan','-input=false','-detailed-exitcode'))
        return int(self.failed)


if __name__ == '__main__':
    try:
        probe = Probe(Path(sys.argv[1]), Path(sys.argv[2]))
    except Exception:
        print('Owned synthetic omission probe blocked: unsafe private paths', file=sys.stderr)
        sys.exit(1)
    try:
        code = probe.execute()
    except Exception:
        try:
            probe.record('unclassified-unexpected-error', False, unreadable_or_invalid=True)
        except Exception:
            print('Owned synthetic omission probe blocked: unsafe private paths', file=sys.stderr)
            sys.exit(1)
        code = 1
    print('Owned synthetic omission probe '+('FAIL' if code else 'PASS')+'; sanitized evidence: '+str(probe.evidence))
    sys.exit(code)
UNCLASSIFIED_PY
