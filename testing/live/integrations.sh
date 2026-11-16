#!/bin/bash
# Broad live configuration coverage; synthetic targets are never connected/synced.
set -euo pipefail
umask 077
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
RUN=${1:?absolute isolated run directory required}
R="$ROOT/testing/live/run.sh"
PREFIX=$(jq -er .prefix "$RUN/manifest.json")
CASES="$ROOT/testing/live/integration-cases.json"
test "$(jq '.cases|length' "$CASES")" = 82
test "$(jq '[.cases[].type]|unique|length' "$CASES")" = 82
if test -f "$RUN/inventory/integration-cases.json"; then
 cmp -s "$CASES" "$RUN/inventory/integration-cases.json" || { printf '%s\n' 'Case inventory changed; start a new run instead of reusing historical phase results.' >&2; exit 1; }
else
 cp "$CASES" "$RUN/inventory/integration-cases.json"
fi
passed() { jq -es --arg c "$fixture" --arg p "$1" '[.[] | select(.case==$c and .phase==$p)] | last | .status=="PASS"' "$RUN/ledger.jsonl" >/dev/null; }
step() { local phase=$1 expected=$2; shift 2; if ! passed "$phase"; then bash "$R" tf "$RUN" "$fixture" "$phase" "$expected" "$@"; fi; }
cohort() {
  local typ=$1 attrs=$2 name="$PREFIX-integration-$1"
  fixture="integration-${typ//[^a-z0-9_-]/-}"
  attrs=$(jq -nc --argjson a "$attrs" --arg n "$name" --arg t "$typ" '$a+{name:$n,type:$t,tags:["campaign=terraform-live","phase=create"]}')
  local dir="$RUN/work/$fixture"
  if test ! -f "$dir/main.tf.json"; then bash "$R" fixture "$RUN" "$fixture" adaptive_resource "$attrs"; fi
  actual=$(jq -cS '.resource.adaptive_resource.test | del(.tags)' "$dir/main.tf.json")
  expected=$(jq -cS 'del(.tags)' <<< "$attrs")
  test "$actual" = "$expected" || { printf '%s\n' "$fixture configuration changed; review before resuming." >&2; return 1; }
  cp "$dir/main.tf.json" "$RUN/evidence/$fixture-current-config-$(date -u '+%Y%m%dT%H%M%SZ')-$$-$RANDOM.tf.json"
  step validate 0 validate
  step create-plan 2 plan -input=false -detailed-exitcode -out=create.tfplan
  step create-apply 0 apply -input=false -auto-approve create.tfplan
  if ! passed readback; then bash "$R" readback "$RUN" "$fixture" resources "$name"; fi
  step create-noop 0 plan -input=false -detailed-exitcode
  step refresh-1 0 apply -refresh-only -input=false -auto-approve
  step refresh-2 0 apply -refresh-only -input=false -auto-approve
  step refreshed-noop 0 plan -input=false -detailed-exitcode
  if ! passed update-apply; then
    cp "$dir/main.tf.json" "$RUN/evidence/$fixture-before-update.tf.json"
    jq '.resource.adaptive_resource.test.tags=["campaign=terraform-live","phase=updated"]' "$dir/main.tf.json" > "$dir/next.json"
    mv "$dir/next.json" "$dir/main.tf.json"
  fi
  step update-plan 2 plan -input=false -detailed-exitcode -out=update.tfplan
  step update-apply 0 apply -input=false -auto-approve update.tfplan
  # Re-read even when create's readback already passed.
  if ! passed updated-readback; then
    bash "$R" readback "$RUN" "$fixture" resources "$name"
    jq -cs --arg c "$fixture" '[.[]|select(.case==$c and .phase=="readback" and .status=="PASS")]|last|.phase="updated-readback"' "$RUN/ledger.jsonl" >> "$RUN/ledger.jsonl"
  fi
  step updated-noop 0 plan -input=false -detailed-exitcode
}
failed=0
while IFS= read -r entry; do
  typ=$(jq -r .type <<< "$entry")
  attrs=$(jq -c .attributes <<< "$entry")
  # Explicit subshell isolates a case failure without disabling errexit inside it.
  set +e
  (set -e; cohort "$typ" "$attrs")
  rc=$?
  set -e
  if test "$rc" != 0; then failed=$((failed+1)); printf '%s cohort FAILED; retained state/evidence, continuing inventory\n' "$typ"; fi
done < <(jq -c '.cases[]' "$CASES")
printf 'Integration cohort failures: %s (82 enumerated; see per-phase ledger)\n' "$failed"
test "$failed" = 0
