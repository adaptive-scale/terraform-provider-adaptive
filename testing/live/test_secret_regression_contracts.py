"""Offline execution of the *actual* assertion code embedded in live probes."""
import copy
import pathlib
import re
import json
import os
import subprocess
import sys
import tempfile
from typing import Any
import unittest

HERE = pathlib.Path(__file__).resolve().parent


def contract(script):
    source = (HERE / script).read_text()
    match = re.search(r"<<'SECRET_CONTRACT_PY'\n(.*?)\nSECRET_CONTRACT_PY", source, re.S)
    if match is None:
        raise AssertionError(f"{script}: executable contract helper is missing")
    namespace: dict[str, Any] = {"__name__": "contract_test"}
    exec(compile(match[1], script, "exec"), namespace)
    return namespace


def state(**attrs):
    return {"resources": [{"mode": "managed", "type": "adaptive_resource", "name": "test",
                           "instances": [{"attributes": dict(id="synthetic-id", name="tflive-offline-fixture", **attrs)}]}]}


class KnownEmptyContracts(unittest.TestCase):
    def test_first_addition_retains_empty_provenance_not_seeded_baseline(self):
        check = contract("empty-baseline-secrets.sh")["check"]
        before = state(password="", applied_digests={}, known_empty_secret_keys=["password"])
        current = copy.deepcopy(before)
        current["resources"][0]["instances"][0]["attributes"]["password"] = "opaque-first-fingerprint"
        self.assertEqual(check("refresh", before, current), [])
        for field, bad in [("applied_digests", {"password": "opaque-first-fingerprint"}),
                           ("known_empty_secret_keys", []), ("password", "")]:
            broken = copy.deepcopy(current)
            broken["resources"][0]["instances"][0]["attributes"][field] = bad
            with self.subTest(field=field):
                self.assertTrue(check("refresh", before, broken))

    def test_independent_remote_readback_detects_rejected_write_or_bad_reconciliation(self):
        check = contract("empty-baseline-secrets.sh")["check"]
        doc = {"configuration": {"name": "tflive-offline-fixture", "type": "postgres",
                                 "password": "tf-live-first-added-canary"}, "userTags": ["initial"]}
        self.assertEqual(check("remote-unchanged", doc, copy.deepcopy(doc)), [])
        bad = copy.deepcopy(doc)
        bad["userTags"] = ["must-not-be-written"]
        self.assertTrue(check("remote-unchanged", doc, bad))
        bad = copy.deepcopy(doc)
        bad["configuration"]["password"] = ""
        self.assertTrue(check("remote-unchanged", doc, bad))
        self.assertEqual(check("remote-clear", doc, bad), [])
        self.assertTrue(check("remote-clear", doc, doc))
        bad["configuration"]["password"] = "tf-live-resupplied-canary"
        self.assertEqual(check("remote-resupply", doc, bad), [])
        self.assertTrue(check("remote-resupply", doc, doc))

    def test_clear_and_resupply_require_correct_per_key_evidence(self):
        check = contract("empty-baseline-secrets.sh")["check"]
        before = state(password="", applied_digests={}, known_empty_secret_keys=["password", "clientKey"])
        self.assertEqual(check("initial", before, before), [])
        for missing in [None, []]:
            broken = copy.deepcopy(before)
            broken["resources"][0]["instances"][0]["attributes"]["known_empty_secret_keys"] = missing
            self.assertTrue(check("initial", broken, broken))
        self.assertEqual(check("clear", before, before), [])
        supplied = state(password="tf-live-resupplied-canary", applied_digests={"password": "verified"},
                         known_empty_secret_keys=["clientKey"])
        self.assertEqual(check("resupply", before, supplied), [])
        bad = copy.deepcopy(supplied)
        bad["resources"][0]["instances"][0]["attributes"]["known_empty_secret_keys"].append("password")
        self.assertTrue(check("resupply", before, bad))
        self.assertTrue(check("clear", before, supplied))

    def test_guard_accepts_sdk_proposed_fields_but_refresh_equality_stays_strict(self):
        check = contract("empty-baseline-secrets.sh")["check"]
        marked = state(password="opaque-first-fingerprint", applied_digests={},
                       known_empty_secret_keys=["password"], tags=["initial"])
        for password in ["", None, "another-fingerprint"]:
            proposed = copy.deepcopy(marked)
            proposed["resources"][0]["instances"][0]["attributes"].update(
                password=password, tags=["initial", "phase=must-not-clear-first-added"])
            with self.subTest(password=password):
                self.assertEqual(check("guard", marked, proposed,
                                       "server-held values for password"), [])
                self.assertTrue(check("stable", marked, proposed))

    def test_guard_preserves_protection_metadata_identity_and_expected_diagnostic(self):
        check = contract("empty-baseline-secrets.sh")["check"]
        marked = state(password="opaque-first-fingerprint", applied_digests={},
                       known_empty_secret_keys=["password"], tags=["initial"])
        self.assertEqual(check("stable", marked, marked), [])
        self.assertEqual(check("guard", marked, marked, "server-held values for password"), [])
        self.assertTrue(check("guard", marked, marked, "unrelated Terraform error"))
        for field, value in [("applied_digests", {"password": "advanced"}),
                             ("known_empty_secret_keys", []), ("id", "other"), ("name", "other")]:
            bad = copy.deepcopy(marked)
            bad["resources"][0]["instances"][0]["attributes"][field] = value
            self.assertTrue(check("stable", marked, bad))
            self.assertTrue(check("guard", marked, bad, "server-held values for password"))

    def test_offline_rejected_write_loop_checks_remote_before_restoring_each_intent(self):
        source = (HERE / "empty-baseline-secrets.sh").read_text()
        helper = source.split('\nROOT=', 1)[0]
        loop = 'for intent in omitted null; do\n' + source.split(
            'for intent in omitted null; do\n', 1)[1].split('\ndone', 1)[0] + '\ndone\n'
        # Only the actual loop/assertions run. TF and independent export are
        # offline doubles; no live CLI, Terraform binary, or API is invoked.
        doubles = r'''
bash() {
 printf '%s\n' "$5" >> "$E/trace"
 case "$5" in
  *-rejected) cp "$E/proposed-state.json" "$STATE" ;;
  *-restore-refresh)
   cmp -s "$E/original-config.json" "$DIR/main.tf.json" || return 97
   cp "$E/refresh-2-state.json" "$STATE" ;;
  *-still-actionable)
   contract refresh "$E/original-state.json" "$STATE" - "$E/offline-actionable.json" "$RUN/ledger.jsonl" "$CASE" offline-actionable ;;
  *-plan) ;;
  *) return 98 ;;
 esac
}
remote_check() {
 printf '%s\n' "$2" >> "$E/trace"
 contract "$1" "$E/added.json" "$E/remote.json" - "$E/$2.json" "$RUN/ledger.jsonl" "$CASE" "$2"
}
'''
        marked = state(password="opaque-first-fingerprint", applied_digests={},
                       known_empty_secret_keys=["password"], tags=["initial"])
        initial = copy.deepcopy(marked)
        initial["resources"][0]["instances"][0]["attributes"]["password"] = ""
        proposed = copy.deepcopy(initial)
        proposed["resources"][0]["instances"][0]["attributes"]["tags"] = ["must-not-clear"]
        remote = {"configuration": {"name": "fixture", "type": "postgres", "password": "canary"},
                  "userTags": ["initial"]}
        for mutation in [None, "password", "userTags"]:
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory(
                    dir=os.environ.get("TMPDIR")) as directory:
                root = pathlib.Path(directory)
                current = copy.deepcopy(remote)
                if mutation == "password":
                    current["configuration"]["password"] = ""
                elif mutation == "userTags":
                    current["userTags"] = ["must-not-clear"]
                config = {"resource": {"adaptive_resource": {"test": {"password": "", "tags": ["initial"]}}}}
                for filename, value in [("original-config.json", config), ("original-state.json", initial),
                                        ("refresh-2-state.json", marked), ("proposed-state.json", proposed),
                                        ("terraform.tfstate", marked), ("added.json", remote), ("remote.json", current)]:
                    (root / filename).write_text(json.dumps(value))
                (root / "guard.log").write_text("server-held values for password")
                (root / "ledger.jsonl").write_text("".join(json.dumps({
                    "case": "offline", "phase": f"first-added-{intent}-rejected",
                    "evidence": str(root / "guard.log")}) + "\n" for intent in ["omitted", "null"]))
                env = dict(os.environ, E=directory, RUN=directory, DIR=directory, CASE="offline",
                           STATE=str(root / "terraform.tfstate"), R="offline-double-only")
                result = subprocess.run(["/bin/bash"], input=helper + '\n' + doubles + loop,
                                        text=True, capture_output=True, env=env)
                self.assertEqual(result.returncode, 0 if mutation is None else 1, result.stderr)
                trace = (root / "trace").read_text().splitlines()
                if mutation is None:
                    for intent in ["omitted", "null"]:
                        self.assertLess(trace.index(f"first-added-{intent}-remote-unchanged"),
                                        trace.index(f"first-added-{intent}-restore-refresh"))
                        self.assertIn(f"first-added-{intent}-still-actionable", trace)
                        self.assertEqual(json.loads((root / f"{intent}-post-error-state.json").read_text()), proposed)
                        self.assertEqual(json.loads((root / f"{intent}-restored-state.json").read_text()), marked)
                else:
                    self.assertEqual(trace[-1], "first-added-omitted-remote-unchanged")
                    self.assertFalse(any("restore-refresh" in item or "first-added-null" in item for item in trace))
                    row = json.loads((root / "ledger.jsonl").read_text().splitlines()[-1])
                    self.assertEqual((row["status"], row["phase"]),
                                     ("FAIL", "first-added-omitted-remote-unchanged"))

    def test_rejected_write_loop_saves_diagnostic_and_restores_actionable_probe(self):
        source = (HERE / "empty-baseline-secrets.sh").read_text()
        loop = source.split('for intent in omitted null; do\n', 1)[1].split('\ndone', 1)[0]
        steps = [
            'cp "$STATE" "$E/$intent-post-error-state.json"',
            'contract guard "$E/refresh-2-state.json" "$E/$intent-post-error-state.json"',
            'remote_check remote-unchanged "first-added-$intent-remote-unchanged"',
            'cp "$E/original-config.json" "$DIR/main.tf.json"',
            '"first-added-$intent-restore-refresh" 0 apply -refresh-only',
            'contract refresh "$E/original-state.json" "$STATE"',
            '"first-added-$intent-still-actionable" 2 plan',
        ]
        for step in steps:
            self.assertIn(step, loop)
        positions = [loop.index(step) for step in steps]
        self.assertEqual(positions, sorted(positions))
        self.assertIn('"$E/$intent-restored-state.json"', loop)


class AssertionLedgerContracts(unittest.TestCase):
    def test_guard_and_remote_malformed_evidence_is_ledgered(self):
        source = (HERE / "empty-baseline-secrets.sh").read_text()
        match = re.search(r"<<'SECRET_CONTRACT_PY'\n(.*?)\nSECRET_CONTRACT_PY", source, re.S)
        assert match is not None
        code = match[1]
        marked = state(password="opaque", applied_digests={}, known_empty_secret_keys=["password"])
        remote = {"configuration": {"name": "fixture", "type": "postgres", "password": "canary"},
                  "userTags": ["initial"]}
        missing = copy.deepcopy(marked)
        del missing["resources"][0]["instances"][0]["attributes"]["applied_digests"]
        with tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR")) as directory:
            root = pathlib.Path(directory)
            for mode, before, after in [
                ("guard", marked, missing), ("guard", marked, {"resources": [None]}),
                ("remote-unchanged", remote, {"configuration": None}),
                ("remote-unchanged", remote, {"configuration": remote["configuration"]}),
            ]:
                with self.subTest(mode=mode, after=after):
                    (root / "before").write_text(json.dumps(before))
                    (root / "after").write_text(json.dumps(after))
                    (root / "log").write_text("server-held values for password")
                    args = [sys.executable, "-", mode, str(root / "before"), str(root / "after"),
                            str(root / "log"), str(root / "result"), str(root / "ledger"), "case", mode]
                    result = subprocess.run(args, input=code, text=True, capture_output=True)
                    self.assertEqual(result.returncode, 1)
                    self.assertEqual(result.stdout + result.stderr, "")
                    self.assertEqual(json.loads((root / "ledger").read_text().splitlines()[-1])["status"], "FAIL")
                    self.assertEqual(json.loads((root / "result").read_text())["failures"],
                                     ["invalid or incomplete assertion evidence"])

    def test_failed_and_malformed_evidence_is_ledgered_before_exit(self):
        for script in ["nested-secrets.sh", "empty-baseline-secrets.sh"]:
            source = (HERE / script).read_text()
            match = re.search(r"<<'SECRET_CONTRACT_PY'\n(.*?)\nSECRET_CONTRACT_PY", source, re.S)
            assert match is not None
            code = match[1]
            with tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR")) as directory:
                root = pathlib.Path(directory)
                before = root / "before.json"
                after = root / "after.json"
                evidence = root / "result.json"
                ledger = root / "ledger.jsonl"
                before.write_text(json.dumps(state(password="tf-live-do-not-print", applied_digests={})))
                for content in [before.read_text(), "{malformed"]:
                    after.write_text(content)
                    args = [sys.executable, "-", "refresh", str(before), str(after), "-",
                            str(evidence), str(ledger), "synthetic-case", "synthetic-assertion"]
                    result = subprocess.run(args, input=code, text=True, capture_output=True)
                    with self.subTest(script=script, content=content):
                        self.assertEqual(result.returncode, 1)
                        self.assertEqual(result.stdout + result.stderr, "")
                        row = json.loads(ledger.read_text().splitlines()[-1])
                        self.assertEqual((row["status"], row["exit"], row["phase"]),
                                         ("FAIL", 1, "synthetic-assertion"))
                        self.assertEqual(row["evidence"], str(evidence))
                        self.assertNotIn("tf-live-do-not-print", evidence.read_text() + ledger.read_text())
                        self.assertEqual(evidence.stat().st_mode & 0o777, 0o600)
                        self.assertEqual(ledger.stat().st_mode & 0o777, 0o600)


class AggregateRDPContracts(unittest.TestCase):
    def test_explicit_resupply_reconciles_without_repeating_warning(self):
        check = contract("nested-secrets.sh")["check"]
        before = state(type="adaptive_rdp", applied_digests={"targets": "old-aggregate"},
                       targets=[{"id": "one", "password": "tf-live-synthetic"}])
        self.assertEqual(check("reconciled", before, before, "No changes."), [])
        self.assertTrue(check("reconciled", before, before,
                              "RDP configuration changed outside this configuration"))
        bad = copy.deepcopy(before)
        bad["resources"][0]["instances"][0]["attributes"]["targets"][0]["password"] = "opaque-marker"
        self.assertTrue(check("reconciled", before, bad, "No changes."))

    def test_exact_aggregate_warning_preserves_targets_and_baseline(self):
        helper = contract("nested-secrets.sh")
        check = helper["check"]
        before = state(type="adaptive_rdp", applied_digests={"targets": "old-aggregate"},
                       targets=[{"id": "one", "password": "tf-live-synthetic"}])
        name = "tflive-offline-fixture"
        log = (f'Warning: resource "{name}": RDP configuration changed outside this configuration\n'
               'The aggregate RDP configuration no longer matches the last-applied fingerprint. '
               'This warning does not attribute the change to any target or credential and does not force a Terraform update. '
               'Explicitly resupply the intended targets configuration to reconcile it.')
        self.assertEqual(check("refresh", before, copy.deepcopy(before), log), [])
        # Terraform wraps diagnostic paragraphs and decorates them with a box.
        self.assertEqual(check("refresh", before, before, log.replace(" ", " \n│ ")), [])
        for broken_log in [log.replace("RDP configuration", "a secret", 1),
                           log.replace("does not force a Terraform update.", ""),
                           log + "\nThe value behind targets[0].password changed."]:
            with self.subTest(log=broken_log):
                self.assertTrue(check("refresh", before, before, broken_log))
        for field, bad in [("applied_digests", {"targets": "new-aggregate"}),
                           ("targets", [{"id": "one", "password": "opaque-marker"}])]:
            broken = copy.deepcopy(before)
            broken["resources"][0]["instances"][0]["attributes"][field] = bad
            with self.subTest(field=field):
                self.assertTrue(check("refresh", before, broken, log))


if __name__ == "__main__":
    unittest.main()
