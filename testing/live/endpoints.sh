#!/bin/bash
# Recover recorded endpoints after their external cluster dependency is repaired.
# Only untaints an exact run-owned ID independently observed ready; never replaces.
set -euo pipefail
umask 077
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
RUN=${1:?absolute isolated run directory required}
PREFIX=$(jq -er .prefix "$RUN/manifest.json")
R="$ROOT/testing/live/run.sh"
adaptive whoami -o json | jq -e '.URL=="http://localhost:8080" and .CredentialSource=="saved context"' >/dev/null
export TF_CLI_CONFIG_FILE="$RUN/terraform.rc" TF_IN_AUTOMATION=1 CHECKPOINT_DISABLE=1
unset ADAPTIVE_SVC_TOKEN ADAPTIVE_URL ADAPTIVE_PROFILE TF_LOG TF_LOG_PATH TF_REATTACH_PROVIDERS TF_CLI_ARGS TF_CLI_ARGS_apply
for transport in cli client; do
 fixture="endpoint-$transport"
 dir="$RUN/work/$fixture"
 name="$PREFIX-postgres-$transport"
 python3 "$ROOT/testing/live/guard.py" config "$RUN" "$fixture" >/dev/null
 jq -e --arg n "$name" '.resources[0].type=="adaptive_endpoint" and .resources[0].instances[0].attributes.name==$n and (.resources[0].instances[0].attributes.id|length>0)' "$dir/terraform.tfstate" >/dev/null
 evidence="$RUN/evidence/$fixture-recovery-ready-$(date -u '+%Y%m%dT%H%M%SZ')-$$.txt"
 adaptive admin endpoint list --endpoint-type postgres | python3 -c 'import sys; name=sys.argv[1]; rows=[s for s in sys.stdin if len(s.split("│"))>5 and s.split("│")[1].strip()==name]; print("".join(rows)); raise SystemExit(0 if len(rows)==1 and rows[0].split("│")[4].strip()=="created" else 1)' "$name" > "$evidence"
 if jq -e '.resources[0].instances[0].status=="tainted"' "$dir/terraform.tfstate" >/dev/null; then
  cp "$dir/terraform.tfstate" "$RUN/evidence/$fixture-before-untaint-$(date -u '+%Y%m%dT%H%M%SZ')-$$.json"
  terraform -chdir="$dir" untaint -no-color adaptive_endpoint.test > "$RUN/evidence/$fixture-untaint.txt" 2>&1
  jq -e '.resources[0].instances[0].status!="tainted"' "$dir/terraform.tfstate" >/dev/null
 fi
 jq -nc --arg c "$fixture" --arg e "$evidence" '{case:$c,phase:"recover-recorded-ready-object",status:"PASS",exit:0,evidence:$e}' >> "$RUN/ledger.jsonl"
 bash "$R" tf "$RUN" "$fixture" recovered-refresh 0 apply -refresh-only -input=false -auto-approve
 bash "$R" tf "$RUN" "$fixture" recovered-noop 0 plan -input=false -detailed-exitcode
 cp "$dir/main.tf.json" "$RUN/evidence/$fixture-policy-baseline.json"
 # All policy changes are confined to this run's endpoint, with the current user
 # as the sole approver. They are restored before runtime acceptance continues.
 jq '.resource.adaptive_endpoint.test += {is_jit_enabled:true,jit_approvers:["dhaivat@adaptive.live"],jit_multi_approver:true,jit_total_approvers:1,max_jit_request_duration:"30m",jit_mode:"both",script_only_access:true,disable_web_cli:true,disable_data_studio:true,disable_output_capture:true,auto_approval:false}' "$dir/main.tf.json" > "$dir/next.json"
 mv "$dir/next.json" "$dir/main.tf.json"
 bash "$R" tf "$RUN" "$fixture" policy-plan 2 plan -input=false -detailed-exitcode -out=policy.tfplan
 bash "$R" tf "$RUN" "$fixture" policy-apply 0 apply -input=false -auto-approve policy.tfplan
 bash "$R" readback "$RUN" "$fixture" endpoints "$name"
 bash "$R" tf "$RUN" "$fixture" policy-noop 0 plan -input=false -detailed-exitcode
 jq '.resource.adaptive_endpoint.test += {is_jit_enabled:false,jit_approvers:[],jit_multi_approver:false,jit_total_approvers:0,max_jit_request_duration:"",jit_mode:"session",script_only_access:false,disable_web_cli:false,disable_data_studio:false,disable_output_capture:false,auto_approval:false}' "$dir/main.tf.json" > "$dir/next.json"
 mv "$dir/next.json" "$dir/main.tf.json"
 bash "$R" tf "$RUN" "$fixture" policy-clear-plan 2 plan -input=false -detailed-exitcode -out=policy-clear.tfplan
 bash "$R" tf "$RUN" "$fixture" policy-clear-apply 0 apply -input=false -auto-approve policy-clear.tfplan
 bash "$R" readback "$RUN" "$fixture" endpoints "$name"
 bash "$R" tf "$RUN" "$fixture" policy-clear-noop 0 plan -input=false -detailed-exitcode
 # Immutable transport transition: plan only; restore immediately afterwards.
 cp "$dir/main.tf.json" "$RUN/evidence/$fixture-final-policy.json"
 other=cli; test "$transport" != cli || other=client
 jq --arg t "$other" '.resource.adaptive_endpoint.test.type=$t' "$dir/main.tf.json" > "$dir/next.json"
 mv "$dir/next.json" "$dir/main.tf.json"
 set +e
 bash "$R" tf "$RUN" "$fixture" transport-transition-rejected 1 plan -input=false -detailed-exitcode
 rc=$?
 set -e
 cp "$RUN/evidence/$fixture-final-policy.json" "$dir/main.tf.json"
 test "$rc" = 0
 bash "$R" tf "$RUN" "$fixture" final-noop 0 plan -input=false -detailed-exitcode
done
