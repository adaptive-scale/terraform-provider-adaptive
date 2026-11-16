import importlib.util
import pathlib
import json
import tempfile
import unittest
from unittest import mock

P = pathlib.Path(__file__).with_name('field-updates.py')

class SelectionTests(unittest.TestCase):
    def test_safety_and_native_mutations(self):
        self.assertTrue(P.exists(), 'field-updates implementation is missing')
        spec = importlib.util.spec_from_file_location('field_updates', P)
        m = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(m)
        for key in ['name', 'type', 'tags', 'create_if_not_exists', 'use_role_arn', 'use_iam_auth', 'auth_mode']:
            self.assertIsNone(m.mutate('mysql', key, False)[0])
        self.assertEqual(m.mutate('mysql', 'ssl_mode', 'require')[0], 'required')
        self.assertEqual(m.mutate('postgres', 'ssl_mode', 'require')[0], 'verify-full')
        self.assertEqual(m.mutate('x', 'host', 'old.invalid')[0], 'field-update.invalid')
        self.assertEqual(m.mutate('x', 'uri', 'mongodb://u:p@old.invalid/db')[0], 'mongodb://u:p@field-update.invalid/db')
        self.assertEqual(m.mutate('x', 'port', '3389')[0], '3390')
        self.assertEqual(m.mutate('x', 'record', True)[0], False)
        self.assertEqual(m.mutate('adaptive_rdp', 'name', 'target', nested=True)[0], 'target-field-update')
        self.assertIsNone(m.mutate('x', 'mystery', {'a': 1})[0])
        with self.assertRaises(RuntimeError):
            m.compare_remote(None, False, sensitive=False)
        self.assertEqual(m.compare_remote(None, 'synthetic', sensitive=True), 'exact-state-and-redacted-remote-readback')
        self.assertEqual(m.compare_remote(False, False, sensitive=False), 'exact-state-and-exact-remote-value')

    def test_proxysql_mutations_use_native_mysql_modes(self):
        spec = importlib.util.spec_from_file_location('field_updates_proxy', P)
        assert spec is not None and spec.loader is not None
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        self.assertEqual(module.mutate('proxysql', 'ssl_mode', 'require')[0], 'required')
        self.assertEqual(module.mutate('proxysql', 'ssl_mode', 'required')[0], 'preferred')

    def test_mongodb36_omitted_false_is_a_named_typed_contract_not_null_equivalence(self):
        spec = importlib.util.spec_from_file_location('field_updates_false', P)
        assert spec is not None and spec.loader is not None
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        compare = getattr(module, 'compare_config_field', None)
        self.assertIsNotNone(compare, 'typed omitted-false readback comparison is missing')
        assert callable(compare)
        self.assertEqual(compare('mongodb36', 'useTLS', {}, None, False, False),
                         'exact-state-and-source-defined-omitted-false')
        for typ, key, document, expected in [('mongodb36', 'useTLS', {'useTLS': None}, False),
                                               ('mongodb36', 'useTLS', {}, True),
                                               ('mongodb', 'useTLS', {}, False),
                                               ('mongodb36', 'other', {}, False)]:
            with self.assertRaises(RuntimeError):
                compare(typ, key, document, None, expected, False)
        with self.assertRaises(RuntimeError):
            module.compare_remote(0, False, False)

    def test_completed_field_failure_returns_nonzero(self):
        spec = importlib.util.spec_from_file_location('field_updates_exit', P)
        assert spec is not None and spec.loader is not None
        m = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(m)
        with tempfile.TemporaryDirectory() as directory:
            cohort = mock.Mock()
            cohort.cases = ['integration-unit-only']
            cohort.out = pathlib.Path(directory).resolve()
            (cohort.out / 'field-updates').mkdir(mode=0o700)
            cohort.inventory.return_value = [{'selected': True}]
            cohort.case.return_value = [{'status': 'FAIL', 'attempted': True, 'restored': True}]
            with mock.patch.object(m, 'Cohort', return_value=cohort), mock.patch('sys.argv', ['field-updates', str(pathlib.Path(directory).resolve()), '--execute']), mock.patch('builtins.print'):
                self.assertEqual(m.main(), 1)

    def test_yaml_reader_uses_existing_ruby_without_python_packages(self):
        spec = importlib.util.spec_from_file_location('field_updates_yaml', P)
        assert spec is not None and spec.loader is not None
        m = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(m)
        document = m.RubyYaml().safe_load("configuration:\n  enabled: false\n  port: '5432'\n  password: unit-only-canary\n")
        self.assertIs(document['configuration']['enabled'], False)
        self.assertEqual(document['configuration']['port'], '5432')
        with self.assertRaisesRegex(RuntimeError, '^cannot parse private YAML export$'):
            m.RubyYaml().safe_load('password: [unit-only-canary')

class CohortEligibilityTests(unittest.TestCase):
    def setUp(self):
        spec = importlib.util.spec_from_file_location('field_updates_cohort', P)
        assert spec is not None and spec.loader is not None
        self.module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.module)
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = pathlib.Path(temporary.name).resolve() / 'repo'
        self.run_dir = pathlib.Path(temporary.name).resolve() / 'run'
        (self.root / 'testing/live').mkdir(parents=True)
        (self.root / 'docs').mkdir()
        (self.root / 'testing/live/run.sh').write_text('#!/bin/bash\nexit 1\n')
        (self.root / 'docs/parity-current-integration-fields.csv').write_text('Integration,Reverse,Key\n')
        (self.run_dir / 'inventory').mkdir(parents=True)
        (self.run_dir / 'manifest.json').write_text(json.dumps({'prefix': 'unit-owned'}))
        schema = {'provider_schemas': {'registry.terraform.io/adaptive-scale/adaptive': {
            'resource_schemas': {'adaptive_resource': {'block': {'attributes': {}}}}}}}
        (self.run_dir / 'inventory/schema.json').write_text(json.dumps(schema))
        patcher = mock.patch.object(self.module, 'ROOT', self.root)
        patcher.start()
        self.addCleanup(patcher.stop)

    def fixture(self, count=82, passing=None):
        (self.run_dir / 'evidence').mkdir(mode=0o700, exist_ok=True)
        entries = [{'type': 'unit-' + str(i), 'attributes': {'host': 'unit.invalid'}} for i in range(count)]
        inventory = json.dumps({'cases': entries})
        (self.run_dir / 'inventory/integration-cases.json').write_text(inventory)
        (self.root / 'testing/live/integration-cases.json').write_text(inventory)
        self.cases = ['integration-' + entry['type'] for entry in entries]
        rows = []
        for i, (case, entry) in enumerate(zip(self.cases, entries)):
            directory = self.run_dir / 'work' / case
            directory.mkdir(parents=True, exist_ok=True)
            config = {'resource': {'adaptive_resource': {'test': {
                'name': 'unit-owned-' + case, 'type': entry['type'], **entry['attributes']}}}}
            (directory / 'main.tf.json').write_text(json.dumps(config))
            (directory / 'terraform.tfstate').write_text('{}')
            rows.append({'case': case, 'phase': 'updated-noop',
                         'status': 'PASS' if passing is None or i < passing else 'FAIL'})
        (self.run_dir / 'ledger.jsonl').write_text(''.join(json.dumps(row) + '\n' for row in rows))
        for path in [self.run_dir, *self.run_dir.rglob('*')]:
            path.chmod(0o700 if path.is_dir() else 0o600)

    def test_unsafe_paths_rejected_before_any_output_or_process(self):
        for relative in ['work/integration-unit-0/main.tf.json', 'work/integration-unit-0',
                         'inventory', 'ledger.jsonl', 'field-updates', 'evidence']:
            with self.subTest(relative=relative), tempfile.TemporaryDirectory() as outside:
                self.fixture(1)
                path = self.run_dir / relative
                if not path.exists():
                    path.mkdir(mode=0o700)
                target = pathlib.Path(outside) / 'credential-canary'
                path.rename(target)
                path.symlink_to(target, target_is_directory=target.is_dir())
                before = {str(p.relative_to(outside)): p.read_bytes() for p in pathlib.Path(outside).rglob('*') if p.is_file()}
                try:
                    with mock.patch.object(self.module.subprocess, 'run') as process:
                        with self.assertRaises(ValueError):
                            self.module.Cohort(self.run_dir)
                        process.assert_not_called()
                    self.assertEqual(before, {str(p.relative_to(outside)): p.read_bytes() for p in pathlib.Path(outside).rglob('*') if p.is_file()})
                    self.assertFalse((self.run_dir / 'field-updates').exists() and relative != 'field-updates')
                finally:
                    path.unlink()
                    target.rename(path)
                    if relative in {'evidence', 'field-updates'}:
                        path.rmdir()

    def test_nonprivate_file_and_directory_fail_before_output(self):
        for relative, mode in [('manifest.json', 0o644), ('inventory', 0o755),
                               ('work/integration-unit-0/main.tf.json', 0o660)]:
            self.fixture(1)
            path = self.run_dir / relative
            path.chmod(mode)
            with self.assertRaises(ValueError):
                self.module.Cohort(self.run_dir)
            self.assertFalse((self.run_dir / 'field-updates').exists())
            path.chmod(0o700 if path.is_dir() else 0o600)

    def append(self, case, status):
        with (self.run_dir / 'ledger.jsonl').open('a') as stream:
            stream.write(json.dumps({'case': case, 'phase': 'updated-noop', 'status': status}) + '\n')

    def test_healthy_cohorts_follow_pinned_inventory_not_historical_count(self):
        for count, passing in [(69, 69), (82, 82), (82, 69)]:
            with self.subTest(count=count, passing=passing):
                self.fixture(count, passing)
                cohort = self.module.Cohort(self.run_dir)
                self.assertEqual(cohort.cases, sorted(self.cases[:passing]))

    def test_inventory_is_required_unique_and_matches_source(self):
        for problem in ['missing', 'empty', 'duplicate', 'unknown-type', 'source-mismatch']:
            with self.subTest(problem=problem):
                self.fixture()
                path = self.run_dir / 'inventory/integration-cases.json'
                document = json.loads(path.read_text())
                if problem == 'missing':
                    path.unlink()
                else:
                    if problem == 'empty':
                        document['cases'] = []
                    elif problem == 'duplicate':
                        document['cases'].append(document['cases'][0])
                    elif problem == 'unknown-type':
                        document['cases'][0]['type'] = '../unknown'
                    else:
                        document['cases'][0]['attributes']['host'] = 'changed.invalid'
                    path.write_text(json.dumps(document))
                    if problem != 'source-mismatch':
                        (self.root / 'testing/live/integration-cases.json').write_text(json.dumps(document))
                with self.assertRaises((ValueError, FileNotFoundError)):
                    self.module.Cohort(self.run_dir)

    def test_unknown_and_nonoriginal_ledger_cases_are_rejected(self):
        for case in ['integration-unknown', 'integration-unit-0-imported']:
            with self.subTest(case=case):
                self.fixture()
                self.append(case, 'PASS')
                with self.assertRaises(ValueError):
                    self.module.Cohort(self.run_dir)

    def test_latest_failure_or_started_never_revives_older_pass(self):
        for status in ['FAIL', 'STARTED', 'UNKNOWN']:
            with self.subTest(status=status):
                self.fixture()
                self.append(self.cases[0], status)
                self.assertEqual(self.module.Cohort(self.run_dir).cases, sorted(self.cases[1:]))

    def test_empty_latest_passing_cohort_is_rejected(self):
        for count in [0, 82]:
            with self.subTest(count=count):
                self.fixture(count, passing=0)
                with self.assertRaises(ValueError):
                    self.module.Cohort(self.run_dir)

    def test_selected_config_must_match_exact_original_name_and_type(self):
        for key, value in [('name', 'foreign-integration-unit-0'),
                           ('name', 'unit-owned-integration-unit-1'),
                           ('name', 'unit-owned-integration-unit-0-imported'),
                           ('type', 'unit-1')]:
            with self.subTest(key=key, value=value):
                self.fixture()
                path = self.run_dir / 'work' / self.cases[0] / 'main.tf.json'
                config = json.loads(path.read_text())
                config['resource']['adaptive_resource']['test'][key] = value
                path.write_text(json.dumps(config))
                with self.assertRaises(ValueError):
                    self.module.Cohort(self.run_dir)

    def test_saved_plan_still_rejects_delete_replacement_and_foreign_updates(self):
        self.fixture(1)
        cohort = self.module.Cohort(self.run_dir)
        for address, actions in [('adaptive_resource.test', ['delete']),
                                 ('adaptive_resource.test', ['delete', 'create']),
                                 ('adaptive_resource.foreign', ['update'])]:
            with self.subTest(address=address, actions=actions):
                plan = {'resource_changes': [{'address': address, 'change': {'actions': actions}}]}
                with mock.patch.object(cohort, 'tf', return_value=True) as tf, \
                     mock.patch.object(cohort, 'show', return_value=(plan, cohort.out)):
                    with self.assertRaisesRegex(RuntimeError, 'refusing non-update saved plan'):
                        cohort.plan_apply(self.cases[0], 'unit')
                    self.assertEqual(tf.call_count, 1, 'unsafe plan must never be applied')

    def test_failed_mutation_still_restores_original_bytes(self):
        self.fixture(1)
        cohort = self.module.Cohort(self.run_dir)
        case = self.cases[0]
        path = self.run_dir / 'work' / case / 'main.tf.json'
        original = path.read_bytes()
        with mock.patch.object(cohort, 'plan_apply', side_effect=[RuntimeError('unit rejection'), None]) as apply, \
             mock.patch.object(cohort, 'verify'), mock.patch.object(cohort, 'show'), \
             mock.patch.object(cohort, 'readback'), mock.patch('builtins.print'):
            rows = cohort.case(case)
        self.assertEqual(path.read_bytes(), original)
        attempted = [row for row in rows if row.get('attempted')]
        self.assertEqual(len(attempted), 1)
        self.assertEqual(attempted[0]['status'], 'FAIL')
        self.assertIs(attempted[0]['restored'], True)
        self.assertEqual(apply.call_args_list[-1], mock.call(case, 'field-host-restore', allow_noop=True))

    def test_unrestored_backup_still_blocks_execution(self):
        self.fixture(1)
        cohort = self.module.Cohort(self.run_dir)
        (cohort.out / (self.cases[0] + '-original.tf.json')).write_text('unrestored unit fixture')
        with mock.patch.object(cohort, 'plan_apply') as apply:
            with self.assertRaisesRegex(RuntimeError, 'unrestored original exists'):
                cohort.case(self.cases[0])
            apply.assert_not_called()

    def test_duplicate_cases_rejected_before_constructor_or_writes(self):
        with mock.patch.object(self.module, 'Cohort') as constructor, \
             mock.patch('sys.argv', ['field-updates', str(self.run_dir), '--execute',
                                     '--case', 'integration-unit-0', '--case', 'integration-unit-0']):
            with self.assertRaises(ValueError):
                self.module.main()
            constructor.assert_not_called()

    def test_attempts_preserve_selection_state_results_and_saved_plans(self):
        self.fixture(1)
        attempts = []
        for i in range(2):
            c = self.module.Cohort(self.run_dir)
            c.inventory()
            def process(command, **kwargs):
                kwargs['stdout'].write(json.dumps({'attempt': i}).encode())
                return mock.Mock(returncode=0)
            with mock.patch.object(self.module.subprocess, 'run', side_effect=process):
                c.show(self.cases[0], 'unit-state')
            plans = []
            def tf(case, phase, expected, *args):
                for arg in args:
                    if arg.startswith('-out='):
                        p = self.run_dir / 'work' / case / arg[5:]
                        self.module.private_write(self.run_dir, p, b'private-plan')
                        plans.append(p)
                return True
            plan = {'resource_changes': [{'address': 'adaptive_resource.test', 'change': {'actions': ['update']}}]}
            with mock.patch.object(c, 'tf', side_effect=tf), mock.patch.object(c, 'show', return_value=(plan, c.out)):
                c.plan_apply(self.cases[0], 'field-host')
            self.module.private_write(self.run_dir, c.out / 'results.json', json.dumps({'attempt': i}))
            snapshots = {p: p.read_bytes() for p in c.out.rglob('*') if p.is_file()}
            attempts.append((c.out, snapshots, plans))
        self.assertNotEqual(attempts[0][0], attempts[1][0])
        self.assertTrue(set(attempts[0][2]).isdisjoint(attempts[1][2]))
        for _, snapshots, _ in attempts:
            for path, original in snapshots.items():
                self.assertEqual(path.read_bytes(), original)

    def test_exclusive_lock_blocks_other_process_and_stale_lock_is_retained(self):
        import subprocess
        import sys
        self.fixture(1)
        lockpath = self.run_dir / '.field-updates.lock'
        script = """import importlib.util,sys
from pathlib import Path
spec=importlib.util.spec_from_file_location('fields',sys.argv[1]); m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
with m.ExclusiveLock(Path(sys.argv[2]),Path(sys.argv[2])/'.field-updates.lock'):
 print('LOCKED',flush=True);sys.stdin.readline()
"""
        owner = subprocess.Popen([sys.executable, '-c', script, str(P), str(self.run_dir)],
                                 stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        try:
            self.assertIsNotNone(owner.stdout)
            assert owner.stdout is not None
            self.assertEqual(owner.stdout.readline().strip(), 'LOCKED')
            blocked = subprocess.run([sys.executable, str(P), str(self.run_dir), '--execute'], capture_output=True, text=True)
            self.assertEqual(blocked.returncode, 1)
            self.assertTrue(lockpath.exists())
            self.assertFalse((self.run_dir / 'field-updates').exists())
        finally:
            owner.communicate('\n', timeout=10)
        self.assertFalse(lockpath.exists(), 'only owner cleans its own lock')
        lockpath.write_text('interrupted owner\n'); lockpath.chmod(0o600)
        blocked = subprocess.run([sys.executable, str(P), str(self.run_dir), '--execute'], capture_output=True, text=True)
        self.assertEqual(blocked.returncode, 1)
        self.assertEqual(lockpath.read_text(), 'interrupted owner\n')
        self.assertFalse((self.run_dir / 'field-updates').exists())
        lockpath.unlink()  # Test setup only; the harness never removes stale locks.
        interrupted = subprocess.Popen([sys.executable, '-c', script, str(P), str(self.run_dir)],
                                       stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        try:
            assert interrupted.stdout is not None
            self.assertEqual(interrupted.stdout.readline().strip(), 'LOCKED')
            interrupted.kill()
            interrupted.communicate(timeout=10)
            token = lockpath.read_bytes()
            blocked = subprocess.run([sys.executable, str(P), str(self.run_dir), '--execute'], capture_output=True, text=True)
            self.assertEqual(blocked.returncode, 1)
            self.assertEqual(lockpath.read_bytes(), token, 'a killed owner must block replay, not be auto-recovered')
        finally:
            if interrupted.poll() is None:
                interrupted.kill()
                interrupted.communicate(timeout=10)

    def test_fixture_lock_blocks_worker_and_cleanup_never_removes_replacement(self):
        self.fixture(1)
        c = self.module.Cohort(self.run_dir)
        path = self.run_dir / 'work' / self.cases[0] / '.field-updates.lock'
        with self.module.ExclusiveLock(self.run_dir, path), mock.patch.object(c, 'plan_apply') as apply:
            with self.assertRaises(ValueError):
                c.case(self.cases[0])
            apply.assert_not_called()
            self.assertEqual(list(c.out.iterdir()), [])
        self.assertFalse(path.exists())
        with self.module.ExclusiveLock(self.run_dir, path):
            path.rename(path.with_name('retained-owned-lock'))
            self.module.private_write(self.run_dir, path, 'another owner\n')
        self.assertEqual(path.read_text(), 'another owner\n')

    def test_unsafe_replacement_after_constructor_blocks_config_write_and_record(self):
        self.fixture(1)
        c = self.module.Cohort(self.run_dir)
        path = self.run_dir / 'work' / self.cases[0] / 'main.tf.json'
        target = self.run_dir / 'external-canary.json'
        path.rename(target)
        original = target.read_bytes()
        path.symlink_to(target)
        with mock.patch.object(c, 'plan_apply') as apply:
            with self.assertRaises(ValueError):
                c.case(self.cases[0])
            apply.assert_not_called()
        self.assertEqual(target.read_bytes(), original)

    def test_source_inventory_symlink_rejected_before_output(self):
        self.fixture(1)
        path = self.root / 'testing/live/integration-cases.json'
        target = self.root / 'credential-canary.json'
        path.rename(target); path.symlink_to(target)
        with self.assertRaises(ValueError):
            self.module.Cohort(self.run_dir)
        self.assertFalse((self.run_dir / 'field-updates').exists())

    def test_write_rejects_lexical_traversal_before_touching_external_file(self):
        self.fixture(1)
        target = self.run_dir.parent / 'credential-canary.json'
        target.write_text('preserve'); target.chmod(0o600)
        with self.assertRaises(ValueError):
            self.module.private_write(self.run_dir, self.run_dir / '..' / target.name, 'overwrite')
        self.assertEqual(target.read_text(), 'preserve')

    def test_runner_log_directory_blocks_before_process_call(self):
        self.fixture(1)
        c = self.module.Cohort(self.run_dir)
        (c.out / (self.cases[0] + '-runner.log')).mkdir(mode=0o700)
        with mock.patch.object(self.module.subprocess, 'run') as process:
            with self.assertRaises((ValueError, OSError)):
                c.tf(self.cases[0], 'field-host-plan', 2, 'plan')
            process.assert_not_called()

    def test_show_cannot_inherit_external_log_or_data_output_paths(self):
        self.fixture(1)
        c = self.module.Cohort(self.run_dir)
        poisoned = {'TF_LOG_PATH': '/outside/credential-canary', 'TF_DATA_DIR': '/outside/data',
                    'TF_LOG': 'TRACE', 'TF_CLI_ARGS_show': '-no-color', 'TF_REATTACH_PROVIDERS': '{}'}
        def process(command, **kwargs):
            self.assertTrue(set(poisoned).isdisjoint(kwargs['env']))
            kwargs['stdout'].write(b'{}')
            return mock.Mock(returncode=0)
        with mock.patch.dict(self.module.os.environ, poisoned), mock.patch.object(self.module.subprocess, 'run', side_effect=process):
            c.show(self.cases[0], 'unit-state')

    def test_unreadable_directory_cannot_be_silently_skipped_by_preflight(self):
        self.fixture(1)
        path = self.run_dir / 'unreadable'
        path.mkdir(mode=0o700); path.chmod(0)
        try:
            with self.assertRaises(ValueError):
                self.module.Cohort(self.run_dir)
            self.assertFalse((self.run_dir / 'field-updates').exists())
        finally:
            path.chmod(0o700)

    def test_output_directory_roles_rejected_before_any_creation(self):
        for relative in ['evidence', 'field-updates']:
            self.fixture(1)
            path = self.run_dir / relative
            if path.exists():
                path.rmdir()
            path.write_text('not a directory'); path.chmod(0o600)
            with self.assertRaises(ValueError):
                self.module.Cohort(self.run_dir)
            self.assertEqual(path.read_text(), 'not a directory')
            path.unlink()
        self.fixture(1)
        path = self.run_dir / 'work' / self.cases[0] / 'terraform.tfstate'
        if path.exists():
            path.unlink()
        path.mkdir(mode=0o700)
        with self.assertRaises(ValueError):
            self.module.Cohort(self.run_dir)
        self.assertFalse((self.run_dir / 'field-updates').exists())

    def test_canonical_run_ancestors_and_nonregular_inputs(self):
        import os
        self.fixture(1)
        parent = self.run_dir.parent
        link = parent / 'credential-canary-link'
        link.symlink_to(self.run_dir, target_is_directory=True)
        for run in [link, link / '..' / 'run']:
            with self.assertRaises(ValueError):
                self.module.Cohort(run)
        parent.chmod(0o777)
        try:
            with self.assertRaises(ValueError):
                self.module.Cohort(self.run_dir)
        finally:
            parent.chmod(0o700)
        path = self.run_dir / 'ledger.jsonl'
        path.unlink(); os.mkfifo(path, 0o600)
        with self.assertRaises(ValueError):
            self.module.Cohort(self.run_dir)
        self.assertFalse((self.run_dir / 'field-updates').exists())

    def test_completed_results_discovery_keeps_legacy_and_attempt_evidence(self):
        self.fixture(1)
        base = self.run_dir / 'field-updates'; base.mkdir(mode=0o700)
        latest = base / 'results.json'
        legacy = b'{"legacy": "FAIL"}\n'
        latest.write_bytes(legacy); latest.chmod(0o600)
        with mock.patch.object(self.module.Cohort, 'case', return_value=[]), mock.patch('builtins.print'):
            for _ in range(2):
                with mock.patch('sys.argv', ['fields', str(self.run_dir), '--execute']):
                    self.assertEqual(self.module.main(), 0)
        document = json.loads(latest.read_text())
        self.assertTrue(document['mutable_latest'])
        result = pathlib.Path(document['immutable_result'])
        self.assertNotEqual(result, latest)
        self.assertEqual(json.loads(result.read_text())['fields'], [])
        preserved = list(base.glob('attempts/*/previous-latest-results.json'))
        self.assertIn(legacy, [p.read_bytes() for p in preserved])
        self.assertEqual(len(list(base.glob('attempts/*/results.json'))), 2)

    def test_process_doubles_preserve_two_full_attempts_and_report_phases(self):
        import os
        import subprocess
        import sys
        self.fixture(1)
        live = self.root / 'testing/live'
        (live / 'field-updates.py').write_bytes(P.read_bytes())
        (self.root / 'docs/parity-current-integration-fields.csv').write_text('Integration,Reverse,Key\nunit-0,host,host\n')
        schema_path = self.run_dir / 'inventory/schema.json'
        schema = json.loads(schema_path.read_text())
        schema['provider_schemas']['registry.terraform.io/adaptive-scale/adaptive']['resource_schemas']['adaptive_resource']['block']['attributes']['host'] = {'type': 'string'}
        schema_path.write_text(json.dumps(schema))
        tool = self.root / 'tool.py'
        tool.write_text('''import json,os,pathlib,sys,uuid
run=pathlib.Path(os.environ['OFFLINE_RUN']); a=sys.argv[1:]
with (run/'process-trace').open('a') as stream: stream.write(json.dumps(a)+'\\n')
case='integration-unit-0'; directory=run/'work'/case
attrs=json.loads((directory/'main.tf.json').read_text())['resource']['adaptive_resource']['test']
if a[0].startswith('-chdir='):
 if len(a)>3:
  result={'resource_changes':[{'address':'adaptive_resource.test','change':{'actions':['update']}}]}
 else:
  result={'values':{'root_module':{'resources':[{'address':'adaptive_resource.test','values':attrs}]}}}
 print(json.dumps(result));sys.exit(0)
if a[0]=='tf':
 _,_,case,phase,expected,*command=a
 for arg in command:
  if arg.startswith('-out='): (directory/arg[5:]).write_text('private saved plan')
 evidence=run/'evidence'/(phase+'-'+uuid.uuid4().hex+'.txt');evidence.write_text('offline process double')
else:
 phase='readback'
 evidence=run/'evidence'/(case+'-readback-'+uuid.uuid4().hex+'.yaml')
 evidence.write_text(json.dumps({'configuration':attrs}))
with (run/'ledger.jsonl').open('a') as stream:
 stream.write(json.dumps({'case':case,'phase':phase,'status':'PASS','evidence':str(evidence)})+'\\n')
''')
        (live / 'run.sh').write_text('#!/bin/bash\nexec python3 "$OFFLINE_TOOL" "$@"\n')
        bindir = self.root / 'bin'; bindir.mkdir()
        terraform = bindir / 'terraform'
        terraform.write_text('#!/bin/bash\nexec python3 "$OFFLINE_TOOL" "$@"\n'); terraform.chmod(0o700)
        env = dict(os.environ, PATH=str(bindir)+':'+os.environ['PATH'], OFFLINE_RUN=str(self.run_dir), OFFLINE_TOOL=str(tool))
        snapshots = []
        original = (self.run_dir / 'work' / self.cases[0] / 'main.tf.json').read_bytes()
        for _ in range(2):
            result = subprocess.run([sys.executable, str(live / 'field-updates.py'), str(self.run_dir), '--execute', '--workers', '1'], env=env, capture_output=True, text=True)
            diagnostics = '\n'.join(p.read_text() for p in (self.run_dir / 'field-updates').rglob('*') if p.is_file() and p.suffix in {'.log', '.json'})
            self.assertEqual(result.returncode, 0, result.stdout+result.stderr+diagnostics)
            latest = json.loads((self.run_dir / 'field-updates/results.json').read_text())
            self.assertEqual(latest['attempted'], 1)
            self.assertEqual(latest['restored'], 1)
            resultpath = pathlib.Path(latest['immutable_result'])
            snapshots.append({p: p.read_bytes() for p in resultpath.parent.rglob('*') if p.is_file()})
            self.assertEqual((self.run_dir / 'work' / self.cases[0] / 'main.tf.json').read_bytes(), original)
        for snapshot in snapshots:
            for path, data in snapshot.items():
                self.assertEqual(path.read_bytes(), data)
        self.assertEqual(len(list((self.run_dir / 'work' / self.cases[0]).glob('*.tfplan'))), 4)
        rows = [json.loads(line) for line in (self.run_dir / 'ledger.jsonl').read_text().splitlines()]
        self.assertEqual(sum(r['phase']=='field-host-verify' for r in rows), 2)
        self.assertTrue(all('field-updates/results.json' not in r.get('evidence','') for r in rows))
        spec = importlib.util.spec_from_file_location('offline_report', P.with_name('report.py'))
        assert spec is not None and spec.loader is not None
        report = importlib.util.module_from_spec(spec); spec.loader.exec_module(report)
        fields = report.field_mutations(list(report.latest_records(rows).values()))
        self.assertEqual(fields[0]['status'], 'PASS')

    def test_unexercised_reason_describes_dynamic_eligibility(self):
        self.fixture(82, passing=69)
        (self.root / 'docs/parity-current-integration-fields.csv').write_text(
            'Integration,Reverse,Key\nunit-81,host,host\n')
        cohort = self.module.Cohort(self.run_dir)
        cohort.inventory()
        mapped = json.loads((cohort.out / 'mapped-unexercised.json').read_text())
        self.assertEqual(mapped[0]['reason'], 'integration outside eligible passing originals')


if __name__ == '__main__':
    unittest.main()
