#!/bin/bash
# Live ignore_changes guard: never serialize a drift fingerprint as a password.
set -euo pipefail
umask 077
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
RUN=${1:?absolute isolated run directory required}
PREFIX=$(jq -er .prefix "$RUN/manifest.json")
R="$ROOT/testing/live/run.sh"
CASE=secret-ignore-changes
NAME="$PREFIX-secret-ignore"
DIR="$RUN/work/$CASE"
attrs=$(jq -nc --arg n "$NAME" '{name:$n,type:"postgres",host:"tf-live.invalid",port:"5432",username:"tf_live",password:"tf-live-ignore-configured-canary",database_name:"tf_live",ssl_mode:"require",tags:["campaign=terraform-live","scenario=ignore-changes"]}')
if test ! -e "$DIR"; then
 bash "$R" fixture "$RUN" "$CASE" adaptive_resource "$attrs"
 bash "$R" tf "$RUN" "$CASE" create-plan 2 plan -input=false -detailed-exitcode -out=create.tfplan
 bash "$R" tf "$RUN" "$CASE" create-apply 0 apply -input=false -auto-approve create.tfplan
fi
jq -e --arg n "$NAME" '.resources[0].instances[0].attributes.name==$n and .resources[0].instances[0].attributes.password=="tf-live-ignore-configured-canary"' "$DIR/terraform.tfstate" >/dev/null
STAMP=$(date -u '+%Y%m%dT%H%M%SZ')-$$
E="$RUN/evidence/secret-ignore-$STAMP"
mkdir "$E"
cp "$DIR/main.tf.json" "$E/original-config.json"
jq '.resources[0].instances[0].attributes.applied_digests' "$DIR/terraform.tfstate" > "$E/baseline.json"
adaptive admin export resources --name "$NAME" --include-secrets > "$E/owned-synthetic-export.yaml" 2> "$E/export.stderr"
ruby -ryaml -rjson - "$E/owned-synthetic-export.yaml" "$E/rotated.json" "$NAME" <<'RUBY'
d=YAML.load(File.read(ARGV[0])); c=d.fetch('configuration'); abort 'wrong owned identity' unless c['name']==ARGV[2] && c['password']=='tf-live-ignore-configured-canary'
c['password']='tf-live-ignore-external-canary'; File.write(ARGV[1],JSON.pretty_generate(d)+"\n")
RUBY
record() { jq -nc --arg c "$CASE" --arg p "$1" --arg s "$2" --arg e "$3" --argjson rc "$4" '{case:$c,phase:$p,status:$s,exit:$rc,evidence:$e}' >> "$RUN/ledger.jsonl"; }
set +e; adaptive admin apply -f "$E/rotated.json" > "$E/external-rotate.txt" 2>&1; rc=$?; set -e
status=FAIL; test "$rc" != 0 || status=ACKNOWLEDGED
record ignore-external-rotate "$status" "$E/external-rotate.txt" "$rc"
test "$rc" = 0
jq '.resource.adaptive_resource.test.lifecycle={ignore_changes:["password"]} | .resource.adaptive_resource.test.tags += ["phase=must-not-write"]' "$E/original-config.json" > "$DIR/next.json"
mv "$DIR/next.json" "$DIR/main.tf.json"
bash "$R" tf "$RUN" "$CASE" ignore-unrelated-plan 2 plan -input=false -detailed-exitcode -out=ignore.tfplan
bash "$R" tf "$RUN" "$CASE" ignore-unrelated-rejected 1 apply -input=false -auto-approve ignore.tfplan
log=$(jq -sr --arg c "$CASE" '[.[]|select(.case==$c and .phase=="ignore-unrelated-rejected")]|last|.evidence' "$RUN/ledger.jsonl")
status=PASS
/usr/bin/grep -Fq ignore_changes "$log" || status=FAIL
record ignore-exact-guard-assertion "$status" "$log" 0
test "$status" = PASS
bash "$R" readback "$RUN" "$CASE" resources "$NAME"
readback=$(jq -sr --arg c "$CASE" '[.[]|select(.case==$c and .phase=="readback")]|last|.evidence' "$RUN/ledger.jsonl")
set +e
ruby -ryaml -rjson - "$E/owned-synthetic-export.yaml" "$readback" > "$E/remote-tags-unchanged.json" <<'RUBY'
before=YAML.load(File.read(ARGV[0])); after=YAML.load(File.read(ARGV[1])); good=before.fetch('userTags',[]).sort==after.fetch('userTags',[]).sort
puts JSON.generate({remote_tags_unchanged:good}); exit(good ? 0 : 1)
RUBY
ASSERT_RC=$?
set -e
status=FAIL; test "$ASSERT_RC" != 0 || status=PASS
record ignore-no-metadata-mutation "$status" "$E/remote-tags-unchanged.json" "$ASSERT_RC"
test "$ASSERT_RC" = 0
for cycle in 1 2; do
 bash "$R" tf "$RUN" "$CASE" "ignore-persisted-refresh-$cycle" 0 apply -refresh-only -input=false -auto-approve
 status=PASS
 jq -e --slurpfile b "$E/baseline.json" '.resources[0].instances[0].attributes.applied_digests==$b[0]' "$DIR/terraform.tfstate" > "$E/baseline-$cycle-unchanged.json" || status=FAIL
 record "ignore-baseline-$cycle" "$status" "$E/baseline-$cycle-unchanged.json" 0
 test "$status" = PASS
done
# Remove the ignore rule and restore exact desired input. The original secret
# must be explicitly supplied; no secret-preserving backend patch is assumed.
cp "$E/original-config.json" "$DIR/main.tf.json"
bash "$R" tf "$RUN" "$CASE" ignore-remediate-plan 2 plan -input=false -detailed-exitcode -out=remediate.tfplan
bash "$R" tf "$RUN" "$CASE" ignore-remediate-apply 0 apply -input=false -auto-approve remediate.tfplan
bash "$R" readback "$RUN" "$CASE" resources "$NAME"
bash "$R" tf "$RUN" "$CASE" ignore-restored-noop 0 plan -input=false -detailed-exitcode
printf '%s\n' 'Ignored secret drift blocked the unrelated write; baseline survived two refreshes; explicit desired credential restored. Fixture retained.'
