# Terraform–Pulumi parity remediation plan

## Decision and scope

The providers have matching integration/resource/route breadth, but **are not yet at functional parity**. Keep the existing lifecycle stack as the starting point; close state/credential-safety gaps before expanding field coverage. Treat the backend contract—not either provider's implementation—as the authority when the providers disagree.

This document is the staged remediation roadmap. The first provider-only implementation step is implemented and locally verified on `feature/terraform-parity-hardening`, based on `fix/secret-drift-visible`; the complete roadmap is not implemented. No live workspace operations have been performed.

## First implementation step

Implemented scope: malformed successful-read identity validation across all seven families; nested resource decode validation before state mutation; durable last-write secret baselines and explicit-clear pruning; a fail-closed preflight guard for known held credentials; five additional sensitive resource arguments and sensitive provider token; independent SSH password/key mapping; endpoint maximum JIT request duration; updated generated documentation.

Verification complete for this step: regression reproductions failed before fixes and pass afterward; `make test`, the full race-enabled Go suite, build, vet, formatting, documentation generation and diff checks pass. The real Terraform CLI suite passed against a loopback fake API, including persisted repeated refresh, saved-plan reconciliation, rejected imported-credential omission, explicit clear, omit-after-clear convergence and safe rejection of a tag update after an ignored credential rotates. Independent review findings are closed: planned/configured disagreement cannot submit a fingerprint as a credential; conditional IAM builders cannot silently discard a held credential; existing normalization is allowed only for explicitly tested integration/YAML-key pairs, not arbitrary passwords or whitespace-only implicit clears. Final independent re-review passed with no security concerns, logic errors or suggestions. The engine CI job is configured and structurally validated; remote CI and live backend/endpoint behavior have not been exercised.

Deferred: native write-only input architecture and backend preserve/replace/delete semantics, atomic write-response fingerprints, comprehensive field expansion, endpoint security-ownership corrections, schedule/backend changes and the remaining roadmap packages. Existing sensitive arguments still persist plaintext in Terraform state. The field CSV below is the original pinned audit inventory, not regenerated current-branch schema output.

## Second implementation step — canonical integration contracts

Branch `feature/terraform-parity-mappings` is stacked on first-step commit `01bfc3c` (`feature/terraform-parity-hardening`). This step is implemented and locally verified; it does not change the backend or Pulumi checkout.

- ClickHouse now writes/reads canonical `database` and `use-tls` keys. Nonempty legacy `ssl_mode` is rejected with `use_tls` migration guidance during planning and before HTTP writes; PostgreSQL-style modes are not silently collapsed into a boolean.
- The existing shared `use_tls` argument is optional/computed for ClickHouse and MongoDB 3.6. Omission preserves observed TLS, including imported TLS-enabled resources; explicit false still disables it. Removing the argument no longer requests a reset to false. This shared-field ownership change is intentional and requires engine regressions for both types.
- Aruba Instant On uses canonical hostname, browser-login and SSH fields. Existing Terraform `host` and alternative `hostname` ownership are handled explicitly; managed refresh without raw configuration must not move the configured alias. Unsupported API-token input and conflicting aliases are rejected rather than dropped. Browser automation expansion is deferred.
- Redshift forwards and refreshes `sslMode`. This fixes provider payload parity, not every backend runtime: the proxy path honors the field while other connection-string paths still require TLS with a hardcoded `require` mode.
- Read materializes an empty computed `applied_digests` map so empty-credential imports do not propose a write solely to initialize that output. Existing nonempty baselines still survive missing metadata; this state-shape fix does not establish backend digest completeness or prove that no credentials exist.

Verification passed: independent backend-shaped wire fixtures; explicit false/empty and missing/withheld cases; real Terraform CLI TLS-enabled imports with clean first plans, unrelated updates, explicit disable and external drift/restore for ClickHouse and MongoDB 3.6; both managed Aruba aliases with repeated clean plans; Redshift SSL metadata/update convergence; plan-time rejection without credential-canary disclosure. The mapping suite failed against the safely built first-step baseline, and its empty-baseline import failures were separately reproduced before the Read fix. `make test`, full race-enabled Go tests with both CLI suites opted in, build, vet, formatting, documentation generation, workflow structure and diff checks pass. Independent review passed with no security or logic blockers. Nonblocking follow-up coverage: switching an established Aruba resource between aliases, and removing an explicit `use_tls` argument from an existing TLS-enabled resource. Remote CI and live connections were not exercised. The pinned field CSV remains the original audit inventory.

## Third implementation step — endpoint policy ownership

Branch `feature/terraform-parity-endpoint-ownership` is stacked on second-step commit `ff78c24`. Implementation, full local verification and independent review are complete. The backend and Pulumi remain unchanged.

- Endpoint-local policy controls and approvers use Optional+Computed observation without false/zero defaults. Omission or removal relinquishes management and preserves remote policy; explicit false/zero/empty/list-empty retains the backend-supported clear semantics. Full-replacement bool/list fields and pointer-preserving fields are serialized differently.
- Refresh observes both directions of boolean changes and zero-to-nonzero quorum changes. Workspace-derived cluster/CPU/memory compatibility remains separate from endpoint-local flags and effective workspace-limited behavior.
- `jit_mode` only sends a configured nonempty planned value. Empty is unmanaged/preserve, not a reset. Duration empty is an explicit clear. Real CLI tests cover a clear alone scheduling an update, saved-plan application, acknowledged empty state, and an ignored clear not overriding lifecycle intent; raw-plan handling avoids SDKv2 computed-empty-string fallback to prior state.
- Private JSON-presence metadata distinguishes missing/null scalar policies from authoritative false/zero. Complete resource/type and full-replacement policy fields are required before state hydration or update mutation; a present null approver slice is an authoritative empty list. Optional pointer-field absence retains prior observations. Empty mode/duration omissions follow the supported backend's explicit omitempty contract, not general version/capability negotiation.
- Updates reject unsupported type transitions before mutation and verify the exact tracked object's requested policy after HTTP success, retaining the original ID on mismatched response IDs or unconfirmed/partial writes. This does not make backend multi-step updates atomic or eliminate concurrent-writer races. Create/provisioning readiness and live enforcement remain separate.
- Supported endpoint wire types are `cli` and `client`; Terraform empty/`direct` aliases `cli`. Unsupported `services` is now rejected rather than sent to a backend that cannot accept it. This is also a Pulumi follow-up finding, not a reason to copy its advertised type. Actual type transitions require explicit migration, not automatic destruction.
- Endpoint documentation now states omission/reset and backend-read compatibility requirements, corrects invalid type/CPU/TTL examples, and checks documented literal inputs against the actual schema.

Verification passed: source-confirmed handler/service/read contracts; observed failing direct regressions before fixes; real Terraform CLI 1.14.3 coverage for import preservation, explicit false drift, pointer omission across an injected concurrent change, standalone duration clear and ignored clear, explicit clears/removal, JIT mode semantics, aliases/type rejection, incomplete-read state preservation, and 200-but-ignored policy failure. `make test`, the full race-enabled Go suite with all three CLI suites opted in, build, vet, formatting, documentation generation, workflow structure and diff checks pass. Independent review passed with no security or logic blockers. Its nonblocking follow-up suggestion is additional post-POST 404/incomplete-read regression coverage; refresh/preflight incompleteness and ignored-write postconditions are already covered. Remote CI, real endpoints, workspace policy, connections and live enforcement were not exercised.

## Fourth implementation step — cloud and application-key fields

Branch `feature/terraform-parity-cloud-fields` is stacked on endpoint-ownership commit `42c5f3a`. Implementation, final gates and independent review passed: `make test`, the full race-enabled Go suite with all four real Terraform CLI suites opted in (including 19 cloud scenarios), build, vet, documentation generation, formatting and diff checks. Review identified and corrected an overstatement of the backend command allowlist; final review passed without remaining security or logic findings. Backend and Pulumi checkouts remain unchanged.

- AWS CLI forwards/reverses role ARN, service-account identity/creation, and command-allowlist controls alongside the existing static credentials. Role mode takes precedence if both auth flags are true. When `enable_allowed_commands` is true and both CLI `command` and `parameter` are nonempty strings, the backend parses the request and checks its exact service/subcommand pair against `allowed_commands`. If CLI parameters are absent or either value is missing, empty, or not a string, it falls back to an interactive `/bin/bash` shell before allowlist validation, even when enabled. This is not a command-only security boundary or IAM resource scoping. Empty/whitespace-only `allowed_commands` means allow all, not deny all.
- Keyspaces forwards/reverses keyspace/endpoint/version, region, static credential pair, role and service-account controls. Public Keyspaces inputs use `aws_region_name`, `aws_access_key_id`, and `aws_secret_access_key`; AWS CLI retains its existing `region_name`, `access_key_id`, and `secret_access_key` names. The shared IAM capability guard allows only the required new AWS/Keyspaces uses, not unrelated RDS/IRSA inputs.
- Cloud-only omission diff suppression preserves observed nonsecret settings, while explicit false/empty values remain resets and refresh is not suppressed. This avoids changing 1Password's shared sensitive `service_account` or Postgres IAM's shared `aws_role_arn` into globally computed fields. Missing/null cloud observations are not interpreted as clears; legacy response completeness still limits absence-as-removal detection.
- Datadog's existing sensitive `app_key` independently maps to `dd_app_key` in write, reverse mapping and fingerprint attribution. Both Datadog credentials preserve missing/null reads and accept explicit empty clears. Known held credentials remain subject to the established serialized-payload/resupply guard.

Evidence so far includes independent backend-shaped wire fixtures, observed RED→GREEN credential regressions, and the actual Terraform CLI loopback suite for cloud auth modes, imported policy preservation, false/empty resets and ignored changes, credential omission/markers, and Datadog application-key rotation/repeated refresh/saved-plan reconciliation/clear-then-omit. These are configuration API tests, not live IAM permission, allowlist-enforcement, Kubernetes provisioning, or connection proof. Native write-only and backend redaction/patch work remains deferred; the field CSV is still the pinned original inventory.

Baselines inspected:

- Terraform: `f3ddb78843c8ee2a736317c822ee122e6eeba33a`, branch `fix/secret-drift-visible`.
- Pulumi: `6f2a9ce88764100d1238859356d470ef25f7f36b`, including merged Google directory credentials support.
- Inventorize: `52bce48e42c617c8ea6f4a4d0044a0dec0a4260d`, branch `release/v0.9.x`.
- GitHub inspection confirmed #89, #90 and #91 are open and stacked in that order; local Terraform HEAD matches #91. Their reported checks pass. This is not evidence of deployed backend compatibility or live lifecycle success.

Path prefixes below: **TF** = this repository; **PU** = `../pulumi-adaptive`; **BE** = `../inventorize`.

## What the existing stack establishes

| PR | Responsibility | Remaining qualification |
| --- | --- | --- |
| #89 | Real read/import for all seven resource families; 82 integration types and reverse mappings; masking resource; endpoint/JIT fields; secret fingerprints; client/tooling changes. | Registration and self-roundtrip coverage do not establish complete backend field coverage or engine lifecycle correctness. Read API availability is a backend deployment prerequisite. |
| #90 | Reject nonempty mismatched response IDs and identity-less imports. Terraform core handles missing-object imports differently from Pulumi. | Identity-less refresh retains the ID but still blanks attributes; the existing test asserts only ID retention. |
| #91 | Map rotated-secret fingerprints onto sensitive attributes so drift can reach plans; expand certificate/key sensitivity. | Read still overwrites `applied_digests`, contrary to the intended last-write baseline. Additional credential-bearing fields remain non-sensitive. No live convergence verification was supplied by the PR. |

Setup: TF uses Terraform Plugin SDK v2, a registered component layer, per-integration YAML builders/reverse mapping, and the `internal/terraform-client` transport. Pulumi is a native inferred Go provider, not a Terraform bridge. Both use the backend's `/api/v1/terraform` API namespace. TF `make test` runs Go tests; `make build` compiles the plugin. Pulumi's root `make test` runs provider tests and builds its Go SDK; `make build` compiles its plugin.

## Coverage inventory

Computed from production Go ASTs and checked against source; counts are inventory, not test coverage:

- Seven resource families in each provider: Resource, Endpoint, Authorization, Group, Script, Schedule, DataProtection.
- The same 82 integration types, with forward/reverse switch coverage for all 82 in both providers.
- The same 29 HTTP method/path pairs: create/read/update/delete for seven families, plus endpoint force-delete.
- 570 distinct integration-type/YAML-key rows in the combined inventory. There are 87 Pulumi-only and four Terraform-only key occurrences across 24 integrations with different key sets. These include constants and derived values, so they are **not** 87 independent missing features.
- The four Terraform-only keys are stale Aruba Instant On and ClickHouse keys, not capabilities that should be ported into Pulumi.
- Backend-supported browser automation is missing in both providers for eleven device integrations; direct provider parity alone would miss it.

The complete production integration field inventory is `parity-integration-fields.csv` next to this plan. It records input provenance, emitted YAML key/type, reverse destination, sensitivity, and source anchors for every union row. Key-set equality alone cannot detect a declared field that a constructor never fills, such as Redshift SSL.

## Priority and delivery approach

- **P0:** merge/release blockers for credible state and credential safety.
- **P1:** required for the targeted functional parity release.
- **P2:** shared platform completeness, additional feedback/observability, or compatibility-sensitive enhancements.

Use focused PRs rather than one parity mega-PR. Prefer correcting #90/#91 defects on their owning branches if the stack is still under review; otherwise use explicit follow-ups. Recheck remote heads before implementation. Do not reorder or rewrite the stack as part of this plan.

### 1. P0 — Atomic reads and durable secret drift

**Terraform changes**

- Preserve `applied_digests` through refresh; seed only when no valid baseline exists, capture after successful writes, and prune only on an explicitly established clear. A missing digest response must not erase a valid baseline.
- Validate successful read envelopes before any state setter. Reject `{}`, `null`, identity-less or otherwise unusable successful responses without overwriting prior state. Do not reinterpret malformed success as deletion.
- Retain current 404 deletion semantics, wrong-ID rejection, import failure, and valid legacy responses that omit ID but carry a usable identity.

**Evidence:** TF `internal/provider/components/resource.go:1237-1259` compares then overwrites the baseline. `read_helpers.go:38-43` protects imports only; e.g. `session.go:572-580` subsequently overwrites identity. PU already preserves resource baselines at `provider/resource_resource.go:370-379`, but shares the identity-less-refresh problem.

**Acceptance tests**

- Full populated-state snapshots for all seven families, including lists, maps, outputs, credentials and fingerprints. Invalid reads preserve all attributes and ID—not merely ID.
- A→B out-of-band rotation, two or more refreshes, then plan; baseline stays A until reconciliation. Unsupported/nested-secret warnings recur until resolved.
- Managed and omitted scalar secrets, no-digest/old-server responses, initial seed, explicit clear, failed write and failed post-write capture.
- For the legacy compatibility path, actual provider/engine tests prove markers never become request credentials and a successful apply is followed by a clean plan. The preferred native path in work package 2 uses separate rotation intent and never writes a marker into a credential argument.

**Qualification:** a mapped sensitive marker can remain in state even after the current buggy code advances the baseline. The confirmed defect is false last-write history and lost warning-only evidence, not that every secret diff necessarily disappears.

### 2. P0 — Terraform-native secret architecture, not a copy of Pulumi

**Architecture decision:** functional parity must not require storing credentials in Terraform state or writing opaque drift fingerprints into password inputs. Preserve the existing mechanism only as a tested, time-bounded compatibility path. The preferred design follows HashiCorp's native write-only/ephemeral model and centralized secret-management principles.

#### Preferred order

1. **Avoid transporting long-lived credentials through Terraform where possible.** For integrations whose runtime supports it, manage workload identity or a secret reference, not resolved plaintext. Adaptive's authorized runtime resolves the reference with least-privilege access. Treat reference identifiers as potentially sensitive metadata and scope permissions accordingly. Do not fetch a secret via an ordinary Terraform data source and assume the resulting value is absent from state. Reference resolution and rotation support are integration-specific capabilities, not a global feature already present.
2. **For direct credentials, add native write-only arguments.** Proposed names such as `password_wo` and `password_wo_version` are design examples, not existing provider fields. Use `WriteOnly: true`, plus appropriate sensitivity metadata, with ephemeral input variables/resources upstream where supported. Keep plaintext out of plan/state, read outputs, logs, errors, private state and other computed attributes. This still requires provider/backend handling in memory and secure transport/storage; it is not a claim that secrets never exist outside the secret manager.
3. **Use explicit non-secret update intent.** Pair each independently managed credential or credential bundle with a version/generation trigger. A changed trigger authorizes writing the supplied apply-time secret; an unchanged trigger must not overwrite a credential during an unrelated update. Version triggers do not automatically detect secret changes or remote rotation. Never derive a public trigger from an unkeyed password hash.
4. **Preserve omitted secrets server-side.** Introduce an explicit backend secret-patch contract: omitted = preserve, supplied value = replace, explicit delete = clear. Prefer explicit clear intent over inferring deletion from omission/null; preserve existing empty-string semantics only as a documented compatibility path. Apply config and credential mutations atomically with authorization and, where needed, revision preconditions. Never reconstruct missing secrets from redacted reads or implement a client-side read/merge that races external rotation. A metadata-only apply must work without resupplying imported credentials.
5. **Retain drift observability without impersonating a secret.** Keep server revisions or object/field-scoped keyed fingerprints as metadata separate from credential inputs. Preserve applied versus observed provenance. Report remote credential changes without printing values; distinguish Terraform-owned credentials from externally rotated credentials. Do not automatically roll back an external rotation unless explicit ownership/policy authorizes reconciliation. A computed drift flag alone does not guarantee Terraform schedules Update; native write-only values also cannot produce diffs themselves. The default proposed behavior is durable diagnostics plus an explicit version bump to reapply. If automatic reconciliation remains a requirement, design and engine-test a separate planned trigger before claiming equivalent automatic drift remediation.

#### Drift detection does not require backend versioning

The existing backend already emits object/field/workspace-scoped HMAC fingerprints through `redactedDigests` (`BE/inventorize-app/internal/handler/secret_digest.go:55-60`; `terraform_read.go:199-213`). Retain that mechanism for comparison; do not require a new backend version store to detect out-of-band changes.

- Capture an applied fingerprint baseline after a successful credential write. Compare each later observed fingerprint against it without advancing the applied baseline on Read. Different fingerprints mean the current value differs from the acknowledged value, not a full history of intermediate rotations.
- A `password_wo_version`-style input is a practitioner-controlled Terraform update trigger, not a backend secret version and not the detector. Remote fingerprint comparison, local write-only input change detection, and scheduling remediation are three distinct concerns.
- Native write-only inputs plus remote fingerprints can detect remote value drift without retaining plaintext. They do not automatically detect an edited local write-only value or schedule Update. An explicit trigger is the recommended initial design; if users must never bump a trigger, prototype a secure private-state change-detection/planning alternative and verify actual SDK/engine support rather than promising it from computed outputs alone.
- Initial import/absent baseline cannot establish historical drift; seed without acknowledging that Terraform wrote the credential. Track baseline origin/ownership. Missing digests mean unknown coverage, not unchanged/empty. Fix redaction coverage before guaranteeing all integrations/nested fields are monitored.
- Digest-key changes or inconsistent replica keys can mimic value drift. Require stable shared server key configuration and design a key-generation/capability signal or conservative diagnostic policy; do not rewrite every credential automatically when comparison context is uncertain.

#### SDK and compatibility constraints

- HashiCorp documents write-only managed-resource arguments for Terraform **1.11 and later**; ephemeral variables/resources are available from **1.10**. SDKv2 supports write-only arguments: a Framework migration is not a prerequisite. The checked-in dependency is SDKv2 **2.40.0**, and its local schema implementation exposes `WriteOnly`.
- SDKv2 write-only values must be read from raw configuration with `GetRawConfigAt`, not ordinary `Get`; prior/planned/final state values are null. They cannot be provider-config/data-source attributes, computed attributes, aggregate TypeList/TypeMap/TypeSet attributes, or children of set blocks. Supported list nested blocks can contain scalar write-only leaves. Design nested RDP credentials around stable target identities and supported leaves/reference inputs, not by marking the whole list write-only.
- Add write-only alternatives alongside existing sensitive fields; reject conflicting reference/legacy/write-only sources with value-safe diagnostics. Do not change existing attribute storage semantics in place without migration tests. Decide the supported Terraform minimum/release boundary and validate older-core behavior explicitly; current old-core CI does not prove compatibility with a schema that advertises write-only fields. Do not imply older releases support the new path.
- Retain legacy sensitive arguments temporarily with clear state-exposure warnings and deprecation guidance. Migration removes plaintext from the **current** state only after verified engine behavior; historical state snapshots, saved plans, CI artifacts and logs require retention/access remediation and credential rotation where exposure is suspected. No claim of automatic historical erasure.
- Provider authentication is a separate path: prefer approved ambient/short-lived credentials where the platform supports them; redact configured `service_token` and do not try to use a managed-resource WriteOnly flag on provider configuration.

#### Immediate safeguards while the backend/schema evolve

- Fix durable baselines and malformed-read handling in work package 1, but do not expand fingerprint-as-password markers into the preferred architecture.
- Until secret-preserving backend updates exist, reject full-blob updates that would erase known held secrets. This guard is a fail-closed interim measure, not the desired permanent requirement to resupply every credential on every metadata update. Digest absence must not be interpreted as proof no secret exists; legacy/opaque nested configurations need an explicit backend capability/held-field inventory or a conservative refusal.
- Mark existing `service_account`, `value`, `fields`, `script`, and `prestart` sensitive immediately. Keep warnings actionable in public Terraform argument names. Sensitive alone masks display; it does not remove plaintext from Terraform state.
- Fix backend contextual redaction independently. Google service-account JSON and 1Password tokens must be redacted regardless of compact/multiline formatting. Preserve ordinary account-name identifiers by integration context rather than globally treating every serviceAccount key the same. Review `adminEmail` as identifier metadata, not automatically as a credential.

**Evidence:** TF `components/resource.go:1328-1368` replaces configuration without a held-value guard; PU `provider/resource_resource.go:291-325,458-481` supplies an interim guard but does not solve the API replacement contract. TF `resource.go:633-651,743-756` contains five scalar classification gaps; TF `internal/provider/provider.go:44-49` declares the token input. BE `inventorize-app/pkg/configexport/classify.go:79-87` allowlists normalized `serviceaccount` and its embedded-document handling creates the format-dependent redaction issue.

**Security acceptance gate**

- Use unique synthetic canaries through real provider/engine calls and assert absence from current state, JSON/binary saved-plan artifacts using supported inspection, outputs, diagnostics and provider debug logs for the new path. Audit legacy-path exposure explicitly; no test prints the canary on failure.
- Verify ephemeral input at apply, saved-plan application and absent/unknown/null values; require an appropriate apply-time value only when a credential write is intended. No plaintext or reversible secret surrogate is persisted to support a later apply.
- Import → metadata-only update preserves credentials without secret retrieval or resupply once backend patch support exists. Version-bumped credential updates, explicit clear and subsequent omission converge. Unchanged version plus unrelated update never overwrites remote rotation.
- Concurrent rotation/update is preserved or rejected with a revision conflict; no read/merge race, marker serialization, blind secret retry or partial credential/config commit.
- Nested RDP, compact/pretty Google JSON, 1Password tokens, script bodies and TLS private keys follow the same lifecycle rules. Required new-object credentials are validated without making imports destructive.
- State/plan storage still requires encryption, least-privilege access, audit and retention controls because other sensitive metadata/legacy fields may remain. Centralized rotation/revocation and sanitized logging remain mandatory operational controls.

**Primary design references checked:**

- HashiCorp SDKv2 Resources / Write-only Arguments: https://developer.hashicorp.com/terraform/plugin/sdkv2/resources/write-only-arguments
- Terraform Manage sensitive data: https://developer.hashicorp.com/terraform/language/manage-sensitive-data
- HashiCorp Framework Write-only Arguments / Best Practices: https://developer.hashicorp.com/terraform/plugin/framework/resources/write-only-arguments
- OWASP Secrets Management Cheat Sheet: https://cheatsheetseries.owasp.org/cheatsheets/Secrets_Management_Cheat_Sheet.html

Native behavior is documented; the Adaptive field names and secret-patch API above are proposals requiring implementation and engine verification.

### 3. P1 — Correct existing broken mappings before adding more fields

| Integration | Problem | Proposed correction |
| --- | --- | --- |
| SSH | TF writes `password` from `key`; password-only authentication sends an empty password. | Forward `password` from the password argument, preserve key mode, and add the independent reverse password mapping. |
| ClickHouse | TF emits `databaseName`/`sslMode`; backend expects `database`/`use-tls`. | Emit canonical backend keys; use the TLS boolean and define backward-compatible treatment/conflicts for existing `ssl_mode`. |
| Aruba Instant On | TF emits `host`/`apiToken`, not the current hostname/browser/SSH contract. | Map canonical hostname/login/SSH fields, explicitly handle legacy host input, and resolve unsupported token intent instead of silently discarding it. |
| Redshift | `sslMode` exists in the struct but is not initialized or reversed. | Wire `ssl_mode` in both directions and test nonempty SSL configuration. |

**Anchors:** TF `internal/provider/integrations/ssh.go:30-39`, `clickhouse.go:5-28`, `aruba_instant_on.go:5-21`, `awsredshift.go:16-24`; PU canonical builders in `provider/integrations.go`.

**Acceptance:** password-only/key-only SSH and redacted preservation; backend-compatible decode fixtures for ClickHouse and Aruba; Redshift nonempty SSL sentinel; legacy input migration/conflicts. A forward→reverse roundtrip of the same wrong keys is not an acceptable oracle.

### 4. P1 — Endpoint control parity and ownership semantics

- Add TF `max_jit_request_duration` end to end: public schema, presence-aware pointer request, read DTO, refresh/import and docs. Omitted preserves/inherits; explicit empty clears the override; a duration sets it.
- Correct false→true security drift for endpoint-local `is_jit_enabled`, `script_only_access`, and disable flags; also test zero→nonzero approver counts.
- Separate full-replacement fields from pointer/PATCH fields (`auto_approval`, `jit_multi_approver`, quorum, maximum duration). Do not adopt an omitted remote value as a removal diff when omission cannot clear it.
- Keep workspace-derived cluster/CPU/memory defaults separate from endpoint-local settings. Backend reads of disable flags are endpoint-local; runtime OR with workspace policy does not justify suppressing those read changes.
- Resolve endpoint type changes: the current backend update path validates type but does not implement the observed transition. Either implement a supported transition or plan replacement/reject before apply.
- Retain direct↔cli equivalence, membership set/order handling, computed status/exposure fields, and explicitly defined readiness semantics.

**Anchors:** TF `components/session.go:427-494,592-668`; PU `provider/client.go:90-116` and `provider/read_helpers.go:183-232`; BE `inventorize-app/internal/handler/terraform.go` JIT application, session read/update handlers.

**Acceptance:** absent/false/true/zero/empty × remote change × refresh→plan→apply→refresh matrix; exact JIT wire presence; membership additions/removals, creator-added user, exposure outputs, import, and no-op convergence. No unrelated update should silently weaken approval policy.

### 5. P1 — Fill Terraform integration fields in cohesive batches

For every affected type update schema only where genuinely absent, forward mapping, reverse mapping, secret metadata, type-specific validation, docs and independent contract fixtures.

1. **IAM/cloud:** AWS role/service-account/allowed-command controls; MySQL RDS IAM; Kafka MSK IAM/bootstrap servers; ElastiCache IAM/cache identity; Keyspaces endpoint/keyspace/identity; Postgres auth mode.
2. **TLS:** MySQL certificates/key/root/old-version; MongoDB client TLS; DocumentDB TLS; syslog TLS/mTLS; services TLS; SQL Server/Azure SQL SSL mode.
3. **Identity/other:** Google directory `service_account` + new `admin_email`; Datadog app key; RDP file-transfer control; network-device SSH/browser fields.

Replace TF's Postgres-only IAM option restriction with per-integration capabilities. Reuse existing fields where meanings match; don't add duplicate aliases merely because providers use different public names.

**Important Pulumi correction:** its MySQL/Postgres/Kafka builders flatten omitted IAM switches into explicit false. Backend pointer semantics distinguish omission from false. Correct Pulumi's private wire structs/emission and establish tri-state fixtures before copying those builders to TF. ElastiCache already uses pointer switches. TF's own raw-config/default behavior also needs tests; don't assume its schema distinguishes omission correctly.

**Acceptance:** every matrix delta below is either implemented or explicitly justified as a constant/alias/deprecation; omitted/true/false IAM modes and legacy auth-mode-only configuration are preserved; all new strings test absent/redacted/empty; exact backend key spelling and types verified independently.

### 6. P1 — Schedule read/reset contract and backend persistence

- Use a documented full-read capability/version or equivalent presence-aware contract: legacy missing data must not erase inputs, while modern empty lists/zero/false/empty strings must reflect actual removal.
- TF currently misses final membership removals and clears; Pulumi's full mapper can clobber legacy omissions. Fix both against the same fixtures, not by blindly copying either mapper.
- Do not substitute legacy `mappedEndpoints` IDs for canonical endpoint names without explicit resolution/contract support.
- Define duration reset and `is_active` omission semantics. TF positive-only `max_access_time` cannot express reset; TF defaults active true while Pulumi nil preserves an existing update value.
- Fix BE schedule update timezone persistence: the update branch copies other request fields but never assigns `existing.Timezone`.

**Anchors:** TF `components/schedule.go:221-330`, `internal/terraform-client/schedule.go:50-88`; PU `provider/schedule.go:175-245`; BE `inventorize-app/internal/handler/terraform_schedule.go:301-342`.

**Acceptance:** full versus legacy fixtures; clear final users/teams/endpoints, description, expiry and window; timezone set/change/clear with isolated persisted read-back; active/duration removal convergence; case/order/equivalent-instant stability. Modern omitempty fields require a capability-aware design—not assuming every absent field is legacy.

### 7. P1/P2 — Remaining resource-family lifecycle correctness

| Family | Coverage and remediation | Acceptance |
| --- | --- | --- |
| Resource | All 82 registered; field and credential gaps covered above. Type/name mutability must match actual update support. | No silently ineffective accepted change; supported changes update or plan replacement/rejection. |
| Group | Four corresponding fields. Keep create→Slack follow-up; specify remote-only Slack ownership and empty-entry validation. | Follow-up failure retains a tracked/recoverable ID; retry updates the same group; members/endpoints remove to empty. |
| Authorization | All four TF inputs exist in PU. Align description ownership and validate supported aliases without inventing spellings. | Required fields, permissions and description changes/removals roundtrip; readiness failure is recoverable, not blind duplicate create. Optional TF status output is P2. |
| Script | Complete input correspondence. Endpoint changes currently fail only during apply. | Plan replacement or an explicit plan-time error, retain apply guard; test repeated command-digest drift, withheld-body import warning, metadata/map removal. Optional auto-generated output is P2. |
| DataProtection | Full nested field correspondence; scoped/masked defaults are semantically equivalent. Resource rebinding currently fails during apply. | Plan replacement/rejection safely; retain masking-off deletion contract; add PU empty-ID success guard; all database/table/schema/column/mask-type leaves and reorder/removal cases tested. |

Script endpoint and masking resource replacement need engine-specific implementations (TF replacement metadata and Pulumi diff policy), not just descriptions. Test destroy ordering: replacement must not disable a newly created policy. DataProtection destroy intentionally applies the platform's off shape; it is not authorization/view deletion. Do not introduce destructive cleanup to make state look symmetrical.

### 8. P1 — Client/endpoint lifecycle conformance

- Match sensible deadline/cancellation behavior: TF has a 60-second HTTP timeout, PU currently does not. Make requests and polling sleeps context-aware and keep permanent errors from expensive retry loops.
- Port terminal-status polling/404 short-circuit to PU endpoint deletion, align permitted force-delete states and idempotency races with the backend.
- Backend normal termination uses a synchronous service path, so immediate PU force-delete is not proven broken on every normal deletion. The real gaps are adverse transitions, misleading HTTP 200 error bodies, and status preconditions.
- Clarify create means recorded versus ready; current readiness handling can accept failed statuses or swallow readiness errors. Preserve recoverable identity on partial success.
- Decode bounded sanitized errors once; preserve string/object/unresolved-name reasons. Correct update DTOs that imply an ID where backend returns an ok/success flag. Reject unusable create IDs.
- Do not automatically replay non-idempotent create POSTs after ambiguous failures.

**Acceptance:** a shared 29-route fixture matrix asserts method/path/headers/payload/status/body closure, malformed success/error bodies, auth denial, conflicts, transport failure and cancellation. Endpoint transitions cover creating/restarting/failed/terminated/absent, terminate failure, disappearance before force-delete and deadline exhaustion. Separate backend row absence from asynchronous infrastructure cleanup.

### 9. P2 — Shared platform completeness and input validation

These are separate follow-ups, not reasons to reproduce known Pulumi defects in TF:

- Both providers omit `automationMode`, `fields`, `script` for eleven device types: `aruba_instant_on`, `aruba_sw`, `cisco_ngfw`, `fortinet_ngfw`, `fortinet_analyzer`, `fortinet_manager`, `hpe_switch`, `ivanti`, `juniper_sw`, `paloalto_ngfw`, `sophos_fw`. Add both mappings, shared private configuration helpers and secret treatment. Do not add `prestart` where backend settings do not support it.
- Six accepted Resource fields are unused in both providers: `app_id`, `app_key`, `database_account`, `database_username`, `database_password`, `version` and their PU counterparts. Deprecate with actionable canonical names or implement explicit conflict-checked aliases. Don't silently repurpose version.
- Add Pulumi plan-time validation for known invalid values, empty credentials/membership entries and wrong-type fields; defer unknown preview inputs safely. Backend contract decides valid enums, not stale TF allowlists.
- Keep PU RDP targets' existing secret YAML string compatible while adding parser/structural validation. A typed target array is an optional versioned/migration-sensitive enhancement, not an in-place string→array change.
- All eight RDP target leaves must be accounted for: id, name, host, port, username, password, domain, record. Test port default 3389, duplicate/blank identity, password validation, record absent/false/true and non-RDP rejection. Withheld target blobs cannot recover per-target credentials or all nonsecret remote drift; require an explicit backend protocol enhancement for that guarantee.

### 10. P1 release gate — Contract fixtures, engine lifecycles and documentation

**Offline CI, first**

- Tie production registry, forward/reverse mappings and test inventory together; remove confidence based only on duplicated permissive test schema and self-roundtrips.
- Test production TF schema, actual provider callbacks/protocol calls and actual plan behavior, not only helper output. Independent reviewed backend DTO/YAML fixtures must catch wrong wire keys.
- Add a credential-free Terraform-engine suite against a loopback fake API for all seven families: create→update→repeated refresh→plan, import, missing import, out-of-band delete and destroy. Preserve no-op plans and explicit clears.
- Current CI's `TF_ACC=1` and Terraform-version matrix run Go unit packages; they do not themselves establish that Terraform performs acceptance lifecycles.
- Make Pulumi redaction checks fail: `tests/integration/resource_config_coverage_test.go:54-64` currently only logs missing expected redaction. Its matrix has 24 cases/23 wire types, not all 82, and is not the entirety of its live suite. Expand meaningful config-field updates, not tag-only writes.

**Separately authorized live release gate**

- Use an explicit disposable workspace/URL, pinned local plugin revisions and a supported backend capability baseline. No automatic credential fallback or existing-resource adoption.
- Configuration lifecycle contract coverage for all 82 integrations where isolated storage tests are supported; record unsupported cases and do not present skipped cases as passed.
- Real connection/runtime checks require appropriate disposable targets. Test representative protocol/authentication classes (IAM/static key/password, TLS/mTLS, Kubernetes, SSH/RDP, browser, messaging), plus every changed runtime-sensitive configuration. Configuration persistence is not connection success.
- For all seven families verify create/update/read, no-op repeat plans, both drift directions/removal, import, missing-object import, out-of-band deletion, destroy and authoritative remote read-back. Assert resource-specific changes, not aggregate change counts.
- Secrets: managed/unmanaged rotation, certs, script body, import-safe unrelated update, explicit clear, omit-after-clear, repeated refresh persistence and final convergence. Enforce strict cleanup and state/log handling. DataProtection teardown verifies off-state, not nonexistent rows.

**Documentation and compatibility**

- Regenerate TF docs from templates for changed schema; publish a per-integration supported-field table and omission/clear rules.
- Fix README's stale `redacted_digests` name and the `ttl = "8h"` example that current TF validation rejects; reconcile version constraints and missing example/guide links against actual tracked files.
- Explain the removed standalone Teams workflow migration, backend read/digest capability requirements, secret import limits, schedule adoption/reset rules, and masking-off destroy semantics.
- Pin a deliberate supported Terraform-version range in CI; do not infer broad compatibility from installing old core versions without exercising them.
- Regenerate Pulumi schema/SDKs only when public inputs/annotations change; private wire corrections do not inherently require SDK churn. Release/version bumps and publishing remain separate authorized actions.

## Suggested PR boundaries and dependencies

1. TF read atomicity + seven-family state regression tests (#90 correction); parallel PU shared fix.
2. TF durable digest baseline + repeated refresh tests (#91 correction).
3. TF immediate sensitivity/held-secret guard safeguards; parallel BE contextual redaction fix. Follow with an explicit backend preserve/replace/delete secret-patch contract, then native TF write-only inputs + version triggers and legacy-state migration. Native inputs depend on preserve semantics for unrelated updates; do not ship them as a cosmetic schema-only change.
4. TF SSH/ClickHouse/Aruba/Redshift contract fixes and compatibility notes.
5. TF endpoint duration + ownership/drift matrix; shared backend type-transition decision.
6. TF IAM/cloud field coverage; prerequisite tri-state contract fixtures and PU IAM correction.
7. TF TLS/Google/network field coverage in independently reviewable batches.
8. Schedule read-contract/reset fixes plus a separate BE timezone persistence PR.
9. Immutable-field planning and partial-lifecycle/client hardening, shared where applicable.
10. Shared browser automation/unused-input policy as separately scoped follow-ups.

Build offline fixture/engine harness infrastructure alongside steps 1–3 and require it for later PRs. Do not defer all testing to step 10. Live validation and documentation completion gate the parity release, rather than being an unverified promise after merge.

## Complete differing integration key-set appendix

Literal keys are preserved. A key present in a struct is not necessarily populated. Envelope/derived constants require justification rather than automatic schema expansion. The CSV covers matching integrations and every field as well.

| Integration | Terraform-only YAML keys | Pulumi-only YAML keys |
| --- | --- | --- |
| aruba_instant_on | apiToken, host | hostname, login_url, sshKey, usePassword, version, webui_port |
| aruba_sw | — | login_url, port, sshKey, usePassword, version, webui_port |
| aws | — | allowed_commands, aws_role_arn, create_if_not_exists, enable_allowed_commands, service_account, use_role_arn, use_service_account |
| awsdocumentdb | — | tlsEnabled |
| awselasticcache | — | aws_role_arn, aws_service_account, cache_name, cache_type, useIamAuth, useIrsa |
| azuresqlserver | — | sslMode |
| cisco_ngfw | — | sshKey, usePassword |
| clickhouse | databaseName, sslMode | database, use-tls |
| datadog | — | dd_app_key |
| fortinet_ngfw | — | sshKey, usePassword |
| google | — | adminEmail, serviceAccount |
| hpe_switch | — | sshKey, usePassword |
| juniper_sw | — | login_url, webui_port |
| kafka | — | awsAccessKeyId, awsRegion, awsRoleArn, awsSecretAccessKey, awsServiceAccount, bootstrapServers, useIrsa, useMskIam |
| keyspaces | — | aws_access_key_id, aws_region_name, aws_role_arn, aws_secret_access_key, keyspace, keyspaces_endpoint, service_account, use_role_arn, version |
| mongodb | — | clientCertificate, useTLS, version |
| mysql | — | authMode, awsAccessKeyId, awsRegion, awsRoleArn, awsSecretAccessKey, awsServiceAccount, clientCert, clientKey, oldVersion, rootCert, useIrsa, useRdsIam |
| paloalto_ngfw | — | port, sshKey, usePassword, version |
| postgres | — | authMode |
| rdp_windows | — | allowFileTransfer |
| services | — | enableTLS |
| sophos_fw | — | login_url, webui_port |
| sql_server | — | sslMode |
| syslog | — | caCertificate, clientCertificate, clientKey, insecureSkipVerify, tlsEnabled |

Redshift SSL and SSH credential sourcing are additionally broken despite no relevant key-set delta. Shared browser automation and dead schema inputs are additional backend-completeness issues, not represented as provider key-set differences.

## Evidence and completion boundary

Executed during this audit:

- TF `make test` and `make build`: passed.
- PU `make test` and `make build`: passed, including Go SDK compilation in its test target.
- TF instrumented internal-package tests: passed; aggregate statement coverage **54.8%**. Many mutation callbacks/client methods have no executed statements in this run. Coverage percentage is not a substitute for the named lifecycle acceptance criteria.
- Audit-only Go overlay probes, without changing repository tests: failed as expected, showing refresh advances the digest baseline and all seven families overwrite their identity attribute on `{}` successful refresh. These are regression probes exposing gaps, not failures of the unchanged baseline suite.
- AST inventory/CSV uniqueness and row counts checked programmatically. Integration wire behavior and selected backend paths inspected; an isolated copied-source builder probe establishes PU omitted-IAM boolean emission, not live authentication success.

No infrastructure apply, live provider/API CRUD, backend startup or production credential reads were performed. Backend's pre-existing `inventorize-ui/package-lock.json` edit was left untouched. Planning does not establish API deployment status, runtime connection success, masked views, or completed remediation.
