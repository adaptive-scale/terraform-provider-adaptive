"""Stdlib-only tests: python3 -m unittest discover -s testing/live -p test_report.py."""
import importlib.util
import json
import os
import sys
from pathlib import Path
import tempfile
import unittest

SPEC = importlib.util.spec_from_file_location("live_report", Path(__file__).with_name("report.py"))
assert SPEC is not None
report = importlib.util.module_from_spec(SPEC)
if SPEC.loader and SPEC.origin and Path(SPEC.origin).exists():
    SPEC.loader.exec_module(report)


class LedgerTests(unittest.TestCase):
    def test_started_completion_acknowledgement_and_incomplete_restart(self):
        rows = [dict(case="x", phase="create-apply", status=s, exit=0)
                for s in ("STARTED", "FAIL", "ACKNOWLEDGED")]
        self.assertEqual(report.latest_records(rows)[("x", "create-apply")]["status"], "FAIL")
        rows.extend(dict(case="x", phase="create-apply", status=s, exit=0)
                    for s in ("STARTED", "PASS"))
        self.assertEqual(report.latest_records(rows)[("x", "create-apply")]["status"], "PASS")
        rows.append(dict(case="x", phase="create-apply", status="STARTED"))
        self.assertEqual(report.latest_records(rows)[("x", "create-apply")]["status"], "INCOMPLETE")
        self.assertEqual(report.coverage(list(report.latest_records(rows).values()))["create"]["status"], "INCOMPLETE")

    def test_latest_per_case_phase_uses_status_not_exit(self):
        rows = [
            {"case": "x", "phase": "negative", "status": "FAIL", "exit": 0},
            {"case": "x", "phase": "negative", "status": "PASS", "exit": 1},
            {"case": "x", "phase": "plan", "status": "PASS", "exit": 2},
            {"case": "y", "phase": "create", "status": "PASS", "exit": 0},
            {"case": "y", "phase": "create", "status": "FAIL", "exit": 0},
        ]
        latest = report.latest_records(rows)
        self.assertEqual(len(latest), 3)
        self.assertEqual(latest[("x", "negative")]["status"], "PASS")
        self.assertEqual(latest[("y", "create")]["status"], "FAIL")


class ReportTests(unittest.TestCase):
    def test_real_inventory_gaps_and_safe_configured_only(self):
        repo = Path(__file__).resolve().parents[2]
        scratch = Path(os.environ.get("TMPDIR", str(repo)))
        with tempfile.TemporaryDirectory(dir=scratch) as directory:
            run = Path(directory)
            (run / "inventory").mkdir()
            (run / "inventory/schema.json").write_text(json.dumps({"provider_schemas": {
                "registry.terraform.io/adaptive-scale/adaptive": {"resource_schemas": {
                    family: {"block": {"attributes": {"name": {}, "password": {}}}}
                    for family in ["adaptive_resource", "adaptive_endpoint", "adaptive_authorization",
                                   "adaptive_group", "adaptive_script", "adaptive_schedule", "adaptive_dataprotection"]
                }}}}))
            (run / "manifest.json").write_text(json.dumps({"prefix": "owned", "destruction": "MANUAL HOLD", "token": "DO_NOT_COPY"}))
            fixture = run / "work/integration-postgres"
            fixture.mkdir(parents=True)
            (fixture / "main.tf.json").write_text(json.dumps({"provider": {"adaptive": {"service_token": "DO_NOT_COPY"}}, "resource": {
                "adaptive_resource": {"test": {"name": "owned-postgres", "type": "postgres", "password": "DO_NOT_COPY", "tags": ["changed"]}}}}))
            rows = [{"case": "integration-postgres", "phase": p, "status": "PASS", "exit": 0, "evidence": str(run / "evidence/log.txt")}
                    for p in ["create-apply", "update-apply", "refresh-1", "create-noop"]]
            rows.append({"case": "integration-postgres", "phase": "negative-plan", "status": "PASS", "exit": 1, "evidence": "guard"})
            rows.append({"case": "import-mysql", "phase": "prerequisite", "status": "BLOCKED", "exit": 1, "evidence": str(run / "work/integration-mysql/main.tf.json")})
            (run / "ledger.jsonl").write_text("\n".join(json.dumps(r) for r in rows) + "\n")
            data = report.build_report(repo, run)
            self.assertEqual(data["inventory"]["counts"], {"integrations": 82, "families": 7, "authorization_types": 16, "wire_mappings": 599})
            postgres = next(r for r in data["integrations"] if r["type"] == "postgres")
            self.assertEqual(postgres["coverage"]["create"]["status"], "PASS")
            self.assertEqual(postgres["coverage"]["import"]["status"], "NOT RUN")
            self.assertEqual(next(r for r in data["integrations"] if r["type"] == "mysql")["coverage"]["import"]["status"], "BLOCKED")
            self.assertEqual(postgres["coverage"]["live_enforcement"]["status"], "NOT RUN")
            self.assertEqual(postgres["coverage"]["per_field_mutation"]["status"], "NOT RUN")
            self.assertEqual(next(f for f in data["fixtures"] if f["type"] == "postgres")["attributes"]["password"], "configured-only")
            self.assertEqual(len(data["failures"]), 0)
            self.assertNotIn("DO_NOT_COPY", json.dumps(data))
            self.assertEqual(len(data["integrations"]), 82)
            self.assertEqual(len(data["authorization_types"]), 16)
            self.assertEqual(len(data["families"]), 7)
            self.assertEqual({r["mode"] for r in data["endpoints"]}, {"cli", "client"})
            self.assertEqual(data["runtime"]["sync"]["status"], "NOT RUN")
            self.assertEqual(data["auth_variants"][0]["alternate_modes"], "NOT RUN")
            self.assertEqual(data["auth_variants"][0]["selected_inputs_status"], "inventory-only")
            self.assertEqual(data["summary"]["integration_operations"]["create"], {"PASS": 1, "NOT RUN": 81})
            self.assertEqual(data["inventory"]["case_count"], 82)
            paths = report.write_report(data, run)
            self.assertEqual([p.name for p in paths], ["coverage.json", "COVERAGE.md"])
            self.assertEqual(json.loads(paths[0].read_text())["inventory"]["counts"], data["inventory"]["counts"])
            markdown = paths[1].read_text()
            self.assertIn("configured-only", markdown)
            self.assertIn("MANUAL HOLD", markdown)
            self.assertIn("NOT RUN", markdown)
            self.assertIn("work/integration-postgres/main.tf.json", markdown)
            self.assertNotIn("DO_NOT_COPY", markdown)
            self.assertNotIn("](", markdown)  # Plain evidence paths, not markdown links.
            self.assertEqual(paths[0].stat().st_mode & 0o777, 0o600)
            # Exercise the public CLI on the same non-credential fixture tree.
            import subprocess
            result = subprocess.run([sys.executable, str(Path(__file__).with_name("report.py")), str(run), "--repo", str(repo)], capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("coverage.json", result.stdout)
            # Duplicate case types must fail closed, rather than inflate coverage.
            bad_cases = json.loads((repo / "testing/live/integration-cases.json").read_text())
            bad_cases["cases"].append(bad_cases["cases"][0])
            (run / "inventory/integration-cases.json").write_text(json.dumps(bad_cases))
            with self.assertRaisesRegex(ValueError, "exact unique"):
                report.build_report(repo, run)

    def test_campaign_scope_history_runtime_and_source_only_snapshot(self):
        repo = Path(__file__).resolve().parents[2]
        with tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR", str(repo))) as directory:
            run = Path(directory)
            (run / "inventory").mkdir()
            families = ["adaptive_resource", "adaptive_endpoint", "adaptive_authorization", "adaptive_group",
                        "adaptive_script", "adaptive_schedule", "adaptive_dataprotection"]
            (run / "inventory/schema.json").write_text(json.dumps({"provider_schemas": {
                "registry.terraform.io/adaptive-scale/adaptive": {"resource_schemas": {
                    f: {"block": {"attributes": {"name": {}, "password": {}, "host": {}}}} for f in families}}}}))
            (run / "manifest.json").write_text('{"destruction":"MANUAL HOLD"}')
            # Active work configs must not be read; even malformed bodies are irrelevant.
            (run / "work/auth-postgres-iam-static").mkdir(parents=True)
            (run / "work/auth-postgres-iam-static/main.tf.json").write_text('DO_NOT_COPY malformed secret')
            def row(case, phase, status):
                return dict(case=case, phase=phase, status=status, exit=0,
                            reason="DO_NOT_COPY", arguments=["DO_NOT_COPY"], config_sha256="DO_NOT_COPY")
            rows = [row("integration-postgres", "create-apply", s) for s in ("STARTED", "FAIL", "ACKNOWLEDGED", "STARTED", "PASS")]
            rows += [row("integration-postgres", "update-apply", "FAIL"),
                     row("auth-postgres-iam-static", "update-apply", "PASS"),
                     row("auth-postgres-iam-static", "create-apply", "PASS"),
                     row("endpoint-cli", "runtime-privileged-smoke", "PASS"),
                     row("endpoint-client", "runtime-privileged-smoke", "PASS"),
                     row("endpoint-masked", "create-apply", "FAIL"),
                     row("endpoint-readonly", "create-apply", "FAIL"),
                     row("cohort-auth-cockroachdb", "create", "FAIL"),
                     row("cohort-schedule-custom", "create", "PASS"),
                     row("dataprotection", "create-apply", "PASS"),
                     row("integration-mysql", "create-apply", "STARTED"),
                     row("sync-runtime-postgres", "sync-completion", "BLOCKED"),
                     row("integration-aws", "create-plan", "INCOMPLETE")]
            rows += [dict(row('integration-postgres', 'field-host-' + suffix, 'PASS'),
                          verification='exact-state-and-exact-remote-value')
                     for suffix in ('apply', 'verify', 'noop')]
            rows += [row('import-family-group', 'import', 'PASS')]
            rows += [row("integration-postgres", "field-password-plan", "PASS")]
            (run / "ledger.jsonl").write_text("\n".join(map(json.dumps, rows)) + "\n")
            data = report.build_report(repo, run)
            pg = next(x for x in data["integrations"] if x["type"] == "postgres")
            self.assertEqual(pg["coverage"]["update"]["status"], "FAIL")
            self.assertEqual(pg["coverage"]["per_field_mutation"]["status"], "PASS")
            self.assertEqual(pg['field_mutations'][0].get('verification'), 'exact-state-and-exact-remote-value')
            self.assertEqual(pg["tested_fields"], ["host"])
            self.assertIn("password", pg["untested_fields"])
            self.assertEqual(data['summary'].get('independent_field_mutations', {}).get('attempted'), 1)
            self.assertEqual(data["summary"]["static_cohort"]["size"], 82)
            self.assertEqual(data["summary"]["unresolved_started"], 1)
            self.assertEqual(data["summary"]["historical_failures"], 5)
            self.assertEqual(len(data["historical_failed_creates"]), 4)
            self.assertEqual(data["runtime"]["session"]["status"], "PASS")
            self.assertIn("PostgreSQL only", data["runtime"]["session"]["reason"])
            self.assertEqual(data["runtime"]["basic_password_connection"]["status"], "PASS")
            self.assertEqual(data['runtime']['sync']['status'], 'BLOCKED')
            self.assertEqual(data["runtime"]["authentication"]["status"], "NOT RUN")
            self.assertEqual(data["runtime"]["mask_enforcement"]["status"], "BLOCKED")
            self.assertEqual(data["runtime"]["readonly_enforcement"]["status"], "BLOCKED")
            self.assertEqual(data["masking"]["mask_variants"]["configured_count"], 11)
            self.assertEqual(len(data["schedules"]), 6)
            self.assertEqual(len(data["authorization_types"]), 16)
            self.assertEqual(len(data["summary"]["family_create_results"]), 7)
            self.assertEqual(len(data["summary"]["authorization_case_results"]), 16)
            self.assertEqual(len(data["summary"]["schedule_configuration_results"]), 6)
            self.assertEqual(data["summary"]["static_cohort"]["exact_phases"]["import-noop"], {"NOT RUN": 82})
            self.assertEqual(pg["untested_configured_fields"], sorted(set(pg["case_attributes"]) - {"host"}))
            self.assertEqual(len(data["events"]), len(rows))
            self.assertEqual(next(x for x in data["latest_failure_by_case_phase"] if x["case"] == "integration-postgres" and x["phase"] == "create-apply")["status"], "FAIL")
            self.assertEqual(len(data.get('family_imports', [])), 6)
            self.assertEqual(next(x for x in data['family_imports'] if x['case'] == 'import-family-group')['coverage']['import']['status'], 'PASS')
            self.assertEqual(len(data["auth_mode_cases"]), 19)
            alternate = next(x for x in data["auth_mode_cases"] if x["case"] == "auth-postgres-iam-static")
            self.assertEqual(alternate["coverage"]["update"]["status"], "PASS")
            self.assertEqual(alternate["authentication"], "NOT RUN")
            text = report.render_markdown(data)
            self.assertIn("partial coverage", text)
            self.assertIn("Historical failed creates", text)
            self.assertIn("cohort-auth-cockroachdb", text)
            self.assertNotIn("DO_NOT_COPY", json.dumps(data) + text)
            self.assertNotIn("TF_LIVE_AUTH_DUMMY_ACCESS", json.dumps(data) + text)

    def test_controlled_family_phase_contracts_do_not_imply_enforcement(self):
        rows = [dict(case="cohort-schedule-custom", phase=p, status=s)
                for p, s in (("create", "PASS"), ("update", "PASS"),
                             ("supported-clear", "PASS"), ("refresh-supported-clear", "PASS"),
                             ("noop-supported-clear", "PASS"), ("exact-readback-create", "FAIL"),
                             ("reject-timezone-diagnostic", "PASS"))]
        covered = report.coverage(rows)
        for category in ("create", "update", "clear", "refresh", "noop", "negative_validation"):
            self.assertEqual(covered[category]["status"], "PASS", category)
        self.assertEqual(covered["configuration_assertions"]["status"], "FAIL")
        self.assertEqual(covered["live_enforcement"]["status"], "NOT RUN")
        unknown = report.coverage([dict(case="uncontrolled", phase="create", status="PASS")])
        self.assertEqual(unknown["create"]["status"], "NOT RUN")
        group = report.coverage([dict(case="cohort-group", phase="membership-update", status="FAIL"),
                                 dict(case="cohort-group", phase="exact-readback-post-failed-membership-update", status="PASS")])
        self.assertEqual(group["update"]["status"], "FAIL")
        self.assertEqual(group["configuration_assertions"]["status"], "PASS")

    def test_canonical_family_comparison_and_raw_observations(self):
        self.assertIn('OBSERVED', report.STATUSES)
        rows = [dict(case='import-family-schedule', phase='raw-source-attrs-state', status='OBSERVED'),
                dict(case='import-family-schedule', phase='canonical-source-attrs-state', status='FAIL')]
        self.assertEqual(report.coverage(rows)['configuration_assertions']['status'], 'FAIL')
        rows[1]['status'] = 'PASS'
        self.assertEqual(report.coverage(rows)['configuration_assertions']['status'], 'PASS')
        self.assertEqual(report.coverage(rows[:1])['configuration_assertions']['status'], 'NOT RUN')

    def test_imported_family_reconciliation_and_exact_assertions(self):
        rows = [dict(case='import-family-script', phase=phase, status=status)
                for phase, status in [('import', 'PASS'), ('refresh-1', 'PASS'), ('reconcile-apply', 'PASS'),
                                      ('noop', 'PASS'), ('independent-exact-document', 'PASS'),
                                      ('exact-source-attrs-state', 'FAIL')]]
        data = report.coverage(rows)
        self.assertEqual(data['update']['status'], 'PASS')
        self.assertEqual(data['import']['status'], 'PASS')
        self.assertEqual(data['configuration_assertions']['status'], 'FAIL')

    def test_nested_fields_failure_and_restore_are_scoped(self):
        rows = [dict(case='integration-adaptive_rdp', phase='field-targets-0-password-' + suffix, status='PASS')
                for suffix in ('apply', 'verify', 'noop', 'restore-apply', 'restore-verify', 'restore-noop')]
        fields = report.field_mutations(rows)
        self.assertEqual(len(fields), 1)
        self.assertEqual(fields[0]['field'], 'targets[].password')
        self.assertEqual(fields[0]['status'], 'PASS')
        rows += [dict(case='integration-mongodb36', phase='field-use_tls-' + suffix, status=status)
                 for suffix, status in (('apply', 'PASS'), ('noop', 'PASS'), ('failure', 'FAIL'))]
        self.assertEqual(report.field_mutations(rows)[1]['status'], 'FAIL')

    def test_safe_evidence_and_unknown_phase_does_not_imply_runtime(self):
        run = Path("/private/owned-run")
        self.assertIsNone(report.evidence_path("token=DO_NOT_COPY", run))
        self.assertIsNone(report.evidence_path("../elsewhere/log.txt", run))
        self.assertEqual(report.evidence_path('family-imports/owned-readback.txt', run), 'family-imports/owned-readback.txt')
        self.assertEqual(report.evidence_path("evidence/log.txt", run), "evidence/log.txt")
        covered = report.coverage([{"phase": "readback", "status": "PASS"}, {"phase": "update-apply", "status": "FAIL"}])
        self.assertEqual(covered["update"]["status"], "FAIL")
        self.assertEqual(covered["live_enforcement"]["status"], "NOT RUN")
        self.assertEqual(covered["per_field_mutation"]["status"], "NOT RUN")
        blocked = report.coverage([{"case": "import-postgres", "phase": "prerequisite", "status": "BLOCKED"}])
        self.assertEqual(blocked["import"]["status"], "BLOCKED")


class ContinuationReportTests(unittest.TestCase):
    def fixture(self):
        self.repo = Path(__file__).resolve().parents[2]
        temporary = tempfile.TemporaryDirectory(dir=os.environ['TMPDIR'])
        self.addCleanup(temporary.cleanup)
        self.run_dir = Path(temporary.name).resolve()
        self.root = self.run_dir / 'field-updates/attempts/20260101T000000Z-root'
        self.root.mkdir(parents=True, mode=0o700)
        (self.run_dir / 'inventory').mkdir(mode=0o700)
        families = ['adaptive_resource', 'adaptive_endpoint', 'adaptive_authorization',
                    'adaptive_group', 'adaptive_script', 'adaptive_schedule', 'adaptive_dataprotection']
        self.save(self.run_dir / 'inventory/schema.json', {'provider_schemas': {
            'registry.terraform.io/adaptive-scale/adaptive': {'resource_schemas': {
                f: {'block': {'attributes': {'host': {}, 'password': {}}}} for f in families}}}})
        self.save(self.run_dir / 'manifest.json', {'destruction': 'MANUAL HOLD'})
        self.selection = [dict(case='integration-postgres', field=f, selected=True, reason='DO_NOT_COPY')
                          for f in ('host', 'password')]
        self.save(self.root / 'selection.json', self.selection)
        self.save(self.root / 'results.json', self.raw([
            dict(case='integration-postgres', field='host', status='PASS', attempted=True, restored=True,
                 verification='exact-state-and-exact-remote-value'),
            dict(case='integration-postgres', field='password', status='FAIL', attempted=True, restored=False)]))
        spec = importlib.util.spec_from_file_location('report_test_continuation', Path(__file__).with_name('field_continuation.py'))
        assert spec is not None and spec.loader is not None
        self.driver = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.driver)
        self.fields = self.driver.load_fields()
        rows = [dict(case='integration-postgres', phase='field-password-failure', status='FAIL', exit=1,
                     reason='DO_NOT_COPY')]
        (self.run_dir / 'ledger.jsonl').write_text('\n'.join(map(json.dumps, rows)) + '\n')
        for path in self.run_dir.rglob('*'):
            path.chmod(0o700 if path.is_dir() else 0o600)
        return self.child(self.root, [dict(case='integration-postgres', field='password', status='PASS',
            attempted=True, restored=True, verification='exact-state-and-redacted-remote-readback')])

    def save(self, path, value):
        path.write_text(json.dumps(value))
        path.chmod(0o600)

    def raw(self, rows, fixtures=1):
        from collections import Counter
        return dict(fixtures=fixtures, attempted=sum(r.get('attempted') is True for r in rows),
                    restored=sum(r.get('restored') is True for r in rows),
                    counts=dict(Counter(r['status'] for r in rows)), fields=rows)

    def child(self, parent, rows, name='20260102T000000Z-child'):
        d, f = self.driver, self.fields
        inherited, pins = d.verified_history(f, self.run_dir, parent, self.selection)
        reused, pending = d.coverage_partition(self.selection, inherited)
        out = parent.parent / name
        out.mkdir(mode=0o700)
        raw = self.raw(rows, len({c for c, _ in pending}))
        self.save(out / 'selection.json', self.selection)
        self.save(out / 'results.json', raw)
        for n in ('selection.json', 'results.json'):
            (out / ('source-' + n)).write_bytes((parent / n).read_bytes())
        (out / 'field-continuation.py').write_text("raise AssertionError('archived driver executed')\n")
        (out / 'gate-helper.py').write_text("raise AssertionError('archived helper executed')\n")
        self.save(out / 'provenance.json', dict(selected=len(reused | pending), reused=len(reused),
            pending=len(pending), pending_fixtures=len({c for c, _ in pending}), source_attempt=str(parent),
            source_sha256={n: d.digest(parent / n) for n in ('selection.json', 'results.json')},
            ancestor_sha256=pins, workers=4, start_method='fork',
            driver_sha256=d.digest(out / 'field-continuation.py'), gate_helper_sha256=d.digest(out / 'gate-helper.py')))
        combined = d.combine(self.selection, inherited, rows, parent, out)
        self.save(out / 'combined-coverage.json', combined)
        self.save(out / 'final-receipt.json', dict(checks=1, dispatch_stopped=False, auth_gate_failed=False,
            serving_gate_failed=False, unknown_gate_failed=False, blocked=False, source_unchanged=True,
            exit=0 if combined['accepted'] else 1, immutable_result=str(out / 'results.json'),
            combined_coverage=str(out / 'combined-coverage.json')))
        self.save(self.run_dir / 'field-updates/results.json', dict(raw, mutable_latest=True,
            immutable_result=str(out / 'results.json'), combined_coverage=str(out / 'combined-coverage.json')))
        for path in out.iterdir():
            path.chmod(0o600)
        return out

    def test_cli_does_not_canonicalize_away_symlinked_proof_run(self):
        import subprocess
        self.fixture()
        temporary = tempfile.TemporaryDirectory(dir=os.environ['TMPDIR'])
        self.addCleanup(temporary.cleanup)
        link = Path(temporary.name) / 'alias'
        link.symlink_to(self.run_dir, target_is_directory=True)
        result = subprocess.run([sys.executable, str(Path(__file__).with_name('report.py')),
                                 str(link), '--repo', str(self.repo)], capture_output=True, text=True)
        self.assertEqual(result.returncode, 1)
        self.assertFalse((self.run_dir / 'coverage.json').exists())

    def test_same_second_incomplete_attempt_cannot_hide_behind_uuid_sort_order(self):
        child = self.fixture()
        (child.parent / '20260102T000000Z-aaa').mkdir(mode=0o700)
        with self.assertRaisesRegex(ValueError, 'stale|newer|ambiguous'):
            report.build_report(self.repo, self.run_dir)

    def test_legacy_raw_latest_pointer_without_combined_proof_keeps_ledger_behavior(self):
        import shutil
        child = self.fixture()
        shutil.rmtree(child)
        raw = json.loads((self.root / 'results.json').read_text())
        self.save(self.run_dir / 'field-updates/results.json', dict(raw, mutable_latest=True,
            immutable_result=str(self.root / 'results.json')))
        data = report.build_report(self.repo, self.run_dir)
        self.assertIsNone(data['field_proof'])
        self.assertEqual(data['summary']['independent_field_mutations']['status'], {'FAIL': 1})
        self.assertEqual(data['summary']['historical_failures'], 1)

    def test_reviewed_ancestry_rejects_tampered_missing_and_duplicate_metadata(self):
        import copy
        child = self.fixture()
        controls = [('combined-coverage.json', lambda x: x['fields'][0].update(status='FAIL')),
                    ('final-receipt.json', lambda x: x.update(exit=1)),
                    ('provenance.json', lambda x: x['ancestor_sha256'].clear()),
                    ('selection.json', lambda x: x.append(copy.deepcopy(x[0]))),
                    ('results.json', lambda x: x['fields'].append(copy.deepcopy(x['fields'][0])))]
        for name, mutate in controls:
            path = child / name
            original = path.read_bytes()
            try:
                value = json.loads(original)
                mutate(value)
                self.save(path, value)
                with self.subTest(artifact=name), self.assertRaises((ValueError, OSError)):
                    report.build_report(self.repo, self.run_dir)
            finally:
                path.write_bytes(original)
        for name in ('combined-coverage.json', 'selection.json', 'provenance.json', 'final-receipt.json'):
            path = child / name
            original = path.read_bytes()
            try:
                path.unlink()
                with self.subTest(missing=name), self.assertRaises((ValueError, OSError)):
                    report.build_report(self.repo, self.run_dir)
            finally:
                path.write_bytes(original)
                path.chmod(0o600)
        path = self.run_dir / 'field-updates/results.json'
        original = path.read_text()
        path.write_text(original[:-1] + ', "mutable_latest": true}')
        with self.assertRaisesRegex(ValueError, 'duplicate'):
            report.build_report(self.repo, self.run_dir)

    def test_nested_indices_are_counted_concretely_with_separate_schema_names(self):
        import shutil
        child = self.fixture()
        shutil.rmtree(child)
        cases = json.loads((self.repo / 'testing/live/integration-cases.json').read_text())
        rdp = next(x for x in cases['cases'] if x['type'] == 'adaptive_rdp')
        rdp['attributes']['targets'].append(dict(rdp['attributes']['targets'][0]))
        self.save(self.run_dir / 'inventory/integration-cases.json', cases)
        self.selection = [dict(case='integration-adaptive_rdp', field='targets.' + str(i) + '.password', selected=True)
                          for i in range(2)]
        self.save(self.root / 'selection.json', self.selection)
        rows = [dict(case=x['case'], field=x['field'], status='PASS', attempted=True, restored=True,
                     verification='exact-state-and-exact-remote-value', error='DO_NOT_COPY') for x in self.selection]
        self.save(self.root / 'results.json', self.raw(rows))
        self.child(self.root, [])
        data = report.build_report(self.repo, self.run_dir)
        fields = next(x for x in data['integrations'] if x['type'] == 'adaptive_rdp')['field_mutations']
        self.assertEqual([x['field'] for x in fields], ['targets.0.password', 'targets.1.password'])
        self.assertEqual({x['schema_field'] for x in fields}, {'targets[].password'})
        self.assertEqual(data['summary']['independent_field_mutations']['status'], {'PASS': 2})
        self.assertNotIn('DO_NOT_COPY', json.dumps(data) + report.render_markdown(data))

    def test_new_attempt_after_proof_validation_withholds_report(self):
        from unittest import mock
        child = self.fixture()
        self.assertTrue(report.build_report(self.repo, self.run_dir)['field_proof']['accepted'])
        original = Path.read_text
        def changed(path, *args, **kwargs):
            value = original(path, *args, **kwargs)
            if path == self.run_dir / 'manifest.json':
                (child.parent / '20260103T000000Z-incomplete').mkdir(mode=0o700)
            return value
        with mock.patch.object(Path, 'read_text', changed):
            with self.assertRaisesRegex(ValueError, 'changed.*snapshot'):
                report.build_report(self.repo, self.run_dir)

    def test_pointer_change_after_proof_validation_withholds_report(self):
        from unittest import mock
        self.fixture()
        pointer = self.run_dir / 'field-updates/results.json'
        metadata = json.loads(pointer.read_text())
        self.assertTrue(report.build_report(self.repo, self.run_dir)['field_proof']['accepted'])
        original = Path.read_text
        def changed(path, *args, **kwargs):
            value = original(path, *args, **kwargs)
            if path == self.run_dir / 'manifest.json':
                self.save(pointer, dict(metadata, mutable_latest=False))
            return value
        with mock.patch.object(Path, 'read_text', changed):
            with self.assertRaisesRegex(ValueError, 'changed.*snapshot'):
                report.build_report(self.repo, self.run_dir)

    def test_ancestry_pin_change_after_proof_validation_withholds_report(self):
        from unittest import mock
        self.fixture()
        ancestor = self.root / 'results.json'
        metadata = json.loads(ancestor.read_text())
        self.assertTrue(report.build_report(self.repo, self.run_dir)['field_proof']['accepted'])
        original = Path.read_text
        def changed(path, *args, **kwargs):
            value = original(path, *args, **kwargs)
            if path == self.run_dir / 'manifest.json':
                metadata['fields'][0]['status'] = 'FAIL'
                self.save(ancestor, metadata)
            return value
        with mock.patch.object(Path, 'read_text', changed):
            with self.assertRaisesRegex(ValueError, 'changed.*snapshot'):
                report.build_report(self.repo, self.run_dir)

    def test_writer_starting_after_proof_validation_withholds_report(self):
        from unittest import mock
        self.fixture()
        original = Path.read_text
        def changed(path, *args, **kwargs):
            value = original(path, *args, **kwargs)
            if path == self.run_dir / 'manifest.json':
                (self.run_dir / '.field-updates.lock').touch(mode=0o600)
            return value
        with mock.patch.object(Path, 'read_text', changed):
            with self.assertRaisesRegex(ValueError, 'writer|lock'):
                report.build_report(self.repo, self.run_dir)

    def test_current_family_field_view_and_markdown_distinguish_ledger_diagnostics(self):
        self.fixture()
        data = report.build_report(self.repo, self.run_dir)
        family = next(x for x in data['families'] if x['family'] == 'adaptive_resource')
        self.assertEqual([x['field'] for x in family['field_mutations']], ['host', 'password'])
        self.assertEqual(family['tested_fields'], ['host', 'password'])
        self.assertEqual(family['untested_fields'], [])
        text = report.render_markdown(data)
        self.assertIn('Latest ledger diagnostic failures and blockers', text)
        self.assertIn('not current validated field acceptance', text)
        self.assertIn('immutable names/status/provenance proof allowlist', text)
        self.assertNotIn('No active child files, work configs, state, credentials, CLI-origin documents or evidence bodies were read.', text)

    def test_partial_proof_counts_concrete_attempts_not_old_ledger_passes(self):
        import shutil
        child = self.fixture()
        shutil.rmtree(child)
        self.selection.append(dict(case='integration-postgres', field='port', selected=True))
        self.save(self.root / 'selection.json', self.selection)
        self.child(self.root, [dict(case='integration-postgres', field='password', status='FAIL',
                                 attempted=True, restored=True)])
        rows = [dict(case='integration-postgres', phase='field-' + field + '-' + phase,
                     status='PASS', exit=0) for field in ('password', 'port') for phase in ('apply', 'verify', 'noop')]
        (self.run_dir / 'ledger.jsonl').write_text('\n'.join(map(json.dumps, rows)) + '\n')
        data = report.build_report(self.repo, self.run_dir)
        summary = data['summary']['independent_field_mutations']
        self.assertEqual(summary['attempted'], 2)
        self.assertEqual(summary['selected'], 3)
        self.assertEqual(summary['restored'], 2)
        self.assertEqual(summary['status'], {'PASS': 1, 'FAIL': 1, 'BLOCKED': 1})
        self.assertFalse(data['field_proof']['accepted'])
        pg = next(x for x in data['integrations'] if x['type'] == 'postgres')
        self.assertEqual(pg['tested_fields'], ['host'])
        self.assertEqual(pg['coverage']['per_field_mutation']['status'], 'FAIL')
        self.assertEqual(data['summary']['ledger_field_diagnostics']['status'], {'PASS': 2})

    def test_immutable_selection_must_use_registered_case_and_concrete_field_identifiers(self):
        import shutil
        for change in ('case', 'field'):
            with self.subTest(change=change):
                child = self.fixture()
                shutil.rmtree(child)
                raw = json.loads((self.root / 'results.json').read_text())
                for rows in (self.selection, raw['fields']):
                    for row in rows:
                        if change == 'case':
                            row['case'] = 'integration-unregistered'
                        elif row['field'] == 'password':
                            row['field'] = 'unregistered_password'
                self.save(self.root / 'selection.json', self.selection)
                self.save(self.root / 'results.json', raw)
                retry = dict(raw['fields'][1], status='PASS', attempted=True, restored=True)
                self.child(self.root, [retry])
                with self.assertRaisesRegex(ValueError, 'registered|identifier'):
                    report.build_report(self.repo, self.run_dir)

    def test_writer_locks_prevent_stale_acceptance_without_opening_work_bodies(self):
        from unittest import mock
        self.fixture()
        work = self.run_dir / 'work/integration-postgres'
        work.mkdir(parents=True, mode=0o700)
        work.parent.chmod(0o700)
        for name in ('main.tf.json', 'terraform.tfstate'):
            (work / name).write_text('DO_NOT_COPY malformed')
            (work / name).chmod(0o600)
        original_open = Path.open
        def guarded_open(path, *args, **kwargs):
            if 'work' in path.parts:
                raise AssertionError('active work body read')
            return original_open(path, *args, **kwargs)
        with mock.patch.object(Path, 'open', guarded_open), mock.patch('subprocess.run', side_effect=AssertionError('no API/CLI')):
            self.assertTrue(report.build_report(self.repo, self.run_dir)['field_proof']['accepted'])
            for lock in (self.run_dir / '.field-updates.lock', self.run_dir / '.finalizer.lock', work / '.field-updates.lock'):
                with self.subTest(lock=lock):
                    # Use low-level writes; the read guard deliberately forbids all work opens.
                    fd = os.open(lock, os.O_CREAT | os.O_WRONLY, 0o600)
                    os.close(fd)
                    try:
                        with self.assertRaisesRegex(ValueError, 'writer|lock'):
                            report.build_report(self.repo, self.run_dir)
                    finally:
                        lock.unlink()

    def test_explicit_pointer_is_fail_closed_not_a_fallback_hint(self):
        child = self.fixture()
        pointer = self.run_dir / 'field-updates/results.json'
        original = pointer.read_bytes()
        metadata = json.loads(original)
        changes = [dict(metadata, mutable_latest=False), dict(metadata, attempted=99),
                   dict(metadata, immutable_result=str(self.root / 'results.json')),
                   dict(metadata, combined_coverage=str(child / 'results.json')),
                   dict(metadata, combined_coverage='/outside/combined-coverage.json'),
                   {k: v for k, v in metadata.items() if k != 'combined_coverage'}]
        for value in changes:
            with self.subTest(value=value):
                self.save(pointer, value)
                with self.assertRaises((ValueError, OSError)):
                    report.build_report(self.repo, self.run_dir)
        pointer.write_bytes(original)
        pointer.unlink()
        with self.assertRaises((ValueError, OSError)):
            report.build_report(self.repo, self.run_dir)
        pointer.symlink_to(child / 'results.json')
        with self.assertRaises((ValueError, OSError)):
            report.build_report(self.repo, self.run_dir)
        pointer.unlink()
        pointer.write_bytes(original)
        pointer.chmod(0o600)
        combined = child / 'combined-coverage.json'
        saved = combined.read_bytes()
        combined.unlink()
        combined.symlink_to(child / 'results.json')
        with self.assertRaises((ValueError, OSError)):
            report.build_report(self.repo, self.run_dir)
        combined.unlink()
        combined.write_bytes(saved)
        combined.chmod(0o600)
        newer = child.parent / '20260103T000000Z-incomplete'
        newer.mkdir(mode=0o700)
        with self.assertRaises((ValueError, OSError)):
            report.build_report(self.repo, self.run_dir)

    def test_retry_pass_retains_old_failure_and_origin_verification(self):
        child = self.fixture()
        data = report.build_report(self.repo, self.run_dir)
        pg = next(x for x in data['integrations'] if x['type'] == 'postgres')
        self.assertEqual(pg['coverage']['per_field_mutation']['status'], 'PASS')
        self.assertEqual(data['summary']['independent_field_mutations']['status'], {'PASS': 2})
        self.assertEqual(data['summary']['independent_field_mutations']['passing_verification'], {
            'exact-state-and-exact-remote-value': 1, 'exact-state-and-redacted-remote-readback': 1})
        self.assertEqual([x['attempt'] for x in pg['field_mutations']],
                         [str(self.root.relative_to(self.run_dir)), str(child.relative_to(self.run_dir))])
        self.assertEqual(next(x for x in data['families'] if x['family'] == 'adaptive_resource')
                         ['coverage']['per_field_mutation']['status'], 'PASS')
        self.assertEqual(data['summary']['historical_failures'], 1)
        self.assertEqual(data['failures'][0]['phase'], 'field-password-failure')
        self.assertEqual(data['events'][0]['status'], 'FAIL')
        self.assertEqual(data['summary']['ledger_field_diagnostics']['status'], {'FAIL': 1})
        self.assertIn('ledger-only', data['summary']['ledger_field_diagnostics']['basis'])
        self.assertNotIn('DO_NOT_COPY', json.dumps(data) + report.render_markdown(data))
        self.assertTrue(all(x['status'] == 'NOT RUN' for x in data['wire_mappings']))
        self.assertEqual(data['runtime']['authentication']['status'], 'NOT RUN')


if __name__ == "__main__":
    unittest.main()
