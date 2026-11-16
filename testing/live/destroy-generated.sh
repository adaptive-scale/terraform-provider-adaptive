#!/bin/bash
# Generated masking authorizations are outside Terraform policy destroy.
# Plan before Terraform teardown; apply only after endpoints have been removed.
set -euo pipefail
umask 077
MODE=${1:?plan or apply required}
RUN=${2:?absolute isolated run directory required}
CONFIRM=${3:-}
PREFIX=$(jq -er '.prefix|select(startswith("tflive-"))' "$RUN/manifest.json")
test "$(cd "$RUN/.." && pwd)" = "$HOME/IdeaProjects/.terraform-provider-adaptive-live"
case "$MODE" in plan) ;; apply) test "$CONFIRM" = "$PREFIX" || { printf '%s\n' 'Wrong manual-validation confirmation.' >&2; exit 1; } ;; *) exit 1 ;; esac
test -z "${ADAPTIVE_SVC_TOKEN:-}" && test -z "${ADAPTIVE_URL:-}" && test -z "${ADAPTIVE_PROFILE:-}"
adaptive whoami -o json | jq -e '.URL=="http://localhost:8080" and .CredentialSource=="saved context"' >/dev/null
mkdir -p "$RUN/destroy/generated"
python3 - "$RUN" <<'PY'
import json,pathlib,sys
r=pathlib.Path(sys.argv[1]); prefix=json.loads((r/'manifest.json').read_text())['prefix']; path=r/'destroy/generated/ownership.json'
known={v['authorization']:v for v in json.loads(path.read_text())} if path.exists() else {}
for state in (r/'work').glob('*/terraform.tfstate'):
 for resource in json.loads(state.read_text()).get('resources',[]):
  if resource['type']!='adaptive_dataprotection': continue
  for instance in resource.get('instances',[]):
   a=instance.get('attributes',{}); name=a.get('resource',''); auth=a.get('authorization_name','')
   if not name.startswith(prefix+'-') or auth!='masked_'+name: raise SystemExit('unowned generated authorization binding')
   known[auth]={'resource':name,'resource_id':a['id'],'authorization':auth}
for entry in known.values():
 if entry['authorization']!='masked_'+entry['resource'] or not entry['resource'].startswith(prefix+'-'): raise SystemExit('unsafe retained generated ownership')
 doc=r/'destroy/generated'/(entry['authorization']+'.json')
 doc.write_text(json.dumps({'type':'authorization','configuration':{'name':entry['authorization']}},indent=2)+'\n'); doc.chmod(0o600)
 entry['document']=str(doc)
path.write_text(json.dumps(list(known.values()),indent=2)+'\n'); path.chmod(0o600)
print('Owned generated authorization targets:',len(known))
PY
if test "$MODE" = apply; then
 for state in "$RUN"/work/*/terraform.tfstate; do
  test -f "$state" || continue
  jq -e '[.resources[]?|select(.type=="adaptive_endpoint")|.instances[]?]|length==0' "$state" >/dev/null || { printf '%s\n' 'Endpoint state remains; remove endpoints before generated authorizations.' >&2; exit 1; }
 done
 while IFS= read -r row; do
  name=$(jq -r .authorization <<< "$row")
  doc=$(jq -r .document <<< "$row")
  log="$RUN/destroy/generated/$name-$(date -u '+%Y%m%dT%H%M%SZ')-$$.txt"
  set +e; adaptive admin delete -f "$doc" > "$log" 2>&1; rc=$?; set -e
  if test "$rc" != 0; then
   /usr/bin/grep -Fq "could not find authorization: $name" "$log" || { printf '%s\n' 'Generated authorization delete failed ambiguously; evidence retained.' >&2; exit 1; }
  else
   # The exporter omits masked authorizations. A second exact-name request must
   # take the backend's not-found lookup branch, not another success branch.
   set +e; adaptive admin delete -f "$doc" > "$log.absence" 2>&1; absent_rc=$?; set -e
   test "$absent_rc" != 0 && /usr/bin/grep -Fq "could not find authorization: $name" "$log.absence" || { printf '%s\n' 'Generated authorization absence was not confirmed.' >&2; exit 1; }
  fi
 done < <(jq -c '.[]' "$RUN/destroy/generated/ownership.json")
fi
printf 'Generated policy cleanup mode=%s. No generated views are dropped here; the isolated target container is a later explicit cleanup stage.\n' "$MODE"
