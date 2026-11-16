#!/bin/bash
# Per-key known-empty provenance plus null/omission pre-write protection.
set -Eeuo pipefail
umask 077
# Pure assertion code is executed here and by the offline regression tests.
contract() {
 python3 - "$@" <<'SECRET_CONTRACT_PY'
import json, pathlib, sys

def attributes(state):
    resources = [r for r in state['resources'] if r.get('mode', 'managed') == 'managed']
    assert len(resources) == 1 and resources[0]['type'] == 'adaptive_resource'
    assert resources[0]['name'] == 'test' and len(resources[0]['instances']) == 1
    return resources[0]['instances'][0]['attributes']

def check(mode, original, current, log=''):
    if mode.startswith('remote-'):
        before, now = original['configuration'], current['configuration']
        errors = []
        if (before['name'], before['type']) != (now['name'], now['type']):
            errors.append('remote owned identity changed')
        if mode == 'remote-unchanged':
            for doc in (original, current):
                assert isinstance(doc['configuration']['password'], str)
                assert isinstance(doc['userTags'], list)
            if before != now or original['userTags'] != current['userTags']:
                errors.append('rejected write changed remote configuration/tags')
        elif mode in ('remote-clear', 'remote-resupply'):
            desired = '' if mode == 'remote-clear' else 'tf-live-resupplied-canary'
            if now.get('password') != desired:
                errors.append('remote credential reconciliation not confirmed')
        else:
            errors.append('unknown remote assertion mode')
        return errors
    before, now = attributes(original), attributes(current)
    errors = []
    if (before['id'], before['name']) != (now['id'], now['name']):
        errors.append('owned identity changed')
    if mode == 'refresh':
        if before['password'] != '' or 'password' not in before.get('known_empty_secret_keys', []):
            errors.append('missing authoritative empty provenance')
        if now.get('applied_digests') != before['applied_digests']:
            errors.append('refresh advanced applied baseline')
        if now.get('known_empty_secret_keys') != before['known_empty_secret_keys']:
            errors.append('refresh lost per-key empty evidence')
        if not isinstance(now.get('password'), str) or not now['password'] or now['password'].startswith('tf-live-'):
            errors.append('first addition lacks opaque actionable marker')
    elif mode in ('initial', 'clear'):
        if now.get('password') != '' or now.get('applied_digests') != {}:
            errors.append('explicit clear lacks empty password/baseline')
        if 'password' not in (now.get('known_empty_secret_keys') or []):
            errors.append('explicit clear lacks per-key empty provenance')
    elif mode == 'stable':
        if now != before:
            errors.append('persisted refresh changed state')
    elif mode == 'guard':
        # SDK Apply may return proposed password/tags even when Update errors.
        # Only identity/protection metadata here; independent readback below
        # is authoritative for whether configuration OR tags changed remotely.
        for field in ('applied_digests', 'known_empty_secret_keys'):
            if now[field] != before[field]:
                errors.append('rejected write changed protected ' + field)
        if 'server-held values for password' not in ' '.join(log.replace('│', ' ').split()):
            errors.append('omission/null failed for unrelated reason')
    elif mode == 'resupply':
        if now.get('password') != 'tf-live-resupplied-canary' or not now.get('applied_digests', {}).get('password'):
            errors.append('explicit resupply lacks plaintext and verified baseline')
        expected = set(before['known_empty_secret_keys']) - {'password'}
        if set(now.get('known_empty_secret_keys') or []) != expected:
            errors.append('resupply did not reconcile only password provenance')
    else:
        errors.append('unknown assertion mode')
    return errors

if __name__ == '__main__':
    mode, original, current, log, evidence, ledger, case, phase = sys.argv[1:]
    try:
        errors = check(mode, json.loads(pathlib.Path(original).read_text()),
                       json.loads(pathlib.Path(current).read_text()),
                       '' if log == '-' else pathlib.Path(log).read_text())
    except (AssertionError, AttributeError, IndexError, KeyError, ValueError, TypeError, OSError):
        errors = ['invalid or incomplete assertion evidence']
    result = pathlib.Path(evidence)
    result.write_text(json.dumps({'failures': errors, 'passed': not errors}, indent=2) + '\n')
    result.chmod(0o600)
    code = 1 if errors else 0
    with pathlib.Path(ledger).open('a') as handle:
        handle.write(json.dumps({'case': case, 'phase': phase, 'status': 'FAIL' if errors else 'PASS',
                                 'exit': code, 'evidence': evidence}) + '\n')
    pathlib.Path(ledger).chmod(0o600)
    raise SystemExit(code)
SECRET_CONTRACT_PY
}
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
RUN=${1:?absolute isolated run directory required}
R="$ROOT/testing/live/run.sh"; CASE=secret-cli-origin; DIR="$RUN/work/$CASE"
STAMP=$(date -u '+%Y%m%dT%H%M%SZ')-$$; E="$RUN/evidence/empty-baseline-$STAMP"; mkdir "$E"
record() { jq -nc --arg c "$CASE" --arg p "$1" --arg s "$2" --arg e "$3" --argjson rc "$4" '{case:$c,phase:$p,status:$s,exit:$rc,evidence:$e}' >> "$RUN/ledger.jsonl"; }
trap 'rc=$?; record empty-baseline-harness-abort FAIL "$E" "$rc"' ERR
PREFIX=$(jq -er .prefix "$RUN/manifest.json"); NAME="$PREFIX-secret-cli-origin"
STATE="$DIR/terraform.tfstate"
jq -e --arg n "$NAME" '.resources[0].instances[0].attributes | .name==$n and .type=="postgres"' "$STATE" >/dev/null
# run.sh checks the pinned binary, absolute run, localhost and saved identity
# before any include-secrets export or write. Only this synthetic fixture is used.
bash "$R" readback "$RUN" "$CASE" resources "$NAME"
cp "$DIR/main.tf.json" "$E/original-config.json"
cp "$STATE" "$E/original-state.json"
contract initial "$E/original-state.json" "$STATE" - "$E/initial-provenance.json" "$RUN/ledger.jsonl" "$CASE" empty-baseline-initial-provenance
adaptive admin export resources --name "$NAME" --include-secrets > "$E/owned-empty-export.yaml" 2> "$E/export.stderr"
ruby -ryaml -rjson - "$E/owned-empty-export.yaml" "$E/added.json" "$NAME" <<'RUBY'
d=YAML.load(File.read(ARGV[0])); c=d.fetch('configuration'); abort 'wrong empty synthetic fixture' unless c['name']==ARGV[2] && c['type']=='postgres' && c['password']==''
c['password']='tf-live-first-added-canary'; File.write(ARGV[1],JSON.pretty_generate(d)+"\n")
RUBY
# Private independent reads verify that failed writes did not change tags OR
# the synthetic credential. No real credentials or external connection probes.
remote_check() {
 local mode=$1 phase=$2
 adaptive admin export resources --name "$NAME" --include-secrets > "$E/$phase.yaml" 2> "$E/$phase.stderr"
 ruby -ryaml -rjson -e 'File.write(ARGV[1],JSON.generate(YAML.load(File.read(ARGV[0]))))' "$E/$phase.yaml" "$E/$phase.json"
 contract "$mode" "$E/added.json" "$E/$phase.json" - "$E/$phase-assertion.json" "$RUN/ledger.jsonl" "$CASE" "$phase"
}
set +e; adaptive admin apply -f "$E/added.json" > "$E/external-add.txt" 2>&1; rc=$?; set -e
status=FAIL; test "$rc" != 0 || status=ACKNOWLEDGED; record first-secret-added "$status" "$E/external-add.txt" "$rc"; test "$rc" = 0
remote_check remote-unchanged first-added-independent-read
for cycle in 1 2; do
 bash "$R" tf "$RUN" "$CASE" "empty-baseline-refresh-$cycle" 0 apply -refresh-only -input=false -auto-approve
 cp "$STATE" "$E/refresh-$cycle-state.json"
 contract refresh "$E/original-state.json" "$STATE" - "$E/refresh-$cycle-assertion.json" "$RUN/ledger.jsonl" "$CASE" "empty-baseline-actionable-$cycle"
 if test "$cycle" = 2; then
  contract stable "$E/refresh-1-state.json" "$STATE" - "$E/repeated-refresh-stable.json" "$RUN/ledger.jsonl" "$CASE" empty-baseline-repeated-refresh-stable
 fi
done
# Known-empty provenance is not a legacy unknown baseline: first addition must
# produce an actionable diff even after two persisted refresh-only applies.
bash "$R" tf "$RUN" "$CASE" first-addition-actionable 2 plan -input=false -detailed-exitcode
for intent in omitted null; do
 jq --arg intent "$intent" '.resource.adaptive_resource.test.tags += ["phase=must-not-clear-first-added"] | if $intent=="null" then .resource.adaptive_resource.test.password=null else del(.resource.adaptive_resource.test.password) end' "$E/original-config.json" > "$DIR/next.json"
 mv "$DIR/next.json" "$DIR/main.tf.json"
 bash "$R" tf "$RUN" "$CASE" "first-added-$intent-plan" 2 plan -input=false -detailed-exitcode -out="$intent.tfplan"
 bash "$R" tf "$RUN" "$CASE" "first-added-$intent-rejected" 1 apply -input=false -auto-approve "$intent.tfplan"
 log=$(jq -sr --arg c "$CASE" --arg p "first-added-$intent-rejected" '[.[]|select(.case==$c and .phase==$p)]|last|.evidence' "$RUN/ledger.jsonl")
 cp "$STATE" "$E/$intent-post-error-state.json"
 # Raw SDK post-error state is diagnostic only, not proof of remote mutation.
 contract guard "$E/refresh-2-state.json" "$E/$intent-post-error-state.json" "$log" "$E/$intent-guard.json" "$RUN/ledger.jsonl" "$CASE" "first-added-$intent-guard-assertion"
 remote_check remote-unchanged "first-added-$intent-remote-unchanged"
 # Restore intent and refresh only AFTER independent exact remote verification.
 # SDK proposed empty values must not erase evidence for the next probe.
 cp "$E/original-config.json" "$DIR/main.tf.json"
 bash "$R" tf "$RUN" "$CASE" "first-added-$intent-restore-refresh" 0 apply -refresh-only -input=false -auto-approve
 cp "$STATE" "$E/$intent-restored-state.json"
 contract refresh "$E/original-state.json" "$STATE" - "$E/$intent-restored-actionable.json" "$RUN/ledger.jsonl" "$CASE" "first-added-$intent-restored-actionable"
 bash "$R" tf "$RUN" "$CASE" "first-added-$intent-still-actionable" 2 plan -input=false -detailed-exitcode
done
jq '.resource.adaptive_resource.test.password="" | .resource.adaptive_resource.test.tags += ["empty_baseline=confirmed-clear"]' "$E/original-config.json" > "$DIR/next.json"; mv "$DIR/next.json" "$DIR/main.tf.json"
bash "$R" tf "$RUN" "$CASE" first-added-explicit-clear-plan 2 plan -input=false -detailed-exitcode -out=first-clear.tfplan
bash "$R" tf "$RUN" "$CASE" first-added-explicit-clear-apply 0 apply -input=false -auto-approve first-clear.tfplan
remote_check remote-clear first-added-clear-independent-read
contract clear "$E/original-state.json" "$STATE" - "$E/confirmed-clear.json" "$RUN/ledger.jsonl" "$CASE" first-added-clear-confirmed
bash "$R" tf "$RUN" "$CASE" first-added-clear-noop 0 plan -input=false -detailed-exitcode
# Exercise explicit resupply separately from clearing; verified capture removes
# password's provenance without dropping unrelated per-key empty evidence.
jq '.resource.adaptive_resource.test.password="tf-live-resupplied-canary" | .resource.adaptive_resource.test.tags += ["empty_baseline=resupplied"]' "$E/original-config.json" > "$DIR/next.json"; mv "$DIR/next.json" "$DIR/main.tf.json"
bash "$R" tf "$RUN" "$CASE" first-added-resupply-plan 2 plan -input=false -detailed-exitcode -out=resupply.tfplan
bash "$R" tf "$RUN" "$CASE" first-added-resupply-apply 0 apply -input=false -auto-approve resupply.tfplan
remote_check remote-resupply first-added-resupply-independent-read
contract resupply "$E/original-state.json" "$STATE" - "$E/resupply-confirmed.json" "$RUN/ledger.jsonl" "$CASE" first-added-resupply-confirmed
bash "$R" tf "$RUN" "$CASE" first-added-resupply-noop 0 plan -input=false -detailed-exitcode
# Leave the cohort cleared and omitted, as its original ownership intended.
jq '.resource.adaptive_resource.test.password=""' "$E/original-config.json" > "$DIR/next.json"; mv "$DIR/next.json" "$DIR/main.tf.json"
bash "$R" tf "$RUN" "$CASE" first-added-final-clear-plan 2 plan -input=false -detailed-exitcode -out=final-clear.tfplan
bash "$R" tf "$RUN" "$CASE" first-added-final-clear-apply 0 apply -input=false -auto-approve final-clear.tfplan
remote_check remote-clear first-added-final-clear-read
contract clear "$E/original-state.json" "$STATE" - "$E/final-clear.json" "$RUN/ledger.jsonl" "$CASE" first-added-final-clear-confirmed
cp "$E/original-config.json" "$DIR/main.tf.json"
bash "$R" tf "$RUN" "$CASE" first-added-final-noop 0 plan -input=false -detailed-exitcode
printf '%s\n' 'Known-empty first addition stayed actionable across two refreshes; null/omission rejected without mutation; explicit clear/resupply verified; final omission converged.'
