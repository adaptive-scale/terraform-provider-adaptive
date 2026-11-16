#!/bin/bash
# Negative import across all registered families; no remote mutation is invoked.
set -euo pipefail
umask 077
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
RUN=${1:?absolute isolated run directory required}
PREFIX=$(jq -er .prefix "$RUN/manifest.json")
R="$ROOT/testing/live/run.sh"
for family in resource endpoint authorization group script schedule dataprotection; do
 fixture="negative-import-$family"
 name="$PREFIX-missing-$family"
 case "$family" in
  resource) attrs=$(jq -nc --arg n "$name" '{name:$n,type:"postgres",host:"tf-live.invalid",port:"5432",username:"tf_live",database_name:"tf_live"}') ;;
  endpoint) attrs=$(jq -nc --arg n "$name" --arg r "$PREFIX-runtime-postgres" '{name:$n,resource:$r,type:"cli"}') ;;
  authorization) attrs=$(jq -nc --arg n "$name" '{name:$n,resource_type:"postgres",permissions:"allow: []"}') ;;
  group) attrs=$(jq -nc --arg n "$name" '{name:$n,members:[],endpoints:[]}') ;;
  script) attrs=$(jq -nc --arg n "$name" --arg ep "$PREFIX-postgres-cli" '{name:$n,endpoint:$ep,command:"SELECT 1;"}') ;;
  schedule) attrs=$(jq -nc --arg n "$name" '{name:$n,schedule_type:"everyday",is_active:false}') ;;
  dataprotection) attrs=$(jq -nc --arg r "$PREFIX-runtime-postgres" '{resource:$r,scoped:true}') ;;
 esac
 if test ! -f "$RUN/work/$fixture/main.tf.json"; then bash "$R" fixture "$RUN" "$fixture" "adaptive_$family" "$attrs"; fi
 id=$(python3 -c 'import uuid; print(uuid.uuid4())')
 if test "$family" = endpoint; then id=$(python3 -c 'import secrets,string; print("".join(secrets.choice(string.ascii_lowercase+string.digits) for _ in range(16)))'); fi
 printf '%s\n' "$id" > "$RUN/evidence/$fixture-requested-id.txt"
 bash "$R" tf "$RUN" "$fixture" missing-import 1 import -input=false "adaptive_$family.test" "$id"
 # Distinguish a proper not-found import from an authentication/guard failure.
 log=$(jq -sr --arg c "$fixture" '[.[]|select(.case==$c and .phase=="missing-import")]|last|.evidence' "$RUN/ledger.jsonl")
 status=PASS
 if ! /usr/bin/grep -Eq 'Cannot import non-existent remote object|non-existent remote object|does not exist' "$log"; then status=FAIL; fi
 if test -f "$RUN/work/$fixture/terraform.tfstate"; then
  if ! jq -e '[.resources[]?.instances[]?]|length==0' "$RUN/work/$fixture/terraform.tfstate" >/dev/null; then status=FAIL; fi
 fi
 jq -nc --arg c "$fixture" --arg s "$status" --arg e "$log" '{case:$c,phase:"missing-import-semantic-assertion",status:$s,exit:(if $s=="PASS" then 0 else 1 end),evidence:$e}' >> "$RUN/ledger.jsonl"
 test "$status" = PASS
done
printf '%s\n' 'Seven missing-ID imports rejected with nonexistent-object diagnostics and no adopted objects.'
