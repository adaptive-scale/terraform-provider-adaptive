#!/bin/bash
# Live secret semantics; synthetic credentials, independent CLI-origin fixture.
set -euo pipefail
umask 077
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
RUN=${1:?absolute isolated run directory required}
R="$ROOT/testing/live/run.sh"
PREFIX=$(jq -er .prefix "$RUN/manifest.json")
FIXTURE=secret-cli-origin
NAME="$PREFIX-secret-cli-origin"
DIR="$RUN/work/$FIXTURE"
# Refuse a second run: resume from retained evidence, do not blindly recreate.
test ! -e "$DIR" || { printf '%s\n' 'Secret cohort already exists; inspect its ledger before resuming.' >&2; exit 1; }
attrs=$(jq -nc --arg n "$NAME" '{name:$n,type:"postgres",host:"tf-live.invalid",port:"5432",username:"tf_live_test",database_name:"tf_live_test",ssl_mode:"require",tags:["campaign=terraform-live","phase=import"]}')
bash "$R" fixture "$RUN" "$FIXTURE" adaptive_resource "$attrs"
mkdir -p "$RUN/cli-origin"
DOC="$RUN/cli-origin/$FIXTURE.json"
jq -n --arg n "$NAME" '{type:"resource",configuration:{name:$n,type:"postgres",hostname:"tf-live.invalid",port:"5432",username:"tf_live_test",password:"tf-live-synthetic-secret-origin-v1",databaseName:"tf_live_test",sslMode:"require",rootCert:"",crtText:"",keyText:""},userTags:["campaign=terraform-live","phase=import"]}' > "$DOC"
record() {
  jq -nc --arg at "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" --arg case "$FIXTURE" --arg phase "$1" --arg status "$2" --arg evidence "$3" --argjson exit "$4" '{at:$at,case:$case,phase:$phase,status:$status,exit:$exit,evidence:$evidence}' >> "$RUN/ledger.jsonl"
}
assert_state() {
  local phase=$1 expression=$2 log="$RUN/evidence/$FIXTURE-$1-assertion.json"
  if jq -e "$expression" "$DIR/terraform.tfstate" > "$log"; then record "$phase" PASS "$log" 0; else record "$phase" FAIL "$log" 1; return 1; fi
}
configure() {
  local stage=$1 replacement=$2
  cp "$DIR/main.tf.json" "$RUN/evidence/$FIXTURE-before-$stage.tf.json"
  jq --argjson a "$replacement" '.resource.adaptive_resource.test=$a' "$DIR/main.tf.json" > "$DIR/next.json"
  mv "$DIR/next.json" "$DIR/main.tf.json"
  cp "$DIR/main.tf.json" "$RUN/evidence/$FIXTURE-$stage.tf.json"
}
cli_apply() {
  local phase=$1 log="$RUN/evidence/$FIXTURE-$1.txt" rc status
  set +e
  adaptive admin apply -f "$DOC" > "$log" 2>&1
  rc=$?
  set -e
  status=FAIL; test "$rc" = 0 && status=PASS
  record "$phase" "$status" "$log" "$rc"
  test "$rc" = 0
}
# Persist intended ownership BEFORE external creation, including crash recovery.
jq -nc --arg name "$NAME" --arg doc "$DOC" --arg fixture "$FIXTURE" '{name:$name,document:$doc,fixture:$fixture,origin:"adaptive-cli",cleanup:"Terraform after successful import; otherwise explicit CLI delete of this document"}' >> "$RUN/cli-origin/ownership.jsonl"
cli_apply cli-create
bash "$R" readback "$RUN" "$FIXTURE" resources "$NAME"
adaptive admin list-resources | python3 -c 'import sys,re; name=sys.argv[1]; ids=[]
for line in sys.stdin:
 cells=line.split("│")
 if len(cells)>3 and cells[1].strip()==name:
  m=re.fullmatch(r"/resources/([0-9a-f-]{36})",cells[3].strip())
  if m: ids.append(m.group(1))
if len(ids)!=1: raise SystemExit("expected exactly one run-owned resource ID")
print(ids[0])' "$NAME" > "$RUN/cli-origin/$FIXTURE.id"
ID=$(<"$RUN/cli-origin/$FIXTURE.id")
bash "$R" tf "$RUN" "$FIXTURE" import 0 import -input=false adaptive_resource.test "$ID"
assert_state imported-secret-withheld '.resources[0].instances[0].attributes | (.password == null or .password == "") and (.applied_digests.password | length > 0)'
bash "$R" tf "$RUN" "$FIXTURE" imported-refresh-1 0 apply -refresh-only -input=false -auto-approve
bash "$R" tf "$RUN" "$FIXTURE" imported-refresh-2 0 apply -refresh-only -input=false -auto-approve
configure omit-update "$(jq -nc --argjson a "$attrs" '$a | .tags=["campaign=terraform-live","phase=omit-update"]')"
bash "$R" tf "$RUN" "$FIXTURE" omit-update-plan 2 plan -input=false -detailed-exitcode -out=omit.tfplan
bash "$R" tf "$RUN" "$FIXTURE" omit-update-rejected 1 apply -input=false -auto-approve omit.tfplan
# A failed apply is only the intended rejection when the exact guard fired.
python3 - "$RUN" "$FIXTURE" <<'PY'
import json,pathlib,sys
run=pathlib.Path(sys.argv[1]); case=sys.argv[2]
rows=[json.loads(s) for s in (run/'ledger.jsonl').read_text().splitlines()]
row=next(x for x in reversed(rows) if x['case']==case and x['phase']=='omit-update-rejected')
s=pathlib.Path(row['evidence']).read_text()
good='server-held values for password' in ' '.join(s.split())
with (run/'ledger.jsonl').open('a') as f:
 f.write(json.dumps({'case':case,'phase':'omit-update-exact-guard-assertion','status':'PASS' if good else 'FAIL','exit':0 if good else 1,'evidence':row['evidence']})+'\n')
raise SystemExit(0 if good else 1)
PY
bash "$R" readback "$RUN" "$FIXTURE" resources "$NAME"
configure resupply "$(jq -nc --argjson a "$attrs" '$a | .password="tf-live-synthetic-secret-origin-v1" | .tags=["campaign=terraform-live","phase=resupply"]')"
bash "$R" tf "$RUN" "$FIXTURE" resupply-plan 2 plan -input=false -detailed-exitcode -out=resupply.tfplan
bash "$R" tf "$RUN" "$FIXTURE" resupply-apply 0 apply -input=false -auto-approve resupply.tfplan
bash "$R" readback "$RUN" "$FIXTURE" resources "$NAME"
bash "$R" tf "$RUN" "$FIXTURE" resupply-noop 0 plan -input=false -detailed-exitcode
jq '.resources[0].instances[0].attributes.applied_digests' "$DIR/terraform.tfstate" > "$RUN/evidence/$FIXTURE-baseline.json"
# Independent out-of-band rotation via Adaptive CLI, keeping the same object.
jq '.configuration.password="tf-live-synthetic-secret-external-v2" | .userTags=["campaign=terraform-live","phase=resupply"]' "$DOC" > "$RUN/cli-origin/rotated.json"
cp "$RUN/cli-origin/rotated.json" "$DOC"
cli_apply external-rotate
bash "$R" readback "$RUN" "$FIXTURE" resources "$NAME"
for cycle in 1 2; do
  bash "$R" tf "$RUN" "$FIXTURE" "rotation-refresh-$cycle" 0 apply -refresh-only -input=false -auto-approve
  if jq -e --slurpfile b "$RUN/evidence/$FIXTURE-baseline.json" '.resources[0].instances[0].attributes | .applied_digests == $b[0] and .password != "tf-live-synthetic-secret-origin-v1" and .password != "tf-live-synthetic-secret-external-v2" and (.password | length > 0)' "$DIR/terraform.tfstate" > "$RUN/evidence/$FIXTURE-rotation-$cycle-assertion.json"; then
    record "rotation-persisted-baseline-$cycle" PASS "$RUN/evidence/$FIXTURE-rotation-$cycle-assertion.json" 0
  else
    record "rotation-persisted-baseline-$cycle" FAIL "$RUN/evidence/$FIXTURE-rotation-$cycle-assertion.json" 1
    exit 1
  fi
done
bash "$R" tf "$RUN" "$FIXTURE" remediate-plan 2 plan -input=false -detailed-exitcode -out=remediate.tfplan
bash "$R" tf "$RUN" "$FIXTURE" remediate-apply 0 apply -input=false -auto-approve remediate.tfplan
bash "$R" readback "$RUN" "$FIXTURE" resources "$NAME"
bash "$R" tf "$RUN" "$FIXTURE" remediate-noop 0 plan -input=false -detailed-exitcode
configure clear "$(jq -nc --argjson a "$attrs" '$a | .password="" | .tags=["campaign=terraform-live","phase=clear"]')"
bash "$R" tf "$RUN" "$FIXTURE" clear-plan 2 plan -input=false -detailed-exitcode -out=clear.tfplan
bash "$R" tf "$RUN" "$FIXTURE" clear-apply 0 apply -input=false -auto-approve clear.tfplan
bash "$R" readback "$RUN" "$FIXTURE" resources "$NAME"
assert_state confirmed-clear '.resources[0].instances[0].attributes | .password == "" and ((.applied_digests // {}) | has("password") | not)'
configure omit-after-clear "$(jq -nc --argjson a "$attrs" '$a | .tags=["campaign=terraform-live","phase=omit-after-clear"]')"
bash "$R" tf "$RUN" "$FIXTURE" omit-after-clear-plan 2 plan -input=false -detailed-exitcode -out=omit-after-clear.tfplan
bash "$R" tf "$RUN" "$FIXTURE" omit-after-clear-apply 0 apply -input=false -auto-approve omit-after-clear.tfplan
bash "$R" readback "$RUN" "$FIXTURE" resources "$NAME"
bash "$R" tf "$RUN" "$FIXTURE" final-noop 0 plan -input=false -detailed-exitcode
printf '%s\n' 'Secret cohort retained for manual validation. No objects destroyed.'
