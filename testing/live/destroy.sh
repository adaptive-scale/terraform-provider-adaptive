#!/bin/bash
# Explicit delayed destruction; no EXIT trap and no implicit deletion.
set -euo pipefail
umask 077
MODE=${1:-help}
RUN=${2:-}
CONFIRM=${3:-}
die() { printf '%s\n' "$*" >&2; exit 1; }
case "$MODE" in plan|apply) ;; *) printf '%s\n' 'Usage: bash destroy.sh plan ABSOLUTE_RUN_DIR' '       bash destroy.sh apply ABSOLUTE_RUN_DIR EXACT_RUN_PREFIX' 'apply creates fresh scoped destroy plans, checks ownership, then applies those exact plans.'; exit 0 ;; esac
test -f "$RUN/manifest.json" || die 'Missing run manifest.'
BASE="$HOME/IdeaProjects/.terraform-provider-adaptive-live"
test "$(cd "$RUN/.." && pwd)" = "$BASE" || die 'Unexpected run location.'
PREFIX=$(jq -er '.prefix | select(startswith("tflive-"))' "$RUN/manifest.json")
jq -e '.url == "http://localhost:8080"' "$RUN/manifest.json" >/dev/null
test -z "${ADAPTIVE_SVC_TOKEN:-}" && test -z "${ADAPTIVE_URL:-}" && test -z "${ADAPTIVE_PROFILE:-}" || die 'Unset Adaptive token/URL/profile overrides.'
adaptive whoami -o json | jq -e '.URL == "http://localhost:8080" and .CredentialSource == "saved context"' >/dev/null
test "$(shasum -a 256 "$RUN/bin/terraform-provider-adaptive" | cut -d ' ' -f 1)" = "$(jq -r .binary_sha256 "$RUN/manifest.json")" || die 'Binary checksum mismatch.'
if test "$MODE" = apply; then
 test "$CONFIRM" = "$PREFIX" || die 'Manual validation hold: supply the exact run prefix to authorize deletion.'
 if test -f "$RUN/sync/discovery-disabled.json"; then
  jq -e --arg p "$PREFIX-" '.type=="DataDiscoverySchedule" and .configuration.enabled==false and (.configuration.resource|startswith($p))' "$RUN/sync/discovery-disabled.json" >/dev/null || die 'Unowned discovery disable document.'
  adaptive admin apply -f "$RUN/sync/discovery-disabled.json" > "$RUN/sync/destroy-disable-$(date -u '+%Y%m%dT%H%M%SZ')-$$.txt" 2>&1
 fi
fi
export TF_CLI_CONFIG_FILE="$RUN/terraform.rc" TF_IN_AUTOMATION=1 CHECKPOINT_DISABLE=1
unset TF_LOG TF_LOG_PATH TF_REATTACH_PROVIDERS TF_WORKSPACE TF_DATA_DIR ADAPTIVE_URL TF_CLI_ARGS TF_CLI_ARGS_plan TF_CLI_ARGS_apply TF_CLI_ARGS_show
GUARD="$(cd "$(dirname "$0")" && pwd)/guard.py"
bash "$(dirname "$GUARD")/destroy-generated.sh" plan "$RUN"
STAMP=$(date -u '+%Y%m%dT%H%M%SZ')-$$
mkdir -p "$RUN/destroy/$STAMP"
COUNT=0
# Each fixture owns its isolated state. Dependency cohorts must be ordered with
# endpoint/script/schedule/group fixtures first, masking next, integrations last.
for tier in adaptive_schedule adaptive_script adaptive_group adaptive_endpoint adaptive_dataprotection adaptive_authorization adaptive_resource; do
  for dir in "$RUN"/work/*; do
    test -f "$dir/terraform.tfstate" || continue
    python3 "$GUARD" config "$RUN" "$(basename "$dir")" >/dev/null
    family=$(terraform -chdir="$dir" show -json | jq -er '.values.root_module.resources[0].type // "empty"')
    test "$family" != empty || continue
    [[ "$family" =~ ^adaptive_(resource|endpoint|authorization|group|script|schedule|dataprotection)$ ]] || die "Unsupported family in $dir"
    test "$family" = "$tier" || continue
    fixture=$(basename "$dir")
    state="$RUN/destroy/$STAMP/$fixture-before.json"
    terraform -chdir="$dir" show -json > "$state"
    jq -e '(.values.root_module.child_modules // [] | length) == 0 and all(.values.root_module.resources[]; .mode == "managed" and (.type | test("^adaptive_(resource|endpoint|authorization|group|script|schedule|dataprotection)$")))' "$state" >/dev/null || die "Unsupported state in $fixture"
    jq -e --arg p "$PREFIX-" 'all(.values.root_module.resources[]; ((.values.name // .values.resource // "") | startswith($p)))' "$state" >/dev/null || die "Non-owned object in $fixture"
    plan="$RUN/destroy/$STAMP/$fixture.tfplan"
    terraform -chdir="$dir" plan -destroy -input=false -lock-timeout=30s -out="$plan" -no-color > "$RUN/destroy/$STAMP/$fixture-plan.txt" 2>&1
    terraform -chdir="$dir" show -json "$plan" > "$RUN/destroy/$STAMP/$fixture-plan.json"
    jq -e --arg p "$PREFIX-" 'all(.resource_changes[]?; (.change.actions == ["delete"] or .change.actions == ["no-op"]) and ((.change.before.name // .change.before.resource // "") | startswith($p)))' "$RUN/destroy/$STAMP/$fixture-plan.json" >/dev/null || die "Unsafe destroy plan for $fixture"
    python3 "$GUARD" destroy-plan "$RUN" "$fixture" "$plan" >/dev/null
    COUNT=$((COUNT + 1))
    printf '%s destroy plan verified\n' "$fixture"
    if test "$MODE" = apply; then
      terraform -chdir="$dir" apply -input=false -auto-approve -no-color "$plan" > "$RUN/destroy/$STAMP/$fixture-apply.txt" 2>&1
      terraform -chdir="$dir" show -json > "$RUN/destroy/$STAMP/$fixture-after.json"
      jq -e '(.values.root_module.resources // [] | length) == 0' "$RUN/destroy/$STAMP/$fixture-after.json" >/dev/null || die "State still contains objects: $fixture"
    fi
  done
done
printf '%s fixture destroy plans checked; mode=%s; evidence=%s\n' "$COUNT" "$MODE" "$RUN/destroy/$STAMP"
if test "$MODE" = apply; then
  printf '%s\n' 'Terraform state cleanup verified. Independently verify remote absence and inspect run-owned generated masking authorization/views; these are not removed by policy destroy.' 'Also reconcile any CLI-origin fixture whose import failed; see the run orphan ledger before declaring teardown complete.'
fi
