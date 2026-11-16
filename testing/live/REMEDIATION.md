# Live-campaign remediation and regression plan

## Scope and ownership

Provider fixes stay in the current terraform-provider-adaptive branch, initially at
84c21f28681ea383a3762ecad677b1c050ddb007. User explicitly approved targeted
Inventorize backend handlers/classifier fixes and regression tests, then required
that they be made in a worktree. That worktree is:

/Users/dhaivatdshah/IdeaProjects/.worktrees/inventorize/fix/terraform-provider-live-regressions

Branch: fix/terraform-provider-live-regressions
Base: b0af003957b2a33562a5fb4862fbb0730876c9d9

The original Inventorize checkout is not an edit target; its existing
inventorize-ui/package-lock.json modification belongs to the user. Permission to
edit backend source does not authorize deployment/restart, CLI/UI edits, commits,
or modification of pre-existing data.

On 2026-11-05 the user separately authorized running the patched worktree on
port 8080 after stopping the previous local backend. The task-owned worktree
binary was launched in app-only mode (no second general scheduler), with debug and
service logging disabled and no copied credentials. Health returned HTTP 200;
the user reauthenticated the saved CLI to the same localhost deployment.
Read-only default exports of the original owned 1Password and compact Google
fixtures passed redaction assertions. Independent review then identified that
newly accepted native CockroachDB grants could reach an existing unsafe shell
interpolation path. The task-owned backend was stopped and port 8080 verified
closed before any fresh campaign. A validator-only fix, regression tests and a
fresh review are required before restart. No exploit payload was executed.
The user subsequently declined any CockroachDB runtime quoting changes: keep
that consumer unchanged and mark native CockroachDB enforcement BLOCKED. The
handler must conservatively reject unsupported shell-active native input;
configuration persistence tests are not runtime authorization proof. PostgreSQL
client.go/masks.go likewise remain read-only by the earlier explicit decision.
These observations do not replace fresh lifecycle verification or alter the
original campaign's failures. Exact build/source hashes, launch configuration,
security hold and protected evidence are recorded outside Git.

The previous run and all 223 state-managed fixtures remain on manual hold:

/Users/dhaivatdshah/IdeaProjects/.terraform-provider-adaptive-live/20261104T042306Z-96512

Do not replace that run's provider binary, rewrite its historical results, mutate
its fixtures for regression, or destroy it. Use a new pinned run for live retests.

## Issue-specific acceptance

| Finding | Fix boundary | Required regression |
|---|---|---|
| F1 usePassword held-secret classification | Approved backend classifier worktree; do not waive provider guards | Both boolean values exported as flags; actual password/key still redacted/fingerprinted; SSH/device tags update and import reconciliation |
| F2 serverlist missing name | Provider builder | Update YAML retains configured name; create/update/read/no-op and independent import |
| F3 ProxySQL SSL incompatibility | Provider enum/builder boundary, grounded in native runtime/CLI validation | Supported spellings round-trip without silent semantic changes; real CLI-origin clone/import |
| F4 1Password visibility/silent clearing | Provider observed-sensitive write safeguard plus approved backend classifier | Omitted/null/unknown inputs fail before POST; explicit resupply/clear succeeds; default export redacted; preservation independently checked |
| F5 mask reorder | Provider plan semantics | Reorder-only no-op; add/remove/mask/scoping changes remain actionable; existing states compatible |
| F6 CockroachDB native SQL | Approved backend create/update authorization validation | Native grants accepted without weakening structured filter validation or inventing YAML policies |
| F7 group acknowledgments/reorder | Provider strict route-specific decoder and plan semantics | Positive capitalized/lowercase response handling, ambiguity/failure rejection, reorder no-op, actual membership changes persist |
| F8 schedule expiry export | Approved backend export helper | Persisted expiry round-trips with correct format/timezone; absent/cleared expiry remains absent |
| F9 MongoDB false export | Investigate typed-default/readback semantics | Do not blindly equate null with false or claim TLS runtime failure; retain a source-grounded assertion |
| F10 compact Google JSON | Approved backend embedded-redaction/fingerprints; provider safeguard remains | Compact/multiline credential documents protected equally; noncredential identifiers/prose not corrupted |
| Import strict null/empty comparisons | Provider-repository harness | Optional typed zero normalization only; nonempty loss and required/malformed values still fail |
| Masked/read-only endpoints and sync | Read-only diagnostic first | Establish exact failing layer; no privileged-smoke substitution for enforcement or acknowledgments for completion |
| Nested-secret/empty-baseline limitations | Provider | Per-key computed known_empty_secret_keys distinguishes an observed explicit scalar empty from an unknown legacy baseline; first additions stay actionable across refreshes. RDP aggregate drift remains warning-only without target attribution or forced updates; exact nested pre-write intent/payload safety remains required |

## Verification already exercised for the provider safeguard

- New component tests reproduced unclassified secret omission across 1Password,
  PostgreSQL, Azure and Datadog; existing Google guard remains protected.
- Full component package tests passed after the generalized observed-sensitive
  guard was implemented.
- TestUnclassifiedSecretTerraformEngine failed against the previous pinned binary:
  an omitted imported 1Password credential reached a successful update.
- The same real Terraform CLI test passed with a newly built patched provider:
  omission/null rejected with zero fixture mutations; explicit resupply, clear,
  subsequent omission and final no-op succeeded.
- These engine tests use isolated API fixtures and are not live-backend evidence.

## Reviewed backend and recovery boundary

The subsequent validator-only hardening and independent backend v2 review passed
before the fresh campaign on 2026-11-05. Its reviewed source snapshot is
49f7c114e79077e91fbc819bf07d9e26d408d81e82f567b8dc12b1e6f5626aba;
its binary SHA-256 is
ad360e54ae748cea18183b5adbf2c70aff81d8d57099d8c3c01462479009a607.
This does not authorize or establish native CockroachDB runtime enforcement.

The new retained run is:

/Users/dhaivatdshah/IdeaProjects/.terraform-provider-adaptive-live/20261105T082511Z-45364

Its prefix is tflive-20261105082514-45364. The provider is pinned to source hash
5321505ff43181886f29a5485ce51d9b6e9a7b4d56d698a291db3b62eca65ebf
and binary hash
9a46314556bb3ff20b87d5fa111c3143a18eceba77d946efc0794e1efcb89d13.
The original run above remains immutable and is not a regression target.

The serving backend changed during the first new-run field campaign. Its 193
attempts, 120 PASS, 73 FAIL and 148 confirmed historical restorations remain
mixed/unknown-backend evidence, not reviewed-build acceptance. The user separately
authorized restoring the reviewed service on 8080 and reconciling affected
new-run fixtures. On 2026-11-12 the identified IDE-launched service was stopped
and replaced with the pinned reviewed binary; listener executable, binary hash,
HTTP health and saved localhost CLI identity were checked independently.

Recovery did not bulk-apply the failure list. All 82 local configurations already
matched their original backups. Default redacted exports of the 45 unresolved
fixtures matched 302 comparable configured fields, leaving 51 withheld fields
unverified. Source-backed wire aliases and newline-serialized host lists were
compared under their actual encoding, not mistaken for drift. Of the 45 final
fields with unresolved restoration evidence, 36 matched and nine were withheld.

Scoped, synthetic-only exact readback then found seven of those nine unchanged.
The CockroachDB and Elasticsearch passwords still matched the campaign's known
synthetic mutations. Only those two fixtures were restored, using inspected
update-only saved plans on unchanged IDs. Exact password readback, unchanged other
remote configuration/tags, full configured state and no-op plans passed. A fresh
nine-field comparison matched all nine originals. Raw include-secrets responses
were compared in memory, not persisted or printed.

The reviewed 1Password probe also passed all 13 assertions: native default
redaction, expected omission rejection with the correct diagnostic, exact
synthetic credential/configuration/tags retention, explicit restoration and final
no-op. Independent review of the completed probe and recovery passed with no
security or logic blockers; sanitized assertions do not enable an independent
reviewer to recompute plaintext comparisons without separately authorized reads.
The old failures and post-error SDK state observations remain retained.

## Completion gates

Fresh offline verification on 2026-11-12 passed make test, make build, 104 Python
harness tests, fixturetool Go tests, 16 native-client fixtures and the complete
opt-in provider package suite. That package recorded 17 top-level tests and 141
subtests; specifically, the isolated Terraform CLI engine portion recorded 11
top-level tests, 129 subtests and 125 terminal leaf tests. Do not count parent pass
events as additional independent cases. All 192 pinned provider source files
were unchanged. These are offline/isolated-API results, not live runtime proof.

The guarded field replay was halted on 2026-11-12 when the saved CLI login expired
(or access was denied); the reviewed backend binary and listener PID were still
unchanged. Its bounded inventory is 82 original fixtures, 330 selected fields and
263 explicit selection skips, with four workers. Before the authentication gate
closed, 117 fields passed and their restorations were verified. Two further
field-attempt records were unresolved (Datadog dd_api_key and DigitalOcean
api_token); neither has an apply-start record for that field, and both local
configurations match their original backups. Fresh authorized remote readback is
still required before calling those fixtures verified. Another 51 queued
fixtures were refused before field execution; their retained case-level FAIL
records are blocked execution, not evidence of 51 provider regressions.

The reviewed field harness is unchanged; an archived wrapper adds serving-build
and saved-identity checks before and after Terraform/export operations and stops
dispatch when a gate fails. Its immutable attempt is
field-updates/attempts/20261112T162328Z-36856efdc3bd4820bbea34430e9759d2.
The raw attempt has 119 field-attempt records, 117 confirmed restorations and 53
FAIL records (two field records plus 51 unexecuted case records). Preserve these
records. A separate authentication-blocker receipt records the actual failure
reason; the wrapper's generic gate exception must not be mistaken for a changed
serving binary. On 2026-11-13 the user reauthenticated to the same saved localhost
context. Scoped exact synthetic readback confirmed both interrupted credentials
and their tags unchanged; no repair was needed. The four-process continuation
then verified another 151 fields and restorations before the saved CLI again
reported login expired/access denied. Combined coverage is now 268 verified
fields, five unresolved failed field records and 57 blocked fields: 62 fields
remain across 19 fixtures. No completed results or raw failures were rewritten.

The five unresolved fields are ProxySQL proxysql_admin_password, legacy RDP
allow_file_transfer, RDPLDAP ldap_encryption_method, Redis password and serverlist
password. Their mutation applies completed, but restoration is not verified.
The legacy RDP restoration blocker occurred before the later shared auth stop;
its root cause is bounded, not established. Its restore plan was saved but no
restore-apply was recorded; do not apply that archived plan blindly. A transient
stale directory listing can reproduce the failure mechanism, but the historical
failing path was not retained, so this is not a confirmed cause or provider bug.
All five local configurations still match their originals.

Validated chained continuation passed independent review, including recomputation
of the 268 actual positive results and their originating attempts. On 2026-11-13
the user reauthenticated again. Scoped readback confirmed known campaign drift
in all five interrupted fields; only those five were restored using newly
inspected update-only saved plans. Exact target readback (synthetic-only for the
three credential fields), unchanged other remote configuration/tags, full
configured state and final no-op checks passed. The old RDP failure remains
unexplained; successful recovery is not proof of its historical cause.

The final chained replay completed on 2026-11-13: all 330 unique selected fields
passed with verified restoration (117 + 151 + 62, preserving their originating
attempts). The final receipt recorded 1,574 successful deployment-gate checks,
no auth/serving/unknown failure and unchanged source ancestry. Of these proofs,
242 compare exact state and remote values, 87 compare exact state with redacted
remote readback, and one uses the source-defined MongoDB 3.6 omitted-false
contract. Redacted readback is not exact credential retention or vendor auth.

Post-run checks verified all 82 original configuration files byte-identical to
their saved originals and all configured state values matching. The stable
inventory contains 217 family-qualified managed identities with no duplicate
same-family IDs/names and the same two retained tainted endpoints. Manual
planning produced 217 saved delete-only plans containing 217 matching delete
actions; no deletion was executed. All 83 ordinary CLI-origin and four family
CLI-origin objects are state-managed, with no orphan candidate in those ledgers.
Two generated authorization targets and one owned Docker container are separate
from these managed identities. The container ownership label was verified, and
it was stopped; no restart or removal was performed. Generated authorization
presence and database views were not independently verified.

Independent completion-proof review passed for field/state/ownership and actual
saved plans, including binary-plan lineage/serial/configuration and retained
state hash stability. Three plans refresh non-identity observations for the two
failed endpoints and runtime PostgreSQL resource without changing retained state.
These planned reads do not establish readiness, connectivity or enforcement.

Final coverage regeneration exposed a reporting defect: ledger-only aggregation
kept old diagnostic failure phases as current field failures even after verified
retries. The narrow continuation-aware correction and its end-of-build snapshot
race fix passed independent review and all 144 Python harness tests. The reporter
validates immutable ancestry and rechecks attempt names, pointer digest and pins
immediately before returning; it does not inspect active work configuration/state
bodies or execute archived code.

Corrected coverage was regenerated and read back successfully: 330 selected,
attempted, restored PASS fields with the verification strengths above; 81
integration fixtures have selected-field PASS and one has no selected mutation.
All 178 historical FAIL rows and the separate ledger-only diagnostic field counts
(250 PASS / 80 FAIL) remain intact. They are not silently reclassified as current
field acceptance. All 599 wire mappings remain NOT RUN for independent wire/runtime
assertions. Final reports, manual inventory, source snapshots, command receipts
and independently reviewed destroy plans are in the completion handoff bundle.
The authorized configuration-regression handoff is complete; runtime prerequisites
and manual teardown remain separate holds.

Twelve new-run schedule expiry captures passed the narrow RFC3339 absolute-instant
replay; the twelve literal offset-spelling failures remain unchanged. This does
not rewrite the original run's missing-expiry finding or prove fresh runtime
behavior. Endpoint provisioning, policy enforcement, masking, sync and dependent
imports retain their recorded prerequisite blockers; no cluster repair, endpoint
restart/untaint/replacement or protected runtime-code edits are implied.

After writers settle, reconcile every selected field and restoration, repeat the
stable state/ownership inventory, check family-qualified duplicate ownership,
archive prior fixed-name summaries, and regenerate coverage/audit and new-run
manual destroy plans. A read-only inventory snapshot found 217 managed instances;
that snapshot is not the final count and must be revalidated. Generated masking
authorizations/views and owned Docker infrastructure are separate from managed
Terraform identities. Plan mode contacts localhost for reads but performs no
remote deletion. Actual teardown remains unexecuted pending manual validation
and separate explicit approval.
