"""Offline qualification; never invokes Adaptive or Terraform."""
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest

HERE = Path(__file__).parent

def driver():
    path = HERE / 'field_continuation.py'
    if not path.exists():
        raise AssertionError('continuation implementation is missing')
    spec = importlib.util.spec_from_file_location('field_continuation', path)
    import sys
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


def selection():
    return [dict(case='integration-a', field=x, selected=True, reason='synthetic') for x in ('one', 'two')]


def source() -> dict:
    return dict(fixtures=1, attempted=1, restored=1, counts={'PASS': 1},
                fields=[dict(case='integration-a', field='one', status='PASS', attempted=True, restored=True)])


class ContinuationTests(unittest.TestCase):
    def fixture(self):
        self.assertTrue(os.environ.get('TMPDIR'), 'explicit private scratch TMPDIR required')
        temporary = tempfile.TemporaryDirectory(dir=os.environ['TMPDIR'])
        self.addCleanup(temporary.cleanup)
        base = Path(temporary.name).resolve()
        run, repo = base / 'run', base / 'repo'
        for p in (run / 'inventory', run / 'evidence', run / 'work/integration-a',
                  run / 'field-updates/attempts/source', repo / 'testing/live', repo / 'docs'):
            p.mkdir(parents=True, exist_ok=True, mode=0o700)
        inventory = dict(cases=[dict(type='a')])
        for p in (run / 'inventory/integration-cases.json', repo / 'testing/live/integration-cases.json'):
            p.write_text(json.dumps(inventory))
        (repo / 'testing/live/run.sh').write_text('exit 99\n')
        (repo / 'docs/parity-current-integration-fields.csv').write_text('Integration,Reverse,Key\n')
        (run / 'manifest.json').write_text(json.dumps(dict(prefix='unit-owned')))
        schema = {'provider_schemas': {'registry.terraform.io/adaptive-scale/adaptive': {
            'resource_schemas': {'adaptive_resource': {'block': {'attributes': {}}}}}}}
        (run / 'inventory/schema.json').write_text(json.dumps(schema))
        (run / 'ledger.jsonl').write_text(json.dumps(dict(case='integration-a', phase='updated-noop', status='PASS')) + '\n')
        config = dict(resource=dict(adaptive_resource=dict(test=dict(name='unit-owned-integration-a', type='a', one='synthetic', two='synthetic'))))
        (run / 'work/integration-a/main.tf.json').write_text(json.dumps(config))
        (run / 'work/integration-a/terraform.tfstate').write_text('{}')
        d = driver()
        f = d.load_fields()
        f.ROOT = repo
        attempt = run / 'field-updates/attempts/source'
        rows = []
        for path, value in f.leaves(f.attrs(config)):
            changed, reason = f.mutate('a', path[-1], value)
            rows.append(dict(case='integration-a', field='.'.join(path), selected=changed is not None, reason=reason))
        (attempt / 'selection.json').write_text(json.dumps(rows))
        (attempt / 'results.json').write_text(json.dumps(source()))
        for path in base.rglob('*'):
            path.chmod(0o700 if path.is_dir() else 0o600)
        return d, f, run, attempt

    def child(self, d, run, parent, rows, name='child'):
        """Offline immutable attempt fixture, never used as campaign evidence."""
        from collections import Counter
        out = parent.parent / name
        out.mkdir(mode=0o700)
        selected = json.loads((parent / 'selection.json').read_text())
        info = d.check(self._fields, run, parent)
        raw = dict(fixtures=info['pending_fixtures'],
                   attempted=sum(r.get('attempted') is True for r in rows),
                   restored=sum(r.get('restored') is True for r in rows),
                   counts=dict(Counter(r['status'] for r in rows)), fields=rows)
        values = {'selection.json': selected, 'results.json': raw,
                  'provenance.json': dict(info, workers=4, start_method='fork',
                      driver_sha256=d.digest(HERE / 'field_continuation.py'),
                      gate_helper_path=str(out / 'gate-helper.py'),
                      gate_helper_sha256=__import__('hashlib').sha256(b'# offline\n').hexdigest())}
        for n in ('selection.json', 'results.json'):
            (out / ('source-' + n)).write_bytes((parent / n).read_bytes())
        (out / 'field-continuation.py').write_bytes((HERE / 'field_continuation.py').read_bytes())
        (out / 'gate-helper.py').write_bytes(b'# offline\n')
        inherited, _ = d.verified_history(self._fields, run, parent, selected)
        coverage = d.combine(selected, inherited, rows, parent, out)
        values['combined-coverage.json'] = coverage
        values['final-receipt.json'] = dict(checks=1, dispatch_stopped=not coverage['accepted'],
            auth_gate_failed=not coverage['accepted'], serving_gate_failed=False,
            unknown_gate_failed=False, exit=0 if coverage['accepted'] else 1,
            blocked=not coverage['accepted'], source_unchanged=True,
            immutable_result=str(out / 'results.json'), combined_coverage=str(out / 'combined-coverage.json'))
        for n, value in values.items():
            (out / n).write_text(json.dumps(value))
        for p in out.iterdir():
            p.chmod(0o600)
        return out

    def test_two_hop_reuses_actual_positive_rows_with_origin_links(self):
        from unittest import mock
        d, f, run, root = self.fixture()
        self._fields = f
        child = self.child(d, run, root, [dict(case='integration-a', field='two',
            status='PASS', attempted=True, restored=True)])
        before = {str(p): (p.stat().st_mtime_ns, p.read_bytes()) for p in run.rglob('*') if p.is_file()}
        with mock.patch('subprocess.run', side_effect=AssertionError('no remote')):
            info = d.check(f, run, child)
        self.assertEqual((info['reused'], info['pending']), (2, 0))
        self.assertIn(str(root / 'results.json'), info['ancestor_sha256'])
        self.assertIn(str(child / 'final-receipt.json'), info['ancestor_sha256'])
        self.assertEqual(before, {str(p): (p.stat().st_mtime_ns, p.read_bytes()) for p in run.rglob('*') if p.is_file()})
        verified, _ = d.verified_history(f, run, child, d.current_selection(f, run))
        report = d.combine(d.current_selection(f, run), verified, [], child, child.parent / 'next')
        self.assertEqual([r['attempt'] for r in report['fields']], [str(root), str(child)])
        self.assertTrue(report['accepted'])
        grandchild = self.child(d, run, child, [], name='grandchild')
        self.assertEqual(d.check(f, run, grandchild)['reused'], 2)
        inherited, _ = d.verified_history(f, run, grandchild, d.current_selection(f, run))
        self.assertEqual([r['attempt'] for r in inherited['fields']], [str(root), str(child)])

    def test_partial_failure_keeps_positive_reuse_and_truthful_failure_origin(self):
        d, f, run, root = self.fixture()
        self._fields = f
        child = self.child(d, run, root, [
            dict(case='integration-a', field='two', status='FAIL', attempted=True, restored=False),
            dict(case='integration-a', status='FAIL')])
        info = d.check(f, run, child)
        self.assertEqual((info['reused'], info['pending']), (1, 1))
        helper = f.ROOT / 'gate.py'
        helper.write_text("def gate():\n    return {'auth_ok': False, 'serving_ok': True}\n")
        helper.chmod(0o600)
        before = {str(p): p.read_bytes() for p in child.iterdir()}
        code, out = d.execute(f, run, child, helper)
        self.assertEqual(code, 1)
        coverage = json.loads((out / 'combined-coverage.json').read_text())
        self.assertEqual((coverage['reused'], coverage['pending']), (1, 1))
        self.assertEqual([(r['status'], r['attempt']) for r in coverage['fields']],
                         [('PASS', str(root)), ('FAIL', str(child))])
        self.assertEqual(json.loads((out / 'results.json').read_text())['fields'], [])
        self.assertEqual(before, {str(p): p.read_bytes() for p in child.iterdir()})
        self.assertEqual(d.check(f, run, out)['reused'], 1)

    def test_chained_metadata_tampering_fails_closed(self):
        import copy
        d, f, run, root = self.fixture()
        self._fields = f
        child = self.child(d, run, root, [dict(case='integration-a', field='two',
            status='PASS', attempted=True, restored=True)])
        cases = [
            ('final-receipt.json', lambda x: x.update(immutable_result='/external/results.json')),
            ('final-receipt.json', lambda x: x.update(source_unchanged=False)),
            ('final-receipt.json', lambda x: x.update(exit=1)),
            ('final-receipt.json', lambda x: x.update(auth_gate_failed=True)),
            ('final-receipt.json', lambda x: x.update(checks=True)),
            ('final-receipt.json', lambda x: x.update(blocked='false')),
            ('provenance.json', lambda x: x.update(source_attempt=str(child))),
            ('provenance.json', lambda x: x.update(source_attempt='/external/attempt')),
            ('provenance.json', lambda x: x['ancestor_sha256'].clear()),
            ('provenance.json', lambda x: x.pop('ancestor_sha256')),
            ('provenance.json', lambda x: x.update(workers=8)),
            ('provenance.json', lambda x: x.update(reused=True)),
            ('results.json', lambda x: x.update(counts={'PASS': True})),
            ('selection.json', lambda x: next(r for r in x if r['selected']).update(selected=1)),
            ('combined-coverage.json', lambda x: x['fields'].pop()),
            ('combined-coverage.json', lambda x: x['fields'].append(copy.deepcopy(x['fields'][0]))),
            ('combined-coverage.json', lambda x: x['fields'][0].update(attempt=str(child))),
            ('results.json', lambda x: x['fields'][0].update(restored=1)),
            ('results.json', lambda x: x.update(fixtures=True)),
        ]
        for name, mutate in cases:
            p = child / name
            original = p.read_bytes()
            value = json.loads(original)
            mutate(value)
            p.write_text(json.dumps(value))
            try:
                with self.subTest(artifact=name, mutation=repr(value)), self.assertRaises((ValueError, OSError)):
                    d.check(f, run, child)
            finally:
                p.write_bytes(original)
        for name in ('provenance.json', 'final-receipt.json', 'source-results.json',
                     'field-continuation.py', 'gate-helper.py', 'selection.json', 'results.json'):
            descendant = self.child(d, run, child, [], name='descendant-' + name)
            self.assertEqual(d.check(f, run, descendant)['reused'], 2)
            p = child / name
            original = p.read_bytes()
            p.write_bytes(original + b' ')
            try:
                with self.subTest(artifact=name), self.assertRaises(ValueError):
                    d.check(f, run, descendant)
            finally:
                p.write_bytes(original)

    def test_legacy_root_failures_are_not_converted_to_blocked_coverage(self):
        d, f, run, root = self.fixture()
        raw = source()
        raw['fields'].append(dict(case='integration-a', field='two', status='FAIL',
                                  attempted=True, restored=True))
        raw.update(attempted=2, restored=2, counts={'PASS': 1, 'FAIL': 1})
        (root / 'results.json').write_text(json.dumps(raw))
        helper = f.ROOT / 'gate.py'
        helper.write_text("def gate():\n    return {'auth_ok': False, 'serving_ok': True}\n")
        helper.chmod(0o600)
        code, out = d.execute(f, run, root, helper)
        self.assertEqual(code, 1)
        rows = json.loads((out / 'combined-coverage.json').read_text())['fields']
        self.assertEqual((rows[1]['status'], rows[1]['attempt'], rows[1]['restored']),
                         ('FAIL', str(root), True))
        self.assertEqual(d.check(f, run, out)['pending'], 1)

    def test_continuation_cannot_downgrade_to_unverified_legacy_root(self):
        d, f, run, root = self.fixture()
        self._fields = f
        child = self.child(d, run, root, [dict(case='integration-a', field='two',
            status='PASS', attempted=True, restored=True)])
        path = child / 'provenance.json'
        saved = path.read_bytes()
        path.unlink()
        with self.assertRaises(ValueError):
            d.check(f, run, child)
        path.write_bytes(saved)
        path.chmod(0o600)

    def test_chained_raw_contradictions_and_ambiguous_json_are_rejected(self):
        d, f, run, root = self.fixture()
        self._fields = f
        child = self.child(d, run, root, [dict(case='integration-a', field='two',
            status='FAIL', attempted=True, restored=True)])
        path = child / 'results.json'
        original = path.read_bytes()
        raw = json.loads(original)
        raw['fields'][0]['attempted'] = False
        raw['attempted'] = 0
        path.write_text(json.dumps(raw))
        combined_path = child / 'combined-coverage.json'
        combined_bytes = combined_path.read_bytes()
        combined = json.loads(combined_bytes)
        combined['fields'][1]['attempted'] = False
        combined_path.write_text(json.dumps(combined))
        with self.assertRaises(ValueError):
            d.check(f, run, child)
        path.write_bytes(original)
        combined_path.write_bytes(combined_bytes)
        path = child / 'provenance.json'
        original = path.read_bytes()
        path.write_text(original.decode().replace('"workers": 4', '"workers": 99, "workers": 4'))
        with self.assertRaises(ValueError):
            d.check(f, run, child)

    def test_ancestor_changes_during_archival_block_before_helper_import(self):
        from contextlib import contextmanager
        from unittest import mock
        d, f, run, root = self.fixture()
        self._fields = f
        child = self.child(d, run, root, [dict(case='integration-a', field='two',
            status='FAIL', attempted=True, restored=True)])
        helper = f.ROOT / 'gate.py'
        helper.write_text("raise AssertionError('no helper import')\n")
        helper.chmod(0o600)
        real_open = f.private_open
        @contextmanager
        def changed_open(run_arg, path, mode):
            with real_open(run_arg, path, mode) as stream:
                yield stream
            if Path(path).name == 'gate-helper.py':
                with (root / 'results.json').open('a') as stream:
                    stream.write(' ')
        with mock.patch.object(f, 'private_open', changed_open), mock.patch.object(d, 'load_module') as load:
            code, out = d.execute(f, run, child, helper)
        self.assertEqual(code, 1)
        load.assert_not_called()
        self.assertFalse(json.loads((out / 'final-receipt.json').read_text())['source_unchanged'])

    def test_recursive_paths_cycles_and_missing_artifacts_fail_closed(self):
        d, f, run, root = self.fixture()
        self._fields = f
        child = self.child(d, run, root, [dict(case='integration-a', field='two',
            status='PASS', attempted=True, restored=True)])
        grandchild = self.child(d, run, child, [], name='grandchild')
        p = child / 'provenance.json'
        original = p.read_bytes()
        value = json.loads(original)
        value['source_attempt'] = str(grandchild)
        p.write_text(json.dumps(value))
        with self.assertRaisesRegex(ValueError, 'cyclic'):
            d.check(f, run, grandchild)
        p.write_bytes(original)
        for attempt, name in ((root, 'results.json'), (child, 'source-selection.json'),
                              (child, 'final-receipt.json'), (child, 'combined-coverage.json')):
            p = attempt / name
            original = p.read_bytes()
            p.unlink()
            try:
                with self.subTest(missing=name), self.assertRaises((ValueError, OSError)):
                    d.check(f, run, grandchild)
                p.symlink_to(root / 'selection.json')
                with self.subTest(symlink=name), self.assertRaises(ValueError):
                    d.check(f, run, grandchild)
            finally:
                p.unlink(missing_ok=True)
                p.write_bytes(original)
                p.chmod(0o600)

    def test_legacy_root_summary_types_are_validated_before_reuse(self):
        d, f, run, root = self.fixture()
        for change in ('fixtures', 'counts', 'restored'):
            raw = source()
            if change == 'counts':
                raw['counts'] = {'PASS': True}
            else:
                raw[change] = True
            (root / 'results.json').write_text(json.dumps(raw))
            with self.subTest(change=change), self.assertRaises(ValueError):
                d.check(f, run, root)

    def test_readonly_check_validates_current_selection_without_writes_or_commands(self):
        from unittest import mock
        d, f, run, attempt = self.fixture()
        def snapshot():
            return {str(p): (p.stat().st_mtime_ns, p.read_bytes()) for p in run.rglob('*') if p.is_file()}
        before = snapshot()
        with mock.patch('subprocess.run', side_effect=AssertionError('remote call forbidden')):
            info = d.check(f, run, attempt)
        self.assertEqual((info['reused'], info['pending'], info['pending_fixtures']), (1, 1, 1))
        self.assertEqual(before, snapshot())
        (run / 'work/integration-a/main.tf.json').write_text(json.dumps(dict(resource=dict(adaptive_resource=dict(test=dict(name='unit-owned-integration-a', type='a', one='synthetic'))))))
        with self.assertRaises(ValueError):
            d.check(f, run, attempt)

    def test_readonly_rejects_unsafe_or_nonimmutable_source(self):
        d, f, run, attempt = self.fixture()
        with self.assertRaises(ValueError):
            d.check(f, run, run / 'field-updates')
        raw = attempt / 'results.json'
        raw.chmod(0o644)
        with self.assertRaises(ValueError):
            d.check(f, run, attempt)
        raw.chmod(0o600)
        link = run / 'field-updates/attempts/link'
        link.symlink_to(attempt)
        with self.assertRaises(ValueError):
            d.check(f, run, link)

    def test_selects_only_not_pass_and_restored(self):
        d = driver()
        reused, pending = d.partition(selection(), selection(), source())
        self.assertEqual(reused, {('integration-a', 'one')})
        self.assertEqual(pending, {('integration-a', 'two')})

    def test_partition_rejects_duplicates_changed_selection_and_bad_metadata(self):
        d = driver()
        import copy
        bad_results = []
        for change in ('duplicate', 'count', 'restored', 'unknown', 'case-duplicate'):
            r = copy.deepcopy(source())
            if change == 'duplicate':
                r['fields'] *= 2
            elif change == 'count':
                r['attempted'] = 2
            elif change == 'restored':
                r['restored'] = 0
            elif change == 'unknown':
                r['fields'][0]['field'] = 'unknown'
            else:
                r['fields'] += [dict(case='integration-a', status='FAIL')] * 2
            bad_results.append(r)
        for r in bad_results:
            with self.subTest(result=r), self.assertRaises(ValueError):
                d.partition(selection(), selection(), r)
        for historical in (selection() * 2, selection()[:1],
                           [dict(x, reason='changed') for x in selection()]):
            with self.subTest(historical=historical), self.assertRaises(ValueError):
                d.partition(selection(), historical, source())

    def test_combined_requires_exact_unique_complete_success_and_provenance(self):
        d = driver()
        passed = dict(case='integration-a', field='two', status='PASS', restored=True, attempted=True)
        report = d.combine(selection(), source(), [passed], '/immutable/source', '/immutable/new')
        self.assertTrue(report['accepted'])
        self.assertEqual([x['attempt'] for x in report['fields']], ['/immutable/source', '/immutable/new'])
        for rows in ([], [dict(case='integration-a', status='FAIL')],
                     [dict(passed, restored=False)], [passed, passed],
                     [dict(passed, field='unknown')]):
            with self.subTest(rows=rows):
                report = d.combine(selection(), source(), rows, '/immutable/source', '/immutable/new')
                self.assertFalse(report['accepted'])
                self.assertEqual(len(report['fields']), 2)
                self.assertEqual(report['fields'][1]['attempt'], '/immutable/new')
        self.assertEqual(d.combine(selection(), source(), [], 'old', 'new')['fields'][1]['status'], 'BLOCKED')

    def test_real_fork_pool_inherits_callback_and_bounds_processes(self):
        d = driver()
        import multiprocessing
        import threading
        import time
        ctx = multiprocessing.get_context('fork')
        lock, active, peak = ctx.Lock(), ctx.Value('i', 0), ctx.Value('i', 0)
        def callback(value):
            with lock:
                active.value += 1
                peak.value = max(peak.value, active.value)
            time.sleep(0.04)
            with lock:
                active.value -= 1
            return dict(value=value, pid=os.getpid())
        rows = d.process_cases(callback, list(range(12)))
        self.assertEqual(sorted(x['value'] for x in rows), list(range(12)))
        self.assertEqual(len({x['pid'] for x in rows}), 4)
        self.assertNotIn(os.getpid(), {x['pid'] for x in rows})
        self.assertLessEqual(peak.value, 4)
        stop = threading.Event()
        thread = threading.Thread(target=stop.wait)
        thread.start()
        try:
            with self.assertRaises(ValueError):
                d.process_cases(callback, [1])
        finally:
            stop.set()
            thread.join()

    def test_filtered_original_case_retains_locks_and_restores_only_pending_leaf(self):
        d, f, run, attempt = self.fixture()
        import multiprocessing
        seen, gates = [], []
        original_case, original_plan = f.Cohort._case, f.Cohort.plan_apply
        c = d.make_cohort(f, run, {('integration-a', 'two')}, lambda: gates.append(True), multiprocessing.get_context('fork'))
        def plan(case, phase, allow_noop=False):
            seen.append((phase, allow_noop))
            self.assertTrue((run / 'work/integration-a/.field-updates.lock').exists())
        c.plan_apply = plan
        c.verify = lambda *args: 'synthetic-offline-only'
        original = (run / 'work/integration-a/main.tf.json').read_bytes()
        rows = c.case('integration-a')
        self.assertEqual([r['field'] for r in rows], ['two'])
        self.assertTrue(rows[0]['restored'])
        self.assertEqual(len(seen), 2)
        self.assertEqual(original, (run / 'work/integration-a/main.tf.json').read_bytes())
        self.assertIs(f.Cohort._case, original_case)
        self.assertIs(f.Cohort.plan_apply, original_plan)
        self.assertFalse((run / 'work/integration-a/.field-updates.lock').exists())
        self.assertGreaterEqual(len(gates), 2)

    def test_shared_gate_latches_safe_distinct_auth_serving_receipts(self):
        d = driver()
        import multiprocessing
        from types import SimpleNamespace
        ctx = multiprocessing.get_context('fork')
        for kind in ('auth', 'serving', 'unknown'):
            def fail():
                raise d.GateFailure(kind)
            gate = d.DeploymentGate(SimpleNamespace(gate=fail), ctx)
            with self.assertRaises(d.GateFailure):
                gate()
            receipt = gate.receipt()
            self.assertTrue(receipt[kind + '_gate_failed'])
            self.assertTrue(receipt['dispatch_stopped'])
            with self.assertRaises(d.GateFailure):
                gate()
            self.assertEqual(receipt, gate.receipt())
        helper = SimpleNamespace(gate=lambda: dict(auth_ok=False, serving_ok=True))
        gate = d.DeploymentGate(helper, ctx)
        with self.assertRaises(d.GateFailure):
            gate()
        self.assertTrue(gate.receipt()['auth_gate_failed'])
        gate = d.DeploymentGate(SimpleNamespace(gate=lambda: None), ctx)
        d.process_cases(lambda _: (gate(), os.getpid())[1], list(range(8)))
        self.assertEqual(gate.receipt()['checks'], 8)

    def test_offline_execution_archives_prior_pointer_and_saves_gated_receipt(self):
        from unittest import mock
        d, f, run, source_attempt = self.fixture()
        helper = f.ROOT / 'gate.py'
        helper.write_text("def gate():\n    return {'auth_ok': False, 'serving_ok': True}\n")
        helper.chmod(0o600)
        latest = run / 'field-updates/results.json'
        latest.write_text('{"historical-pointer": true}')
        latest.chmod(0o600)
        historical = {str(p): p.read_bytes() for p in source_attempt.iterdir()}
        previous = latest.read_bytes()
        with mock.patch('subprocess.run', side_effect=AssertionError('remote operation forbidden')):
            code, out = d.execute(f, run, source_attempt, helper)
        self.assertEqual(code, 1)
        self.assertEqual(historical, {str(p): p.read_bytes() for p in source_attempt.iterdir()})
        self.assertEqual((out / 'previous-latest-results.json').read_bytes(), previous)
        receipt = json.loads((out / 'final-receipt.json').read_text())
        self.assertTrue(receipt['auth_gate_failed'])
        self.assertFalse(receipt['serving_gate_failed'])
        combined = json.loads((out / 'combined-coverage.json').read_text())
        self.assertFalse(combined['accepted'])
        self.assertEqual(combined['fields'][1]['status'], 'BLOCKED')
        self.assertEqual(json.loads(latest.read_text())['immutable_result'], str(out / 'results.json'))
        provenance = json.loads((out / 'provenance.json').read_text())
        self.assertEqual(provenance['gate_helper_sha256'], d.digest(helper))
        self.assertEqual(d.digest(out / 'gate-helper.py'), d.digest(helper))
        self.assertFalse((run / '.field-updates.lock').exists())

    def test_offline_process_continuation_runs_pending_and_preserves_raw_history(self):
        from unittest import mock
        d, f, run, attempt = self.fixture()
        reviewed = ('testing/live/field-updates.py', 'testing/live/test_field_updates.py',
                    'testing/live/unclassified-secrets.sh', 'testing/live/test_unclassified_secrets.py')
        for name in reviewed:
            path = f.ROOT / name
            path.write_text('synthetic fixture source\n')
            path.chmod(0o600)
        proof = run / 'remediation-evidence/harness-replay-review-v2'
        proof.mkdir(parents=True, mode=0o700)
        (run / 'remediation-evidence').chmod(0o700)
        (proof / 'manifest.json').write_text(json.dumps(dict(root=str(f.ROOT), files={name:d.digest(f.ROOT / name) for name in reviewed})))
        (proof / 'manifest.json').chmod(0o600)
        (run / 'bin').mkdir(mode=0o700)
        binary = run / 'bin/terraform-provider-adaptive'
        binary.write_bytes(b'synthetic offline binary')
        binary.chmod(0o600)
        (run / 'manifest.json').write_text(json.dumps(dict(prefix='unit-owned', url='http://localhost:8080', binary_sha256=d.digest(binary))))
        backend = dict(root=str(f.ROOT), files={'testing/live/run.sh':d.digest(f.ROOT / 'testing/live/run.sh')})
        (run / 'inventory/serving-backend-source.json').write_text(json.dumps(backend))
        (run / 'inventory/serving-backend-source.json').chmod(0o600)
        state = dict(resources=[dict(instances=[dict(attributes=dict(id='synthetic-id'))])])
        (run / 'work/integration-a/terraform.tfstate').write_text(json.dumps(state))
        helper = f.ROOT / 'gate.py'
        helper.write_text("def gate():\n    return None\ndef resource_ids(prefix):\n    return {'unit-owned-integration-a': 'synthetic-id'}\ndef export(name, typ):\n    return {'configuration': {'service_account': '<REDACTED>'}}, 'digest'\n")
        helper.chmod(0o600)
        historical = {str(p): p.read_bytes() for p in attempt.iterdir()}
        original = (run / 'work/integration-a/main.tf.json').read_bytes()
        def plan(cohort, case, phase, allow_noop=False):
            self.assertTrue((run / '.field-updates.lock').exists())
            self.assertTrue((run / 'work/integration-a/.field-updates.lock').exists())
        with mock.patch.object(f.Cohort, 'plan_apply', plan), mock.patch.object(f.Cohort, 'verify', return_value='synthetic-only'), mock.patch('subprocess.run', side_effect=AssertionError('no CLI allowed')):
            code, out = d.execute(f, run, attempt, helper)
        self.assertEqual(code, 0)
        raw = json.loads((out / 'results.json').read_text())
        self.assertEqual([r['field'] for r in raw['fields']], ['two'])
        coverage = json.loads((out / 'combined-coverage.json').read_text())
        self.assertTrue(coverage['accepted'])
        self.assertEqual(coverage['reused'], 1)
        self.assertEqual(historical, {str(p): p.read_bytes() for p in attempt.iterdir()})
        self.assertEqual(original, (run / 'work/integration-a/main.tf.json').read_bytes())
        for path in (out, *out.rglob('*')):
            self.assertEqual(path.stat().st_mode & 0o077, 0)

    def test_cli_check_is_readonly_and_execute_requires_explicit_helper(self):
        from unittest import mock
        import contextlib
        import io
        d, f, run, attempt = self.fixture()
        before = {str(p): p.read_bytes() for p in run.rglob('*') if p.is_file()}
        output = io.StringIO()
        with mock.patch.object(d, 'load_fields', return_value=f), mock.patch('subprocess.run', side_effect=AssertionError('remote forbidden')), contextlib.redirect_stdout(output), contextlib.redirect_stderr(io.StringIO()):
            self.assertEqual(d.main([str(run), '--check', '--source-attempt', str(attempt)]), 0)
            with self.assertRaises(SystemExit):
                d.main([str(run), '--execute', '--source-attempt', str(attempt)])
        self.assertEqual(json.loads(output.getvalue())['pending'], 1)
        self.assertEqual(before, {str(p): p.read_bytes() for p in run.rglob('*') if p.is_file()})

    def test_source_success_cannot_expand_selected_set_or_omit_attempt_evidence(self):
        import copy
        d = driver()
        with_skip = selection() + [dict(case='integration-a', field='name', selected=False, reason='identity')]
        for change in ('unselected-pass', 'unattempted-pass', 'unknown-status', 'case-and-leaf'):
            r = copy.deepcopy(source())
            if change == 'unselected-pass':
                r['fields'][0]['field'] = 'name'
            elif change == 'unattempted-pass':
                r['fields'][0]['attempted'] = False
                r['attempted'] = 0
            elif change == 'unknown-status':
                r['fields'][0]['status'] = 'MAYBE'
                r['counts'] = {'MAYBE': 1}
            else:
                r['fields'].append(dict(case='integration-a', status='FAIL'))
                r['counts']['FAIL'] = 1
            with self.subTest(change=change), self.assertRaises(ValueError):
                d.partition(with_skip, with_skip, r)

    def test_shared_ledger_records_survive_four_real_workers(self):
        import multiprocessing
        d, f, run, attempt = self.fixture()
        ctx = multiprocessing.get_context('fork')
        c = d.make_cohort(f, run, {('integration-a', 'two')}, lambda: None, ctx)
        def record(number):
            c.record('integration-a', 'synthetic-' + str(number), 'PASS', c.out)
            return number
        d.process_cases(record, list(range(12)))
        rows = [json.loads(line) for line in (run / 'ledger.jsonl').read_text().splitlines()]
        self.assertEqual(len(rows), 13)
        self.assertEqual({r['phase'] for r in rows[1:]}, {'synthetic-' + str(i) for i in range(12)})

    def test_tf_show_readback_are_gated_before_and_after_even_on_errors(self):
        from unittest import mock
        import multiprocessing
        d, f, run, attempt = self.fixture()
        events = []
        c = d.make_cohort(f, run, set(), lambda: events.append('gate'), multiprocessing.get_context('fork'))
        for method in ('tf', 'show', 'readback'):
            for failed in (False, True):
                events.clear()
                def operation(*args, **kwargs):
                    events.append('operation')
                    if failed:
                        raise RuntimeError('synthetic-canary-never-log')
                    return True
                with mock.patch.object(f.Cohort, method, operation):
                    if failed:
                        with self.assertRaises(RuntimeError):
                            getattr(c, method)()
                    else:
                        self.assertTrue(getattr(c, method)())
                self.assertEqual(events, ['gate', 'operation', 'gate'])

    def test_source_changed_while_archiving_is_blocked_before_helper_import(self):
        from unittest import mock
        d, f, run, attempt = self.fixture()
        helper = f.ROOT / 'gate.py'
        helper.write_text("raise AssertionError('helper-must-not-import')\n")
        helper.chmod(0o600)
        real_open = f.private_open
        from contextlib import contextmanager
        @contextmanager
        def changed_open(run_arg, path, mode):
            with real_open(run_arg, path, mode) as stream:
                yield stream
            if Path(path).name == 'source-results.json':
                with (attempt / 'results.json').open('a') as raw:
                    raw.write(' ')
        with mock.patch.object(f, 'private_open', changed_open), mock.patch.object(d, 'load_module', side_effect=AssertionError('must block before import')) as load:
            code, out = d.execute(f, run, attempt, helper)
        self.assertEqual(code, 1)
        load.assert_not_called()
        receipt = json.loads((out / 'final-receipt.json').read_text())
        self.assertFalse(receipt['source_unchanged'])

    def test_final_receipt_survives_missing_source_when_auth_gate_fails(self):
        d, f, run, attempt = self.fixture()
        helper = f.ROOT / 'gate.py'
        helper.write_text("from pathlib import Path\ndef gate():\n    Path(" + repr(str(attempt / 'results.json')) + ").unlink(missing_ok=True)\n    return {'auth_ok': False, 'serving_ok': True}\n")
        helper.chmod(0o600)
        code, out = d.execute(f, run, attempt, helper)
        self.assertEqual(code, 1)
        receipt = json.loads((out / 'final-receipt.json').read_text())
        self.assertTrue(receipt['auth_gate_failed'])
        self.assertFalse(receipt['source_unchanged'])


if __name__ == '__main__':
    unittest.main()
