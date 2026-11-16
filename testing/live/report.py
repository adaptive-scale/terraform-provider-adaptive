#!/usr/bin/env python3
"""Read-only ledger/source-fixture snapshot; never opens active work or credentials."""
import argparse
import collections
import csv
import json
import importlib.util
import sys
import stat
import os
from pathlib import Path
import re

STATUSES = {"PASS", "FAIL", "BLOCKED", "NOT RUN", "SKIP", "STARTED", "ACKNOWLEDGED", "INCOMPLETE", "OBSERVED"}
PHASES = {
    "create": {"create-apply", "create"},
    "update": {"update-apply", "update"},
    "refresh": {"refresh-1", "refresh-2", "import-refresh-1", "import-refresh-2", "imported-refresh-1", "imported-refresh-2"},
    "noop": {"noop", "create-noop", "created-noop", "updated-noop", "refreshed-noop", "import-noop", "final-noop", "refresh-1-noop", "refresh-2-noop"},
    "import": {"import"},
    "live_enforcement": {"live-enforcement"},
    "per_field_mutation": set(),
    "clear": set(),
    "configuration_assertions": set(),
    "negative_validation": set(),
}
PATTERNS = ("weekdays", "weekends", "everyday", "monthly", "specific", "custom")
MASKS = ("name", "email", "ssn", "phone", "cc", "aadhar", "first4", "last4", "redact", "hash", "disable")


def latest_records(rows):
    """Append order wins; ACK is not remediation; a finished START is resolved."""
    latest = {}
    for row in rows:
        key = row["case"], row["phase"]
        if row["status"] == "ACKNOWLEDGED" and key in latest:
            continue
        latest[key] = dict(row, status="INCOMPLETE", event_status="STARTED") if row["status"] == "STARTED" else row
    return latest


def load_json(path):
    return json.loads(path.read_text())


def enum_values(path, symbol):
    text = path.read_text()
    match = re.search(r"\b" + re.escape(symbol) + r"\s*=\s*(?:\[\]string|map\[string\]bool)\s*\{(.*?)\n\s*\}", text, re.S)
    if not match:
        raise ValueError("Cannot find source enum: " + symbol)
    values = re.findall(r'"([^"\n]+)"', re.sub(r"//[^\n]*", "", match.group(1)))
    if len(values) != len(set(values)):
        raise ValueError("Duplicate source enum: " + symbol)
    return sorted(values)


def evidence_path(value, run):
    """Only plain owned paths; never echo arbitrary command/error/reason strings."""
    if not isinstance(value, str) or not re.fullmatch(r"[a-zA-Z0-9_./-]+", value):
        return None
    path = Path(value)
    path = path if path.is_absolute() else run / path
    try:
        relative = path.resolve().relative_to(run.resolve())
    except ValueError:
        return None
    if relative.parts and relative.parts[0] in {"evidence", "work", "inventory", "families", "family-imports", "auth-modes", "field-updates", "runtime", "sync", "destroy"}:
        return str(relative)
    return None


def aggregate(selected):
    statuses = {r["status"] for r in selected}
    status = ("FAIL" if "FAIL" in statuses else "BLOCKED" if "BLOCKED" in statuses
              else "INCOMPLETE" if statuses & {"INCOMPLETE", "STARTED"}
              else "PASS" if statuses == {"PASS"} else "NOT RUN")
    return {"status": status, "evidence": selected}


def field_mutations(records):
    """Apply+verified readback+noop, not plan-only or restore-only, proves a field."""
    groups = collections.defaultdict(list)
    for row in records:
        match = re.fullmatch(r"field-([a-zA-Z0-9_]+|targets-[0-9]+-[a-zA-Z0-9_]+)-(apply|verify|noop|failure|restore-failure)", row["phase"])
        if match:
            # Restore passes are not new mutations. A failed restore still fails
            # its original field; keep nested indices canonical with the schema.
            name = re.sub(r"^targets-[0-9]+-", "targets[].", match[1])
            groups[(row["case"], name)].append(row)
    result = []
    for (case, name), evidence in sorted(groups.items()):
        item = aggregate(evidence)
        if item["status"] == "PASS" and len({r["phase"] for r in evidence}) != 3:
            item["status"] = "INCOMPLETE"
        verification = next((r.get('verification') for r in evidence if r['phase'].endswith('-verify')), None)
        result.append(dict(item, case=case, field=name, verification=verification,
                           scope="provider attribute mutation; redacted readback is not exact credential verification or runtime enforcement"))
    return result


VERIFICATIONS = {'exact-state-and-exact-remote-value', 'exact-state-and-redacted-remote-readback',
                 'exact-state-and-source-defined-omitted-false'}


def proof_json(path):
    def unique(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise ValueError('duplicate proof metadata key')
            result[key] = value
        return result
    return json.loads(path.read_text(), object_pairs_hook=unique)


def settled_field_snapshot(run):
    """Conservative lock/attempt metadata snapshot; lock bodies are not read."""
    if list(run.glob('*.lock')) or list((run / 'work').glob('*/*.lock')):
        raise ValueError('active or interrupted writer lock; field proof withheld')
    attempts = run / 'field-updates/attempts'
    return tuple(sorted(p.name for p in attempts.iterdir())) if attempts.exists() else ()


def combined_field_proof(run, cases):
    """Read only explicit immutable continuation evidence, using trusted code."""
    latest = run / 'field-updates/results.json'
    attempts = run / 'field-updates/attempts'
    if not os.path.lexists(latest) and not os.path.lexists(attempts):
        return None
    # The run's archived drivers/helpers are evidence bytes, never imports.
    path = Path(__file__).absolute().with_name('field_continuation.py')
    for source in (path, path.with_name('field-updates.py')):
        for node in (source, *source.parents):
            info = node.lstat()
            if (not (stat.S_ISREG(info.st_mode) if node == source else stat.S_ISDIR(info.st_mode))
                    or info.st_uid not in {0, os.getuid()} or info.st_mode & 0o022
                    or (node == source and info.st_nlink != 1)):
                raise ValueError('unsafe trusted proof helper')
    spec = importlib.util.spec_from_file_location('report_field_continuation', path)
    if spec is None or spec.loader is None:
        raise ValueError('missing trusted proof helper')
    module = importlib.util.module_from_spec(spec)
    previous = sys.dont_write_bytecode
    sys.dont_write_bytecode = True
    try:
        spec.loader.exec_module(module)
        f = module.load_fields()
    finally:
        sys.dont_write_bytecode = previous
    # Metadata walk only: never open work configurations, states or credentials.
    f.private_tree(run)
    snapshot = settled_field_snapshot(run)
    pointer_digest = module.digest(latest) if latest.exists() else None
    entries = sorted(attempts.iterdir()) if attempts.exists() else []
    continuation_present = any(any(os.path.lexists(p / name) for name in
        ('provenance.json', 'combined-coverage.json', 'final-receipt.json')) for p in entries)
    pointer = proof_json(f.private_path(run, latest)) if os.path.lexists(latest) else {}
    if not isinstance(pointer, dict):
        raise ValueError('invalid latest proof metadata')
    if 'combined_coverage' not in pointer:
        if continuation_present:
            raise ValueError('missing explicit combined proof pointer')
        return None
    combined = f.private_path(run, Path(pointer['combined_coverage']))
    attempt = combined.parent
    if (combined.name != 'combined-coverage.json' or not combined.is_file()
            or attempt.parent != attempts or pointer.get('mutable_latest') is not True
            or pointer.get('immutable_result') != str(attempt / 'results.json')):
        raise ValueError('invalid latest combined proof pointer')
    for entry in entries:
        f.private_path(run, entry)
        if (not entry.is_dir() or not re.fullmatch(r'[0-9]{8}T[0-9]{6}Z-[a-zA-Z0-9_-]+', entry.name)
                or (entry != attempt and entry.name[:16] >= attempt.name[:16])):
            raise ValueError('newer or invalid field attempt; latest proof is stale')
    raw_latest = proof_json(f.private_path(run, attempt / 'results.json'))
    expected = dict(raw_latest, mutable_latest=True, immutable_result=str(attempt / 'results.json'),
                    combined_coverage=str(combined))
    if json.dumps(pointer, sort_keys=True) != json.dumps(expected, sort_keys=True):
        raise ValueError('latest pointer contradicts immutable raw result')
    selection = proof_json(f.private_path(run, attempt / 'selection.json'))
    registered = {('integration-' + c['type'], '.'.join(map(str, path)))
                  for c in cases for path, _ in f.leaves(dict(c['attributes'], name='', type=c['type'], tags=[]))}
    seen = set()
    if not isinstance(selection, list) or not selection:
        raise ValueError('invalid registered field selection')
    for row in selection:
        key = row['case'], row['field']
        if (key in seen or key not in registered or type(row.get('selected')) is not bool
                or not re.fullmatch(r'[a-zA-Z0-9_-]+', key[0])
                or not re.fullmatch(r'[a-zA-Z0-9_]+(?:\.[0-9]+\.[a-zA-Z0-9_]+)?', key[1])):
            raise ValueError('duplicate or unregistered case/field identifier')
        seen.add(key)
    verified, pins = module.verified_history(f, run, attempt, selection)
    origins = {}
    result = []
    for row in verified['fields']:
        origin = Path(row['attempt'])
        if origin not in origins:
            origins[origin] = load_json(f.private_path(run, origin / 'results.json'))['fields']
        raw = next((r for r in origins[origin] if r.get('field') == row['field'] and r['case'] == row['case']), {})
        verification = raw.get('verification')
        result.append(dict(case=row['case'], field=row['field'],
            schema_field=re.sub(r'\.[0-9]+\.', '[].', row['field']), status=row['status'],
            attempted=row['attempted'], restored=row['restored'],
            attempt=str(origin.relative_to(run)), evidence=[],
            verification=verification if verification in VERIFICATIONS else None,
            scope='validated immutable provider-input mutation; not wire/runtime coverage'))
    def check_snapshot():
        if (settled_field_snapshot(run) != snapshot
                or module.digest(f.private_path(run, latest)) != pointer_digest
                or any(module.digest(f.private_path(run, Path(p))) != h for p, h in pins.items())):
            raise ValueError('field proof changed during read-only snapshot')
    check_snapshot()
    return dict(fields=result, accepted=verified['accepted'], _check_snapshot=check_snapshot,
                attempt=str(attempt.relative_to(run)), basis='validated immutable continuation ancestry',
                artifact_sha256={str(Path(k).relative_to(run)): v for k, v in sorted(pins.items())})


def coverage(records, fields=None):
    result = {}
    for category, phases in PHASES.items():
        selected = [r for r in records if r["phase"] in phases
                    and (r["phase"] not in {"create", "update"} or r.get("case", "").startswith("cohort-"))]
        # Controlled family runner labels actual applies create/update, and
        # scopes refresh/noop/readback to its own cohort cases. No generic
        # substring matching for arbitrary cases or evidence bodies.
        cohort_patterns = {"update": r"(?:membership|endpoint-membership)-update",
                           "clear": r"(?:supported|parameters|membership|endpoint-membership)-clear",
                           "refresh": r"refresh-[a-zA-Z0-9_-]+",
                           "noop": r"noop-[a-zA-Z0-9_-]+|reorder-noop|endpoint-reorder-noop",
                           "configuration_assertions": r"exact-readback-[a-zA-Z0-9_-]+|refreshed-state-[a-zA-Z0-9_-]+",
                           "negative_validation": r"reject-[a-zA-Z0-9_-]+-diagnostic"}
        if category in cohort_patterns:
            selected += [r for r in records if r not in selected and r.get("case", "").startswith("cohort-")
                         and re.fullmatch(cohort_patterns[category], r["phase"])]
        imported_phases = {"update": {"reconcile-apply"},
                           "configuration_assertions": {"independent-exact-document", "exact-source-attrs-state", "canonical-source-attrs-state",
                                                        "independent-command-digest", "endpoint-retained-state", "verified", "complete"}}
        if category in imported_phases:
            selected += [r for r in records if r.get('case') in {
                'import-family-' + family for family in ('authorization', 'group', 'schedule', 'script', 'endpoint', 'dataprotection')}
                and r['phase'] in imported_phases[category]]
        if category == "import":
            selected += [r for r in records if r.get("case", "").startswith("import-")
                         and r["phase"] == "prerequisite" and r["status"] == "BLOCKED"]
        result[category] = aggregate((field_mutations(records) if fields is None else fields)
                                     if category == "per_field_mutation" else selected)
    return result


def flatten_keys(attrs, prefix=""):
    keys = set()
    for key, value in attrs.items():
        name = prefix + key
        keys.add(name)
        if isinstance(value, dict):
            keys.update(flatten_keys(value, name + "."))
        elif isinstance(value, list):
            for child in value:
                if isinstance(child, dict):
                    keys.update(flatten_keys(child, name + "[]."))
    return sorted(keys)


def schema_keys(block, prefix=""):
    keys = {prefix + k for k in block.get("attributes", {})}
    for key, child in block.get("block_types", {}).items():
        keys.add(prefix + key)
        keys.update(schema_keys(child["block"], prefix + key + "[]."))
    return sorted(keys)


def build_report(repo, run):
    repo, run = Path(repo), Path(run)
    components = repo / "internal/provider/components"
    types = enum_values(components / "resource.go", "validIntegrationTypes")
    auth = enum_values(components / "authorization.go", "validAuthorizedResourceTypes")
    schema = load_json(run / "inventory/schema.json")
    families = schema["provider_schemas"]["registry.terraform.io/adaptive-scale/adaptive"]["resource_schemas"]
    with (repo / "docs/parity-current-integration-fields.csv").open(newline="") as stream:
        mappings = list(csv.DictReader(stream))
    wire = {(r["Integration"], r["Key"]) for r in mappings}
    counts = {"integrations": len(types), "families": len(families), "authorization_types": len(auth), "wire_mappings": len(wire)}
    expected = {"integrations": 82, "families": 7, "authorization_types": 16, "wire_mappings": 599}
    if counts != expected or len(mappings) != len(wire):
        raise ValueError("Inventory mismatch: " + json.dumps(counts, sort_keys=True))
    if {t for t, _ in wire} != set(types):
        raise ValueError("Wire inventory integration set differs from source enum")
    cases_file = run / "inventory/integration-cases.json"
    if not cases_file.exists():
        cases_file = repo / "testing/live/integration-cases.json"
    cases = load_json(cases_file)["cases"]
    case_types = [c["type"] for c in cases]
    if len(case_types) != len(set(case_types)) or set(case_types) != set(types):
        raise ValueError("Cases must enumerate the exact unique source integration set")
    auth_cases = load_json(repo / "testing/live/auth-modes.json")["cases"]
    if len({c["id"] for c in auth_cases}) != len(auth_cases):
        raise ValueError("Duplicate auth fixture IDs")
    ledger_file = run / "ledger.jsonl"
    rows = []
    # One immutable read gives a current snapshot without waiting for other writers.
    for number, line in enumerate(ledger_file.read_text().splitlines() if ledger_file.exists() else [], 1):
        if not line.strip():
            continue
        try:
            row = json.loads(line)
            if row["status"] not in STATUSES:
                raise ValueError("unknown status")
            for key in ("case", "phase"):
                if not re.fullmatch(r"[a-zA-Z0-9_-]+", row[key]):
                    raise ValueError("unsafe identifier")
            if row.get("exit") is not None and (not isinstance(row["exit"], int) or isinstance(row["exit"], bool)):
                raise ValueError("unsafe exit")
            rows.append(row)
        except (ValueError, KeyError, TypeError) as exc:
            raise ValueError("Invalid ledger record at line " + str(number)) from exc
    latest = latest_records(rows)
    def safe(row):
        verification = row.get('verification')
        allowed = {'exact-state-and-exact-remote-value', 'exact-state-and-redacted-remote-readback', 'exact-state-and-source-defined-omitted-false'}
        return {"case": row["case"], "phase": row["phase"], "status": row["status"],
                "exit": row.get("exit"), "evidence": evidence_path(row.get("evidence"), run),
                'verification': verification if isinstance(verification, str) and verification in allowed else None}
    records = [safe(r) for _, r in sorted(latest.items())]
    history = [safe(r) for r in rows if r["status"] == "FAIL"]
    proof = combined_field_proof(run, cases)
    check_snapshot = None if proof is None else proof.pop('_check_snapshot')
    def selected_fields(names):
        return None if proof is None else [r for r in proof['fields'] if r['case'] in names]
    def grouped(names):
        return coverage([r for r in records if r["case"] in names], selected_fields(names))
    def slug(typ):
        return re.sub(r"[^a-z0-9_-]", "-", typ)
    # Source fixture names/keys only. No work configs, state, or child summaries read.
    fixtures = []
    for case in cases:
        fixtures.append({"case": "integration-" + slug(case["type"]), "family": "adaptive_resource", "type": case["type"],
                         "config": "work/integration-" + slug(case["type"]) + "/main.tf.json",
                         "source": str(cases_file), "scope": "source fixture definition; active config not inspected",
                         "attributes": {k: "configured-only" for k in flatten_keys(case["attributes"])}})
    integrations = []
    for case in sorted(cases, key=lambda c: c["type"]):
        typ = case["type"]
        names = {"integration-" + slug(typ), "import-" + slug(typ)}
        own = [r for r in records if r["case"] in names]
        mutations = field_mutations(own) if proof is None else selected_fields(names)
        tested = sorted({m.get("schema_field", m["field"]) for m in mutations if m["status"] == "PASS"})
        integrations.append({"type": typ, "coverage": grouped(names), "case_attributes": sorted(case["attributes"]),
                             "tested_fields": tested, "field_mutations": mutations,
                             "untested_configured_fields": sorted(set(case["attributes"]) - set(tested)),
                             "untested_fields": sorted(set(schema_keys(families["adaptive_resource"].get("block", {}))) - set(tested)),
                             "field_scope": "partial coverage; not all configured attributes, wire mappings or runtime"})
    auth_modes = [{"case": c["id"], "type": c["type"], "selected_inputs": flatten_keys(c["attributes"]),
                   "coverage": grouped({c["id"]}), "scope": "alternate authmode CONFIG persistence only",
                   "authentication": "NOT RUN"} for c in auth_cases]
    family_names = {
        "adaptive_resource": {f["case"] for f in fixtures} | {"resource-postgres", "runtime-postgres"},
        "adaptive_endpoint": {"endpoint-cli", "endpoint-client", "endpoint-masked", "endpoint-readonly"},
        "adaptive_authorization": {"authorization-postgres"} | {"cohort-auth-" + t for t in auth},
        "adaptive_group": {"group", "cohort-group"},
        "adaptive_schedule": {"schedule-custom"} | {"cohort-schedule-" + p for p in PATTERNS},
        "adaptive_script": {"cohort-script"},
        "adaptive_dataprotection": {"dataprotection"},
    }
    family_imports = [{'family': family, 'case': 'import-family-' + family.removeprefix('adaptive_'),
                       'coverage': grouped({'import-family-' + family.removeprefix('adaptive_')}),
                       'scope': 'Independent Adaptive-origin import; strict null/empty assertions are separate from import success'}
                      for family in sorted(families) if family != 'adaptive_resource']
    for item in family_imports:
        family_names[item['family']].add(item['case'])
    family_rows = [{"family": f, "coverage": grouped(family_names[f]),
                    "schema_attributes": schema_keys(families[f].get("block", {})),
                    "untested_fields": schema_keys(families[f].get("block", {})),
                    "field_scope": "no independent family-field mutation contract inferred from CRUD"}
                   for f in sorted(families)]
    if proof is not None:
        for family in family_rows:
            fields = selected_fields(family_names[family['family']])
            tested = sorted({r['schema_field'] for r in fields if r['status'] == 'PASS'})
            family.update(field_mutations=fields, tested_fields=tested,
                          untested_fields=sorted(set(family['schema_attributes']) - set(tested)),
                          field_scope='validated concrete provider-input mutations only; not wire/runtime coverage')
    authorizations = [{"type": typ, "case": "cohort-auth-" + typ,
                       "coverage": grouped({"cohort-auth-" + typ}), "scope": "configuration only; not authorization enforcement"} for typ in auth]
    schedules = [{"pattern": p, "case": "cohort-schedule-" + p,
                  "configuration": aggregate([r for r in records if r["case"] == "cohort-schedule-" + p and r["phase"] == "fixture"]),
                  "coverage": grouped({"cohort-schedule-" + p}), "scope": "config presence/persistence only; inactive fixtures; not schedule enforcement"} for p in PATTERNS]
    runtime: dict = {name: {"status": "NOT RUN", "reason": "No dedicated runtime assertion in this snapshot"}
               for name in ("sync", "authentication", "session", "basic_password_connection", "credential_resolution", "mask_enforcement", "readonly_enforcement", "jit_approval", "schedule_enforcement", "notifications", "script_execution")}
    smoke = [r for r in records if r["case"] in {"endpoint-cli", "endpoint-client"} and r["phase"] == "runtime-privileged-smoke"]
    if smoke:
        for name in ("session", "basic_password_connection"):
            runtime[name] = dict(aggregate(smoke), reason="PostgreSQL only; privileged CLI/native client basic-password connection smoke; not IAM, masks or readonly enforcement")
    sync = [r for r in records if r['case'] == 'sync-runtime-postgres' and r['phase'] == 'sync-completion']
    if sync:
        runtime['sync'] = dict(aggregate(sync), reason='Resource-scoped scheduling attempt; authoritative completion/metadata verification unavailable. An acknowledgment is not sync success.')
    runtime["authentication"]["reason"] = "Live IAM/alternate-mode authentication NOT RUN; CONFIG persistence is not authentication. Basic-password PostgreSQL connection is separately scoped."
    for name, case in (("mask_enforcement", "endpoint-masked"), ("readonly_enforcement", "endpoint-readonly")):
        prerequisites = [r for r in records if r["case"] == case and r["phase"] in PHASES["create"]]
        if any(r["status"] in {"FAIL", "BLOCKED", "INCOMPLETE"} for r in prerequisites):
            runtime[name] = {"status": "BLOCKED", "reason": "Endpoint readiness/create failed before enforcement assertions; no runtime success claimed", "evidence": prerequisites}
    variants = []
    for typ in types:
        modes = [c for c in auth_modes if c["type"] == typ]
        variants.append({"type": typ, "selected_inputs": sorted(next(c["attributes"] for c in cases if c["type"] == typ)),
                         "selected_inputs_status": "inventory-only", "alternate_modes": aggregate([r for c in modes for r in c["coverage"]["create"]["evidence"]])["status"],
                         "alternate_mode_cases": [c["case"] for c in modes], "scope": "CONFIG persistence only", "authentication": "NOT RUN"})
    manifest = load_json(run / "manifest.json")
    operations = {category: dict(collections.Counter(row["coverage"][category]["status"] for row in integrations)) for category in PHASES}
    exact_phases = {}
    for phase in ("create-apply", "update-apply", "import", "import-noop", "import-refresh-1", "import-refresh-2"):
        values = []
        for typ in types:
            names = {"integration-" + slug(typ), "import-" + slug(typ)}
            values.append(aggregate([r for r in records if r["case"] in names and r["phase"] == phase])["status"])
        exact_phases[phase] = dict(collections.Counter(values))
    all_fields = [item for integration in integrations for item in integration['field_mutations']]
    family_creates = [{"family": f["family"], **f["coverage"]["create"],
                       "by_status": dict(collections.Counter(r["status"] for r in f["coverage"]["create"]["evidence"]))} for f in family_rows]
    ledger_fields = field_mutations(records)
    data = {"field_proof": None if proof is None else {k: v for k, v in proof.items() if k != 'fields'},
            "inventory": {"counts": counts, "case_count": len(cases), "verified": True, "cases_source": str(cases_file),
                          "wire_source": "docs/parity-current-integration-fields.csv", "wire_scope": "top-level static inventory; not live field coverage"},
            "retention": manifest.get("destruction", "MANUAL HOLD"),
            "summary": {"ledger_records": len(rows), "unique_case_phases": len(latest),
                        "latest_by_status": dict(sorted(collections.Counter(r["status"] for r in records).items())),
                        "unresolved_started": sum(r.get("event_status") == "STARTED" for r in latest.values()),
                        "incomplete_case_phases": sum(r["status"] == "INCOMPLETE" for r in records),
                        'ledger_field_diagnostics': {'basis': 'ledger-only latest diagnostic phases; not current validated field acceptance',
                            'status': dict(collections.Counter(x['status'] for x in ledger_fields)),
                            'fields': ledger_fields},
                        'independent_field_mutations': {'attempted': len(all_fields) if proof is None else sum(x['attempted'] for x in all_fields),
                            'selected': len(all_fields),
                            'restored': None if proof is None else sum(x['restored'] for x in all_fields),
                            'basis': 'ledger-only diagnostic phases' if proof is None else proof['basis'],
                            'status': dict(collections.Counter(x['status'] for x in all_fields)),
                            'passing_verification': dict(collections.Counter(x['verification'] or 'unspecified' for x in all_fields if x['status'] == 'PASS')),
                            'scope': 'Partial configured provider-input mutations; redacted readback is not exact credential verification or wire/runtime coverage'},
                        "integration_operations": operations,
                        "static_cohort": {"size": len(cases), "scope": "integration-* and import-* only; excludes authmode/runtime fixtures", "operations": operations, "exact_phases": exact_phases},
                        "family_create_results": family_creates,
                        "authorization_case_results": [{"type": x["type"], "case": x["case"], "create": x["coverage"]["create"]["status"]} for x in authorizations],
                        "schedule_configuration_results": [{"pattern": x["pattern"], "configuration": x["configuration"]["status"], "create": x["coverage"]["create"]["status"], "scope": x["scope"]} for x in schedules],
                        "authorization_cases": {"count": len(authorizations), "create": dict(collections.Counter(x["coverage"]["create"]["status"] for x in authorizations))},
                        "authmode_cases": {"count": len(auth_modes), "create": dict(collections.Counter(x["coverage"]["create"]["status"] for x in auth_modes)), "scope": "CONFIG persistence only; live authentication NOT RUN"},
                        "schedule_patterns": {"count": len(schedules), "scope": "configuration presence only"},
                        "mask_variants": {"count": len(MASKS), "scope": "source-configured only; runtime not verified"},
                        "historical_failures": len(history)},
            "integrations": integrations, "families": family_rows, 'family_imports': family_imports, "authorization_types": authorizations, "schedules": schedules,
            "wire_mappings": [{"type": typ, "wire_key": key, "status": "NOT RUN", "scope": "independent wire-key mapping/runtime assertion not inferred from provider field mutations"} for typ, key in sorted(wire)],
            "endpoints": [{"mode": mode, "coverage": grouped({"endpoint-" + mode})} for mode in ("cli", "client")],
            "auth_variants": variants, "auth_mode_cases": auth_modes, "runtime": runtime,
            "masking": {"configuration": grouped({"dataprotection"}), "live_enforcement": runtime["mask_enforcement"],
                        "mask_variants": {"status": "configured-only", "configured_count": len(MASKS), "names": list(MASKS), "scope": "source fixture definition only; not all-variant runtime verification"}},
            "fixtures": fixtures, "records": records, "events": [safe(r) for r in rows], "historical_failures": history,
            "latest_failure_by_case_phase": list(latest_records(history).values()),
            "historical_failed_creates": [r for r in history if r["phase"] in PHASES["create"]],
            "acknowledgements": [safe(r) for r in rows if r["status"] == "ACKNOWLEDGED"],
            "secret_audit": {"status": "NOT LOADED", "scope": "No child summary/credential bodies read; names-only allowlist required for future summary loading"},
            "failures": [r for r in records if r["status"] == "FAIL"],
            "blocked": [r for r in records if r["status"] == "BLOCKED"]}
    if check_snapshot is not None:
        check_snapshot()
    return data


def render_markdown(data):
    counts, summary = data["inventory"]["counts"], data["summary"]
    lines = ["# Live coverage snapshot", "", "Retention: " + data["retention"], "",
             "Inventory verified: " + ", ".join(f"{k}={v}" for k, v in counts.items()),
             f"Ledger rows: {summary['ledger_records']}; unique latest case+phase: {summary['unique_case_phases']}.",
             "Ledger-only latest diagnostic statuses (not current validated field acceptance): " + json.dumps(summary["latest_by_status"], sort_keys=True) + ".",
             f"Historical FAIL rows (including superseded attempts): {summary['historical_failures']}.",
             f"Unfinished starts: {summary['unresolved_started']}; incomplete case+phases: {summary['incomplete_case_phases']}.", "",
             "Latest is append order per case+phase; finished STARTED events are resolved. ACKNOWLEDGED does not remediate a failure.",
             "PASS honors expected exits, including negative tests and plan exit 2. A category PASS is scoped evidence, not full lifecycle coverage.",
             "Static 82-cohort counts exclude alternate authmode and runtime fixtures. Historical failures are retained, including recovered environmental/readiness failures.",
             "Current field acceptance uses validated concrete origin rows when an explicit combined proof exists; legacy no-proof reports retain apply + verify + noop ledger aggregation. Restored PASS does not establish wire/runtime coverage.",
             "Only source/ledger and the narrow immutable names/status/provenance proof allowlist were read (archived code hashed, never executed). No active work configs, state, credentials or CLI-origin documents were read.",
             "Ledger-only field diagnostic counts (not current validated field acceptance): " + json.dumps({k: v for k, v in summary['ledger_field_diagnostics'].items() if k != 'fields'}, sort_keys=True),
             "", "## Scoped counts", "", json.dumps({k: summary[k] for k in ("static_cohort", "authorization_cases", "authmode_cases", "independent_field_mutations", "schedule_patterns", "mask_variants")}, sort_keys=True), ""]
    categories = list(PHASES)
    for title, key, label in [("Integration types (static baseline only)", "integrations", "type"), ("Families — all seven create results", "families", "family"),
                               ("Authorization configuration cases (all 16)", "authorization_types", "type"), ("Endpoint modes", "endpoints", "mode"),
                               ('Independent Adaptive-origin family imports', 'family_imports', 'family'),
                               ("Independent alternate authmode CONFIG persistence", "auth_mode_cases", "case"), ("Schedule configuration patterns (all six)", "schedules", "pattern")]:
        lines.extend(["## " + title, "", " | ".join([label] + categories), " | ".join(["---"] * (len(categories) + 1))])
        for row in data[key]:
            lines.append(" | ".join([row[label]] + [row["coverage"][category]["status"] for category in categories]))
        lines.append("")
    lines.extend(["## Runtime and variant gaps", "", "Authmode CRUD/readback is CONFIG persistence, not live IAM/alternate authentication. Basic-password PostgreSQL smoke is separate.",
                  "Masks: 11 source-configured variants only: " + ", ".join(data["masking"]["mask_variants"]["names"]) + ". Not runtime verification.",
                  "All six schedule patterns are configuration fixtures; no schedule enforcement established."])
    for name, item in data["runtime"].items():
        lines.append(f"- {name}: {item['status']} — {item['reason']}")
    for item in data["auth_variants"]:
        lines.append(f"- {item['type']}: inventory-only input names: " + ", ".join(item["selected_inputs"]) + f"; alternate CONFIG persistence {item['alternate_modes']}; live alternate authentication NOT RUN")
    for title, rows in (("Latest ledger diagnostic failures and blockers — not current validated field acceptance", data["failures"] + data["blocked"]),
                        ("Historical failed creates (including recovered attempts)", data["historical_failed_creates"]),
                        ("All historical failures (environmental history retained)", data["historical_failures"])):
        lines.extend(["", "## " + title, ""])
        for row in rows:
            lines.append(f"- {row['case']} / {row['phase']}: {row['status']} (exit {row['exit']}); " + (row["evidence"] or "no owned evidence path supplied"))
        if not rows:
            lines.append("None in this snapshot.")
    lines.extend(["", "## Source fixture configurations", "", "Definitions only; active configurations not read. configured-only does not establish creation or authentication."])
    for fixture in data["fixtures"]:
        lines.append(f"- {fixture['case']} ({fixture['family']}): {fixture['config']}; source-configured-only keys: " + ", ".join(fixture["attributes"]))
    lines.extend(["", "## Independent field mutation — partial coverage", ""])
    for row in data["integrations"]:
        lines.append(f"- {row['type']}: tested provider fields: " + (", ".join(row["tested_fields"]) or "none") + "; untested source-configured fields: " + (", ".join(row["untested_configured_fields"]) or "none") + "; untested schema fields: " + ", ".join(row["untested_fields"]))
    lines.extend(["", "## Family schema attribute gaps", "", "Family CRUD does not establish independent field mutation or enforcement."])
    for family in data["families"]:
        lines.append("- " + family["family"] + ": " + ", ".join(family["untested_fields"]))
    lines.extend(["", "## Wire mapping gaps", "", "All entries below: NOT RUN for independent wire-key/runtime assertion. Provider-field tests are not silently promoted to wire-key coverage. Source: " + data["inventory"]["wire_source"]])
    for typ in (row["type"] for row in data["integrations"]):
        lines.append("- " + typ + ": " + ", ".join(row["wire_key"] for row in data["wire_mappings"] if row["type"] == typ))
    lines.extend(["", "## Latest exact phase evidence", "", "case | phase | status | exit | evidence path", "--- | --- | --- | --- | ---"])
    for row in data["records"]:
        lines.append(f"{row['case']} | {row['phase']} | {row['status']} | {row['exit']} | " + (row["evidence"] or "no owned evidence path supplied"))
    return "\n".join(lines) + "\n"


def write_report(data, run):
    paths = [Path(run) / "coverage.json", Path(run) / "COVERAGE.md"]
    contents = [json.dumps(data, indent=2, sort_keys=True) + "\n", render_markdown(data)]
    for path, content in zip(paths, contents):
        flags = os.O_WRONLY | os.O_CREAT | os.O_TRUNC | getattr(os, "O_NOFOLLOW", 0)
        descriptor = os.open(path, flags, 0o600)
        with os.fdopen(descriptor, "w") as stream:
            os.fchmod(stream.fileno(), 0o600)
            stream.write(content)
    return paths


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("run", type=Path, help="Existing private run directory")
    parser.add_argument("--repo", type=Path, default=Path(__file__).resolve().parents[2])
    args = parser.parse_args()
    try:
        data = build_report(args.repo.resolve(), args.run.absolute())
        paths = write_report(data, args.run.absolute())
    except (OSError, ValueError, KeyError, TypeError) as exc:
        parser.exit(1, "Coverage generation failed: " + type(exc).__name__ + "; check inventory/ledger/source fixture structure.\n")
    print("\n".join(str(path) for path in paths))
    print(json.dumps(data["summary"], sort_keys=True))


if __name__ == "__main__":
    main()
