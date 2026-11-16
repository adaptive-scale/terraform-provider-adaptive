"""Offline regression tests; never open a live run or invoke Terraform."""
import copy
import unittest

from family_compare import compare_attributes


class FamilyCompareTests(unittest.TestCase):
    def test_optional_empty_collection_and_string(self):
        block = {"attributes": {
            "users": {"type": ["list", "string"], "optional": True},
            "expires_at": {"type": "string", "optional": True},
        }}
        result = compare_attributes(
            {"users": None, "expires_at": None},
            {"users": [], "expires_at": ""}, block, "schedule")
        self.assertTrue(result["equal"], result)
        self.assertEqual(result["actual"], result["expected"])

    def compare(self, actual, expected, shape, family="schedule", key="users", **flags):
        return compare_attributes({key: actual}, {key: expected},
                                  {"attributes": {key: dict(type=shape, **flags)}}, family)["equal"]

    def test_optional_map_both_directions_without_mutation(self):
        for a, e in [(None, {}), ({}, None)]:
            self.assertTrue(self.compare(a, e, ["map", "string"], "script",
                                         "parameter_descriptions", optional=True))
        actual = {"users": None}
        expected = {"users": []}
        before = copy.deepcopy((actual, expected))
        compare_attributes(actual, expected, {"attributes": {
            "users": {"type": ["list", "string"], "optional": True}}}, "schedule")
        self.assertEqual((actual, expected), before)

    def test_nonempty_loss_and_failed_clear_are_not_equivalent(self):
        for shape, key, family, value, empty in [
            (["list", "string"], "users", "schedule", ["alice"], []),
            ("string", "expires_at", "schedule", "2026-11-05T00:00:00Z", ""),
            (["map", "string"], "parameter_descriptions", "script", {"p": "help"}, {}),
        ]:
            for a, e in [(None, value), (value, None), (value, empty), (empty, value)]:
                with self.subTest(actual=a, expected=e):
                    self.assertFalse(self.compare(a, e, shape, family, key, optional=True))

    def test_required_missing_and_required_nullable_fields_fail(self):
        for shape, empty in [("string", ""), (["list", "string"], []),
                             (["map", "string"], {})]:
            self.assertFalse(self.compare(None, empty, shape, required=True))
            self.assertFalse(self.compare(None, None, shape, required=True))
        block = {"attributes": {"name": {"type": "string", "required": True}}}
        for a, e in [({}, {}), ({}, {"name": "x"}), ({"name": "x"}, {})]:
            self.assertFalse(compare_attributes(a, e, block, "schedule")["equal"])
        self.assertFalse(compare_attributes({}, {"users": []}, {"attributes": {
            "users": {"type": ["list", "string"], "optional": True}}}, "schedule")["equal"])

    def test_false_zero_and_unknowns_are_not_empty_null(self):
        for shape, value in [("bool", False), ("number", 0), ("bool", True),
                             ("number", 1), ("string", {"unknown": True})]:
            self.assertFalse(self.compare(None, value, shape, optional=True))
        self.assertFalse(self.compare(False, 0, "bool", optional=True))
        self.assertFalse(self.compare(False, 0, "number", optional=True))
        self.assertTrue(self.compare(False, False, "bool", optional=True))
        self.assertTrue(self.compare(0, 0, "number", optional=True))

    def test_unapproved_shapes_and_fields_stay_exact(self):
        for flags in [{"required": True}, {"optional": True, "computed": True},
                      {"optional": True, "sensitive": True}, {}]:
            self.assertFalse(self.compare(None, [], ["list", "string"], **flags))
        self.assertFalse(self.compare(None, "", "string", key="timezone", optional=True))
        self.assertFalse(self.compare(None, [], ["set", "string"], optional=True))
        self.assertFalse(self.compare(None, {}, ["map", "number"], "script",
                                      "parameter_descriptions", optional=True))
        self.assertFalse(self.compare(None, [], ["list", "string"], "other", optional=True))
        self.assertFalse(self.compare([None], [""], ["list", "string"], optional=True))
        self.assertFalse(self.compare({"p": None}, {"p": ""}, ["map", "string"],
                                      "script", "parameter_descriptions", optional=True))

    def test_list_order_preserved_set_order_canonicalized(self):
        self.assertFalse(self.compare(["b", "a"], ["a", "b"], ["list", "string"], optional=True))
        self.assertTrue(self.compare(["b", "a"], ["a", "b"], ["set", "string"], optional=True))
        self.assertFalse(self.compare(["a", "b"], ["a"], ["set", "string"], optional=True))

    def test_nested_blocks_validate_required_and_preserve_nonempty(self):
        block = {"block_types": {"masks": {"nesting_mode": "list", "block": {
            "attributes": {"database_name": {"type": "string", "required": True},
                           "masked": {"type": "bool", "optional": True}}}}}}
        desired = {"masks": [{"database_name": "db", "masked": False}]}
        self.assertTrue(compare_attributes(desired, desired, block, "dataprotection")["equal"])
        for actual in [{"masks": []}, {"masks": None},
                       {"masks": [{"database_name": None, "masked": False}]},
                       {"masks": [{"masked": False}]},
                       {"masks": [{"database_name": "db", "masked": None}]}]:
            self.assertFalse(compare_attributes(actual, desired, block, "dataprotection")["equal"])

    def test_shell_verify_keeps_raw_diagnostic_and_exact_export_gate(self):
        import ast
        import json
        import pathlib
        import sys
        import time
        import types
        from typing import Any

        source = pathlib.Path(__file__).with_name("family-imports.sh").read_text()
        program = source.split("<<'PY'\n", 1)[1].rsplit("\nPY", 1)[0]
        tree = ast.parse(program)
        verify = next(n for n in tree.body if isinstance(n, ast.FunctionDef) and n.name == "verify")
        block = {"attributes": {"users": {"type": ["list", "string"], "optional": True}}}
        schema = {"provider_schemas": {"registry.terraform.io/adaptive-scale/adaptive": {
            "resource_schemas": {"adaptive_schedule": {"block": block}}}}}

        class PinnedSchema:
            def __truediv__(self, other):
                return self

            def read_text(self):
                return json.dumps(schema)

        for exact_export in (True, False):
            saved, rows = [], []
            def save(path, value):
                saved.append(value)
                return path
            def record(case, phase, good, evidence="", **extra):
                rows.append(dict(phase=phase, status="PASS" if good else "FAIL", **extra))
                return good
            def equal(case, phase, actual, expected):
                return record(case, phase, actual == expected)
            env: dict[str, Any] = dict(copy=copy, json=json, pathlib=pathlib, sys=sys, time=time,
                       datetime=types.SimpleNamespace(datetime=types.SimpleNamespace(
                           now=lambda zone: types.SimpleNamespace(isoformat=lambda: "offline")),
                           timezone=types.SimpleNamespace(utc=None)),
                       ROOT=pathlib.Path(__file__).resolve().parents[2], RUN=PinnedSchema(),
                       OUT=pathlib.Path("unused"), RESULTS="unused", save=save, record=record,
                       equal=equal, append=lambda path, value: rows.append(value),
                       export=lambda *args: {"configuration": {} if exact_export else {"wrong": True}},
                       state=lambda *args: {"users": None})
            exec(compile(ast.Module(body=[verify], type_ignores=[]), "verify-only", "exec"), env)
            self.assertEqual(env["verify"]("offline", "schedule", "schedules",
                                            {"users": []}, {"configuration": {}}), exact_export)
            raw = next(v for v in saved if v.get("comparison") == "raw structural comparison")
            semantic = next(v for v in saved if v.get("comparison") == "canonical semantic verification")
            self.assertFalse(raw["equal"])
            self.assertTrue(semantic["equal"])
            self.assertTrue(any(r["status"] == "OBSERVED" for r in rows))
            self.assertFalse(any(r["status"] == "FAIL" and r["phase"] != "independent-exact-document"
                                 for r in rows))

    def test_unsupported_schema_null_does_not_pass(self):
        self.assertFalse(self.compare(None, None, "unsupported", optional=True))


if __name__ == "__main__":
    unittest.main()
