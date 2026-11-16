#!/bin/bash
# Nested RDP secret drift: isolated CLI-origin synthetic resource, no RDP session.
set -Eeuo pipefail
umask 077
# Keep the executable assertion shared with focused offline tests, not a mock.
contract() {
 python3 - "$@" <<'SECRET_CONTRACT_PY'
import json, pathlib, re, sys

def attributes(state):
    resources = [r for r in state['resources'] if r.get('mode', 'managed') == 'managed']
    assert len(resources) == 1 and resources[0]['type'] == 'adaptive_resource'
    assert resources[0]['name'] == 'test' and len(resources[0]['instances']) == 1
    return resources[0]['instances'][0]['attributes']

def check(mode, original, current, log=''):
    before, now = attributes(original), attributes(current)
    errors = []
    if (before['id'], before['name']) != (now['id'], now['name']):
        errors.append('owned identity changed')
    if mode == 'refresh':
        if not before.get('applied_digests', {}).get('targets'):
            errors.append('missing original aggregate fingerprint')
        if now.get('applied_digests') != before['applied_digests']:
            errors.append('refresh advanced aggregate baseline')
        if not before.get('targets') or now.get('targets') != before['targets']:
            errors.append('refresh changed target/password state')
        text = ' '.join(log.replace('│', ' ').split())
        summary = 'resource "' + before['name'] + '": RDP configuration changed outside this configuration'
        detail = ('The aggregate RDP configuration no longer matches the last-applied fingerprint. '
                  'This warning does not attribute the change to any target or credential and does not force a Terraform update. '
                  'Explicitly resupply the intended targets configuration to reconcile it.')
        if summary not in text or detail not in text:
            errors.append('missing source-backed aggregate RDP warning')
        if 'a secret changed outside this configuration' in text or 'provider bug' in text or re.search(r'targets[.\[]', text):
            errors.append('false per-target attribution or generic warning')
    elif mode == 'reconciled':
        if now.get('targets') != before['targets'] or not now.get('applied_digests', {}).get('targets'):
            errors.append('resupply lacks intended targets or aggregate baseline')
        if 'RDP configuration changed outside this configuration' in ' '.join(log.replace('│', ' ').split()):
            errors.append('aggregate warning persists after resupply')
    else:
        errors.append('unknown assertion mode')
    return errors

if __name__ == '__main__':
    mode, original, current, log, evidence, ledger, case, phase = sys.argv[1:]
    try:
        errors = check(mode, json.loads(pathlib.Path(original).read_text()),
                       json.loads(pathlib.Path(current).read_text()),
                       '' if log == '-' else pathlib.Path(log).read_text())
    except (AssertionError, KeyError, ValueError, TypeError, OSError):
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
CASE=import-adaptive_rdp
DIR="$RUN/work/$CASE"
R="$ROOT/testing/live/run.sh"
STAMP=$(date -u '+%Y%m%dT%H%M%SZ')-$$
BASE="$RUN/evidence/nested-rdp-$STAMP"
mkdir "$BASE"
record() { jq -nc --arg c "$CASE" --arg p "$1" --arg s "$2" --arg e "$3" --argjson rc "$4" '{case:$c,phase:$p,status:$s,exit:$rc,evidence:$e}' >> "$RUN/ledger.jsonl"; }
trap 'rc=$?; record nested-harness-abort FAIL "$BASE" "$rc"' ERR
PREFIX=$(jq -er .prefix "$RUN/manifest.json")
NAME="$PREFIX-import-adaptive_rdp"
STATE="$DIR/terraform.tfstate"
jq -e --arg n "$NAME" '.resources[0].instances[0].attributes | .name==$n and .type=="adaptive_rdp"' "$STATE" >/dev/null
# Verify the pinned run and saved localhost identity before exporting secrets.
bash "$R" readback "$RUN" "$CASE" resources "$NAME"
cp "$DIR/main.tf.json" "$BASE/original-config.json"
cp "$STATE" "$BASE/original-state.json"
adaptive admin export resources --name "$NAME" --include-secrets > "$BASE/own-synthetic-export.yaml" 2> "$BASE/export.stderr"
ruby -ryaml -rjson - "$BASE/own-synthetic-export.yaml" "$BASE/rotated.json" "$NAME" <<'RUBY'
doc=YAML.load(File.read(ARGV[0])); cfg=doc.fetch('configuration'); abort 'wrong owned identity' unless cfg['name']==ARGV[2] && cfg['type']=='adaptive_rdp'
original=cfg.fetch('targets'); targets=original.is_a?(String) ? YAML.load(original) : original
abort 'missing synthetic targets' unless targets.is_a?(Array) && !targets.empty?
targets.each { |t| abort 'not a synthetic credential' unless t.fetch('password').start_with?('tf-live-'); t['password']='tf-live-nested-outside-rotation-canary' }
cfg['targets']=original.is_a?(String) ? YAML.dump(targets) : targets
File.write(ARGV[1], JSON.pretty_generate(doc)+"\n")
RUBY
set +e; adaptive admin apply -f "$BASE/rotated.json" > "$BASE/cli-rotate.txt" 2>&1; rc=$?; set -e
status=FAIL; test "$rc" != 0 || status=ACKNOWLEDGED
record nested-external-rotate "$status" "$BASE/cli-rotate.txt" "$rc"
test "$rc" = 0
bash "$R" readback "$RUN" "$CASE" resources "$NAME"
for cycle in 1 2; do
 bash "$R" tf "$RUN" "$CASE" "nested-refresh-$cycle" 0 apply -refresh-only -input=false -auto-approve
 log=$(jq -sr --arg c "$CASE" --arg p "nested-refresh-$cycle" '[.[]|select(.case==$c and .phase==$p)]|last|.evidence' "$RUN/ledger.jsonl")
 cp "$STATE" "$BASE/refresh-$cycle-state.json"
 contract refresh "$BASE/original-state.json" "$STATE" "$log" "$BASE/persisted-contract-$cycle.json" "$RUN/ledger.jsonl" "$CASE" "nested-warning-and-baseline-$cycle"
done
# Warning-only paths cannot be represented by a scalar Terraform argument. A
# clean plan here is a documented limitation, NOT automatic remediation proof.
bash "$R" tf "$RUN" "$CASE" nested-warning-only-plan 0 plan -input=false -detailed-exitcode
jq --arg stamp "$STAMP" '.resource.adaptive_resource.test.tags = ((.resource.adaptive_resource.test.tags // [] | map(select(startswith("nested_secret=")|not))) + ["nested_secret=restored-"+$stamp])' "$BASE/original-config.json" > "$DIR/next.json"
mv "$DIR/next.json" "$DIR/main.tf.json"
bash "$R" tf "$RUN" "$CASE" nested-explicit-resupply-plan 2 plan -input=false -detailed-exitcode -out=nested-restore.tfplan
bash "$R" tf "$RUN" "$CASE" nested-explicit-resupply-apply 0 apply -input=false -auto-approve nested-restore.tfplan
bash "$R" readback "$RUN" "$CASE" resources "$NAME"
bash "$R" tf "$RUN" "$CASE" nested-restored-noop 0 plan -input=false -detailed-exitcode
log=$(jq -sr --arg c "$CASE" '[.[]|select(.case==$c and .phase=="nested-restored-noop")]|last|.evidence' "$RUN/ledger.jsonl")
contract reconciled "$BASE/original-state.json" "$STATE" "$log" "$BASE/reconciled-contract.json" "$RUN/ledger.jsonl" "$CASE" nested-resupply-reconciled
python3 - "$RUN" "$BASE" <<'PY'
import json,pathlib,sys
r=pathlib.Path(sys.argv[1]); base=pathlib.Path(sys.argv[2]); rows=[json.loads(s) for s in (r/'ledger.jsonl').read_text().splitlines()]
start=max(i for i,x in enumerate(rows) if x['case']=='import-adaptive_rdp' and x['phase']=='nested-external-rotate')
selected=[x for x in rows[start:] if x['case']=='import-adaptive_rdp']
exports={pathlib.Path(x['evidence']) for x in selected if x['phase']=='readback'}
logs={pathlib.Path(x['evidence']) for x in selected if x['phase'].startswith('nested-') and x.get('evidence') and pathlib.Path(x['evidence']).suffix=='.txt'}
canary='tf-live-nested-outside-rotation-canary'
result={'export_canary_visible':any(canary in p.read_text() for p in exports),'ordinary_tf_log_canary_visible':any(canary in p.read_text() for p in logs if p.exists()),'exports_checked':len(exports),'unique_logs_checked':len(logs)}
path=base/'redaction-assertion.json';path.write_text(json.dumps(result,indent=2)+'\n');path.chmod(0o600)
good=not result['export_canary_visible'] and not result['ordinary_tf_log_canary_visible']
with (r/'ledger.jsonl').open('a') as f: f.write(json.dumps({'case':'import-adaptive_rdp','phase':'nested-redaction-assertion','status':'PASS' if good else 'FAIL','exit':0 if good else 1,'evidence':str(path)})+'\n')
raise SystemExit(0 if good else 1)
PY
printf '%s\n' 'Nested rotation remained warning-only across two refreshes; explicit resupply restored it. No RDP connection or destruction.'
