"""Conservative typed comparison for family-import raw Terraform state.

This is NOT export comparison, a drift detector, or proof a clear reached the
backend. Exact independent exports and no-op plans remain mandatory. Only the
observed legacy SDK empty encodings below are equivalent to *explicit* null;
missing attributes, unknowns, booleans/numbers and nonempty values are not.

Policy sources: components/schedule.go (users/teams/endpoints stringList,
expires_at empty string), components/script.go (stateStringMap parameter
metadata). The pinned schema must ALSO declare the exact optional input shape.
Do not infer default semantics from optional=True alone (timezone, for example,
inherits a default). No recursive empty/null normalization is performed.
"""
import json


EMPTY_NULL_INPUTS = {
    "schedule": {"users": ["list", "string"],
                 "teams": ["list", "string"],
                 "endpoints": ["list", "string"], "expires_at": "string"},
    "script": {"parameter_descriptions": ["map", "string"]},
}


def _supported(shape):
    if isinstance(shape, str):
        return shape in ("string", "bool", "number")
    if not isinstance(shape, list) or len(shape) != 2:
        return False
    kind, element = shape
    if kind in ("list", "set", "map"):
        return _supported(element)
    if kind == "object":
        return isinstance(element, dict) and all(_supported(s) for s in element.values())
    if kind == "tuple":
        return isinstance(element, list) and all(_supported(s) for s in element)
    return False


def _typed(value, shape):
    if not _supported(shape):
        return False
    if value is None:
        return True
    if shape == "string":
        return type(value) is str
    if shape == "bool":
        return type(value) is bool
    if shape == "number":
        return type(value) in (int, float)
    if not isinstance(shape, list) or len(shape) != 2:
        return False
    kind, element = shape
    if kind in ("list", "set"):
        return type(value) is list and all(_typed(v, element) for v in value)
    if kind == "map":
        return type(value) is dict and all(type(k) is str and _typed(v, element)
                                           for k, v in value.items())
    if kind == "object":
        return (type(value) is dict and value.keys() == element.keys()
                and all(_typed(value[k], s) for k, s in element.items()))
    if kind == "tuple":
        return (type(value) is list and len(value) == len(element)
                and all(_typed(v, s) for v, s in zip(value, element)))
    return False


def _canonical(value, shape):
    if value is None or not isinstance(shape, list):
        return value
    kind, element = shape
    if kind in ("list", "set", "tuple"):
        values = [_canonical(v, s) for v, s in zip(
            value, element if kind == "tuple" else [element] * len(value))]
        return sorted(values, key=lambda v: json.dumps(v, sort_keys=True)) if kind == "set" else values
    if kind in ("map", "object"):
        return {k: _canonical(v, element[k] if kind == "object" else element)
                for k, v in value.items()}
    return value


def compare_attributes(actual, expected, block, family):
    """Compare configured attributes plus validate every schema-required field.

    Unconfigured computed state is intentionally outside this assertion. Return
    raw-independent canonical values and path-only diagnostics for evidence.
    Inputs are never modified. Fail closed on missing/unsupported schema shapes.
    """
    attributes = block.get("attributes", {})
    keys = set(expected) | {k for k, s in attributes.items() if s.get("required")}
    got, want, differences, equivalences = {}, {}, [], []
    for key in sorted(keys):
        if key in block.get("block_types", {}):
            nested = block["block_types"][key]
            if (key not in actual or key not in expected
                    or nested.get("nesting_mode") != "list"
                    or type(actual[key]) is not list or type(expected[key]) is not list
                    or len(actual[key]) != len(expected[key])):
                differences.append({"path": key, "reason": "nested block shape mismatch"})
                continue
            got[key], want[key] = [], []
            for index, (a, e) in enumerate(zip(actual[key], expected[key])):
                if type(a) is not dict or type(e) is not dict:
                    differences.append({"path": f"{key}[{index}]", "reason": "nested block type mismatch"})
                    continue
                result = compare_attributes(a, e, nested["block"], "")
                got[key].append(result["actual"])
                want[key].append(result["expected"])
                differences.extend({"path": f"{key}[{index}].{d['path']}", "reason": d["reason"]}
                                   for d in result["differences"])
            continue
        spec = attributes.get(key, {})
        shape = spec.get("type")
        if key not in actual or key not in expected:
            differences.append({"path": key, "reason": "missing attribute"})
            continue
        a, e = actual[key], expected[key]
        if not spec or not _typed(a, shape) or not _typed(e, shape):
            differences.append({"path": key, "reason": "schema type mismatch"})
            continue
        if spec.get("required") and (a is None or e is None):
            differences.append({"path": key, "reason": "required attribute is null"})
        permitted = (spec.get("optional") is True and not spec.get("required")
                     and not spec.get("computed") and not spec.get("sensitive")
                     and key in EMPTY_NULL_INPUTS.get(family, {})
                     and shape == EMPTY_NULL_INPUTS[family][key])
        empty = "" if shape == "string" else ([] if isinstance(shape, list) and shape[0] in ("list", "set") else {})
        if permitted and ((a is None and e == empty) or (e is None and a == empty)):
            a = e = empty
            equivalences.append(key)
        got[key], want[key] = _canonical(a, shape), _canonical(e, shape)
        # JSON encoding distinguishes false from 0, including inside collections.
        if json.dumps(got[key], sort_keys=True) != json.dumps(want[key], sort_keys=True):
            differences.append({"path": key, "reason": "value mismatch"})
    return {"comparison": "canonical semantic verification", "equal": not differences,
            "actual": got, "expected": want, "differences": differences,
            "optional_empty_null_equivalences": equivalences}
