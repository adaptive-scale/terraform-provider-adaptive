#!/bin/bash
# Prepare only isolated runtime fixtures; preserves partial failures and all state.
set -euo pipefail
umask 077
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
RUN=${1:?absolute isolated run directory required}
CLUSTER=${2:-k8s-local}
R="$ROOT/testing/live/run.sh"
PREFIX=$(jq -er .prefix "$RUN/manifest.json")
USER_EMAIL=$(adaptive whoami -o json | jq -er 'select(.URL=="http://localhost:8080")|.Identity.User.Email|select(length>0)')
bash "$ROOT/testing/live/infra.sh" "$RUN"
docker exec -i "$PREFIX-runtime-postgres-db" psql -v ON_ERROR_STOP=1 -U tf_live -d tf_live_test < "$ROOT/testing/live/masking-fixture.sql" > "$RUN/evidence/seed-all-mask-types.txt"
prepare() {
 local fixture=$1 family=$2 attrs=$3 kind=$4 name=$5
 if test ! -f "$RUN/work/$fixture/main.tf.json"; then bash "$R" fixture "$RUN" "$fixture" "$family" "$attrs"; fi
 if test -f "$RUN/work/$fixture/terraform.tfstate" && jq -e '[.resources[]?.instances[]?]|length>0' "$RUN/work/$fixture/terraform.tfstate" >/dev/null; then
  printf '%s retained existing state; no automatic replacement\n' "$fixture"
  bash "$R" readback "$RUN" "$fixture" "$kind" "$name"
  return
 fi
 bash "$R" tf "$RUN" "$fixture" create-plan 2 plan -input=false -detailed-exitcode -out=create.tfplan
 bash "$R" tf "$RUN" "$fixture" create-apply 0 apply -input=false -auto-approve create.tfplan
 bash "$R" readback "$RUN" "$fixture" "$kind" "$name"
}
attrs=$(jq -nc --arg n "$PREFIX-runtime-postgres" --arg h "$(jq -r .host "$RUN/infra/postgres-target.json")" '{name:$n,type:"postgres",host:$h,port:"5432",username:"tf_live",password:"tf-live-runtime-synthetic-password",database_name:"tf_live_test",ssl_mode:"disable",tags:["campaign=terraform-live","purpose=runtime"]}')
prepare runtime-postgres adaptive_resource "$attrs" resources "$PREFIX-runtime-postgres"
FAILED=0
for transport in cli client; do
 attrs=$(jq -nc --arg n "$PREFIX-postgres-$transport" --arg r "$PREFIX-runtime-postgres" --arg t "$transport" --arg c "$CLUSTER" --arg u "$USER_EMAIL" '{name:$n,resource:$r,type:$t,cluster:$c,users:[$u],tags:["campaign=terraform-live","purpose=runtime"],is_jit_enabled:false,disable_web_cli:false,disable_data_studio:false,disable_output_capture:false,jit_approvers:[],script_only_access:false}')
 # An error remains in the ledger; continue to enumerate the second transport.
 set +e
 (set -e; prepare "endpoint-$transport" adaptive_endpoint "$attrs" endpoints "$PREFIX-postgres-$transport")
 rc=$?
 set -e
 if test "$rc" != 0; then FAILED=$((FAILED+1)); printf '%s failed; inspect readiness and repair dependency, never auto-replace tainted state\n' "$transport"; fi
done
attrs=$(jq -nc --arg r "$PREFIX-runtime-postgres" '{resource:$r,scoped:true,masks:[{database_name:"tf_live_test",tables:[{schema:"public",table_name:"customers",masked:true,masked_columns:(["name","email","ssn","phone","cc","aadhar","first4","last4","redact","hash","disable"]|map({column_name:("mask_"+.),masking_type:.}))}]}]}')
prepare dataprotection adaptive_dataprotection "$attrs" dataprotection "$PREFIX-runtime-postgres"
printf '%s\n' 'Runtime fixtures retained. Inspect endpoint readiness before running endpoints.sh or SQL checks.'
test "$FAILED" = 0
