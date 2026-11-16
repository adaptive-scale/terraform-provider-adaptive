# Local Adaptive Terraform acceptance game plan

## Scope and hold point

Test the checked-out branch against http://localhost:8080 using the existing Adaptive CLI identity. Pin and record the Git SHA, Terraform version, provider binary checksum, CLI version and target for every run. Do not upgrade tools, switch contexts, change backend code, or alter pre-existing objects. All test objects receive a unique run prefix. Retain fixtures and state for manual inspection. Never run an automatic destroy trap.

This is an exhaustive inventory-driven campaign, not a claim that arbitrary combinations can all succeed. Every registered family, integration type, configurable field, endpoint transport, authentication mode and lifecycle scenario must receive a PASS, FAIL, BLOCKED, UNSUPPORTED or NOT RUN record with evidence. Configuration persistence, actual provisioning, sync and live access/masking enforcement are distinct acceptance tiers.

## Phase 1: discovery and safety

1. Confirm branch, clean baseline, local HTTP service, current CLI context and tools without displaying credentials.
2. Inspect provider registration, schema, integration mappings, backend compatibility and existing tests. Generate machine-readable inventory from the built branch's schema and source mappings; check enumeration totals programmatically.
3. Enumerate local prerequisites using read-only metadata: clusters, existing identities, integration availability, database fixtures and sync routes. Do not copy existing connection secrets or reuse production resources.
4. Establish an isolated run directory outside Git, mode 0700; state, plans, raw private evidence and generated dummy credentials stay there (files 0600). Only sanitized summaries/config templates belong in Git. Disable Terraform/SDK debug logs and shell tracing.
5. Create and syntax-test explicit destroy scripts before any live apply. Destroy must recheck localhost identity, run ownership, state names and full saved destroy plan. It must require an explicit run confirmation and retain evidence. Include import-origin objects and orphan reconciliation; data protection destroy turns masking off but may leave generated authorization/views requiring separate owned-object cleanup.

## Phase 2: baseline tests

Run make test, make build and the complete opt-in real Terraform CLI contract suites against their isolated HTTP fixtures. Preserve test counts and failures. These suites exercise the real engine but are NOT live Adaptive evidence. Capture provider schema from the exact built binary through a run-specific CLI dev override without modifying global Terraform configuration.

## Phase 3: live configuration lifecycle

For every supported integration type and all seven Terraform resource families:

- Validate and save a create plan; inspect ownership and expected actions; apply that exact plan; independently read the exact remote object.
- Repeated no-op plan and refresh-only apply; verify IDs and observations remain stable.
- Update each meaningful field: scalar changes, false/zero, empty string, final list/map/nested-rule removal, set reordering, ownership omission and defaults. Verify payload effects by independent readback, followed by a no-op plan.
- Rename where supported; reject type/binding/transport transitions without mutation where immutable.
- Create separate Adaptive-origin fixtures via the supported CLI/API, read back their IDs, import them into Terraform, refresh twice, establish explicit desired configuration, and test reconciliation. Do not present state-rm/reimport of Terraform-created objects as Adaptive-origin creation coverage.
- Apply out-of-band nonsecret changes to owned fixtures, refresh, detect drift, reconcile, and verify the second plan is empty. Exercise configured ownership versus observed-only values and lifecycle.ignore_changes separately.
- Inspect dependencies across resource, authorization, endpoint, group, script, schedule and data protection. Test direct users and group membership using existing authorized local identities; standalone user CRUD is not registered by this provider and must be reported separately.
- Test invalid enums, unsupported field/type combinations, missing imports, duplicates, unknown identities and unsupported clears in isolated fixtures. Distinguish expected rejection from incidental failure; read back remote state after any ambiguous failure before retrying.

## Phase 4: secret-specific acceptance (required, not optional)

For each mapped credential family and supported authentication variant:

- Create with unique synthetic test credentials; ordinary CLI output must not show them. Verify sensitive schema metadata and record that legacy sensitive values DO persist in local Terraform state/saved plans. No write-only guarantee is implied.
- Read/refresh redacted values without replacing known plaintext with placeholders. Preserve the applied digest baseline across two refresh-only runs.
- Import with no recoverable plaintext; an unrelated update omitting a known held secret must fail BEFORE mutation. Resupply matching credentials and verify success.
- Distinguish omission, JSON null, missing digest, explicit backend empty and explicit configured empty. Only an authoritative supported empty proves clearing; mandatory RDP credentials must reject empty. Null read behavior is mapping-specific: legacy str/GetStr mappings can set the argument to empty while retaining its old digest baseline; presence-aware cloudString mappings preserve the argument. Test these separately, never assume global null preservation.
- Rotate in Terraform, verify readback fingerprints and no-op convergence; rotate out of band, refresh twice, assert actionable drift and remediation without displaying raw credentials or opaque markers as plaintext.
- Test ignore_changes combined with unrelated updates, inactive-auth-mode held credentials, secret resupply mismatch, nested RDP credentials, opaque adaptiveencrypt values, Google account documents, script bodies and warning-only/unmapped fingerprints.
- Test transitions between password/key, static/IAM/IRSA/role/service-account/external-secret modes only where the integration supports them; do not silently drop credentials in the old mode.
- Record backend redaction/fingerprint omissions as compatibility or security gaps, not provider success. Avoid storing raw API responses when classification is uncertain; extract nonsecret assertions and field-presence metadata in memory.
- Test read/error failure preservation in isolated contract tests rather than intentionally disrupting the shared local server.

## Phase 5: readiness, sync and runtime

For integrations with available test dependencies, provision CLI and client endpoints, await actual readiness, exercise approved harmless native-client/CLI queries and validate identity/authorization. Exercise explicit resource/identity metadata sync and record job completion plus expected metadata. For data protection, cover every supported mask type, scoped/unscoped rules and final removals; distinguish policy persistence from actual masked results. External cloud/SaaS/device integrations without owned local targets/credentials remain BLOCKED for connectivity, not PASS. Never contact external systems with made-up credentials or invent test results.

## Phase 6: manual validation handoff and delayed teardown

Produce run manifest, coverage matrix, command/result ledger, sanitized failure reports, final fixture IDs/names, known gaps and exact replay/refresh/destroy commands. Retain current working configurations and complete local state. Run destroy PLAN only, never apply it during this campaign. Actual deletion/recreation scenarios remain NOT RUN pending manual approval unless a separate disposable teardown cohort is explicitly authorized. After the user validates, explicit scoped destroy removes owned objects in dependency order, verifies absence, and inventories any generated policy/view residue. Retain sanitized evidence after teardown.

## Reusability requirements

Keep orchestration shell-first and branch-local. Separate static inventory, scenario definitions, run-private input/state, sanitized reports and cleanup. Support preflight, baseline, schema inventory, create, refresh, import, update, drift, secret checks, report and explicit destroy as resumable stages. Every command records start/end, exit code and expected result. Failure does not erase state or trigger cleanup. Reruns must not overwrite prior evidence or duplicate ambiguously-created remote objects.
