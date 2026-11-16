#!/bin/bash
# Policy persistence only; live masking is a separate, cluster-dependent check.
set -euo pipefail
umask 077
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
RUN=${1:?absolute isolated run directory required}
R="$ROOT/testing/live/run.sh"
DIR="$RUN/work/dataprotection"
BASE="$RUN/evidence/dataprotection-original.tf.json"
test -f "$BASE" || cp "$DIR/main.tf.json" "$BASE"
stage() {
 local name=$1 expected=$2 filter=$3
 jq "$filter" "$BASE" > "$DIR/next.json"
 mv "$DIR/next.json" "$DIR/main.tf.json"
 cp "$DIR/main.tf.json" "$RUN/evidence/dataprotection-$name.tf.json"
 bash "$R" tf "$RUN" dataprotection "$name-plan" "$expected" plan -input=false -detailed-exitcode -out="$name.tfplan" || return $?
 if test "$expected" = 2; then
  bash "$R" tf "$RUN" dataprotection "$name-apply" 0 apply -input=false -auto-approve "$name.tfplan"
  resource=$(jq -r '.resource.adaptive_dataprotection.test.resource' "$DIR/main.tf.json")
  bash "$R" readback "$RUN" dataprotection dataprotection "$resource"
  bash "$R" tf "$RUN" dataprotection "$name-noop" 0 plan -input=false -detailed-exitcode
 fi
}
bash "$R" tf "$RUN" dataprotection refresh-1 0 apply -refresh-only -input=false -auto-approve
bash "$R" tf "$RUN" dataprotection refresh-2 0 apply -refresh-only -input=false -auto-approve
# Preserve a failed ordering assertion but continue independent mutation checks.
ORDERING_FAILED=0
if ! stage reorder 0 '.resource.adaptive_dataprotection.test.masks[0].tables[0].masked_columns |= reverse'; then
 ORDERING_FAILED=1
 printf '%s\n' 'Mask-column reorder produced a diff; recorded as a failure, not treated as canonical.'
fi
stage unscoped 2 '.resource.adaptive_dataprotection.test.scoped=false'
stage remove-columns 2 '.resource.adaptive_dataprotection.test.scoped=false | .resource.adaptive_dataprotection.test.masks[0].tables[0].masked_columns=[]'
stage remove-tables 2 '.resource.adaptive_dataprotection.test.scoped=false | .resource.adaptive_dataprotection.test.masks[0].tables=[]'
stage remove-masks 2 '.resource.adaptive_dataprotection.test.scoped=false | .resource.adaptive_dataprotection.test.masks=[]'
stage restore-all-types 2 '.'
cp "$BASE" "$DIR/main.tf.json"
printf '%s\n' 'All 11 mask types retained in the final policy. Runtime enforcement not established by configuration checks.'
test "$ORDERING_FAILED" = 0
