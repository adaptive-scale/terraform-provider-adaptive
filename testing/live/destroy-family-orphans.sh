#!/bin/bash
# Manual-only cleanup for the separate six-family Adaptive-origin ownership.
# No deletion occurs in plan mode. Apply requires the literal run prefix.
set -euo pipefail
umask 077
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
exec python3 - "$ROOT" "$@" <<'PY'
import datetime,json,os,pathlib,re,subprocess,sys,time
ROOT=pathlib.Path(sys.argv[1]); MODE=sys.argv[2]; RUN=pathlib.Path(sys.argv[3]).resolve(); CONFIRM=sys.argv[4] if len(sys.argv)>4 else ''
assert MODE in ('plan','apply')
os.umask(0o077)
m=json.loads((RUN/'manifest.json').read_text()); prefix=m['prefix']
assert m['url']=='http://localhost:8080' and prefix.startswith('tflive-')
assert RUN.parent==pathlib.Path.home()/'IdeaProjects/.terraform-provider-adaptive-live'
if MODE=='apply': assert CONFIRM==prefix, 'Wrong manual-validation confirmation'
assert not any(os.environ.get(k) for k in ('ADAPTIVE_URL','ADAPTIVE_SVC_TOKEN','ADAPTIVE_PROFILE'))
p=subprocess.run(['adaptive','whoami','-o','json'],capture_output=True,text=True); assert p.returncode==0
identity=json.loads(p.stdout);assert identity['URL']==m['url'] and identity['CredentialSource']=='saved context'
OWN=RUN/'family-imports/ownership.jsonl'; assert OWN.exists()
latest={}
for line in OWN.read_text().splitlines():
    entry=json.loads(line);latest[entry['case']]=entry
out=RUN/'destroy'/('family-orphans-'+str(time.time_ns()));out.mkdir(mode=0o700,parents=True)
def save(p,v): p.write_text(json.dumps(v,indent=2)+'\n');p.chmod(0o600);return p
def active_states():
    states=[]
    for p in (RUN/'work').glob('*/terraform.tfstate'):
        for r in json.loads(p.read_text()).get('resources',[]):
            for i in r.get('instances',[]): states.append((p,r['type'],i.get('attributes',{})))
    return states
states=active_states(); entries=[]
for e in latest.values():
    family=e['family']; name=e['name']; case=e['case']
    assert family in ('authorization','group','schedule','script','dataprotection','endpoint')
    assert case=='import-family-'+family or (family=='group' and case=='import-family-group-initial')
    assert name==prefix+'-import-family-'+family or (family=='group' and name==prefix+'-import-family-group-ack') or (family=='dataprotection' and name==prefix+'-import-postgres')
    doc=pathlib.Path(e['document']).resolve();assert doc.parent==RUN/'family-imports'
    content=json.loads(doc.read_text());assert content['configuration'].get('name',content['configuration'].get('resource_name'))==name
    # Name plus family is checked too, so an import with lost ack metadata is
    # never accidentally classified as an orphan.
    managed=any(t=='adaptive_'+family and ((e.get('id') and a.get('id')==e.get('id')) or a.get('name',a.get('resource'))==name) for _,t,a in states)
    status='STATE_MANAGED' if managed else ('ORPHAN_VERIFIED' if e.get('id') else 'ORPHAN_UNVERIFIED')
    if status=='ORPHAN_UNVERIFIED' and family=='group' and e.get('cli_apply_success'):
        ack=pathlib.Path(e['acknowledgement']).resolve()
        assert ack.parent==RUN/'family-imports' and ('group created: '+name) in ack.read_text()
        status='GROUP_NAME_ACKNOWLEDGED'
    conf={'resource_name':name} if family=='dataprotection' else {'name':name}
    if family=='script': conf['endpoint']=content['configuration']['endpoint'];assert conf['endpoint'].startswith(prefix+'-')
    delete=save(out/(case+'-delete.json'),dict(type=content['type'],configuration=conf))
    row=dict(case=case,family=family,name=name,id=e.get('id'),status=status,document=str(delete),kind=e['kind'])
    if family=='dataprotection':
        # CLI delete only clears policy. Keep generated authorization name even
        # when CLI succeeded but Terraform import did not. Parent generated
        # helper discovers the successful-import version from state.
        auth=e.get('generated_authorization');assert auth=='masked_'+name
        row.update(generated_authorization=auth,generated_cleanup='parent destroy-generated.sh if state-managed; otherwise exact residual document below after all references removed',policy_delete_semantics='clears masking; does not delete policy row')
        row['generated_document']=str(save(out/(case+'-generated-delete.json'),dict(type='authorization',configuration=dict(name=auth))))
    entries.append(row)
plan=save(out/'plan.json',dict(mode=MODE,prefix=prefix,entries=entries,teardown_tested=False))
if MODE=='apply':
    assert not any(x['status']=='ORPHAN_UNVERIFIED' for x in entries), 'Unverified orphan identity; manual exact-ID discovery required'
    for e in entries:
        if e['status'] not in ('ORPHAN_VERIFIED','GROUP_NAME_ACKNOWLEDGED'): continue
        assert not any(t=='adaptive_'+e['family'] and a.get('name',a.get('resource'))==e['name'] for _,t,a in active_states()), 'state ownership changed'
        before=out/(e['case']+'-before.txt')
        with before.open('w') as f: rc=subprocess.run(['adaptive','admin','export',e['kind'],'--name',e['name']],stdout=f,stderr=subprocess.STDOUT).returncode
        assert rc==0 and e['name'] in before.read_text(), 'exact target readback failed; do not delete ambiguously'
        log=out/(e['case']+'-delete.txt')
        with log.open('w') as f: rc=subprocess.run(['adaptive','admin','delete','-f',e['document']],stdout=f,stderr=subprocess.STDOUT).returncode
        assert rc==0, 'delete failed; retain evidence'
        after=out/(e['case']+'-after.txt')
        with after.open('w') as f: rc=subprocess.run(['adaptive','admin','export',e['kind'],'--name',e['name']],stdout=f,stderr=subprocess.STDOUT).returncode
        if e['family']=='dataprotection':
            # Masking delete preserves a reachable policy and generated auth.
            # Never label those residuals absent. They require separate explicit
            # cleanup, not automatic cascade or resource deletion.
            assert rc==0, 'cleared policy readback unavailable'
            e['cleanup_result']='POLICY_CLEARED_RESIDUE_RETAINED'
        elif e['family']=='endpoint':
            # CLI marks endpoints; no workspace-wide delete-marked call allowed.
            e['cleanup_result']='MARKED_ONLY_POD_TEARDOWN_NOT_VERIFIED'
        else:
            assert rc!=0 and 'no such ' in after.read_text(), 'absence not independently confirmed'
            e['cleanup_result']='ABSENCE_CONFIRMED'
    save(out/'result.json',dict(entries=entries,teardown_tested=False))
print(json.dumps(dict(mode=MODE,evidence=str(plan),counts={s:sum(x['status']==s for x in entries) for s in ('STATE_MANAGED','ORPHAN_VERIFIED','GROUP_NAME_ACKNOWLEDGED','ORPHAN_UNVERIFIED')},teardown_tested=False)))
PY
