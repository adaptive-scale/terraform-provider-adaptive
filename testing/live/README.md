# Reusable localhost Terraform acceptance campaign

Read PLAN.md first. FINDINGS.md preserves the original campaign's observations; REMEDIATION.md distinguishes subsequent source fixes and new-run verification. The seven Terraform families, 82 integration types, 16 authorization types, six schedule patterns and 599 static wire mappings are separate inventories—not interchangeable coverage counts.

## Boundaries

- Target is only http://localhost:8080, using the existing default Adaptive CLI context. Do not export, paste or commit a service token. The provider resolves the CLI login internally.
- Every run gets a persistent private directory under ~/IdeaProjects/.terraform-provider-adaptive-live, a pinned branch-built provider/checksum and run-specific Terraform dev override. Global Terraform installation/configuration is untouched.
- Use only run-prefixed fixtures. All connection credentials in the checked-in case inventory are intentionally synthetic; .invalid hosts are configuration-persistence cases, never cloud/vendor connectivity probes.
- Retain all objects and local state for manual inspection. No campaign stage runs destroy. Normal saved-plan apply rejects delete/replacement actions. Failed endpoint creation can retain a tainted ID: never blindly apply a replacement plan.
- Sensitive arguments are legacy plaintext state/plan inputs, not write-only values. Keep the entire run directory private. The scalar secret audit does not establish perfect redaction of every nested value or every application log.
- No standalone Terraform user resource is registered. User coverage here is existing identity assignment/membership and endpoint policy, not user CRUD.

## Prerequisites

Go from go.mod, Terraform, logged-in Adaptive CLI, jq, Python 3, and Ruby with standard YAML/JSON. Native-client tests additionally need Expect, lsof and psql on PATH. Runtime setup uses an already-working local kind-endpoint-cluster context, Docker's existing kind network and an existing postgres:16-alpine image; it does not pull an image or modify the existing k8s-local integration. Pass another explicitly approved cluster name to runtime-prepare when appropriate.

The runtime PostgreSQL container is new and run-labeled. It is distinct from the Adaptive application's database. Seeding/updates are confined to tf_live_test in that owned container.

## Start a new run

From the repository root:

    bash testing/live/campaign.sh init

The command prints its new absolute run directory. Supply that exact directory to later commands; never rely on an exported shell variable surviving tool/session boundaries. The manifest records SHA, branch, binary checksum, target and Terraform version. The initial Git status is also retained. Do not silently rebuild/replace the pinned binary during a run.

Run stages explicitly, inspecting failures between them:

    bash testing/live/campaign.sh baseline RUN_DIRECTORY
    bash testing/live/campaign.sh resources RUN_DIRECTORY
    bash testing/live/campaign.sh imports RUN_DIRECTORY
    bash testing/live/campaign.sh auth-modes RUN_DIRECTORY
    bash testing/live/campaign.sh field-updates RUN_DIRECTORY
    bash testing/live/campaign.sh secrets RUN_DIRECTORY
    bash testing/live/campaign.sh runtime-prepare RUN_DIRECTORY k8s-local
    bash testing/live/campaign.sh runtime-checks RUN_DIRECTORY
    bash testing/live/campaign.sh runtime-policies RUN_DIRECTORY k8s-local BACKEND_WORKTREE
    bash testing/live/campaign.sh endpoint-policies RUN_DIRECTORY
    bash testing/live/campaign.sh dataprotection RUN_DIRECTORY
    bash testing/live/campaign.sh negative-imports RUN_DIRECTORY
    bash testing/live/campaign.sh families RUN_DIRECTORY /Users/dhaivatdshah/IdeaProjects/inventorize
    bash testing/live/campaign.sh family-imports RUN_DIRECTORY /Users/dhaivatdshah/IdeaProjects/inventorize
    bash testing/live/campaign.sh sync RUN_DIRECTORY

Do not run two mutation stages against the same fixture concurrently. Distinct owned families/auth-mode cohorts can run independently. Additional auth-mode and non-resource CLI-origin import runners carry their own inventories and result manifests; inspect their usage before execution.

Independent per-field mutations are a separate explicit stage, after resources has finished:

    python3 testing/live/field-updates.py RUN_DIRECTORY
    python3 testing/live/field-updates.py RUN_DIRECTORY --execute --workers 4

The first command writes selection/inventory only. Execution changes one safe configured scalar at a time, checks plan/apply/state/export/no-op, and restores the byte-identical original configuration before the next field. It uses Python's standard library plus the existing Ruby YAML/JSON dependency—no private interpreter or pip installation. Failed comparisons return nonzero even if restoration succeeds. Inspect field-updates/results.json, verification-summary.json and mapped-unexercised.json; configured sensitive values with redacted readback are not exact remote-credential verification. Auth-mode/provisioning transitions and unmapped/absent inputs are not silently included as passing mutations.

For an interrupted field attempt, first verify deployment/CLI identity and reconcile unresolved fixtures using the recovery procedure in REMEDIATION.md. Do not start another full mutation campaign or treat failed restoration flags as confirmed remote drift. The reviewed continuation runner can qualify an immutable source attempt locally:

    python3 testing/live/field_continuation.py RUN_DIRECTORY --check --source-attempt IMMUTABLE_ATTEMPT_DIRECTORY

`--check` makes no CLI/API calls or run writes. It recursively validates selection, raw summaries, ancestor hashes/provenance and stored combined coverage, then reports reusable PASS-plus-restored fields and pending fields. Historical FAIL/BLOCKED records remain intact, and each reused positive result keeps its actual originating attempt.

After separately authorized recovery and review, execute only the pending fields:

    python3 testing/live/field_continuation.py RUN_DIRECTORY --execute --source-attempt IMMUTABLE_ATTEMPT_DIRECTORY --gate-helper REVIEWED_GATE_HELPER_PATH

The gate helper must be an operator-reviewed, exact regular file for the intended serving build and saved localhost identity, not an arbitrary downloaded script. The runner pins/archives it before import, retains original case/plan/private-path guards, and uses four isolated worker processes with whole-run/per-case locks and shared ledger/gate state. Authentication or serving-identity failure stops new dispatch and writes a typed final receipt. A new immutable attempt contains raw results, `combined-coverage.json`, ancestor provenance and `final-receipt.json`; complete acceptance requires every selected field to PASS with verified restoration. A prior partial continuation can itself be the next immutable source without repeating its verified fields. Reauthenticate only to the same target when required; never bypass the saved-identity gate or retrieve real withheld credentials.

The resource stage enumerates all 82 types, performs create, exact-name export, repeated refresh/no-op and tags update/no-op. A name-only export establishes existence; the following Terraform no-op independently checks provider-visible convergence. It does not prove all fields or secrets were preserved. Families perform additional structured exported-field comparisons. Read every scope label in the generated coverage report.

The imports stage exports ONLY the run's synthetic integration fixtures with secrets, changes identity, creates separate CLI-origin objects and imports their actual IDs. It does not pretend that forgetting/reimporting a Terraform-created object is Adaptive-origin coverage. Synthetic script-body export is likewise restricted to the exact script authored by the family cohort.

secrets.sh runs once per run: it refuses an existing cohort instead of recreating it. Its live checks include CLI-origin import, held-secret omission rejection, resupply, external rotation, repeated persisted refresh/digest-baseline checks, remediation, explicit clear and omission after a confirmed clear. Null/malformed-read failure shapes that cannot be induced safely against the shared server are covered in isolated engine/contract tests, not reported as live API tests.

Additional live secret probes are explicit scripts:

    bash testing/live/ignore-secrets.sh RUN_DIRECTORY
    bash testing/live/nested-secrets.sh RUN_DIRECTORY
    bash testing/live/empty-baseline-secrets.sh RUN_DIRECTORY
    bash testing/live/unclassified-secrets.sh RUN_DIRECTORY

The nested probe uses the separate imported RDP fixture (after imports); unclassified-secrets.sh uses only the synthetic imported 1Password fixture and preserves any failed omission-safety assertion while explicitly restoring the credential. The empty-baseline probe requires secrets.sh to have finished with its password explicitly cleared and then omitted. With per-key known_empty_secret_keys evidence, a first external addition must stay actionable across two persisted refreshes; an absent legacy baseline without such evidence remains unknown (covered separately by isolated engine tests). Aggregate RDP drift stays warning-only, without attributing a target or forcing an update. The ignore probe retains its own synthetic PostgreSQL fixture.

families.sh also offers complete, verify and report modes. Its historical failures keep the cohort exit nonzero even if final verification passes; inspect latest exact phase records rather than interpreting that exit as a new failure. Schedule fixtures remain inactive and final audiences are empty. Script definitions are not executed by that cohort.

## Runtime acceptance and dependency recovery

runtime-prepare.sh is replay-safe for existing recorded objects: it does not replace them. endpoints.sh only untaints exact run-owned IDs independently observed created after the user repairs their external cluster dependency. It then checks policy toggles, false/zero/empty clears and immutable transport rejection. Restarts are explicit named CLI actions—not automatic bulk retries.

runtime-checks.sh tests harmless aggregate/assertion SQL via Adaptive exec and a separate native psql connection. Run it from a real terminal or with a PTY. Native helpers and their 16 offline fixtures are included in native/; they are byte-for-byte copies of the already-exercised local endpoint-testing helpers. They keep returned credentials in memory, use a private PTY, pass the required application name, detect only the owned listener and clean up only owned local processes. Offline fixture passes are orchestration evidence, not customer acceptance.

A policy saved successfully, a generated authorization name, a ready pod, a successful SQL read and actual masking/denial are distinct checks. A failed authorization/masked endpoint leaves enforcement BLOCKED. Never substitute privileged SELECT success for denial/masking proof.

runtime-policies.sh setup|verify operates only on a fresh source-pinned run (the original retained run is explicitly refused). Its PostgreSQL SELECT policy uses the native schema list shape, binds only the owned tf_live_test.public.customers resource/table and existing CLI identity, and independently checks authorization/endpoint/masking configuration. Existing or tainted attempts are read back, never recreated, untainted or restarted. It performs no SQL; actual allowed/denied/masked runtime assertions remain a separate acceptance gate. The user explicitly kept PostgreSQL mask-runtime code read-only during remediation.

sync.sh makes a single resource-scoped discovery-schedule attempt with classification disabled and watches only that resource's collector pod. It requests scheduling disabled afterward. ACKNOWLEDGED is not read-back verification or sync completion; the CLI exposes no discovery export/status reader. Pod removal also happens on failure. The report must retain completion as BLOCKED unless a separate authoritative metadata result is added.

## Records and reports

    python3 testing/live/report.py RUN_DIRECTORY
    python3 testing/live/secret_audit.py RUN_DIRECTORY

Files include manifest.json, inventory/schema.json, integration cases, ledger.jsonl, work/*/main.tf.json, state, saved plans, configuration snapshots, failed command outputs, CLI-origin ownership/IDs, family results, runtime TSV/JSON and coverage.json/COVERAGE.md. Later commands record STARTED plus terminal status, exact arguments, config snapshot/hash and evidence. Older records predate that refinement and remain retained, not rewritten.

PASS for an expected exit 1/2 is not an error. Semantic rejection assertions are separate. A historical PASS must not conceal a newer FAIL/STARTED. Configured-only fields are not independently mutated/tested fields. Saved HTTP fixture engine tests are actual Terraform CLI tests but not localhost Adaptive backend evidence.

Current retained run for manual inspection:

    /Users/dhaivatdshah/IdeaProjects/.terraform-provider-adaptive-live/20261104T042306Z-96512

For example:

    python3 testing/live/report.py /Users/dhaivatdshah/IdeaProjects/.terraform-provider-adaptive-live/20261104T042306Z-96512

## Delayed explicit teardown

Only after all mutation stages finish and the user validates the fixtures:

    bash testing/live/campaign.sh destroy-plan RUN_DIRECTORY

This records generated masking authorization ownership before policy state is removed, plans CLI-origin orphan reconciliation and produces dependency-ordered Terraform destroy plans. All plans must contain only owned delete/no-op actions and the literal local provider target. Unexpected config files, edited provider settings and wrong confirmation strings are rejected.

Actual deletion is intentionally a separate explicit sequence:

    bash RUN_DIRECTORY/destroy.sh apply RUN_DIRECTORY EXACT_MANIFEST_PREFIX
    bash RUN_DIRECTORY/destroy-orphans.sh apply RUN_DIRECTORY EXACT_MANIFEST_PREFIX
    bash testing/live/destroy-family-orphans.sh apply RUN_DIRECTORY EXACT_MANIFEST_PREFIX
    bash RUN_DIRECTORY/destroy-generated.sh apply RUN_DIRECTORY EXACT_MANIFEST_PREFIX
    bash RUN_DIRECTORY/destroy-infra.sh RUN_DIRECTORY EXACT_MANIFEST_PREFIX

When present, the separately supplied family-origin orphan cleanup stage also needs to be planned/applied before declaring teardown complete. Inspect its ownership ledger; do not delete state-managed objects twice.

Read the manifest for the exact confirmation prefix. Current retained prefix is tflive-20261104042308. Do not use a different run's prefix or infer an ID from a name. A cleanup acknowledgment is provisional: keep evidence and verify exact remote absence. The group response-casing mismatch is also source-exposed on delete; Terraform destroy may error after deletion. Stop, inspect its evidence and refresh the exact owned target before retrying. Do not weaken acknowledgement checks or blindly force-delete unrelated objects.

Data protection destroy turns masking off; it does not remove generated authorization/views. Generated cleanup is scoped to names derived from the owned policy binding and separately confirmed. The isolated target's remaining views disappear only with its later owned-container removal. Infrastructure cleanup refuses while integration/endpoint/policy state remains and checks the Docker ownership label.

Actual remote deletion has NOT been exercised during this campaign, by design. Syntax checks, negative safety tests and destroy plans do not establish that teardown will pass every backend contract.

## Local harness verification

To run the full Python suite, set `INVENTORIZE_CHECKOUT` to the root of the
Inventorize checkout under test (the directory containing `inventorize-app`):

    INVENTORIZE_CHECKOUT=/path/to/inventorize python3 -m unittest discover -s testing/live -p 'test_*.py'

The PostgreSQL YAML contract test reads the actual backend source and uses the
backend's Go toolchain and vendored dependencies. It does not need a running
backend. Without `INVENTORIZE_CHECKOUT`, only that test is skipped; an invalid
explicit checkout fails with a diagnostic. Run the provider-only suite and the
other local checks with:

    python3 -m unittest discover -s testing/live -p 'test_*.py'
    go test ./testing/live/fixturetool
    bash testing/live/native/test-check-postgres-client.sh
    bash testing/live/test-safety.sh RUN_DIRECTORY
    make test
    make build

Use the provided scratch directory through TMPDIR when running under Hermes. Never publish the run directory, state, saved plan JSON or raw secret-bearing fixture documents.
