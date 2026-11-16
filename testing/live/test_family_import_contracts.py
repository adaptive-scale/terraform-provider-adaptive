"""Offline behavioral contracts for the real inline family-import harness.

Execute only the selected AST control-flow slices, with synthetic files and
external boundaries replaced. Never execute shell startup, CLI, or Terraform.
"""
import ast
import copy
import datetime
import json
import os
import pathlib
import tempfile
import time
import unittest
from typing import Any


HARNESS = pathlib.Path(__file__).with_name("family-imports.sh")
SOURCE = HARNESS.read_text().split("<<'PY'\n", 1)[1].rsplit("\nPY", 1)[0]
TREE = ast.parse(SOURCE)
LOOP = next(n for n in TREE.body if isinstance(n, ast.For))
BODY = next(n for n in LOOP.body if isinstance(n, ast.Try)).body
MISSING = object()


def execute(nodes, env):
    module = ast.fix_missing_locations(ast.Module(body=nodes, type_ignores=[]))
    exec(compile(module, str(HARNESS), "exec"), env)


class FamilyImportContracts(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="family-import-contract-", dir=os.environ["TMPDIR"])
        self.addCleanup(self.temp.cleanup)
        self.run_dir = pathlib.Path(self.temp.name)
        self.out = self.run_dir / "family-imports"
        self.out.mkdir()
        self.prefix = "tflive-offline"
        self.source_attrs: dict[str, Any] = dict(name=self.prefix + "-endpoint-client",
                                 resource=self.prefix + "-runtime-postgres",
                                 cluster="k8s-local")
        self.source_doc: dict[str, Any] = {"configuration": dict(
            name=self.source_attrs["name"], integration_type="postgres",
            access_type="client", integration_name=self.source_attrs["resource"],
            cluster="k8s-local")}
        self.env: dict[str, Any] = dict(copy=copy, datetime=datetime, json=json, os=os,
                        pathlib=pathlib, time=time, RUN=self.run_dir, OUT=self.out,
                        OWN=self.out / "ownership.jsonl", RESULTS=self.out / "results.jsonl",
                        PREFIX=self.prefix, family="endpoint", kind="endpoints",
                        source="endpoint-client", case="import-family-endpoint", prior=None)
        names = {"save", "append", "rows", "record", "attrs", "state"}
        execute([n for n in TREE.body if isinstance(n, ast.FunctionDef) and n.name in names], self.env)
        absence = self.run_dir / "absence.txt"
        absence.write_text("no such endpoint")
        absence.with_suffix(".stderr").write_text("")
        self.env["cmd"] = lambda *args: (True, absence)
        self.env["export"] = lambda *args: copy.deepcopy(self.source_doc)
        self.schema = {"provider_schemas": {"registry.terraform.io/adaptive-scale/adaptive": {
            "resource_schemas": {"adaptive_endpoint": {"block": {"attributes": {
                "idle_timeout": {"type": "string", "optional": True}
            }}}}}}}
        self.write_schema()

    def write_schema(self):
        inventory = self.run_dir / "inventory"
        inventory.mkdir(exist_ok=True)
        (inventory / "schema.json").write_text(json.dumps(self.schema))

    def fixture(self, case, attrs, state):
        work = self.run_dir / "work" / case
        work.mkdir(parents=True, exist_ok=True)
        (work / "main.tf.json").write_text(json.dumps({
            "resource": {"adaptive_endpoint": {"test": attrs}}}))
        (work / "terraform.tfstate").write_text(json.dumps({"resources": [{
            "type": "adaptive_endpoint", "instances": [{"attributes": state}]}]}))

    def prepare(self, configured: Any = MISSING, exported: Any = "15m"):
        self.source_attrs.pop("idle_timeout", None)
        self.source_doc["configuration"].pop("idle_timeout", None)
        if configured is not MISSING:
            self.source_attrs["idle_timeout"] = configured
        if exported is not MISSING:
            self.source_doc["configuration"]["idle_timeout"] = exported
        self.fixture("endpoint-client", self.source_attrs,
                     dict(self.source_attrs, id="synthetic-source-id"))
        # Run the actual source attrs/export/endpoint preparation, stopping
        # immediately before the document/ownership and CLI-write boundary.
        start = next(i for i, n in enumerate(BODY)
                     if isinstance(n, ast.Assign) and ast.unparse(n.targets[0]) == "a"
                     and "copy.deepcopy" in ast.unparse(n.value))
        stop = next(i for i, n in enumerate(BODY)
                    if isinstance(n, ast.Assign) and ast.unparse(n.targets[0]) == "document")
        execute(BODY[start:stop], self.env)
        return self.env["a"], self.env["doc"]

    def test_absent_config_preserves_exported_idle_timeout_in_clone_attrs(self):
        attrs, doc = self.prepare()
        self.assertIn("idle_timeout", attrs)
        self.assertEqual(attrs["idle_timeout"], "15m")
        self.assertEqual(doc["configuration"]["idle_timeout"], "15m")
        self.assertNotIn("idle_timeout", self.source_attrs)
        self.assertEqual(attrs["name"], self.prefix + "-import-family-endpoint")
        self.assertEqual(self.source_doc["configuration"]["name"], self.source_attrs["name"])

    def test_explicit_configured_timeout_is_never_overridden(self):
        for configured in ("", None, "15m", "1h"):
            with self.subTest(configured=configured):
                attrs, doc = self.prepare(configured=configured)
                self.assertIn("idle_timeout", attrs)
                self.assertEqual(attrs["idle_timeout"], configured)
                self.assertEqual(doc["configuration"]["idle_timeout"], "15m")
                self.assertEqual(self.source_attrs["idle_timeout"], configured)

    def test_empty_or_absent_export_is_not_a_default_timeout(self):
        for exported in (MISSING, ""):
            with self.subTest(exported=exported):
                attrs, doc = self.prepare(exported=exported)
                self.assertNotIn("idle_timeout", attrs)
                self.assertEqual("idle_timeout" in doc["configuration"], exported is not MISSING)
                if exported is not MISSING:
                    self.assertEqual(doc["configuration"]["idle_timeout"], exported)

    def test_nondefault_exported_string_is_preserved_exactly(self):
        attrs, doc = self.prepare(exported="1h")
        self.assertEqual(attrs["idle_timeout"], "1h")
        self.assertEqual(doc["configuration"]["idle_timeout"], "1h")

    def test_malformed_exported_timeout_fails_closed(self):
        for value in (None, False, 15, [], {}, ["15m"]):
            with self.subTest(exported=value):
                with self.assertRaisesRegex((AssertionError, ValueError, TypeError), "idle_timeout"):
                    self.prepare(exported=value)

    def test_exported_timeout_requires_pinned_optional_string_input(self):
        specs = ({}, {"type": "number", "optional": True},
                 {"type": "string", "computed": True},
                 {"type": "string", "optional": True, "computed": True},
                 {"type": "string", "required": True},
                 {"type": "string", "optional": True, "sensitive": True})
        block = self.schema["provider_schemas"]["registry.terraform.io/adaptive-scale/adaptive"][
            "resource_schemas"]["adaptive_endpoint"]["block"]
        for spec in specs:
            with self.subTest(spec=spec):
                block["attributes"]["idle_timeout"] = spec
                self.write_schema()
                with self.assertRaisesRegex((AssertionError, ValueError, TypeError), "idle_timeout"):
                    self.prepare()

    def recover(self, verified=True, noop=True):
        case = self.env["case"]
        document = self.out / "owned.json"
        document.write_text(json.dumps(self.source_doc))
        owned = dict(case=case, id="synthetic-clone-id", stage="verification-failed",
                     failure_stage="independent-readback", reason="historical failure",
                     document=str(document))
        self.env["append"](self.env["OWN"], owned)
        self.env["record"](case, "complete", False)
        self.fixture(case, self.source_attrs, dict(self.source_attrs, id=owned["id"]))
        calls = []
        self.env.update(MODE="verify", FAMILIES=[("endpoint", "endpoints", "endpoint-client")],
                        verify=lambda *args: verified,
                        tf=lambda *args: calls.append(args) or noop)
        # Actual retained recovery branch, including its continue and durable
        # append; never enter create/reconcile or execute top-level startup.
        recovery_loop = copy.deepcopy(LOOP)
        recovery_loop.body = [BODY[0], BODY[1]]
        execute([recovery_loop], self.env)
        return self.env["rows"](self.env["OWN"]), calls

    def test_successful_verify_recovery_appends_complete_ownership(self):
        owned, calls = self.recover()
        self.assertEqual(owned[-1]["stage"], "complete")
        self.assertNotIn("reason", owned[-1])
        self.assertNotIn("failure_stage", owned[-1])
        self.assertEqual(owned[0]["stage"], "verification-failed")
        self.assertEqual(calls, [(self.env["case"], "verify-noop", "plan",
                                  "-input=false", "-detailed-exitcode")])
        results = self.env["rows"](self.env["RESULTS"])
        self.assertEqual([r["status"] for r in results], ["FAIL", "PASS"])
        self.assertEqual(self.env["rows"](self.run_dir / "ledger.jsonl"), results)

    def test_failed_verify_or_noop_does_not_mark_ownership_complete(self):
        for verified, noop in ((False, True), (True, False), (False, False)):
            with self.subTest(verified=verified, noop=noop):
                owned, calls = self.recover(verified, noop)
                self.assertEqual(owned[-1]["stage"], "verification-failed")
                self.assertEqual(len(calls), 1)
                self.assertEqual(self.env["rows"](self.env["RESULTS"])[-1]["status"], "FAIL")


if __name__ == "__main__":
    unittest.main()
