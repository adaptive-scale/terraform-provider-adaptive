#!/bin/bash
# Additional Adaptive-origin families. Retain every object and workload.
# Usage: family-imports.sh RUN BACKEND_REPO [all|complete|verify|report]
set -euo pipefail
umask 077
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
exec python3 - "$ROOT" "$@" <<'PY'
import collections, copy, datetime, http.server, json, os, pathlib, re, subprocess, sys, threading, time, urllib.request
ROOT=pathlib.Path(sys.argv[1]); RUN=pathlib.Path(sys.argv[2]).resolve(); BACK=pathlib.Path(sys.argv[3]).resolve()
MODE=sys.argv[4] if len(sys.argv)>4 else 'all'
assert MODE in ('all','complete','verify','report')
os.umask(0o077)
m=json.loads((RUN/'manifest.json').read_text()); PREFIX=m['prefix']; R=ROOT/'testing/live/run.sh'
assert RUN.parent==pathlib.Path.home()/'IdeaProjects/.terraform-provider-adaptive-live'
assert m['url']=='http://localhost:8080' and PREFIX.startswith('tflive-')
assert subprocess.check_output(['git','-C',str(ROOT),'rev-parse','HEAD'],text=True).strip()==m['commit']
assert not any(os.environ.get(k) for k in ('ADAPTIVE_URL','ADAPTIVE_SVC_TOKEN','ADAPTIVE_PROFILE'))
OUT=RUN/'family-imports'; OUT.mkdir(mode=0o700,exist_ok=True)
OWN=OUT/'ownership.jsonl'; RESULTS=OUT/'results.jsonl'
FAMILIES=[('authorization','authorizations','cohort-auth-postgres'),('group','groups','cohort-group'),('schedule','schedules','cohort-schedule-custom'),('dataprotection','dataprotection','dataprotection'),('endpoint','endpoints','endpoint-client'),('script','scripts','cohort-script')]
def save(p,value):
    p.write_text(json.dumps(value,indent=2)+'\n'); p.chmod(0o600); return p
def append(p,value):
    with p.open('a') as f:
        f.write(json.dumps(value)+'\n'); f.flush(); os.fsync(f.fileno())
    p.chmod(0o600)
def rows(p): return [json.loads(s) for s in p.read_text().splitlines()] if p.exists() else []
def record(case,phase,good,evidence='',**kw):
    row=dict(at=datetime.datetime.now(datetime.timezone.utc).isoformat(),case=case,phase=phase,status='PASS' if good else 'FAIL',evidence=str(evidence),**kw)
    append(RESULTS,row); append(RUN/'ledger.jsonl',row); print(case,phase,row['status'],flush=True)
    return good
def cmd(case,phase,args,expected=0):
    p=OUT/(case+'-'+phase+'-'+str(time.time_ns())+'.txt')
    with p.open('w') as f, p.with_suffix('.stderr').open('w') as err: rc=subprocess.run(args,cwd=ROOT,stdout=f,stderr=err).returncode
    record(case,phase,rc==expected,p,exit=rc,expected_exit=expected)
    return rc==expected,p
def tf(case,phase,sub,*args): return cmd(case,phase,['bash',str(R),'tf',str(RUN),case,phase,'0',sub,*args])[0]
def attrs(case,family): return json.loads((RUN/'work'/case/'main.tf.json').read_text())['resource']['adaptive_'+family]['test']
def state(case,family):
    p=RUN/'work'/case/'terraform.tfstate'
    if p.exists():
        for r in json.loads(p.read_text()).get('resources',[]):
            if r.get('type')=='adaptive_'+family and r.get('instances'): return r['instances'][0]['attributes']
    return {}
def yaml(text):
    p=subprocess.run(['ruby','-ryaml','-rjson','-e','puts YAML.load(STDIN.read).to_json'],input=text,text=True,capture_output=True)
    if p.returncode: raise ValueError('YAML parse failed')
    return json.loads(p.stdout)
def equal(case,phase,actual,expected):
    p=save(OUT/(case+'-'+phase+'-'+str(time.time_ns())+'.json'),dict(actual=actual,expected=expected))
    return record(case,phase,actual==expected,p)
def export(case,kind,name,phase,secret=False):
    assert name.startswith(PREFIX+'-')
    # The only plaintext export allowed is the exact authored synthetic source.
    if secret: assert kind=='scripts' and name==PREFIX+'-cohort-script-updated'
    ok,p=cmd(case,phase,['adaptive','admin','export',kind,'--name',name]+(['--include-secrets'] if secret else []))
    if not ok: raise ValueError(phase+' failed')
    doc=yaml(p.read_text())
    # CLI appends an informational WARNING YAML key on cleartext exports;
    # it is not part of the admin document schema (the payload stays exact).
    if 'WARNING' in doc:
        save(OUT/(case+'-'+phase+'-warning.json'),{'WARNING':doc.pop('WARNING')})
    return doc
def api_identity(case,family,name):
    # Installed CLI intentionally drops group/script acknowledgement IDs. Read
    # the authoritative, body-free client listing; filter BEFORE persisting.
    config=json.loads((pathlib.Path.home()/'.adaptive/token').read_text())
    defaults=[x for x in config['deployments'].values() if x.get('default')]
    assert len(defaults)==1 and defaults[0]['url']=='http://localhost:8080'
    route={'group':'/api/v1/team/list','script':'/api/v1/cli/session/script/list.by.endpoint/'+PREFIX+'-import-family-endpoint','endpoint':'/api/v1/cli/admin/endpoint.list'}[family]
    req=urllib.request.Request('http://localhost:8080'+route,headers={'Authorization':defaults[0]['token']})
    with urllib.request.urlopen(req,timeout=30) as response: items=json.load(response)
    selected=[{k:v for k,v in x.items() if k.lower() in ('id','name','sessionid','status','type')} for x in items if x.get('name',x.get('Name'))==name]
    assert len(selected)==1, 'expected one exact owned API identity'
    save(OUT/(case+'-api-identity.json'),dict(route=route,items=selected))
    return selected[0].get('id',selected[0].get('ID'))
def cli_apply_ack(case,family,document):
    # A one-request loopback relay preserves the CLI's actual API response,
    # which its group/script presentation otherwise discards. Tokens remain
    # in memory/child environment; no credential file or saved-context change.
    route={'group':'/api/v1/cli/admin/team.create.update','script':'/api/v1/cli/admin/script.create.update'}[family]
    cfg=json.loads((pathlib.Path.home()/'.adaptive/token').read_text())
    defaults=[x for x in cfg['deployments'].values() if x.get('default')]
    assert len(defaults)==1 and defaults[0]['url']=='http://localhost:8080'
    replies=[]
    class Relay(http.server.BaseHTTPRequestHandler):
        def log_message(self,*args): pass
        def do_POST(self):
            if self.path!=route or replies:
                self.send_error(403); return
            body=self.rfile.read(int(self.headers['Content-Length']))
            request=urllib.request.Request('http://localhost:8080'+route,data=body,headers={'Authorization':self.headers['Authorization']},method='POST')
            try:
                with urllib.request.urlopen(request,timeout=60) as response: status=response.status;answer=response.read()
            except urllib.error.HTTPError as e: status=e.code;answer=e.read()
            replies.append(dict(route=route,status=status,body=answer.decode()))
            self.send_response(status);self.send_header('Content-Type','application/json');self.send_header('Content-Length',str(len(answer)));self.end_headers();self.wfile.write(answer)
    server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Relay)
    thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
    env=os.environ.copy();env.update(ADAPTIVE_URL='http://127.0.0.1:'+str(server.server_port),ADAPTIVE_SVC_TOKEN=defaults[0]['token'])
    log=OUT/(case+'-cli-ack-'+str(time.time_ns())+'.txt')
    try:
        with log.open('w') as f: rc=subprocess.run(['adaptive','admin','apply','-f',str(document)],env=env,stdout=f,stderr=subprocess.STDOUT).returncode
    finally: server.shutdown();server.server_close();thread.join()
    save(log.with_suffix('.ack.json'),replies)
    good=rc==0 and len(replies)==1 and replies[0]['status']==200
    record(case,'cli-create',good,log,exit=rc,acknowledgement=str(log.with_suffix('.ack.json')))
    return good,log
def identity(case,family,name,ack):
    text=re.sub(r'\x1b\[[0-9;]*[A-Za-z]','',ack)
    if family in ('group','script'):
        owned=[x for x in rows(OWN) if x['case']==case][-1]
        path=pathlib.Path(owned['acknowledgement']).with_suffix('.ack.json')
        if path.exists():
            response=json.loads(path.read_text());assert len(response)==1 and response[0]['status']==200
            response=json.loads(response[0]['body']);assert response['name']==name
            return response['id']
        return api_identity(case,family,name)
    if family=='endpoint': return api_identity(case,family,name)
    if family=='authorization': matches=re.findall(re.escape(name)+r': created authorization with id: ([0-9a-f-]{36})',text)
    elif family=='schedule': matches=re.findall(re.escape(name)+r' \(id=([a-zA-Z0-9_-]+),',text)
    elif family=='dataprotection':
        ok,p=cmd(case,'identity-listing',['adaptive','admin','list-resources']); assert ok
        matches=[]
        for line in p.read_text().splitlines():
            c=line.split('│')
            if len(c)>3 and c[1].strip()==name:
                x=re.fullmatch(r'/resources/([0-9a-f-]{36})',c[3].strip())
                if x: matches.append(x.group(1))
    else:
        ok,p=cmd(case,'identity-listing',['adaptive','admin','list-endpoints']); assert ok
        matches=[]
        for line in p.read_text().splitlines():
            c=line.split('│')
            if len(c)>3 and c[1].strip()==name:
                matches.extend(re.findall(r'(?:/sessions/|/endpoints/)([0-9a-f-]{36})',line))
    assert len(set(matches))==1, 'expected one exact owned CLI identity, not an inferred ID'
    return matches[0]
def verify(case,family,kind,a,expected):
    actual=export(case,kind,a.get('name',a.get('resource')),'final-independent-export')
    # Auth description is absent from the CLI surface. The comparison covers
    # every exported field, and the refreshed Terraform state checks description.
    # Script clone plaintext is NOT exported: redaction plus provider digest
    # and the exact synthetic source export establish its body round trip.
    want=copy.deepcopy(expected)
    if family=='script': want['configuration']['command']='<REDACTED>'
    good=equal(case,'independent-exact-document',actual,want)
    s=state(case,family)
    # Preserve raw structure as diagnostic evidence, not an authoritative
    # empty/null assertion. Missing keys stay missing (never .get -> null).
    got={k:s[k] for k in a if k in s}
    raw=dict(comparison='raw structural comparison',actual=got,expected=a,
             equal=json.dumps(got,sort_keys=True)==json.dumps(a,sort_keys=True))
    p=save(OUT/(case+'-raw-source-attrs-state-'+str(time.time_ns())+'.json'),raw)
    observation=dict(at=datetime.datetime.now(datetime.timezone.utc).isoformat(),
                     case=case,phase='raw-source-attrs-state',status='OBSERVED',
                     evidence=str(p),comparison=raw['comparison'],structural_equal=raw['equal'])
    append(RESULTS,observation); append(RUN/'ledger.jsonl',observation)
    # Use this run's pinned provider schema, never a rebuilt/current schema.
    sys.path.insert(0,str(ROOT/'testing/live'))
    from family_compare import compare_attributes
    pinned=json.loads((RUN/'inventory/schema.json').read_text())
    block=pinned['provider_schemas']['registry.terraform.io/adaptive-scale/adaptive']['resource_schemas']['adaptive_'+family]['block']
    semantic=compare_attributes(s,a,block,family)
    p=save(OUT/(case+'-canonical-source-attrs-state-'+str(time.time_ns())+'.json'),semantic)
    good=record(case,'canonical-source-attrs-state',semantic['equal'],p,
                comparison=semantic['comparison']) and good
    if family=='script':
        # Digests are keyed and object-bound, not a plain hash of the body.
        cfg=json.loads((pathlib.Path.home()/'.adaptive/token').read_text())
        defaults=[x for x in cfg['deployments'].values() if x.get('default')]
        assert len(defaults)==1 and defaults[0]['url']=='http://localhost:8080'
        route='/api/v1/cli/admin/export.script?name='+a['name']
        req=urllib.request.Request('http://localhost:8080'+route,headers={'Authorization':defaults[0]['token']})
        with urllib.request.urlopen(req,timeout=30) as response: data=json.load(response)
        assert data['redacted'] is True and len(data['items'])==1 and data['items'][0]['name']==a['name']
        save(OUT/(case+'-independent-redacted-digest-'+str(time.time_ns())+'.json'),data)
        good=equal(case,'independent-command-digest',s.get('command_digest'),data['items'][0]['digests']['command']) and good
    if family=='endpoint':
        ok,p=cmd(case,'retained-pod-status',['adaptive','admin','endpoint','list','--endpoint-type','postgres'])
        selected=[l.split('│') for l in p.read_text().splitlines() if len(l.split('│'))>5 and l.split('│')[1].strip()==a['name']]
        good=equal(case,'endpoint-retained-state',len(selected)==1 and selected[0][4].strip() in ('created','paused'),True) and good and ok
    return good
# Source compatibility is established before any CLI write.
h=BACK/'inventorize-app/internal/handler'
checks={
 'group_ack_id':('cli_group_script.go','"id": team.TeamID'),
 'script_ack_id':('cli_group_script.go','"id": script.ID'),
 'auth_ack_id':('cli.go','created authorization with id:'),
 'schedule_ack_id':('cli_schedule.go','ID:                  schedule.ID'),
 'dp_resource_id':('export_docs.go','ID: row.ResourceID'),
 'endpoint_backend_type':('export_docs.go','IntegrationType:       row.IntegrationType'),
 'endpoint_transport':('export_docs.go','AccessType:            row.SessionType')}
assert all(needle in (h/file).read_text() for file,needle in checks.values())
save(OUT/'source-compatibility.json',dict(checks=checks,backend_head=subprocess.check_output(['git','-C',str(BACK),'rev-parse','HEAD'],text=True).strip(),
 notes=['Group/script CLI discards actual ack ID; loopback relay forwards only that one apply route to localhost:8080 and retains actual response. Token stays in memory/child environment, never in a copied credential file. Parent Terraform runner always uses original saved local context.',
 'Schedule IDs and endpoint IDs are backend literal tokens, not UUIDs. No token normalization or invented identifiers.',
 'Data protection import key is the actual resource ID, not the policy row ID. Generated masked authorization retained separately.',
 'Endpoint backend integration_type=postgres, access_type=client; explicit cluster k8s-local; no native sessions or script executions.',
 'Authorization description export omission is checked via refreshed Terraform state only.']))
if MODE!='report':
    ok,p=cmd('family-imports','identity',['adaptive','whoami','-o','json']); assert ok
    ident=json.loads(p.read_text()); assert ident['URL']==m['url'] and ident['CredentialSource']=='saved context'
for family,kind,source in FAMILIES:
    if MODE=='report': break
    case='import-family-'+family; stage='prerequisite'; a={}; owned=None
    try:
        existing=[x for x in rows(OWN) if x['case']==case]
        if MODE=='verify' or (MODE=='complete' and existing and existing[-1]['stage']=='complete'):
            assert existing and state(case,family).get('id'), 'no imported state'
            owned=existing[-1]; a=attrs(case,family); expected=json.loads(pathlib.Path(owned['document']).read_text())
            ok=verify(case,family,kind,a,expected)
            ok=tf(case,'verify-noop','plan','-input=false','-detailed-exitcode') and ok
            record(case,'verified',ok)
            if ok:
                owned.pop('failure_stage',None);owned.pop('reason',None);owned['stage']='complete';append(OWN,owned)
            continue
        assert MODE=='complete' or (not existing and not (RUN/'work'/case).exists()), 'retained attempt exists; use complete/verify/report (never recreate or state-rm)'
        prior=existing[-1] if existing else None
        if MODE=='complete' and family=='group' and prior and prior.get('cli_apply_success') and not prior.get('id'):
            # Never reapply the first retained group just to recover its ID.
            # Preserve that acknowledged-but-ID-withheld orphan, and create a
            # fresh identity with response capture enabled.
            original=copy.deepcopy(prior);original['case']=case+'-initial'
            original['document']=str(save(OUT/(case+'-initial.json'),json.loads(pathlib.Path(prior['document']).read_text())))
            original['stage']='orphan-unverified';append(OWN,original)
            prior=None
        a=copy.deepcopy(attrs(source,family)); s=state(source,family)
        source_name=a.get('name',a.get('resource')); assert s.get('id') and source_name.startswith(PREFIX+'-')
        assert s.get('name',s.get('resource'))==source_name
        stage='source-export'; doc=export(case,kind,source_name,'source-export',family=='script')
        c=doc['configuration']; assert c.get('name',c.get('resource_name'))==source_name
        name=PREFIX+'-import-family-'+family
        if family=='group' and any(x['case']==case+'-initial' for x in rows(OWN)): name+='-ack'
        if family=='dataprotection':
            # Existing run-owned, CLI-origin fake host; never overwrite the runtime policy.
            name=PREFIX+'-import-postgres'; target=state('import-postgres','resource')
            assert target['id'] and target['name']==name
            assert target['host']=='tf-live.invalid'
            a['resource']=name; c['resource_name']=name
            if not prior:
                ok,p=cmd(case,'policy-absence',['adaptive','admin','export','dataprotection','--name',name],1)
                assert ok and 'no such data protection policy' in (p.read_text()+p.with_suffix('.stderr').read_text()), 'target policy absence not proven'
        else:
            a['name']=name; c['name']=name
            # Exact-name source lookup before fresh create prevents accidental upserts.
            if not prior or not prior.get('cli_apply_success'):
                ok,p=cmd(case,'fresh-name-absence',['adaptive','admin','export',kind,'--name',name],1)
                assert ok and ('no such ' in (p.read_text()+p.with_suffix('.stderr').read_text())), 'fresh-name absence not proven'
        if family=='schedule': assert a['is_active'] is False and not any(a.get(k) for k in ('users','teams','endpoints','expires_at'))
        if family=='group': assert not a.get('members') and not a.get('endpoints')
        if family=='script':
            assert source_name==PREFIX+'-cohort-script-updated'
            assert c['command']==a['command'] and a['command'].endswith('-- '+PREFIX) and a['command'].startswith('SELECT ')
            endpoint=state('import-family-endpoint','endpoint')
            assert endpoint['id'] and endpoint['name']==PREFIX+'-import-family-endpoint'
            a['endpoint']=endpoint['name'];c['endpoint']=endpoint['name']
        if family=='endpoint':
            assert c['integration_type']=='postgres' and c['access_type']=='client'
            assert c['integration_name']==a['resource']==PREFIX+'-runtime-postgres'
            assert c['cluster']==a['cluster']=='k8s-local'
            # Reconciliation must retain an exported input cloned by the CLI,
            # but never override an explicitly configured value (even empty/null).
            if 'idle_timeout' in c:
                assert isinstance(c['idle_timeout'],str), 'exported idle_timeout must be a string'
                if 'idle_timeout' not in a and c['idle_timeout']:
                    pinned=json.loads((RUN/'inventory/schema.json').read_text())
                    spec=pinned['provider_schemas']['registry.terraform.io/adaptive-scale/adaptive']['resource_schemas']['adaptive_endpoint']['block']['attributes'].get('idle_timeout',{})
                    assert (spec.get('type')=='string' and spec.get('optional') is True
                            and not any(spec.get(k) for k in ('required','computed','sensitive'))), 'pinned idle_timeout must be an optional string input'
                    a['idle_timeout']=c['idle_timeout']
        document=save(OUT/(case+'.json'),doc)
        owned=prior or dict(case=case,family=family,kind=kind,name=name,source=source_name,document=str(document),origin='adaptive-cli',id=None,stage='before-cli-write',generated_authorization='masked_'+name if family=='dataprotection' else None)
        append(OWN,owned) # Durable separate ownership MUST precede every CLI create.
        if not prior or not prior.get('cli_apply_success'):
            if prior:
                reason=pathlib.Path(prior['acknowledgement']).read_text()
                assert 'no changes were made' in reason or (family=='script' and 'same script already exists with name' in reason), 'Ambiguous failed write is never retried'
            stage='cli-create'
            if family in ('group','script'): ok,p=cli_apply_ack(case,family,document)
            else: ok,p=cmd(case,stage,['adaptive','admin','apply','-f',str(document),'--timeout','3m'])
            owned.update(stage=stage,cli_apply_success=ok,acknowledgement=str(p));append(OWN,owned)
        else: ok=True;p=pathlib.Path(prior['acknowledgement'])
        # Endpoint CLI can time out after creation; try identity capture regardless.
        stage='identity'; actual_id=identity(case,family,name,p.read_text())
        assert isinstance(actual_id,str) and re.fullmatch(r'[a-zA-Z0-9_-]+',actual_id), 'invalid literal source ID'
        owned.update(id=actual_id,stage=stage);append(OWN,owned);record(case,stage,True,p,id=actual_id)
        if not ok: raise ValueError('CLI create failed; exact identity retained, import not attempted')
        if not (RUN/'work'/case/'main.tf.json').exists():
            stage='fixture'; assert cmd(case,stage,['bash',str(R),'fixture',str(RUN),case,'adaptive_'+family,json.dumps(a)])[0]
        if not state(case,family).get('id'):
            stage='import'; assert tf(case,stage,'import','-input=false','adaptive_'+family+'.test',actual_id)
        else: assert state(case,family)['id']==actual_id, 'existing state identity differs'
        owned.update(stage='imported');append(OWN,owned)
        for cycle in (1,2):
            stage='refresh-'+str(cycle); assert tf(case,stage,'apply','-refresh-only','-input=false','-auto-approve')
        stage='reconcile-plan'; assert tf(case,stage,'plan','-input=false','-out=reconcile.tfplan')
        stage='reconcile-apply'; assert tf(case,stage,'apply','-input=false','-auto-approve','reconcile.tfplan')
        stage='noop'; noop=tf(case,stage,'plan','-input=false','-detailed-exitcode')
        stage='independent-readback'; good=verify(case,family,kind,a,doc)
        record(case,'complete',noop and good,id=actual_id)
        owned.pop('failure_stage',None);owned.pop('reason',None)
        owned.update(stage='complete' if noop and good else 'verification-failed')
        if not (noop and good): owned['failure_stage']='noop' if not noop else 'independent-readback'
        append(OWN,owned)
    except Exception as e:
        record(case,stage+'-semantic',False,reason=str(e),id=owned.get('id') if owned else None)
        if owned:
            owned.update(stage='failed',failure_stage=stage,reason=str(e));append(OWN,owned)
        # Failures do not prevent the other registered families from being attempted.
allowned={x['case']:x for x in rows(OWN)}
latest={k:v for k,v in allowned.items() if k in ['import-family-'+f for f,_,_ in FAMILIES]}
allrows=rows(RESULTS); expected=['import-family-'+f for f,_,_ in FAMILIES]
summary=dict(expected_cases=expected,attempted=len(latest),fixtures=list(latest.values()),additional_orphans=[v for k,v in allowned.items() if k not in latest],by_status=dict(collections.Counter(x['status'] for x in allrows)),failures=[x for x in allrows if x['status']=='FAIL'],
 boundaries='All fixtures retained. No state rm, untaint, destroy, existing-object mutation, native authentication or script execution. Cleanup only planned/guarded, not live-tested.')
save(OUT/'report.json',summary)
print(json.dumps(dict(attempted=len(latest),fixtures=[dict(case=x['case'],id=x.get('id'),stage=x['stage'],failure_stage=x.get('failure_stage')) for x in latest.values()],failures=len(summary['failures'])),indent=2))
# A complete family success requires semantic assertions, not just a zero CLI exit.
sys.exit(0 if len(latest)==6 and not summary['failures'] else 1)
PY
