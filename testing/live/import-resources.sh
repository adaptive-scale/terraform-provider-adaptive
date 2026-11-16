#!/bin/bash
# Adaptive-origin clones from ONLY this run's synthetic resource fixtures.
set -euo pipefail
umask 077
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
RUN=${1:?absolute isolated run directory required}
R="$ROOT/testing/live/run.sh"
PREFIX=$(jq -er .prefix "$RUN/manifest.json")
TOOL="$RUN/bin/fixturetool"
test -x "$TOOL"
mkdir -p "$RUN/cli-origin"
passed() { jq -es --arg c "$fixture" --arg p "$1" '[.[]|select(.case==$c and .phase==$p)]|last|.status=="PASS"' "$RUN/ledger.jsonl" >/dev/null; }
record() { jq -nc --arg at "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" --arg c "$fixture" --arg p "$1" --arg s "$2" --arg e "$3" --argjson rc "$4" '{at:$at,case:$c,phase:$p,status:$s,exit:$rc,evidence:$e}' >> "$RUN/ledger.jsonl"; }
cohort() {
 local typ=$1 source="integration-${1//[^a-z0-9_-]/-}"
 fixture="import-${typ//[^a-z0-9_-]/-}"
 local old="$PREFIX-integration-$typ" name="$PREFIX-import-$typ" doc="$RUN/cli-origin/$fixture.json" exported="$RUN/cli-origin/$fixture-source.yaml"
 local sourceconfig="$RUN/work/$source/main.tf.json" dir="$RUN/work/$fixture"
 test -f "$sourceconfig" || return 1
 if ! jq -es --arg c "$source" 'any(.[];.case==$c and .phase=="create-apply" and .status=="PASS")' "$RUN/ledger.jsonl" >/dev/null; then record prerequisite BLOCKED "$sourceconfig" 1; return 1; fi
 if test ! -f "$dir/main.tf.json"; then
  attrs=$(jq -c --arg n "$name" '.resource.adaptive_resource.test | .name=$n' "$sourceconfig")
  bash "$R" fixture "$RUN" "$fixture" adaptive_resource "$attrs"
 fi
 if test ! -f "$doc"; then
  # No pre-existing objects: the exact old name must belong to this run and
  # source state must identify it before any include-secrets export is allowed.
  jq -e --arg n "$old" '.resources[0].instances[0].attributes.name==$n' "$RUN/work/$source/terraform.tfstate" >/dev/null
  adaptive admin export resources --name "$old" --include-secrets > "$exported" 2> "$RUN/evidence/$fixture-export-stderr.txt"
  "$TOOL" clone-resource "$exported" "$old" "$name" "$doc" > "$RUN/evidence/$fixture-clone-transform.txt"
 fi
 if ! passed cli-create; then
  jq -nc --arg name "$name" --arg doc "$doc" --arg fixture "$fixture" '{name:$name,document:$doc,fixture:$fixture,origin:"adaptive-cli",cleanup:"Terraform after successful import; otherwise explicit CLI delete"}' >> "$RUN/cli-origin/ownership.jsonl"
  local log="$RUN/evidence/$fixture-cli-create.txt" rc status
  set +e; adaptive admin apply -f "$doc" > "$log" 2>&1; rc=$?; set -e
  status=FAIL; test "$rc" = 0 && status=PASS
  record cli-create "$status" "$log" "$rc"
  test "$rc" = 0
 fi
 bash "$R" readback "$RUN" "$fixture" resources "$name"
 if test ! -f "$RUN/cli-origin/$fixture.id"; then
  adaptive admin list-resources | python3 -c 'import sys,re; name=sys.argv[1]; ids=[]
for line in sys.stdin:
 c=line.split("│")
 if len(c)>3 and c[1].strip()==name:
  m=re.fullmatch(r"/resources/([0-9a-f-]{36})",c[3].strip())
  if m: ids.append(m.group(1))
if len(ids)!=1: raise SystemExit("expected exactly one owned resource identity")
print(ids[0])' "$name" > "$RUN/cli-origin/$fixture.id"
 fi
 local id=$(<"$RUN/cli-origin/$fixture.id")
 if ! passed import; then bash "$R" tf "$RUN" "$fixture" import 0 import -input=false adaptive_resource.test "$id"; fi
 for cycle in 1 2; do
  if ! passed "import-refresh-$cycle"; then bash "$R" tf "$RUN" "$fixture" "import-refresh-$cycle" 0 apply -refresh-only -input=false -auto-approve; fi
 done
 # Explicit configured credentials are intentionally reapplied after import.
 # This is reconciliation, not proof that import recovered plaintext secrets.
 if ! passed reconcile-apply; then
  bash "$R" tf "$RUN" "$fixture" reconcile-plan 0 plan -input=false -out=reconcile.tfplan
  bash "$R" tf "$RUN" "$fixture" reconcile-apply 0 apply -input=false -auto-approve reconcile.tfplan
 fi
 bash "$R" readback "$RUN" "$fixture" resources "$name"
 if ! passed import-noop; then bash "$R" tf "$RUN" "$fixture" import-noop 0 plan -input=false -detailed-exitcode; fi
}
failed=0
while IFS= read -r typ; do
 set +e; (set -e; cohort "$typ"); rc=$?; set -e
 if test "$rc" != 0; then failed=$((failed+1)); printf '%s import cohort FAILED; preserving evidence\n' "$typ"; fi
done < <(jq -r '.cases[].type' "$ROOT/testing/live/integration-cases.json")
printf 'Adaptive-origin import cohort failures: %s (82 enumerated)\n' "$failed"
test "$failed" = 0
