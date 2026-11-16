# Live acceptance findings

Baseline provider: feature/terraform-parity-release at 84c21f28681ea383a3762ecad677b1c050ddb007. The provider binary is pinned by checksum in each private run manifest. Findings below are observations or source-backed diagnoses, not fixes. The application serving localhost may differ from its checked-out source; a source explanation does not establish the running server revision.

## F1: derived usePassword is reported as a held secret

Observed: tags-only updates fail for SSH and all eleven tested managed-device types, even when the synthetic password is explicitly configured. Imported copies fail reconciliation for the same reason. The diagnostic identifies `usePassword` as having no supported credential mapping; the provider fails before the update POST.

Cause in inspected backend source: pkg/configexport/classify.go gives the substring `password` secret hint precedence over its `usepassword` allowlist entry. True is redacted/fingerprinted; false is exempted as a zero boolean. The provider intentionally does not map a derived boolean back to a credential argument (internal/provider/integrations/apply_config.go) and its full-replacement guard correctly rejects unknown held fields (internal/provider/components/secret_update.go).

Affected live types: ssh, paloalto_ngfw, fortinet_ngfw, cisco_ngfw, hpe_switch, aruba_sw, aruba_instant_on, ivanti, fortinet_analyzer, fortinet_manager, juniper_sw, sophos_fw.

Do not waive the provider guard globally or omit actual held passwords to work around this. A backend classifier regression should preserve both boolean values while still redacting password and sshKey; the provider lifecycle regression must use the real backend response shape.

## F2: serverlist creates but cannot update

Observed: create/read/refresh work; tags-only update returns HTTP 500. An independently CLI-created/imported copy also fails Terraform reconciliation.

Source explanation: internal/provider/integrations/serverlist.go serializes no YAML name. Create has a separate request Name, whereas UpdateResource sends the serialized configuration without a separate name. The backend's integration update service requires configuration.name before it saves. This deterministic mismatch explains the symptom in source; the generic live HTTP 500 alone does not uniquely prove that precise server branch executed.

Recommended regression: assert that the serverlist builder emits the configured name; exercise the real backend update service and read back name/hosts/credentials/tags. Do not turn this into an expected-success test by ignoring HTTP 500.

## F3: ProxySQL SSL contract differs across Terraform and CLI routes

Observed: Terraform-created ProxySQL persists sslMode=require. Exporting that run-owned fixture with its synthetic credentials and creating a distinct Adaptive-origin copy through the CLI is rejected: CLI validation accepts verify_identity, verify_ca, required, preferred, disabled, but not require. No origin object was created for that failed case; the orphan plan independently reports it absent.

Related source constraint: provider resource_contract.go permits MySQL-specific spellings only for type=mysql, so blindly substituting required for a ProxySQL input is not an established provider fix. Preserve the rejected input and route-specific diagnostics. This is a compatibility failure, not a reason to normalize an input silently.

## F4: schema-sensitive 1Password credential is visible in default CLI export

Observed with a synthetic canary only: type=1password, service_account remains visible without --include-secrets. That input is sensitive in the provider schema. A follow-up live test on the separate CLI-origin imported fixture omitted service_account during an unrelated update. The update unexpectedly succeeded, and an independent default export confirmed service_account was explicitly empty. The expected rejection and remote-retention assertions are retained as FAIL. Explicitly resupplying the original configured canary restored the remote value; an exact independent restoration assertion and final no-op passed. The failing regression was reproduced twice by unclassified-secrets.sh, with restoration each time. This is a silent credential-clearing risk as well as a redaction gap: absent held-secret metadata cannot be treated as proof that no credential exists.

Ordinary Terraform logs in the scalar audit did not contain configured canaries; legacy sensitive inputs do persist in protected state.

Scope: an authenticated admin export observation, not proof of unauthenticated exposure. Eight other visible schema-sensitive uri inputs in the same audit contained no userinfo/password; those are nonsecret URL instances and must not be mislabeled as eight additional credential leaks. Google service-account JSON is a separately configured contextual-redaction scenario.

## F5: mask-column configuration reorder creates a diff

Observed: reversing otherwise equivalent masked_columns configuration produces exit 2 rather than an empty plan. No reorder-only plan was applied. Scoped/unscoped changes and final column/table/mask removal all succeeded and converged; all eleven mask types were restored for manual inspection.

Nuance: the provider's sameMasks read helper tolerates server ordering differences but does not suppress a desired-configuration list reorder during planning. This is unnecessary-update/contract documentation behavior, not observed data loss.

## F6: native CockroachDB authorization conflicts with Terraform request validation

All sixteen allowed authorization types were attempted. Fifteen completed creation and configuration updates. A scoped native GRANT SELECT for CockroachDB was rejected with HTTP 400 before an ID was returned. The inspected Terraform handler parses these permissions as a YAML mapping, while the integration's native consumer expects SQL. The test retained the native input rather than inventing a YAML substitute that would not establish runtime compatibility. This case's remaining lifecycle is BLOCKED.

## F7: group mutation acknowledgment casing differs from the actual backend response

Live group membership and endpoint updates/clears persisted, but Terraform reported `API mutation lacks its route-specific positive acknowledgement`. Independent exports and refresh established the actual final empty sets; failed pre-refresh state was retained.

Source diagnosis: internal/terraform-client/mutation_acknowledgement.go expects lower-case `status`. The backend sends model.DefaultResponse, whose untagged Go field serializes as `Status` (internal/model/responses.go), from TerraformUpdateTeam. TerraformDeleteTeam uses the same untagged model, so its delete acknowledgment is source-exposed to the same mismatch; actual deletion has deliberately NOT been tested. Read back before retrying any ambiguous mutation. Group member/endpoint configuration reorders also produce diffs because they are TypeList inputs; refresh-order tolerance is not desired-order diff suppression.

## F8: schedule export omits configured expiry

All six schedule patterns created, updated and supported clears persisted. Twelve create/update exact export comparisons failed because the exporter omits expires_at, despite provider refresh confirming that it was stored. Final fixtures are inactive with empty audiences/endpoint mappings and cleared expiry, so final exports and no-op plans converge. Do not erase the intermediate export failures because the final state is clean. Authorization descriptions are likewise absent from that kind's export and were checked through provider refresh instead.

## F9: MongoDB 3.6 false TLS value cannot be confirmed by exact CLI export

The independent field cohort attempted 258 unique mutations across the 69 original integrations that had converged after update. 257 passed; MongoDB 3.6 use_tls=false failed its exact exported-value comparison: Terraform state held false and its plan converged, while the CLI readback supplied null/absence rather than an explicit false. This establishes an independent-readback gap, not proof that TLS remained enabled or a real database connection was unsafe. The original configuration was restored.

All 258 attempted mutations were restored; a parent audit independently confirmed all 69 original configuration files byte-for-byte. Of the passing mutations, 184 had exact remote-value comparisons and 73 had exact state values plus redacted remote readback, not independently retrieved credentials. All eight configured nested RDP target fields were attempted. 223 selection skips and 194 mapped fields outside this configured cohort are explicit; this is not exhaustive mapped-field or credential-validity coverage. Full results and parent-verification.json are retained under field-updates/ in the private run.

## F10: compact Google service-account JSON bypasses nested export redaction

The alternate-auth cohort's Google default exports retained the synthetic private_key canary inside a compact JSON service_account string on both create and update. admin_email was redacted. This is an authenticated, owned-fixture canary observation; no real Google key or external Google authentication was used. Do not interpret the passing configuration lifecycle as secret-redaction acceptance.

Inspected backend source explains the formatting dependency: serviceaccount is an allowed identifier key (pkg/configexport/classify.go:85-86), while redactEmbedded (pkg/configexport/resource.go:182-203) treats a mapping string as embedded only when its trimmed text contains a newline. Compact JSON is a mapping without a newline, so its private_key member is not visited. This explanation is source-backed, not proof of the running server revision. Google omission/clearing was not separately exercised; the reproduced silent-clear finding remains the 1Password case.

The expanded scalar audit covers 101 original/auth fixtures. Its raw matches include six AWS/Keyspaces service-account identifiers in exports and ordinary logs, plus eight URIs without embedded credentials; these are not fourteen additional credential leaks. Read the names-only report's scope. No state file in this audit had group/world permissions.

## Adaptive-origin import verification scope

Independent CLI-origin objects for all six non-resource families were imported, repeatedly refreshed and reconciled, in addition to the 81 successful integration imports. Four passed the final strict source/state/export checks. Schedule and script checks retained strict JSON comparison failures: optional empty users/teams/endpoints/expires_at and parameter_descriptions respectively came back null in raw Terraform state. Independent exported documents and no-op plans passed for those objects. These are raw null-versus-empty verification discrepancies, not evidence of lost nonempty membership or script data. Historical harness/identity-capture failures remain visible; no expected values or source IDs were rewritten to hide them.

## Nested-secret drift is warning-only, unlike scalar password drift

A separate live RDP fleet test rotated a synthetic nested target password through the CLI. Two persisted refreshes retained the old applied-digest baseline and repeated the secret-change warning. The subsequent plan was empty: the nested fingerprint cannot be represented as a scalar credential diff. Explicitly resupplying the configured target credentials with a deliberate metadata update restored the desired password and clean convergence. This scenario was replayed successfully; the rotated canary was not visible in the checked default exports or ordinary Terraform logs. This is a detected-but-not-automatically-actionable limitation, not evidence that Terraform would silently repair nested credentials on an unchanged plan.

A second live scalar-secret case confirmed that lifecycle.ignore_changes blocks an unrelated update when the planned credential is a drift marker rather than the configured value; remote tags stayed unchanged, two persisted refreshes preserved the baseline, and removing the ignore rule allowed explicit remediation. Another live case confirmed the empty-baseline ambiguity: the first externally added password was observed/seeding-only and produced no actionable plan. Subsequent omitted and null password updates were both rejected before mutation; an explicit empty clear and later omission succeeded. Treat an empty baseline as initialization evidence, not proof that no secrets exist.

## Environment and runtime observations

Initial CLI/client endpoint creation retained IDs but failed readiness while the existing k8s-local API URL was unreachable. The user repaired that existing resource. Restarting only the run-owned endpoints then yielded ready pods. Direct CLI SQL and native-client SQL passed against the run-owned PostgreSQL target. The tainted-but-recorded endpoints were recovered without replacement; JIT/policy flags, explicit false/zero/empty clears and rejected transport transitions passed subsequent configuration checks.

The masked endpoint recorded an ID but failed readiness. Policy persistence and a generated authorization_name are not proof of a usable masked session or successful enforcement. Exact failure artifacts and remaining runtime scope are in the private coverage report; do not collapse this into a masking PASS.

## Evidence and teardown

The private run directory contains the append-only ledger, exact plans/configuration snapshots, state, CLI-origin ownership and ID records, and a secret audit that emits field names rather than values. State and plans contain synthetic credentials and must remain private. All remote fixtures are retained. Destroy is a manual hold; no actual deletion has been exercised during the campaign. Syntax/guard tests and scoped destroy plans do not establish remote teardown success.
