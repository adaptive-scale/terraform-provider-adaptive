#!/bin/bash
# Independent synthetic auth persistence cohorts. No runtime tests or cleanup.
set -euo pipefail
umask 077
exec python3 - "$0" "$@" <<'PY'
import collections, copy, csv, json, os, re, subprocess, sys, unittest
from pathlib import Path
SCRIPT=Path(sys.argv[1]).resolve()
ROOT=SCRIPT.parents[2]
INVENTORY=SCRIPT.with_suffix('.json')
CASES=json.loads(INVENTORY.read_text())
ROWS=list(csv.DictReader((ROOT/'docs/parity-current-integration-fields.csv').open()))
SUPPORTED=collections.defaultdict(set)
for row in ROWS:
    SUPPORTED[row['Integration']].add(row['Reverse'])

def validate_inventory(cases, schema=None):
    ids=set()
    for c in cases:
        assert re.fullmatch(r'auth-[a-z0-9-]+',c['id']) and c['id'] not in ids
        ids.add(c['id'])
        assert c['runtime_auth_precedence']=='NOTRUN'
        assert set(c['attributes']) <= SUPPORTED[c['type']], 'unsupported inventory field'
        for k,v in c['attributes'].items():
            if k=='create_if_not_exists': assert v is False
            if k in ('host','keyspaces_endpoint','domain'): assert v.endswith('.invalid')
            if k=='cache_type': assert v in ('replication_group','serverless')
            if schema is not None:
                assert k in schema and schema[k].get('optional'), 'field not configurable in pinned schema'
                typ=schema[k]['type']
                assert (typ=='bool' and type(v) is bool) or (typ=='string' and type(v) is str)
    assert len(ids)==19

class InventoryTests(unittest.TestCase):
    def test_supported_auth_keys(self): validate_inventory(CASES['cases'])
    def test_unsupported_key_rejected(self):
        cases=copy.deepcopy(CASES['cases']); cases[0]['attributes']['use_password']=False
        with self.assertRaises(AssertionError): validate_inventory(cases)
    def test_provisioning_rejected(self):
        cases=copy.deepcopy(CASES['cases']); cases[0]['attributes']['create_if_not_exists']=True
        with self.assertRaises(AssertionError): validate_inventory(cases)
    def test_mode_inventory(self):
        self.assertEqual(collections.Counter(c['type'] for c in CASES['cases']),dict(aws=3,keyspaces=3,postgres=2,mysql=2,kafka=2,awselasticcache=4,ssh=2,google=1))
    def test_source_anchors(self):
        for anchor in CASES['source_anchors']: self.assertTrue((ROOT/anchor).is_file())

suite=unittest.defaultTestLoader.loadTestsFromTestCase(InventoryTests)
if not unittest.TextTestRunner(verbosity=1).run(suite).wasSuccessful(): sys.exit(1)
if sys.argv[2:]==['--test']: sys.exit(0)
if len(sys.argv) not in (3,4): sys.exit('Usage: bash testing/live/auth-modes.sh --test | ABSOLUTE_RUN [--verify]')
if len(sys.argv)==4 and sys.argv[3]!='--verify': sys.exit('Unsupported argument')
VERIFY=len(sys.argv)==4
RUN=Path(sys.argv[2]).resolve()
manifest=json.loads((RUN/'manifest.json').read_text())
assert manifest['url']=='http://localhost:8080'
assert subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip()==CASES['source_commit']
schema=json.loads((RUN/'inventory/schema.json').read_text())['provider_schemas']['registry.terraform.io/adaptive-scale/adaptive']['resource_schemas']['adaptive_resource']['block']['attributes']
validate_inventory(CASES['cases'],schema)
(RUN/'inventory/auth-modes.json').write_text(json.dumps(CASES,indent=2)+'\n')
RESULT=RUN/'evidence/auth-modes-results.json'
if VERIFY:
    prior=json.loads(RESULT.read_text())
    assert {c['id'] for c in prior['cases']}=={c['id'] for c in CASES['cases']}
    for c in prior['cases']:
        state=json.loads((RUN/'work'/c['id']/'terraform.tfstate').read_text())['resources'][0]['instances'][0]['attributes']
        assert state['id']==c['fixture_id'] and state['name']==c['name']
        configured=c['configured_attributes']
        c['final_verification']={'exact_id_matches':True,'exact_name_matches':True,'updated_tags_present':'phase=updated' in state['tags'],'configured_fields_match_state':all(state.get(k)==v for k,v in configured.items())}
        assert c['final_verification']['configured_fields_match_state']
        for e in c['exports']:
            text=Path(e['path']).read_text()
            e['updated_tag_present']='phase=updated' in text
            e['exact_name_present']=c['name'] in text
            assert e['exact_name_present']
            if e['phase']=='updated-readback': assert e['updated_tag_present']==c['final_verification']['updated_tags_present']
            if c['type']=='ssh':
                e['derived_usePassword_false']=bool(re.search(r'^\s+usePassword: false',text,re.M))
            if c['type']=='google':
                for g in c['google_default_export']:
                    if g['phase']==e['phase']:
                        g['admin_email_redacted']=bool(re.search(r'^\s+adminEmail:[^\n]*REDACTED',text,re.M))
        if c['type']=='ssh': c['expected_held_secret_blocker']['reproduction']='NOT_REPRODUCED' if c['status']=='PASS' else 'See raw protected update evidence; failures remain FAIL'
    counts=collections.Counter(c['status'] for c in prior['cases'])
    prior['case_counts']={k:counts[k] for k in ('PASS','FAIL')}
    phases=collections.Counter(p['status'] for c in prior['cases'] for p in c['phases'])
    prior['phase_counts']={k:phases[k] for k in ('PASS','FAIL')}
    prior['verification']='PASS'
    RESULT.write_text(json.dumps(prior,indent=2)+'\n'); RESULT.chmod(0o600)
    print(json.dumps({'verification':'PASS','case_counts':prior['case_counts'],'phase_counts':prior['phase_counts'],'fixture_ids':[{k:c[k] for k in ('id','fixture_id')} for c in prior['cases']]}))
    sys.exit(0)
if RESULT.exists(): sys.exit('Existing results: refuse overwrite. Use --verify for read-only fixture verification.')
report={'inventory_tests':'PASS','pinned_schema_inventory':'PASS','source_commit':CASES['source_commit'],'runtime_auth_precedence':'NOTRUN','cases':[]}

def save():
    counts=collections.Counter(c.get('status','INCOMPLETE') for c in report['cases'])
    phases=collections.Counter(p['status'] for c in report['cases'] for p in c['phases'])
    report['case_counts']=dict(counts); report['phase_counts']=dict(phases)
    RESULT.write_text(json.dumps(report,indent=2)+'\n'); RESULT.chmod(0o600)

def call(c, phase, args, expected=0):
    # Runner itself enforces localhost context, pinned provider and no deletes.
    path=RUN/'evidence'/f"{c['id']}-{phase}-driver.txt"
    with path.open('wb') as f:
        rc=subprocess.run(['bash',str(ROOT/'testing/live/run.sh'),*args],cwd=ROOT,stdout=f,stderr=subprocess.STDOUT).returncode
    path.chmod(0o600)
    item={'phase':phase,'status':'PASS' if rc==0 else 'FAIL','driver_exit':rc,'expected_terraform_exit':expected,'evidence':str(path)}
    c['phases'].append(item); save()
    return rc==0

def tf(c, phase, expected, *args):
    return call(c,phase,['tf',str(RUN),c['id'],phase,str(expected),*args],expected)

def readback(c,phase):
    ok=call(c,phase,['readback',str(RUN),c['id'],'resources',c['name']])
    if ok:
        exports=list((RUN/'evidence').glob(c['id']+'-readback-*.yaml'))
        latest=max(exports,key=lambda p:p.stat().st_mtime_ns)
        text=latest.read_text(); latest.chmod(0o600)
        c.setdefault('exports',[]).append({'phase':phase,'path':str(latest),'exact_name_present':c['name'] in text})
        if c['type']=='google':
            sa=json.loads(c['configured_attributes']['service_account'])
            c.setdefault('google_default_export',[]).append(dict(phase=phase,service_account_field_present=bool(re.search(r'^\s+serviceAccount:',text,re.M)),admin_email_field_present=bool(re.search(r'^\s+adminEmail:',text,re.M)),admin_email_context_visible=c['configured_attributes']['admin_email'] in text,private_key_field_present='private_key' in text,dummy_private_key_visible=sa['private_key'] in text,credential_values_printed=False))
        save()
    return ok

for case in CASES['cases']:
    c=dict(id=case['id'],type=case['type'],name=manifest['prefix']+'-'+case['id'],configured_attributes=case['attributes'],notes=case['notes'],runtime_auth_precedence='NOTRUN',phases=[])
    report['cases'].append(c); save()
    directory=RUN/'work'/c['id']; config=directory/'main.tf.json'
    if config.exists():
        c['status']='FAIL'; c['issue']='Existing auth fixture: refuse overwrite/recreate; inspect prior results.'; save(); continue
    attrs=dict(case['attributes'],name=c['name'],type=c['type'],tags=['campaign=terraform-live','cohort=auth-modes','phase=create'])
    if not call(c,'fixture',['fixture',str(RUN),c['id'],'adaptive_resource',json.dumps(attrs)]):
        c['status']='FAIL'; save(); continue
    tf(c,'validate',0,'validate')
    planned=tf(c,'create-plan',2,'plan','-input=false','-detailed-exitcode','-out=create.tfplan')
    created=planned and tf(c,'create-apply',0,'apply','-input=false','-auto-approve','create.tfplan')
    if created:
        state=json.loads((directory/'terraform.tfstate').read_text())
        c['fixture_id']=state['resources'][0]['instances'][0]['attributes']['id']
        readback(c,'created-readback')
        tf(c,'created-noop',0,'plan','-input=false','-detailed-exitcode')
        for i in (1,2):
            tf(c,f'refresh-{i}',0,'apply','-refresh-only','-input=false','-auto-approve')
            tf(c,f'refresh-{i}-noop',0,'plan','-input=false','-detailed-exitcode')
        original=json.loads(config.read_text())
        original['resource']['adaptive_resource']['test']['tags']=['campaign=terraform-live','cohort=auth-modes','phase=updated']
        # Full original credential inputs retained after refresh; never derive from redacted state/export.
        assert all(original['resource']['adaptive_resource']['test'][k]==v for k,v in case['attributes'].items())
        config.write_text(json.dumps(original,indent=2)+'\n'); config.chmod(0o600)
        c['update_all_configured_credentials_resupplied']=True
        updateplan=tf(c,'update-plan',2,'plan','-input=false','-detailed-exitcode','-out=update.tfplan')
        if updateplan:
            tf(c,'update-apply',0,'apply','-input=false','-auto-approve','update.tfplan')
        readback(c,'updated-readback')
        tf(c,'updated-noop',0,'plan','-input=false','-detailed-exitcode')
        if c['type']=='ssh':
            paths=[p for p in (RUN/'evidence').glob(c['id']+'-update-apply-*.txt') if not p.name.endswith('-driver.txt')]
            text='\n'.join(p.read_text() for p in paths)
            c['expected_held_secret_blocker']={'observed': 'usePassword' in text and ('withheld' in text.lower() or 'held' in text.lower()),'unknown_usePassword_present':'usePassword' in text,'status_policy':'FAIL remains FAIL; no expected-exit flip'}
    c['status']='PASS' if created and all(p['status']=='PASS' for p in c['phases']) else 'FAIL'
    save(); print(c['id'],c['status'],flush=True)
# Private paths contain synthetic credential content and Terraform state. No values emitted.
save()
print(json.dumps({'cases':report['case_counts'],'phases':report['phase_counts'],'results':str(RESULT),'runtime_auth_precedence':'NOTRUN'}))
sys.exit(1 if report['case_counts'].get('FAIL') else 0)
PY
