#!/usr/bin/env python3
"""Bounded continuation of an immutable field attempt; --check is local/read-only."""


import concurrent.futures
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import sys
import multiprocessing
import threading

_CALLBACK = None


def _dispatch(case):
    if _CALLBACK is None:
        raise ValueError('no inherited callback')
    return _CALLBACK(case)


def process_cases(callback, cases):
    """Fork an inherited callback/cohort; only case names cross the work queue."""
    global _CALLBACK
    if threading.active_count() != 1:
        raise ValueError('fork requires a single-threaded parent')
    _CALLBACK = callback
    try:
        with concurrent.futures.ProcessPoolExecutor(
                max_workers=4, mp_context=multiprocessing.get_context('fork')) as pool:
            futures = [pool.submit(_dispatch, case) for case in cases]
            return [future.result() for future in futures]
    finally:
        _CALLBACK = None


def load_module(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    if spec is None or spec.loader is None:
        raise ValueError('invalid Python source')
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


def load_fields():
    return load_module('approved_field_updates', Path(__file__).absolute().with_name('field-updates.py'))


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def current_selection(f, run):
    """Read-only equivalent of eligibility/inventory, using unchanged leaf rules."""
    f.private_tree(run)
    for relative in ('testing/live/run.sh', 'testing/live/integration-cases.json',
                     'docs/parity-current-integration-fields.csv'):
        f.source_file(f.ROOT / relative)
    inventory = json.loads((run / 'inventory/integration-cases.json').read_text())
    if inventory != json.loads((f.ROOT / 'testing/live/integration-cases.json').read_text()):
        raise ValueError('run inventory differs from source')
    types = [row['type'] for row in inventory['cases']]
    if (not types or len(types) != len(set(types))
            or any(not re.fullmatch(r'[a-z0-9_-]+', typ) for typ in types)):
        raise ValueError('invalid integration inventory')
    originals = {'integration-' + typ: typ for typ in types}
    latest = {}
    for line in (run / 'ledger.jsonl').read_text().splitlines():
        row = json.loads(line)
        if row['phase'] == 'updated-noop' and row['case'].startswith('integration-'):
            if row['case'] not in originals:
                raise ValueError('unknown eligibility record')
            latest[row['case']] = row['status']
    cases = sorted(c for c, status in latest.items() if status == 'PASS')
    if not cases:
        raise ValueError('no eligible originals')
    prefix = json.loads((run / 'manifest.json').read_text())['prefix']
    rows = []
    for case in cases:
        f.private_path(run, run / 'work' / case / 'terraform.tfstate')
        attrs = f.attrs(json.loads((run / 'work' / case / 'main.tf.json').read_text()))
        if attrs['type'] != originals[case] or attrs['name'] != prefix + '-' + case:
            raise ValueError('fixture identity mismatch')
        for path, value in f.leaves(attrs):
            changed, reason = f.mutate(attrs['type'], path[-1], value, nested=len(path) > 1)
            rows.append(dict(case=case, field='.'.join(map(str, path)),
                             selected=changed is not None, reason=reason))
    return rows


def check(f, run, source_attempt) -> dict:
    run, source_attempt = Path(run), Path(source_attempt)
    f.private_tree(run)
    f.private_path(run, source_attempt)
    if source_attempt.parent != run / 'field-updates/attempts' or not source_attempt.is_dir():
        raise ValueError('source must be an exact immutable attempt directory')
    current = current_selection(f, run)
    source, pins = verified_history(f, run, source_attempt, current)
    reused, pending = coverage_partition(current, source)
    return dict(selected=len(reused | pending), reused=len(reused), pending=len(pending),
                pending_fixtures=len({c for c, _ in pending}), source_attempt=str(source_attempt),
                source_sha256={n: digest(source_attempt / n) for n in ('selection.json', 'results.json')},
                ancestor_sha256=pins)


def coverage_partition(selection, coverage):
    selected = {key(r) for r in selection if r['selected']}
    reused = {key(r) for r in coverage['fields'] if r['status'] == 'PASS'
              and r.get('attempted') is True and r.get('restored') is True}
    return reused, selected - reused


def verified_history(f, run, attempt, selection, visiting=None):
    """Verify immutable local ancestry; derived rows are never raw source results."""
    visiting = set() if visiting is None else visiting
    attempt = Path(attempt)
    f.private_path(run, attempt)
    if (not attempt.is_absolute() or '..' in attempt.parts
            or attempt.parent != run / 'field-updates/attempts' or not attempt.is_dir()
            or attempt in visiting):
        raise ValueError('invalid or cyclic immutable ancestry')
    visiting.add(attempt)
    pins = {}
    def artifact(name):
        path = f.private_path(run, attempt / name)
        if not path.is_file():
            raise ValueError('ancestor artifact must be regular')
        pins[str(path)] = digest(path)
        return path
    def read(name):
        def unique(pairs):
            result = {}
            for k, value in pairs:
                if k in result:
                    raise ValueError('duplicate artifact metadata key')
                result[k] = value
            return result
        return json.loads(artifact(name).read_text(), object_pairs_hook=unique)
    historical, raw = read('selection.json'), read('results.json')
    if (any(type(raw.get(k)) is not int or raw[k] < 0 for k in ('fixtures', 'attempted', 'restored'))
            or any(type(v) is not int or v <= 0 for v in raw['counts'].values())):
        raise ValueError('invalid raw summary integer')
    provenance_path = attempt / 'provenance.json'
    if not provenance_path.exists():
        if any((attempt / n).exists() for n in ('combined-coverage.json', 'final-receipt.json',
                'source-selection.json', 'source-results.json', 'field-continuation.py', 'gate-helper.py')):
            raise ValueError('missing continuation provenance')
        partition(selection, historical, raw)
        # Legacy root has no continuation receipt/provenance. Its exact raw bytes
        # are pinned by its child's archived source and provenance, not invented.
        report = combine(selection, raw, [], attempt, attempt)
        observed = {key(r): r for r in raw['fields'] if 'field' in r}
        for row in report['fields']:
            original = observed.get(key(row))
            if original is not None:
                row.update(status=original['status'], attempted=original.get('attempted') is True,
                           restored=original.get('restored') is True)
        for name in ('deployment-preflight.json', 'guarded-driver.py'):
            if (attempt / name).exists():
                artifact(name)
        visiting.remove(attempt)
        return report, pins
    provenance = read('provenance.json')
    parent = Path(provenance['source_attempt'])
    inherited, parent_pins = verified_history(f, run, parent, selection, visiting)
    pins.update(parent_pins)
    reused, pending = coverage_partition(selection, inherited)
    source_hashes = {n: parent_pins[str(parent / n)] for n in ('selection.json', 'results.json')}
    expected = dict(selected=len(reused | pending), reused=len(reused), pending=len(pending),
                    pending_fixtures=len({c for c, _ in pending}), source_attempt=str(parent),
                    source_sha256=source_hashes)
    if any(json.dumps(provenance.get(k), sort_keys=True) != json.dumps(v, sort_keys=True)
           for k, v in expected.items()):
        raise ValueError('ancestor provenance metadata mismatch')
    for name, value in source_hashes.items():
        if digest(artifact('source-' + name)) != value:
            raise ValueError('archived source mismatch')
    if (provenance.get('workers') != 4 or provenance.get('start_method') != 'fork'
            or digest(artifact('field-continuation.py')) != provenance.get('driver_sha256')
            or digest(artifact('gate-helper.py')) != provenance.get('gate_helper_sha256')):
        raise ValueError('ancestor source pin mismatch')
    if ('ancestor_sha256' not in provenance and
            (provenance.get('driver_sha256') !=
             '6d94111c72d58c7d6994982b73e0a11ec757c7a959a6f763d8fa0e5702809a29'
             or (parent / 'provenance.json').exists())):
        raise ValueError('missing recursive provenance')
    if 'ancestor_sha256' in provenance and provenance['ancestor_sha256'] != parent_pins:
        raise ValueError('ancestor artifact pins mismatch')
    if json.dumps(historical, sort_keys=True) != json.dumps(selection, sort_keys=True):
        raise ValueError('selection changed')
    # Validate the continuation's raw summary against precisely its dispatched
    # pending fixtures, not the complete root fixture count.
    validate_continuation_raw(selection, raw, pending)
    receipt = read('final-receipt.json')
    boolean_keys = ('dispatch_stopped', 'auth_gate_failed', 'serving_gate_failed',
                    'unknown_gate_failed', 'blocked', 'source_unchanged')
    if (any(type(receipt.get(k)) is not bool for k in boolean_keys)
            or type(receipt.get('checks')) is not int or receipt['checks'] < 0
            or type(receipt.get('exit')) is not int or receipt['exit'] not in (0, 1)
            or receipt.get('immutable_result') != str(attempt / 'results.json')
            or receipt.get('combined_coverage') != str(attempt / 'combined-coverage.json')
            or receipt['source_unchanged'] is not True
            or receipt['dispatch_stopped'] != any(receipt[k] for k in boolean_keys[1:4])
            or (receipt['dispatch_stopped'] and not receipt['blocked'])):
        raise ValueError('invalid final receipt')
    stored = read('combined-coverage.json')
    # Existing first-generation coverage was derived from a flat raw source.
    parent_raw = json.loads((parent / 'results.json').read_text())
    basis = inherited if 'ancestor_sha256' in provenance else parent_raw
    report = combine(selection, basis, raw['fields'], parent, attempt)
    report['accepted'] &= (not receipt['blocked'] and receipt['source_unchanged']
                           and not receipt['dispatch_stopped'])
    if (json.dumps(stored, sort_keys=True) != json.dumps(report, sort_keys=True)
            or receipt['exit'] != (0 if report['accepted'] else 1)):
        raise ValueError('combined coverage or receipt mismatch')
    visiting.remove(attempt)
    return report, pins


def validate_continuation_raw(selection, raw, pending):
    from collections import Counter
    now = {key(r): r for r in selection}
    if any(type(raw.get(k)) is not int or raw[k] < 0 for k in ('fixtures', 'attempted', 'restored')):
        raise ValueError('invalid summary integer')
    seen = set()
    for row in raw['fields']:
        k = (row['case'], row.get('field'))
        if k in seen or (k[1] is not None and k not in pending):
            raise ValueError('duplicate or undispatched result')
        if k[1] is None:
            if row['status'] != 'FAIL' or k[0] not in {c for c, _ in pending}:
                raise ValueError('invalid case failure')
        elif (row['status'] not in {'PASS', 'FAIL'} or not now[k]['selected']
              or (row['status'] == 'PASS' and
                  (row.get('attempted') is not True or row.get('restored') is not True))):
            raise ValueError('invalid field result')
        if (any(k in row and type(row[k]) is not bool for k in ('attempted', 'restored'))
                or (row.get('restored') is True and row.get('attempted') is not True)):
            raise ValueError('contradictory attempt/restoration evidence')
        seen.add(k)
    if (raw['fixtures'] != len({c for c, _ in pending})
            or any(type(v) is not int or v <= 0 for v in raw['counts'].values())
            or raw['counts'] != dict(Counter(r['status'] for r in raw['fields']))
            or raw['attempted'] != sum(r.get('attempted') is True for r in raw['fields'])
            or raw['restored'] != sum(r.get('restored') is True for r in raw['fields'])):
        raise ValueError('continuation summary metadata mismatch')


class GateFailure(RuntimeError):
    def __init__(self, kind='unknown'):
        self.kind = kind if kind in {'auth', 'serving'} else 'unknown'
        super().__init__('deployment gate blocked')


class DeploymentGate:
    """Shared latched stop and safe dimensional receipt, including legacy helper."""
    def __init__(self, helper, ctx):
        self.helper = helper
        self.stop, self.checks, self.failures = ctx.Event(), ctx.Value('i', 0), ctx.Value('i', 0)
        self.dimension = 'unknown'
        if hasattr(helper, 'command'):
            command = helper.command
            def tracked(args):
                self.dimension = 'auth' if args[0] == 'adaptive' else 'serving' if args[0] in {'lsof', 'ps'} else 'unknown'
                try:
                    return command(args)
                except Exception:
                    raise GateFailure(self.dimension) from None
            helper.command = tracked

    def __call__(self):
        if self.stop.is_set():
            raise GateFailure()
        self.dimension = 'unknown'
        try:
            result = self.helper.gate()
            if result is not None:
                if (not isinstance(result, dict) or set(result) != {'auth_ok', 'serving_ok'}
                        or any(type(v) is not bool for v in result.values())):
                    raise GateFailure()
                if not result['serving_ok']:
                    raise GateFailure('serving')
                if not result['auth_ok']:
                    raise GateFailure('auth')
            with self.checks.get_lock():
                self.checks.value += 1
        except Exception as exc:
            kind = exc.kind if isinstance(exc, GateFailure) else self.dimension
            # These are the legacy helper's fixed messages, never persisted.
            if str(exc) in {'Serving listener changed', 'Serving executable changed', 'Reviewed binary changed'}:
                kind = 'serving'
            elif str(exc) in {'CLI identity override present', 'Saved local identity invalid'}:
                kind = 'auth'
            with self.failures.get_lock():
                self.failures.value |= {'auth': 1, 'serving': 2}.get(kind, 4)
            self.stop.set()
            raise GateFailure(kind) from None

    def receipt(self):
        return dict(checks=self.checks.value, dispatch_stopped=self.stop.is_set(),
                    auth_gate_failed=bool(self.failures.value & 1),
                    serving_gate_failed=bool(self.failures.value & 2),
                    unknown_gate_failed=bool(self.failures.value & 4))


def make_cohort(f, run, pending, gate, ctx):
    class ContinuationCohort(f.Cohort):
        def case(self, case):
            gate()
            original_leaves = f.leaves
            def filtered(attrs):
                for path, value in original_leaves(attrs):
                    if (case, '.'.join(map(str, path))) in pending:
                        gate()
                        yield path, value
            f.leaves = filtered
            try:
                return super().case(case)
            finally:
                f.leaves = original_leaves

        def tf(self, *args, **kwargs):
            gate()
            try:
                return super().tf(*args, **kwargs)
            finally:
                gate()

        def show(self, *args, **kwargs):
            gate()
            try:
                return super().show(*args, **kwargs)
            finally:
                gate()

        def readback(self, *args, **kwargs):
            gate()
            try:
                return super().readback(*args, **kwargs)
            finally:
                gate()

    cohort = ContinuationCohort(run)
    cohort.lock = ctx.Lock()  # inherited by all four workers for ledger appends
    return cohort


def preflight(f, c, helper, guard):
    """Retain the approved wrapper's manifest, binary, baseline and ID checks."""
    run = c.run
    review_path = f.private_path(run, run / 'remediation-evidence/harness-replay-review-v2/manifest.json')
    review = json.loads(review_path.read_text())
    expected = {'testing/live/field-updates.py', 'testing/live/test_field_updates.py',
                'testing/live/unclassified-secrets.sh', 'testing/live/test_unclassified_secrets.py'}
    if set(review['files']) != expected or review['root'] != str(f.ROOT):
        raise ValueError('approved manifest membership changed')
    if any(digest(f.source_file(f.ROOT / name)) != value for name, value in review['files'].items()):
        raise ValueError('approved harness changed')
    manifest = json.loads((run / 'manifest.json').read_text())
    if manifest['url'] != 'http://localhost:8080' or digest(f.private_path(run, run / 'bin/terraform-provider-adaptive')) != manifest['binary_sha256']:
        raise ValueError('provider or target pin mismatch')
    backend = json.loads(f.private_path(run, run / 'inventory/serving-backend-source.json').read_text())
    if not backend['files']:
        raise ValueError('empty backend source pin')
    for name, value in backend['files'].items():
        relative = Path(name)
        if relative.is_absolute() or '..' in relative.parts or digest(f.source_file(Path(backend['root']) / relative)) != value:
            raise ValueError('backend source changed')
    guard()
    baseline, _ = helper.export(manifest['prefix'] + '-import-1password', '1password')
    if baseline['configuration']['service_account'] != '<REDACTED>':
        raise ValueError('synthetic redaction baseline failed')
    guard()
    ids = helper.resource_ids(manifest['prefix'])
    for case in json.loads((run / 'inventory/integration-cases.json').read_text())['cases']:
        fixture = 'integration-' + case['type']
        attrs = f.attrs(json.loads((run / 'work' / fixture / 'main.tf.json').read_text()))
        state = json.loads((run / 'work' / fixture / 'terraform.tfstate').read_text())
        if state['resources'][0]['instances'][0]['attributes']['id'] != ids[attrs['name']]:
            raise ValueError('owned fixture remote ID mismatch')
    guard()


def execute(f, run, source_attempt, helper_path):
    """Local artifacts are created before any helper import or deployment check."""
    run, source_attempt, helper_path = Path(run), Path(source_attempt), Path(helper_path)
    if threading.active_count() != 1:
        raise ValueError('fork requires a single-threaded parent')
    if not helper_path.is_absolute() or '..' in helper_path.parts:
        raise ValueError('gate helper must be an exact absolute regular path')
    f.source_file(helper_path)
    old_umask = os.umask(0o077)
    try:
        with f.ExclusiveLock(run, run / '.field-updates.lock'):
            info = check(f, run, source_attempt)
            if list((run / 'work').glob('*/.field-updates.lock')):
                raise ValueError('interrupted or overlapping case writer')
            selection = current_selection(f, run)
            source, pins = verified_history(f, run, source_attempt, selection)
            if pins != info['ancestor_sha256']:
                raise ValueError('ancestry changed before dispatch')
            _, pending = coverage_partition(selection, source)
            ctx = multiprocessing.get_context('fork')
            gate = None
            def guard():
                if gate is None:
                    raise GateFailure()
                gate()
            c = make_cohort(f, run, pending, guard, ctx)
            out = c.out
            def save(name, value):
                data = value if isinstance(value, bytes) else json.dumps(value, indent=2) + '\n'
                with f.private_open(run, out / name, 'xb' if isinstance(data, bytes) else 'x') as stream:
                    stream.write(data)
            raw, blocked = [], True
            latest = run / 'field-updates/results.json'
            try:
                if latest.exists():
                    save('previous-latest-results.json', latest.read_bytes())
                save('selection.json', selection)
                for name in ('selection.json', 'results.json'):
                    save('source-' + name, (source_attempt / name).read_bytes())
                    if (digest(source_attempt / name) != info['source_sha256'][name]
                            or digest(out / ('source-' + name)) != info['source_sha256'][name]):
                        raise ValueError('source changed while archiving')
                driver_path = f.source_file(Path(__file__).absolute())
                save('field-continuation.py', driver_path.read_bytes())
                save('gate-helper.py', helper_path.read_bytes())
                provenance = dict(info, workers=4, start_method='fork',
                                  driver_sha256=digest(out / 'field-continuation.py'),
                                  gate_helper_path=str(helper_path),
                                  gate_helper_sha256=digest(out / 'gate-helper.py'))
                save('provenance.json', provenance)  # pin BEFORE executing imported source
                if digest(helper_path) != provenance['gate_helper_sha256']:
                    raise ValueError('gate helper changed during archival')
                if any(digest(f.private_path(run, Path(n))) != v
                       for n, v in info['ancestor_sha256'].items()):
                    raise ValueError('ancestor changed during archival')
                helper = load_module('archived_deployment_gate', out / 'gate-helper.py')
                gate = DeploymentGate(helper, ctx)
                guard()
                preflight(f, c, helper, guard)
                cases = sorted({case for case, _ in pending})
                def dispatch(case):
                    try:
                        c.case(case)
                        return dict(case=case, completed=True)
                    except Exception:
                        return dict(case=case, completed=False)
                terminals = process_cases(dispatch, cases)
                for terminal in terminals:
                    case = terminal['case']
                    result_path = f.private_path(run, out / (case + '-results.json'), missing=True)
                    if result_path.exists():
                        raw.extend(json.loads(result_path.read_text()))
                    if not terminal['completed']:
                        raw.append(dict(case=case, status='FAIL', error='case blocked; private details suppressed'))
                guard()
                blocked = False
            except Exception:
                blocked = True  # never persist exception text / CLI output
            finally:
                from collections import Counter
                try:
                    source_unchanged = all(digest(f.private_path(run, Path(n))) == v
                                           for n, v in info['ancestor_sha256'].items())
                except (OSError, ValueError):
                    source_unchanged = False
                summary = dict(fixtures=len({case for case, _ in pending}),
                               attempted=sum(r.get('attempted') is True for r in raw),
                               restored=sum(r.get('restored') is True for r in raw),
                               counts=dict(Counter(r['status'] for r in raw)), fields=raw)
                save('results.json', summary)
                coverage = combine(selection, source, raw, source_attempt, out)
                coverage['accepted'] &= not blocked and source_unchanged and gate is not None and not gate.stop.is_set()
                save('combined-coverage.json', coverage)
                receipt = gate.receipt() if gate else dict(checks=0, dispatch_stopped=True,
                           auth_gate_failed=False, serving_gate_failed=False, unknown_gate_failed=True)
                code = 0 if coverage['accepted'] else 1
                save('final-receipt.json', dict(receipt, exit=code, blocked=blocked,
                     source_unchanged=source_unchanged, immutable_result=str(out / 'results.json'),
                     combined_coverage=str(out / 'combined-coverage.json')))
                f.private_write(run, latest, json.dumps(dict(summary, immutable_result=str(out / 'results.json'),
                                combined_coverage=str(out / 'combined-coverage.json'), mutable_latest=True), indent=2))
            return code, out
    finally:
        os.umask(old_umask)


def key(row):
    return row['case'], row['field']


def partition(current, historical, result):
    from collections import Counter
    def indexed(rows):
        out = {}
        for row in rows:
            k = key(row)
            if k in out or type(row['selected']) is not bool:
                raise ValueError('duplicate or invalid selection')
            out[k] = row
        return out
    now, old = indexed(current), indexed(historical)
    if not now or now != old:
        raise ValueError('selection changed')
    rows = result['fields']
    seen = set()
    for row in rows:
        k = (row['case'], row.get('field'))
        if k in seen or (k[1] is not None and k not in now) or row['case'] not in {c for c, _ in now}:
            raise ValueError('duplicate or unknown result')
        if k[1] is None and row['status'] != 'FAIL':
            raise ValueError('invalid case result')
        if row['status'] not in {'PASS', 'FAIL', 'SKIP'}:
            raise ValueError('invalid source status')
        if row['status'] == 'PASS' and (k[1] is None or not now[k]['selected']
                or row.get('attempted') is not True or row.get('restored') is not True):
            raise ValueError('invalid source success')
        seen.add(k)
    if any((case, None) in seen and field is not None for case, field in seen):
        raise ValueError('ambiguous case and field results')
    if (result['counts'] != dict(Counter(r['status'] for r in rows))
            or result['attempted'] != sum(r.get('attempted') is True for r in rows)
            or result['restored'] != sum(r.get('restored') is True for r in rows)
            or result['fixtures'] != len({c for c, _ in now})):
        raise ValueError('source summary metadata mismatch')
    selected = {key(row) for row in current if row['selected']}
    reused = {key(row) for row in result['fields']
              if row.get('field') and row['status'] == 'PASS' and row.get('restored') is True}
    return reused, selected - reused


def combine(selection, source, raw, source_attempt, continuation) -> dict:
    """Derived coverage only: absent leaves are BLOCKED, never invented PASS."""
    derived = 'continuation' in source
    reused, pending = (coverage_partition(selection, source) if derived
                       else partition(selection, selection, source))
    grouped = {}
    invalid = False
    for row in raw:
        if 'field' not in row:
            invalid = True
            continue
        k = key(row)
        if k not in pending or k in grouped:
            invalid = True
        grouped.setdefault(k, []).append(row)
    fields = []
    for k in sorted(reused | pending):
        if k in reused:
            row = next(r for r in source['fields'] if r.get('field') and key(r) == k)
            attempt = row['attempt'] if derived else source_attempt
        else:
            candidates = grouped.get(k, [])
            if not candidates and derived:
                row = next(r for r in source['fields'] if key(r) == k)
                attempt = row['attempt']
            else:
                row = candidates[0] if len(candidates) == 1 else dict(status='BLOCKED', restored=False)
                attempt = continuation
        # Deliberate names/status/booleans allowlist; never copy error strings or values.
        fields.append(dict(case=k[0], field=k[1], status=row['status'],
                           restored=row.get('restored') is True,
                           attempted=row.get('attempted') is True, attempt=str(attempt)))
    accepted = (not invalid and len(fields) == len(reused | pending)
                and all(r['status'] == 'PASS' and r['restored'] and r['attempted'] for r in fields))
    return dict(accepted=accepted, reused=len(reused), pending=len(pending),
                source_attempt=str(source_attempt), continuation=str(continuation), fields=fields)


def main(argv=None):
    import argparse
    sys.dont_write_bytecode = True  # --check must not even create import caches
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('run', type=Path)
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument('--check', action='store_true')
    mode.add_argument('--execute', action='store_true')
    parser.add_argument('--source-attempt', type=Path, required=True)
    parser.add_argument('--gate-helper', type=Path)
    args = parser.parse_args(argv)
    if args.execute and args.gate_helper is None:
        parser.error('--execute requires --gate-helper')
    if args.check and args.gate_helper is not None:
        parser.error('--gate-helper is only allowed with --execute')
    f = load_fields()
    if args.check:
        print(json.dumps(check(f, args.run, args.source_attempt), sort_keys=True))
        return 0
    code, out = execute(f, args.run, args.source_attempt, args.gate_helper)
    print(json.dumps(dict(exit=code, attempt=str(out))))
    return code


if __name__ == '__main__':
    try:
        raise SystemExit(main())
    except Exception:
        print('Field continuation blocked; unsafe paths, metadata, locks or prerequisites', file=sys.stderr)
        raise SystemExit(1)
