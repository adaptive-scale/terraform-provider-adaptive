#!/bin/bash
# Fresh-run policy endpoints. No SQL, real-user CRUD, restarts or state surgery.
set -euo pipefail
umask 077
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
R="$ROOT/testing/live/run.sh"
die() { printf '%s\n' "$*" >&2; exit 1; }
template() {
 python3 - "$@" <<'PY'
import json, re, sys
p,c,u,m=sys.argv[1:]
assert re.fullmatch(r'tflive-[0-9]{14}-[0-9]+',p), 'Invalid run prefix'
assert re.fullmatch(r'[A-Za-z0-9_.-]+',c), 'Invalid cluster name'
assert u and '@' in u and not any(x.isspace() for x in u), 'Existing whoami email required'
# Native postgres FilterVal.Allow []Opt; Opt.Schema []string, not scalar.
a=dict(name=p+'-authorization-postgres',resource_type='postgres',description='Run-owned SELECT on tf_live_test.public.customers only',permissions=json.dumps(dict(allow=[dict(database='tf_live_test',schema=['public'],objects=['customers'],privileges=['SELECT'])]),separators=(',',':')))
e=dict(name=p+'-endpoint-readonly',resource=p+'-runtime-postgres',type='client',cluster=c,users=[u],authorization=a['name'],tags=['campaign=terraform-live','purpose=runtime'],is_jit_enabled=False,auto_approval=False,jit_approvers=[],disable_web_cli=False,disable_data_studio=False,disable_output_capture=False,script_only_access=False)
out={'authorization-postgres':a,'endpoint-readonly':e}
if m:
 assert m=='masked_'+p+'-runtime-postgres', 'Mask authorization must belong to exact runtime resource'
 out['endpoint-masked']=dict(e,name=p+'-endpoint-masked',authorization=m)
print(json.dumps(out))
PY
}
check_run() {
 python3 - "$ROOT" "$1" <<'PY'
import hashlib, importlib.util, json, pathlib, re, sys
sys.dont_write_bytecode=True
root=pathlib.Path(sys.argv[1]); run=pathlib.Path(sys.argv[2])
assert run.name!='20261104T042306Z-96512', 'legacy retained run is forbidden'
assert run.is_absolute() and run==run.resolve(), 'Canonical absolute run path required'
m=json.loads((run/'manifest.json').read_text())
assert re.fullmatch('[0-9a-f]{64}',m.get('provider_source_sha256','')), 'Fresh source-pinned manifest required'
assert run.parent==pathlib.Path.home()/'IdeaProjects/.terraform-provider-adaptive-live', 'Run must be in private run base'
assert m.get('run')==str(run) and m.get('url')=='http://localhost:8080', 'Run/workspace mismatch'
assert re.fullmatch(r'tflive-[0-9]{14}-[0-9]+',m['prefix']), 'Invalid prefix'
spec=importlib.util.spec_from_file_location('source_capture',root/'testing/live/source_capture.py'); mod=importlib.util.module_from_spec(spec); spec.loader.exec_module(mod)
saved=json.loads((run/'inventory/provider-source.json').read_text())
assert saved['sha256']==m['provider_source_sha256']==mod.capture(root)['sha256'], 'Provider source pin mismatch'
assert hashlib.sha256((run/'bin/terraform-provider-adaptive').read_bytes()).hexdigest()==m['binary_sha256'], 'Provider binary pin mismatch'
print(m['prefix'])
PY
}
inspect_state() {
 python3 - "$@" <<'PY'
import json, pathlib, sys
p,family,expected=sys.argv[1:]; expected=json.loads(expected)
s=json.loads(pathlib.Path(p).read_text()); resources=s.get('resources',[])
assert len(resources)==1, 'Exactly one managed state resource required'
r=resources[0]
assert r.get('mode')=='managed' and r.get('type')==family and r.get('name')=='test', 'Unexpected state address'
instances=r.get('instances',[])
assert len(instances)==1, 'Partial/deposed state refused'
i=instances[0]
assert i.get('status')!='tainted' and not i.get('deposed'), 'tainted/deposed state retained; no automatic repair'
a=i['attributes']; assert a.get('id'), 'Missing recorded ID; no retry'
for k,v in expected.items():
 assert a.get(k)==v, 'State binding mismatch: '+k
print(json.dumps(a))
PY
}
compare_export() {
 local kind=$1 attrs=$2 file=$3
 # CLI export_documents.go emits stderr warnings before stdout YAML. The
 # masking handler reports orphan ResourceIDs before applying its name filter.
 # Separate only this exact diagnostic prefix; never rewrite saved evidence.
 ruby -ryaml -rjson -e '
text=File.read(ARGV[0])
orphan=/\Awarning: skipped "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}": the resource this policy belongs to no longer exists; the policy is orphaned and cannot be exported or applied\n/
if ARGV[1]=="dataprotection"
 while (warning=orphan.match(text))
  STDERR.write(warning[0])
  text=text[warning[0].length..-1]
 end
end
note="\nnote: secret values are shown as <REDACTED>. Fill them in before applying, or re-run with --include-secrets.\n"
if ARGV[1]=="resources" && text.end_with?(note)
 STDERR.write(note)
 text=text[0...-note.length]
end
stream=YAML.parse_stream(text)
abort "exactly one export document required" unless stream.children.length==1
# Native exported documents carry plain data, never explicit YAML tags.
stream.each { |node| abort "tagged export refused" if node.respond_to?(:tag) && node.tag }
d=YAML.safe_load(text, permitted_classes: [], permitted_symbols: [], aliases: false)
abort "export mapping required" unless d.is_a?(Hash)
# adminconfig.Resource declares only these document envelope fields. Otherwise
# a diagnostic such as warning: text could silently become an extra YAML key.
abort "unexpected export fields" unless (d.keys-["version","type","configuration","userTags"]).empty?
puts JSON.generate(d)
' "$file" "$kind" | python3 -c '
import json,sys
kind,a=sys.argv[1],json.loads(sys.argv[2]); d=json.load(sys.stdin); c=d["configuration"]
if kind=="authorizations":
 assert d["type"]=="authorization" and c["name"]==a["name"] and c["integration_type"]==a["resource_type"]
 # Parse either native YAML text or JSON emitted by our template.
 import subprocess
 got=subprocess.run(["ruby","-ryaml","-rjson","-e","puts JSON.generate(YAML.load(STDIN.read))"],input=c["auth"],text=True,capture_output=True,check=True)
 assert json.loads(got.stdout)==json.loads(a["permissions"]), "Authorization scope mismatch"
elif kind=="endpoints":
 assert d["type"]=="endpoint"
 for k,n in (("name","name"),("access_type","type"),("integration_name","resource"),("cluster","cluster"),("authorization","authorization")):
  assert c.get(k,"")==a.get(n,""), "Endpoint binding mismatch: "+k
 assert not c.get("groups"), "Unexpected group grants"
 jit=c.get("jit",{}); assert jit.get("is_jit_enabled") is False and not jit.get("auto_approval",False) and not jit.get("access_approvers",[]), "Unexpected approval configuration"
elif kind=="resources":
 assert d["type"]=="resource" and c["name"]==a["name"] and c["type"]=="postgres" and c["databaseName"]=="tf_live_test", "Runtime resource mismatch"
elif kind=="dataprotection":
 assert d["type"]=="dataprotection" and c["resource_name"]==a["resource"], "Wrong masking resource"
 import subprocess
 got=subprocess.run(["ruby","-ryaml","-rjson","-e","puts JSON.generate(YAML.load(STDIN.read))"],input=c["configuration"],text=True,capture_output=True,check=True)
 policy=json.loads(got.stdout)
 assert policy["scoped"] is True and policy.get("masks"), "Missing scoped masking policy"
 assert all(m["database_name"]=="tf_live_test" for m in policy["masks"]), "Wrong masking database"
 tables=[t for m in policy["masks"] for t in m.get("configuration",[])]
 assert tables and all(t.get("schema")=="public" and t["table_name"]=="customers" and t.get("masked_columns") for t in tables), "Wrong masking scope"
else: raise ValueError("Unsupported export kind")
print("Exact independent configuration readback verified; not runtime enforcement")
' "$kind" "$attrs"
}
record() {
 jq -nc --arg at "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" --arg case "$1" --arg phase "$2" --arg status "$3" --arg evidence "${4:-}" '{at:$at,case:$case,phase:$phase,status:$status,evidence:$evidence}' | tee -a "$OUT/results.jsonl" >> "$RUN/ledger.jsonl"
 printf '%s %s %s\n' "$1" "$2" "$3"
}
readback() {
 local case=$1 kind=$2 attrs=$3 name path
 name=$(jq -r '.name // .resource' <<< "$attrs")
 if ! bash "$R" readback "$RUN" "$case" "$kind" "$name"; then record "$case" independent-readback FAIL; return 1; fi
 path=$(jq -rs --arg c "$case" '[.[]|select(.case==$c and .phase=="readback")][-1].evidence' "$RUN/ledger.jsonl")
 if compare_export "$kind" "$attrs" "$path" > "$OUT/$case-export-compare-$RANDOM.txt" 2>&1; then
  record "$case" exact-bindings PASS "$path"
 else record "$case" exact-bindings FAIL "$path"; return 1; fi
}
ready() {
 local case=$1 name=$2 file="$OUT/$1-readiness-$RANDOM.txt"
 if adaptive admin endpoint list --endpoint-type postgres --session-type client --status created --names-only > "$file" 2>&1 && python3 -c 'import sys; rows=[s.strip() for s in open(sys.argv[1])]; raise SystemExit(0 if rows.count(sys.argv[2])==1 else 1)' "$file" "$name"; then
  record "$case" provisioning-ready PASS "$file"
 else record "$case" provisioning-ready BLOCKED "$file"; return 1; fi
}
ensure_case() {
 local case=$1 family=$2 attrs=$3 kind=$4 dir="$RUN/work/$1" name failed=0
 name=$(jq -r .name <<< "$attrs")
 if test -e "$dir/main.tf.json" || test -e "$dir/terraform.tfstate" || test -e "$OUT/../$case-attempted"; then
  # Always read independently, including tainted/partial failed creates.
  readback "$case" "$kind" "$attrs" || failed=1
  if test "$kind" = endpoints; then ready "$case" "$name" || failed=1; fi
  if ! jq -e --arg f "$family" --argjson a "$attrs" '.resource[$f].test==$a' "$dir/main.tf.json" >/dev/null 2>&1 || ! inspect_state "$dir/terraform.tfstate" "$family" "$attrs" > "$OUT/$case-retained-state-check.txt" 2>&1; then
   record "$case" retained-state BLOCKED "$dir"; return 1
  fi
  record "$case" retained-no-mutation PASS "$dir"
  return "$failed"
 fi
 if test "$MODE" = verify; then record "$case" missing-fixture BLOCKED; return 1; fi
 # Name lists are refreshed for every new object; a collision is never adopted.
 local names="$OUT/$case-before-create-names.txt" noun=endpoint
 test "$kind" != authorizations || noun=authorization
 adaptive admin "$noun" list --names-only > "$names" 2>&1 || { record "$case" collision-inventory BLOCKED "$names"; return 1; }
 if ! python3 -c 'import sys; rows=[s.strip() for s in open(sys.argv[1])]; raise SystemExit(1 if sys.argv[2] in rows else 0)' "$names" "$name"; then record "$case" remote-name-collision BLOCKED "$names"; return 1; fi
 # Persistent attempt receipt closes crash/partial-create retries, even without ID.
 (set -C; printf '%s\n' "$name" > "$OUT/../$case-attempted") || return 1
 bash "$R" fixture "$RUN" "$case" "$family" "$attrs" || return 1
 if bash "$R" tf "$RUN" "$case" runtime-create-plan 2 plan -input=false -detailed-exitcode -out=runtime-create.tfplan; then
  # Stronger than shared no-delete guard: this stage only accepts a fresh create.
  if bash "$R" tf "$RUN" "$case" runtime-inspect-plan 0 show -json runtime-create.tfplan; then
   local plan
   plan=$(jq -rs --arg c "$case" '[.[]|select(.case==$c and .phase=="runtime-inspect-plan")][-1].evidence' "$RUN/ledger.jsonl")
   if jq -e --arg f "$family" '.resource_changes|length==1 and .[0].type==$f and .[0].address==($f+".test") and .[0].change.actions==["create"]' "$plan" >/dev/null; then
    bash "$R" tf "$RUN" "$case" create-apply 0 apply -input=false -auto-approve runtime-create.tfplan || failed=1
   else record "$case" create-only-plan BLOCKED "$plan"; failed=1; fi
  else failed=1; fi
 else failed=1; fi
 # Do not let set -e discard evidence after a failed apply/readiness timeout.
 if test -f "$dir/terraform.tfstate"; then cp "$dir/terraform.tfstate" "$OUT/$case-post-create.tfstate"; fi
 readback "$case" "$kind" "$attrs" || failed=1
 if test "$kind" = endpoints; then ready "$case" "$name" || failed=1; fi
 inspect_state "$dir/terraform.tfstate" "$family" "$attrs" > "$OUT/$case-post-create-state-check.txt" 2>&1 || failed=1
 if test "$failed" != 0; then record "$case" create FAIL "$dir"; record "$case" retry BLOCKED "$dir"; fi
 return "$failed"
}
# Loading functions for offline tests must never trigger discovery or live work.
if test "${BASH_SOURCE[0]}" != "$0"; then return 0; fi
case "${1:-help}" in
 template) shift; template "$@"; exit ;;
 check-run) check_run "${2:?run required}"; exit ;;
 inspect-state) shift; inspect_state "$@"; exit ;;
 compare-export) shift; compare_export "$@"; exit ;;
 setup|verify) MODE=$1 ;;
 *) printf '%s\n' 'Usage: bash testing/live/runtime-policies.sh setup|verify RUN [CLUSTER=k8s-local] [BACKEND_REPO]' 'Requires a NEW source-pinned run, runtime-prepare resource tf_live_test, ready owned endpoint-client on CLUSTER, existing saved whoami identity, and an owned scoped DataProtection policy for masked setup.' 'verify never invokes Terraform. Existing fixtures are read back only; tainted/partial states block, never retry/untaint/restart. SQL is never run. Legacy retained run is forbidden.'; exit ;;
esac

RUN=${2:?absolute fresh source-pinned run required}
CLUSTER=${3:-k8s-local}
BACKEND=${4:-/Users/dhaivatdshah/IdeaProjects/.worktrees/inventorize/fix/terraform-provider-live-regressions}
PREFIX=$(check_run "$RUN")
for variable in ADAPTIVE_SVC_TOKEN ADAPTIVE_URL ADAPTIVE_PROFILE; do
 test -z "${!variable:-}" || die "Unset $variable: saved local CLI identity required"
done
mkdir -p "$RUN/runtime-policies"
LOCK="$RUN/runtime-policies/.lock"
mkdir "$LOCK" || die 'Another runtime-policies process or retained lock exists; inspect manually, never automatically resume an interrupted writer.'
OUT="$RUN/runtime-policies/$(date -u '+%Y%m%dT%H%M%SZ')-$$-$RANDOM"
mkdir "$OUT"
finish() {
 local rc=$?
 trap - EXIT
 if test "$rc" != 0; then record runtime-policies stage BLOCKED "$OUT"; fi
 rmdir "$LOCK"
 exit "$rc"
}
trap finish EXIT
# Capture only source hashes and native structural contract; backend is read-only.
python3 - "$BACKEND" > "$OUT/native-consumer-source.json" <<'PY'
import hashlib,json,pathlib,re,sys
r=pathlib.Path(sys.argv[1])/'inventorize-app/internal/service/integrations/postgres'
a=(r/'authorization.go').read_text(); consumer=(r/'authorization_allow_deny.go').read_text()
assert re.search(r'type FilterVal struct\s*\{[^}]*Allow\s+\[\]Opt',a,re.S), 'Unknown native postgres filter shape'
opt=re.search(r'type Opt struct\s*\{([^}]+)\}',a).group(1)
for name,kind,tag in [('Schema',r'\[\]string','schema'),('Objects',r'\[\]string','objects'),('Database','string','database'),('Privileges',r'\[\]string','privileges')]:
 assert re.search(name+r'\s+'+kind+r'\s+`yaml:"'+tag+r',omitempty"`',opt), 'Native field changed: '+name
assert 'yaml.Unmarshal([]byte(yamlFilterConfig), &filterConfig)' in consumer and 'var filterConfig FilterVal' in consumer, 'Unknown native consumer'
files={str(r/n):hashlib.sha256((r/n).read_bytes()).hexdigest() for n in ('authorization.go','authorization_allow_deny.go')}
print(json.dumps(dict(native='postgres.FilterVal.Allow []Opt (Opt.Schema []string)',files=files),indent=2))
PY
record runtime-policies native-consumer-contract PASS "$OUT/native-consumer-source.json"
adaptive whoami -o json > "$OUT/identity.json"
EMAIL=$(jq -er 'select(.URL=="http://localhost:8080" and .CredentialSource=="saved context")|.Identity.User.Email|select(type=="string" and length>0)' "$OUT/identity.json")
jq -e '.provider_schemas["registry.terraform.io/adaptive-scale/adaptive"].resource_schemas.adaptive_endpoint.block.attributes | has("users") and has("authorization") and has("resource") and has("cluster")' "$RUN/inventory/schema.json" >/dev/null
RESOURCE=$(jq -nc --arg n "$PREFIX-runtime-postgres" '{name:$n,type:"postgres",database_name:"tf_live_test"}')
inspect_state "$RUN/work/runtime-postgres/terraform.tfstate" adaptive_resource "$RESOURCE" >/dev/null
readback runtime-postgres resources "$RESOURCE"
BASELINE=$(jq -nc --arg n "$PREFIX-postgres-client" --arg r "$PREFIX-runtime-postgres" --arg c "$CLUSTER" --arg u "$EMAIL" '{name:$n,resource:$r,type:"client",cluster:$c,users:[$u]}')
inspect_state "$RUN/work/endpoint-client/terraform.tfstate" adaptive_endpoint "$BASELINE" >/dev/null
readback endpoint-client endpoints "$BASELINE"
ready endpoint-client "$PREFIX-postgres-client"
# Configuration readiness is not forwarding/SQL or policy acceptance.
FAILED=0
MASK=''
DP=$(jq -nc --arg r "$PREFIX-runtime-postgres" '{resource:$r,scoped:true}')
if MASK=$(inspect_state "$RUN/work/dataprotection/terraform.tfstate" adaptive_dataprotection "$DP" 2> "$OUT/masking-prerequisite.txt" | jq -er --arg expected "masked_$PREFIX-runtime-postgres" '.authorization_name|select(.==$expected)') && readback dataprotection dataprotection "$DP"; then
 record endpoint-masked masking-prerequisite PASS
else
 MASK=''; FAILED=1
 record endpoint-masked masking-prerequisite BLOCKED "$OUT/masking-prerequisite.txt"
fi
TEMPLATES=$(template "$PREFIX" "$CLUSTER" "$EMAIL" "$MASK")
printf '%s\n' "$TEMPLATES" > "$OUT/requested-fixtures.json"
AUTH=$(jq -c '."authorization-postgres"' <<< "$TEMPLATES")
if ensure_case authorization-postgres adaptive_authorization "$AUTH" authorizations; then
 READONLY=$(jq -c '."endpoint-readonly"' <<< "$TEMPLATES")
 ensure_case endpoint-readonly adaptive_endpoint "$READONLY" endpoints || FAILED=1
else
 FAILED=1; record endpoint-readonly authorization-prerequisite BLOCKED
fi
if test -n "$MASK"; then
 MASKED=$(jq -c '."endpoint-masked"' <<< "$TEMPLATES")
 ensure_case endpoint-masked adaptive_endpoint "$MASKED" endpoints || FAILED=1
fi
for case in endpoint-readonly endpoint-masked; do
 record "$case" policy-sql BLOCKED "$OUT"
done
printf '%s\n' "Evidence: $OUT" 'No SQL executed; users checked in Terraform state, endpoint export does not expose user membership. Provisioning-ready does not establish forwarding or enforcement. Failed/tainted states and all historical failures remain retained.'
test "$FAILED" = 0
