#!/bin/bash
# Retained live configuration cohort; no runtime, destroy, or credential reads.
# Usage: bash testing/live/families.sh RUN BACKEND_REPO [all|authorizations|schedules|groups|scripts|complete|verify|report] [OWNED_ENDPOINT]
# complete performs only post-failure readback/clear and synthetic-body verification
# on this script's existing cohort objects. It never retries failed creates.
# Dependencies: Python 3, Ruby standard YAML/JSON, Terraform, adaptive, run.sh.
set -euo pipefail
umask 077
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
exec python3 - "$ROOT" "$@" <<'PY'
import collections, copy, datetime, json, os, pathlib, re, subprocess, sys, time
if len(sys.argv)<4:
    raise SystemExit('Usage: families.sh RUN BACKEND_REPO [MODE] [OWNED_ENDPOINT]')
ROOT=pathlib.Path(sys.argv[1]); RUN=pathlib.Path(sys.argv[2]).resolve(); BACK=pathlib.Path(sys.argv[3]).resolve()
sys.path.insert(0,str(ROOT/'testing/live'))
from family_expiry_compare import compare_schedule
MODE=sys.argv[4] if len(sys.argv)>4 else 'all'; ENDPOINT=sys.argv[5] if len(sys.argv)>5 else ''
assert MODE in ('all','authorizations','schedules','groups','scripts','complete','verify','report')
manifest=json.loads((RUN/'manifest.json').read_text()); PREFIX=manifest['prefix']
assert manifest['url']=='http://localhost:8080' and PREFIX.startswith('tflive-')
assert RUN.parent==pathlib.Path.home()/'IdeaProjects/.terraform-provider-adaptive-live'
OUT=RUN/'families'; OUT.mkdir(exist_ok=True,mode=0o700)
RUNNER=ROOT/'testing/live/run.sh'; RECORDS=OUT/'results.jsonl'
def now(): return datetime.datetime.now(datetime.timezone.utc).isoformat()
def versioned_output(stem,suffix):
    stamp=datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ')
    return OUT/(stem+'-v2-'+stamp+'-'+str(time.time_ns())+'.'+suffix)

def record_findings(rows):
    latest={}
    for row in rows: latest[(row['case'],row['phase'])]=row
    return dict(report_version=2,outcome_basis='recorded outcomes only; no product-defect inference',
      by_status=dict(collections.Counter(row['status'] for row in rows)),
      latest_by_case_phase=list(latest.values()),
      historical_nonpass_records=[row for row in rows if row['status']!='PASS'],
      boundary_labels=['configuration_readback_only','runtime_enforcement_not_tested',
        'script_execution_not_tested','deletion_not_tested','historical_failures_preserved',
        'schedule_expiry_semantic_not_literal_byte_proof'],
      comparison_policy='schedule-expiry-aware-instant-v1',
      policy_scope='Only schedule export expires_at uses aware-instant equality; raw actual/expected spelling is retained. Other fields and refreshed-state comparisons retain their existing gates.')

def record(case,phase,status,evidence='',exit=0,**extra):
    row=dict(at=now(),case=case,phase=phase,status=status,exit=exit,evidence=str(evidence),**extra)
    for p in (RECORDS,RUN/'ledger.jsonl'):
        with p.open('a') as f: f.write(json.dumps(row)+'\n')
    print(case,phase,status,flush=True); return status=='PASS'
def command(case,phase,args,expected=0):
    path=OUT/(case+'-'+phase+'-'+str(time.time_ns())+'.txt')
    with path.open('w') as f:
        r=subprocess.run(args,stdout=f,stderr=subprocess.STDOUT,cwd=ROOT)
    record(case,phase,'PASS' if r.returncode==expected else 'FAIL',path,r.returncode,expected_exit=expected,command=args)
    return r.returncode==expected,path

def tf(case,phase,sub='plan',expected=0,*args):
    return command(case,phase,['bash',str(RUNNER),'tf',str(RUN),case,phase,str(expected),sub,*args])

def state(case):
    p=RUN/'work'/case/'terraform.tfstate'
    if not p.exists(): return {}
    d=json.loads(p.read_text())
    for r in d.get('resources',[]):
        if r.get('mode')=='managed' and r.get('instances'): return r['instances'][0]['attributes']
    return {}

def config(case): return RUN/'work'/case/'main.tf.json'
def stage(case,attrs,label):
    p=config(case); d=json.loads(p.read_text()); family=next(iter(d['resource']))
    assert attrs['name'].startswith(PREFIX+'-')
    if family=='adaptive_schedule': assert attrs.get('is_active') is False
    if family=='adaptive_group': assert all(x.startswith(PREFIX+'-') for x in attrs.get('endpoints',[]))
    if family=='adaptive_script': assert attrs['endpoint'].startswith(PREFIX+'-')
    d['resource'][family]['test']=attrs
    snapshot=OUT/(case+'-'+label+'-'+str(time.time_ns())+'.tf.json'); snapshot.write_text(json.dumps(d,indent=2)+'\n')
    p.write_text(json.dumps(d,indent=2)+'\n'); return snapshot

def fixture(case,family,attrs):
    if config(case).exists():
        record(case,'existing-fixture','BLOCKED',config(case),reason='Retained fixture is never recreated or retried automatically; select a fresh run.')
        return False
    ok,_=command(case,'fixture',['bash',str(RUNNER),'fixture',str(RUN),case,family,json.dumps(attrs)])
    if ok: stage(case,attrs,'create')
    return ok

def yaml_parse(text):
    r=subprocess.run(['ruby','-ryaml','-rjson','-e','puts YAML.load(STDIN.read).to_json'],input=text,text=True,capture_output=True)
    if r.returncode: raise ValueError(r.stderr)
    return json.loads(r.stdout)

def compare(case,phase,actual,expected,policy=None):
    comparison=compare_schedule(actual,expected) if policy=='schedule-expiry-aware-instant-v1' else dict(policy='literal-structured-equality-v1',equal=actual==expected)
    good=comparison['equal']
    p=OUT/(case+'-'+phase+'-'+str(time.time_ns())+'.json')
    with p.open('x') as f: f.write(json.dumps(dict(actual=actual,expected=expected,comparison=comparison),indent=2)+'\n')
    return record(case,phase,'PASS' if good else 'FAIL',p,comparison_policy=comparison['policy'],observations=comparison.get('observations',[]))

def readback(case,kind,a,label):
    ok,p=command(case,'export-'+label,['bash',str(RUNNER),'readback',str(RUN),case,kind,a['name']])
    if not ok: return False
    # run.sh emits the YAML path in the ledger, not stdout. Filter by case + phase.
    rows=[json.loads(x) for x in (RUN/'ledger.jsonl').read_text().splitlines()]
    paths=[x['evidence'] for x in rows if x['case']==case and x['phase']=='readback']
    try:
        doc=yaml_parse(pathlib.Path(paths[-1]).read_text()); c=doc['configuration']
        if kind=='authorizations':
            actual={k:c.get(k) for k in ('name','integration_type','auth')}
            expected=dict(name=a['name'],integration_type=a['resource_type'],auth=a['permissions'])
        elif kind=='groups':
            actual=dict(name=c.get('name'),members=sorted(c.get('members',[])),endpoints=sorted(c.get('endpoints',[])),slack_channel_id=c.get('slack_channel_id',''))
            expected=dict(name=a['name'],members=sorted(a.get('members',[])),endpoints=sorted(a.get('endpoints',[])),slack_channel_id=a.get('slack_channel_id',''))
        elif kind=='scripts':
            actual={k:c.get(k,{} if k=='parameter_descriptions' else '') for k in ('name','endpoint','description','parameter_descriptions')}
            expected={k:a.get(k,{} if k=='parameter_descriptions' else '') for k in actual}
            actual['command_redacted']=c.get('command')=='<REDACTED>';expected['command_redacted']=True
        else:
            pattern=c.get('pattern',{}); mapping=dict(schedule_type='scheduleType',is_active='isActive',operation_type='operationType',max_access_time='maxAccessTime',expires_at='expiresAt')
            pm=dict(all_day='allDay',start_hour='startHour',start_minute='startMinute',end_hour='endHour',end_minute='endMinute',start_day='startDay',end_day='endDay',specific_dates='specificDates',weekdays='weekdays')
            expected=copy.deepcopy(a);actual={}
            for k,v in a.items():
                source=pattern if k in pm else c; key=pm.get(k,mapping.get(k,k)); default=[] if isinstance(v,list) else (False if isinstance(v,bool) else (0 if isinstance(v,int) else ''))
                if k=='expires_at' and key not in source: continue
                got=source.get(key,default)
                if isinstance(v,list): got=sorted(got or []);expected[k]=sorted(v)
                actual[k]=got
            if 'expiresAt' in c: actual['expires_at']=c['expiresAt']
        return compare(case,'exact-readback-'+label,actual,expected,policy='schedule-expiry-aware-instant-v1' if kind=='schedules' else None)
    except Exception as e: return record(case,'exact-readback-'+label,'FAIL',p,reason=str(e))

def refresh(case,a,label):
    tf(case,'refresh-'+label,'apply',0,'-refresh-only','-auto-approve')
    s=state(case);actual={k:s.get(k,[] if isinstance(v,list) else {}) for k,v in a.items()}
    expected=copy.deepcopy(a)
    for k,v in a.items():
        if isinstance(v,list): actual[k]=sorted(actual[k] or []);expected[k]=sorted(v)
    compare(case,'refreshed-state-'+label,actual,expected)
    tf(case,'noop-'+label,'plan',0,'-detailed-exitcode')

def apply_stage(case,kind,a,label):
    stage(case,a,label)
    # A repeated clear can legitimately be a no-op; the saved plan still gets
    # inspected by the shared no-delete guard before apply.
    ok,_=tf(case,label+'-plan','plan',0,'-input=false','-out='+label+'.tfplan')
    if ok: ok,_=tf(case,label,'apply',0,'-input=false','-auto-approve',label+'.tfplan')
    # Even a failing apply can have persisted changes; independent read is mandatory.
    readback(case,kind,a,label)
    if state(case).get('id'):
        p=RUN/'work'/case/'terraform.tfstate'
        (OUT/(case+'-post-'+label+'-'+str(time.time_ns())+'.tfstate')).write_text(p.read_text())
        refresh(case,a,label)
    return ok

def reject(case,a,field,value,label,diagnostic):
    bad=copy.deepcopy(a); bad[field]=value; snap=stage(case,bad,label)
    ok,p=tf(case,label,'plan',1,'-detailed-exitcode')
    # Actual Terraform output is in the inner runner ledger; normalize wrapping only.
    rows=[json.loads(x) for x in (RUN/'ledger.jsonl').read_text().splitlines()]
    paths=[x['evidence'] for x in rows if x['case']==case and x['phase']==label and x['evidence']!=str(p)]
    text=pathlib.Path(paths[-1]).read_text() if paths else p.read_text()
    compare(case,label+'-diagnostic',diagnostic in ' '.join(text.split()),True)
    stage(case,a,label+'-restore')

def create(case,kind,family,a):
    if not fixture(case,family,a): return False
    ok,_=tf(case,'create-plan','plan',2,'-input=false','-detailed-exitcode','-out=create.tfplan')
    if ok: ok,_=tf(case,'create','apply',0,'-input=false','-auto-approve','create.tfplan')
    readback(case,kind,a,'create')
    if ok: refresh(case,a,'create')
    else: record(case,'remaining-lifecycle','BLOCKED',config(case),reason='Create failed; remote evidence and local failed state retained without retry.')
    return ok

DESC='the backend update contract preserves empty descriptions; clearing requires backend support'
TYPE='resource_type cannot be updated: the backend filters updates by the requested integration type and cannot persist a type transition; backend support is required'
TZ='timezone cannot be updated: the backend Terraform schedule update contract does not persist timezone changes; backend support is required (no unsafe create/upsert replacement is attempted)'
DURATION='max_access_time cannot be cleared: the backend update contract preserves an omitted max_access_time; zero is a real 0m TTL, not an unset duration; clearing requires backend support'

# Derive the allowlist from source rather than silently freezing its count.
src=(ROOT/'internal/provider/components/authorization.go').read_text()
block=src.split('var validAuthorizedResourceTypes = map[string]bool{',1)[1].split('\n}',1)[0]
TYPES=re.findall(r'"([a-z0-9_]+)":\s+true',block)
assert len(TYPES)==16 and len(set(TYPES))==16
backend=(BACK/'inventorize-app/internal/handler/terraform.go').read_text()
bblock=backend.split('var mapAuthorizedResources = map[string]bool{',1)[1].split('\n}',1)[0]
assert len(re.findall(r'model\.\w+:\s+true',bblock))==16
PATTERNS=['weekdays','weekends','everyday','monthly','specific','custom']
validation=(BACK/'inventorize-app/internal/service/requestapproval/schedule_validation.go').read_text()
assert set(re.findall(r'model.ScheduleType(\w+)',validation))=={x.title() for x in PATTERNS}
source=dict(provider_authorization_types=TYPES,backend_authorization_count=16,schedule_patterns=PATTERNS,
 backend_head=subprocess.check_output(['git','-C',str(BACK),'rev-parse','HEAD'],text=True).strip(),
 provider_head=subprocess.check_output(['git','-C',str(ROOT),'rev-parse','HEAD'],text=True).strip(),
 limitations=['No external policy enforcement or runtime execution tested.','Authorization export omits description; description persistence is checked through refreshed provider state, not independent export.','Script normal export redacts body; all/complete additionally compares plaintext only for the exact synthetic cohort script.','Schedule expiry export uses schedule-expiry-aware-instant-v1 (not literal byte proof), retaining raw values and spelling observations. Historical failures remain unchanged.'])
assert source['provider_head']==manifest['commit'], 'Source HEAD differs from pinned run; no live cohort writes permitted.'
with versioned_output('source-counts','json').open('x') as f: f.write(json.dumps(source,indent=2)+'\n')
if MODE!='report':
    ok,p=command('families','identity',['adaptive','whoami','-o','json'])
    assert ok
    identity=json.loads(p.read_text());assert identity['URL']=='http://localhost:8080' and identity['CredentialSource']=='saved context'
    EMAIL=identity['Identity']['User']['Email']
    others=[x['Email'] for x in identity['Identity']['Details'].get('OtherUsers',[]) if x.get('Email')!=EMAIL]
    # Groups only affect new run-owned objects; no preexisting endpoint association.
    MEMBERS=[EMAIL]+others[:1]
    if ENDPOINT:
        assert ENDPOINT.startswith(PREFIX+'-')
        ok,_=command('families','endpoint-prerequisite',['bash',str(RUNNER),'readback',str(RUN),'families','endpoints',ENDPOINT])
        if not ok: ENDPOINT=''

def permission(t,updated=False):
    table='records_updated' if updated else 'records'
    db=PREFIX.replace('-','_')
    if t=='kubernetes': return 'apiVersion: rbac.authorization.k8s.io/v1\nkind: Role\nmetadata:\n  name: '+PREFIX+'-read\n  namespace: '+PREFIX+'\nrules:\n- apiGroups: [""]\n  resources: ["configmaps"]\n  verbs: ['+('get, list' if updated else 'get')+']\n'
    if t.startswith('mongodb'):
        return json.dumps(dict(role=db+'_read',privileges=[dict(resource=dict(db=db,collection=table),actions=['find'])],roles=[]))
    if t=='ssh': return 'deny:\n  operations: [sudo, su, rm'+(', shutdown' if updated else '')+']\nforbid_other_home_dirs: true\n'
    if t=='elasticsearch': return 'allow:\n- operation_type: GET\n  route: /'+PREFIX+'-'+table+'/_search\n'
    if t in ('sql_server','sqlserver_aws_secrets_manager'):
        return 'databases:\n- include_databases: ['+db+']\n  permissions:\n  - effect: allow\n    action: SELECT\n    object:\n      schema: dbo\n      name: '+table+'\n'
    if t=='cockroachdb':
        # Exercise the native SQL consumer contract through both write routes.
        return 'GRANT SELECT ON TABLE '+db+'.public.'+table+' TO '+db+'_read;'
    if t in ('mysql','mysql_aws_secrets_manager','proxysql'):
        return 'allow:\n- objects: ['+db+'.'+table+']\n  privileges: [SELECT]\n'
    return 'allow:\n- database: '+db+'\n  schema: [public]\n  objects: ['+table+']\n  privileges: [SELECT]\n'

if MODE in ('all','authorizations'):
    for t in TYPES:
        case='cohort-auth-'+t; a=dict(name=PREFIX+'-cohort-auth-'+t,resource_type=t,permissions=permission(t),description='Synthetic unbound read policy')
        if not create(case,'authorizations','adaptive_authorization',a): continue
        updated=dict(a,name=a['name']+'-updated',description='Synthetic unbound updated read policy',permissions=permission(t,True))
        if apply_stage(case,'authorizations',updated,'update'):
            reject(case,updated,'resource_type','mysql' if t!='mysql' else 'postgres','reject-type',TYPE)
            reject(case,updated,'description','','reject-description-clear','description cannot be cleared: '+DESC)
            reject(case,updated,'permissions','','reject-permissions-clear','permissions cannot be empty or whitespace')
            readback(case,'authorizations',updated,'final')

if MODE in ('all','groups'):
    case='cohort-group'; a=dict(name=PREFIX+'-cohort-group',members=MEMBERS,endpoints=[])
    if create(case,'groups','adaptive_group',a):
        reorder=dict(a,members=list(reversed(MEMBERS)));stage(case,reorder,'reorder')
        tf(case,'reorder-noop','plan',0,'-detailed-exitcode');readback(case,'groups',reorder,'reorder')
        updated=dict(a,members=[MEMBERS[-1]])
        apply_stage(case,'groups',updated,'membership-update')
        # A failing acknowledgement can follow a persisted membership mutation.
        # Always preserve state and exercise clear instead of hiding this phase.
        p=RUN/'work'/case/'terraform.tfstate'
        if p.exists(): (OUT/(case+'-before-clear-'+str(time.time_ns())+'.tfstate')).write_text(p.read_text())
        cleared=dict(a,members=[],endpoints=[]);apply_stage(case,'groups',cleared,'membership-clear')

if MODE in ('all','schedules'):
    # Bind only to the new empty owned group, and explicitly keep every schedule inactive.
    group=PREFIX+'-cohort-group'; exists=state('cohort-group').get('id')
    for pattern in PATTERNS:
        case='cohort-schedule-'+pattern
        a=dict(name=PREFIX+'-cohort-schedule-'+pattern,schedule_type=pattern,is_active=False,all_day=False,start_hour=9,start_minute=0,end_hour=17,end_minute=0,description='Inactive synthetic schedule',timezone='UTC',max_access_time=15,operation_type='autoreject',users=[EMAIL],teams=[group] if exists else [],endpoints=[ENDPOINT] if ENDPOINT else [],expires_at='2030-12-31T00:00:00Z')
        if pattern=='custom': a['weekdays']=['monday','wednesday']
        if pattern=='monthly': a.update(start_day=2,end_day=4)
        if pattern=='specific': a['specific_dates']=['2030-12-01T00:00:00Z','2030-12-02T00:00:00Z']
        if not create(case,'schedules','adaptive_schedule',a): continue
        updated=dict(a,description='Inactive updated synthetic schedule',start_hour=10,end_hour=18,max_access_time=20,operation_type='autoapprove')
        if pattern=='custom': updated['weekdays']=['tuesday','thursday']
        if pattern=='monthly': updated.update(start_day=3,end_day=6)
        if pattern=='specific': updated['specific_dates']=['2030-12-03T00:00:00Z']
        if not apply_stage(case,'schedules',updated,'update'): continue
        reject(case,updated,'timezone','Asia/Kolkata','reject-timezone',TZ)
        reject(case,updated,'timezone','','reject-timezone-clear',TZ)
        reject(case,updated,'max_access_time',0,'reject-duration-clear',DURATION)
        cleared=dict(updated,description='',users=[],teams=[],endpoints=[],expires_at='')
        apply_stage(case,'schedules',cleared,'supported-clear')

if MODE in ('all','scripts'):
    case='cohort-script'
    if not ENDPOINT: record(case,'prerequisite','BLOCKED',reason='An exact readback-verified run-owned endpoint must be supplied; no script create attempted.')
    else:
        a=dict(name=PREFIX+'-cohort-script',endpoint=ENDPOINT,command="SELECT '{{value}}' AS synthetic_value; -- "+PREFIX,description='Synthetic definition only; never executed',parameter_descriptions={'value':'Synthetic value'})
        if create(case,'scripts','adaptive_script',a):
            old=state(case).get('command_digest')
            updated=dict(a,name=a['name']+'-updated',command="SELECT '{{value}}' AS updated_synthetic_value; -- "+PREFIX,description='Updated synthetic definition only',parameter_descriptions={'value':'Updated synthetic value'})
            if apply_stage(case,'scripts',updated,'update'):
                new=state(case).get('command_digest');compare(case,'digest-changed',bool(old and new and old!=new),True)
                alternatives=[]
                for p in sorted((RUN/'work').glob('endpoint-*/terraform.tfstate')):
                    for r in json.loads(p.read_text()).get('resources',[]):
                        if r.get('type')=='adaptive_endpoint':
                            for i in r.get('instances',[]):
                                v=i.get('attributes',{}); n=v.get('name','')
                                if v.get('id') and n.startswith(PREFIX+'-') and n!=ENDPOINT: alternatives.append(n)
                if alternatives:
                    reject(case,updated,'endpoint',alternatives[0],'reject-endpoint','endpoint cannot be updated: script endpoint bindings are immutable')
                else: record(case,'reject-endpoint','BLOCKED',reason='No second existing owned endpoint; no invented binding substituted.')
                reject(case,updated,'description','','reject-description-clear','description cannot be cleared: '+DESC)
                reject(case,updated,'command','','reject-command-clear','command cannot be empty or whitespace')
                cleared=dict(updated,parameter_descriptions={});apply_stage(case,'scripts',cleared,'parameters-clear')

if MODE in ('all','complete'):
    # Preserve the observed post-error Terraform state before a read reconciles it.
    case='cohort-group'
    if state(case).get('id'):
        p=RUN/'work'/case/'terraform.tfstate'
        (OUT/(case+'-failed-state-'+str(time.time_ns())+'.tfstate')).write_text(p.read_text())
        a=json.loads(config(case).read_text())['resource']['adaptive_group']['test']
        readback(case,'groups',a,'post-failed-membership-update')
        refresh(case,a,'post-failed-membership-update')
        cleared=dict(a,members=[],endpoints=[])
        apply_stage(case,'groups',cleared,'membership-clear')
        # Endpoint membership only on exact verified run-owned endpoints and with
        # no users in this group. The second endpoint is optional, never fabricated.
        endpoints=[]
        for p in sorted((RUN/'work').glob('endpoint-*/terraform.tfstate')):
            d=json.loads(p.read_text())
            for r in d.get('resources',[]):
                if r.get('type')!='adaptive_endpoint': continue
                for i in r.get('instances',[]):
                    v=i.get('attributes',{}); name=v.get('name','')
                    if v.get('id') and name.startswith(PREFIX+'-'):
                        ok,_=command(case,'verify-owned-endpoint',['bash',str(RUNNER),'readback',str(RUN),case,'endpoints',name])
                        if ok: endpoints.append(name)
        endpoints=sorted(set(endpoints))
        if endpoints:
            bound=dict(cleared,endpoints=endpoints)
            apply_stage(case,'groups',bound,'endpoint-membership-update')
            if len(endpoints)>1:
                reordered=dict(bound,endpoints=list(reversed(endpoints)))
                stage(case,reordered,'endpoint-reorder')
                tf(case,'endpoint-reorder-noop','plan',0,'-detailed-exitcode')
                readback(case,'groups',reordered,'endpoint-reorder')
            apply_stage(case,'groups',cleared,'endpoint-membership-clear')
    case='cohort-script'
    if state(case).get('id'):
        a=json.loads(config(case).read_text())['resource']['adaptive_script']['test']
        assert a['name']==PREFIX+'-cohort-script-updated' and a['command'].endswith('-- '+PREFIX)
        # This one exact script was authored above and contains only synthetic SQL.
        # No other script or resource secret export is permitted.
        ok,p=command(case,'synthetic-body-export',['adaptive','admin','export','scripts','--name',a['name'],'--include-secrets'])
        if ok:
            doc=yaml_parse(p.read_text());c=doc['configuration']
            compare(case,'synthetic-body-exact',c.get('command'),a['command'])
        readback(case,'scripts',a,'final')


if MODE in ('all','verify'):
    kinds=dict(adaptive_authorization='authorizations',adaptive_group='groups',adaptive_schedule='schedules',adaptive_script='scripts')
    for p in sorted((RUN/'work').glob('cohort-*/main.tf.json')):
        case=p.parent.name;d=json.loads(p.read_text());family=next(iter(d['resource']));a=d['resource'][family]['test']
        if not state(case).get('id'): continue
        readback(case,kinds[family],a,'verified-final')
        tf(case,'verified-final-noop','plan',0,'-detailed-exitcode')
    case='cohort-script'
    if state(case).get('id'):
        a=json.loads(config(case).read_text())['resource']['adaptive_script']['test']
        alternatives=[]
        for p in sorted((RUN/'work').glob('endpoint-*/terraform.tfstate')):
            for r in json.loads(p.read_text()).get('resources',[]):
                if r.get('type')=='adaptive_endpoint':
                    for i in r.get('instances',[]):
                        v=i.get('attributes',{});n=v.get('name','')
                        if v.get('id') and n.startswith(PREFIX+'-') and n!=a['endpoint']: alternatives.append(n)
        if alternatives:
            ok,_=command(case,'verify-alternate-binding',['bash',str(RUNNER),'readback',str(RUN),case,'endpoints',alternatives[0]])
            if ok: reject(case,a,'endpoint',alternatives[0],'reject-existing-endpoint','endpoint cannot be updated: script endpoint bindings are immutable')

rows=[json.loads(x) for x in RECORDS.read_text().splitlines()] if RECORDS.exists() else []
fixtures=[]
for p in sorted((RUN/'work').glob('cohort-*/main.tf.json')):
    d=json.loads(p.read_text());family=next(iter(d['resource']));a=d['resource'][family]['test'];s=state(p.parent.name)
    fixtures.append(dict(case=p.parent.name,family=family,name=s.get('name',a['name']),id=s.get('id'),state=str(p.parent/'terraform.tfstate'),configuration=str(p)))
expected_counts=dict(adaptive_authorization=len(TYPES),adaptive_schedule=len(PATTERNS),adaptive_group=1,adaptive_script=1)
observed_counts=dict(collections.Counter(x['family'] for x in fixtures))
coverage=dict(expected_fixture_counts=expected_counts,observed_fixture_counts=observed_counts,fixture_counts_match=observed_counts==expected_counts,
 created_ids=sum(bool(x['id']) for x in fixtures),authorization_create_attempts=len({x['case'] for x in rows if x['case'].startswith('cohort-auth-') and x['phase']=='create'}),
 diagnostic_assertions=sum(x['phase'].endswith('-diagnostic') for x in rows),diagnostic_assertions_passed=sum(x['phase'].endswith('-diagnostic') and x['status']=='PASS' for x in rows))
with versioned_output('fixtures','tsv').open('x') as f:
    f.write('case\tfamily\texact_name\tid\n')
    for x in fixtures: f.write('\t'.join(str(x[k] or '-') for k in ('case','family','name','id'))+'\n')
report=dict(source=source,records=len(rows),by_status=dict(collections.Counter(x['status'] for x in rows)),fixtures=fixtures,coverage=coverage,
 fixture_counts=observed_counts,failures=[x for x in rows if x['status']!='PASS'])
with versioned_output('findings','json').open('x') as f: f.write(json.dumps(record_findings(rows),indent=2)+'\n')
with versioned_output('report','json').open('x') as f: f.write(json.dumps(dict(report_version=2,**report),indent=2)+'\n')
print(json.dumps({k:v for k,v in report.items() if k not in ('failures','source')},indent=2))
sys.exit(1 if report['failures'] else 0)
PY
