"""Execute the real probe offline with isolated CLI/Terraform process doubles."""
import copy
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

HERE = Path(__file__).resolve().parent
CANARY = "offline-owned-synthetic-account"
PREFIX = "tflive-offline"
ID = "11111111-1111-1111-1111-111111111111"

TOOL = r'''
import json, os, pathlib, sys
r=pathlib.Path(os.environ['OFFLINE_RUN']); scenario=os.environ.get('SCENARIO','good')
a=sys.argv[1:]
with (r/'trace').open('a') as f: f.write(json.dumps(a)+'\n')
if a[0]=='whoami':
 print(json.dumps({'URL':'http://localhost:8080','CredentialSource':'saved context'})); sys.exit(0)
if a[:2]==['admin','export']:
 d=json.loads((r/'remote.json').read_text())
 if '--include-secrets' not in a:
  d['configuration']['service_account']='<REDACTED>'
  if scenario=='defaultvisible': d['configuration']['service_account']=os.environ['CANARY']
 if (r/'probed').exists() and not (r/'restored').exists():
  if scenario=='malformed': print('not: [yaml');sys.exit(0)
  if scenario=='missing': del d['configuration']['service_account']
  if scenario=='wrongname': d['configuration']['name']='not-owned'
  if scenario=='wrongtype': d['configuration']['type']='postgres'
 if scenario=='restorewrong' and (r/'restored').exists() and '--include-secrets' in a:
  d['configuration']['service_account']='wrong'
 print(json.dumps(d));sys.exit(0)
# run.sh tf RUN CASE PHASE EXPECTED COMMAND ...
_,run,case,phase,expected,*command=a
rc=0
if phase=='unclassified-omit-plan': rc=2
if phase=='unclassified-omission-rejected':
 (r/'probed').touch(); rc=1
 d=json.loads((r/'remote.json').read_text())
 if scenario=='remoteempty': d['configuration']['service_account']=''
 if scenario=='wrongcredential': d['configuration']['service_account']='wrong'
 if scenario=='tagmutation': d['userTags'].append('guard=must-not-drop-account')
 if scenario=='configmutation': d['configuration']['connect_server_url']='changed'
 (r/'remote.json').write_text(json.dumps(d))
if phase=='unclassified-restore-apply':
 (r/'restored').touch(); (r/'remote.json').write_text((r/'baseline.json').read_text())
if phase=='unclassified-restored-noop' and scenario=='noopfail': rc=2
log=r/'evidence'/(phase+'.txt')
log.write_text('unrelated failure' if scenario=='wrongdiagnostic' else 'updating resource would clear server-held values for service_account; provide the matching configuration arguments')
with (r/'ledger.jsonl').open('a') as f:
 f.write(json.dumps(dict(case=case,phase=phase,status='PASS' if rc==int(expected) else 'FAIL',exit=rc,evidence=str(log)))+'\n')
sys.exit(0 if rc==int(expected) else 1)
'''


class ProbeTests(unittest.TestCase):
    def execute(self, scenario="good", corrupt=None, extra_env=None):
        with tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR")) as tmp:
            root = Path(tmp).resolve()
            live = root / "testing/live"
            live.mkdir(parents=True)
            (live / "unclassified-secrets.sh").write_bytes((HERE / "unclassified-secrets.sh").read_bytes())
            (live / "run.sh").write_text('#!/bin/bash\nexec python3 "$OFFLINE_TOOL" "$@"\n')
            run = root / "run"
            for part in ["evidence", "inventory", "cli-origin", "bin", "work/import-1password", "work/integration-1password"]:
                (run / part).mkdir(parents=True)
            inventory = {"cases": [{"type": "1password", "attributes": {"service_account": CANARY}}]}
            for p in [live / "integration-cases.json", run / "inventory/integration-cases.json"]:
                p.write_text(json.dumps(inventory))
            (run / "manifest.json").write_text(json.dumps(dict(prefix=PREFIX,url="http://localhost:8080")))
            config = dict(name=PREFIX+'-import-1password',type='1password',service_account=CANARY,tags=['initial'])
            def state(attrs, ident):
                return {'resources':[dict(mode='managed',type='adaptive_resource',name='test',instances=[dict(attributes=dict(attrs,id=ident))])]}
            for fixture, ident in [('import-1password', ID),('integration-1password','22222222-2222-2222-2222-222222222222')]:
                attrs = dict(config, name=PREFIX+'-'+fixture)
                (run / 'work' / fixture / 'terraform.tfstate').write_text(json.dumps(state(attrs,ident)))
                (run / 'work' / fixture / 'main.tf.json').write_text(json.dumps({'resource':{'adaptive_resource':{'test':attrs}}}))
            remote = dict(configuration=dict(name=config['name'],type='1password',service_account=CANARY,connect_server_url='https://offline.invalid'),type='resource',userTags=['initial'])
            for name in ['remote.json','baseline.json']:
                (run / name).write_text(json.dumps(remote))
            (run / 'cli-origin/import-1password.json').write_text(json.dumps(remote))
            (run / 'cli-origin/import-1password.id').write_text(ID+'\n')
            (run / 'cli-origin/ownership.jsonl').write_text(json.dumps(dict(name=config['name'],document=str(run/'cli-origin/import-1password.json'),fixture='import-1password',origin='adaptive-cli'))+'\n')
            # Old evidence must remain byte-for-byte, including historical failures.
            (run / 'evidence/historic-failure.json').write_text('{"status":"FAIL"}\n')
            (run / 'ledger.jsonl').write_text('{"phase":"historic","status":"FAIL"}\n')
            for path in [run, *run.rglob('*')]:
                path.chmod(0o700 if path.is_dir() else 0o600)
            if corrupt:
                corrupt(run)
            before = {str(p.relative_to(run)): p.read_bytes() for p in run.rglob('*') if p.is_file()}
            tool = root / 'tool.py'; tool.write_text(TOOL)
            bindir = root / 'bin'; bindir.mkdir()
            adaptive = bindir/'adaptive'; adaptive.write_text('#!/bin/bash\nexec python3 "$OFFLINE_TOOL" "$@"\n'); adaptive.chmod(0o700)
            env = dict(os.environ, PATH=str(bindir)+':'+os.environ['PATH'], OFFLINE_TOOL=str(tool),OFFLINE_RUN=str(run),SCENARIO=scenario,CANARY=CANARY)
            env.update(extra_env or {})
            result = subprocess.run(['bash',str(live/'unclassified-secrets.sh'),str(run)],env=env,text=True,capture_output=True)
            trace = [json.loads(x) for x in (run/'trace').read_text().splitlines()] if (run/'trace').exists() else []
            ledger = [json.loads(x) for x in (run/'ledger.jsonl').read_text().splitlines()]
            evidence = '\n'.join(p.read_text() for p in (run/'evidence').rglob('*.json') if p.name not in {'original-config.json', 'post-error-state.json'})
            self.assertEqual((run/'evidence/historic-failure.json').read_text(),'{"status":"FAIL"}\n')
            self.assertEqual(ledger[0],dict(phase='historic',status='FAIL'))
            self.assertNotIn(CANARY,result.stdout+result.stderr+evidence)
            if scenario == 'unsafe':
                self.assertEqual(result.returncode, 1)
                self.assertEqual(trace, [], 'unsafe paths must cause zero process calls')
                self.assertEqual(before, {str(p.relative_to(run)): p.read_bytes() for p in run.rglob('*') if p.is_file()},
                                 'unsafe constructor must not create evidence or change ledger/fixtures')
                self.assertEqual(result.stderr.strip(), 'Owned synthetic omission probe blocked: unsafe private paths')
                return result, trace, ledger
            original = json.loads((run/'work/import-1password/main.tf.json').read_text())
            self.assertEqual(original['resource']['adaptive_resource']['test'],config)
            return result, trace, ledger

    def test_redacted_backend_exact_synthetic_retention_and_restoration(self):
        result, trace, ledger = self.execute()
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(sum('--include-secrets' in a for a in trace),3)
        self.assertTrue(any(x['phase']=='unclassified-restored-noop' for x in ledger))
        self.assertTrue(any(x['phase']=='unclassified-secret-retained' and x['status']=='PASS' for x in ledger))

    def test_post_probe_negatives_fail_with_ledger_and_always_restore_and_noop(self):
        scenarios = ['malformed', 'missing', 'wrongname', 'wrongtype', 'remoteempty',
                     'wrongcredential', 'tagmutation', 'configmutation', 'wrongdiagnostic',
                     'restorewrong', 'noopfail']
        for scenario in scenarios:
            with self.subTest(scenario=scenario):
                result, trace, ledger = self.execute(scenario)
                self.assertEqual(result.returncode,1,result.stderr)
                phases = [a[3] for a in trace if a[0]=='tf']
                self.assertIn('unclassified-restore-plan',phases)
                self.assertIn('unclassified-restore-apply',phases)
                self.assertIn('unclassified-restored-noop',phases)
                self.assertLess(phases.index('unclassified-omission-rejected'),phases.index('unclassified-restore-apply'))
                self.assertTrue(any(x['status']=='FAIL' and x['phase']!='historic' for x in ledger))
                self.assertTrue(any(x['phase']=='unclassified-secret-restored' for x in ledger))

    def test_default_plaintext_is_intentional_security_failure_without_probe(self):
        result, trace, ledger = self.execute('defaultvisible')
        self.assertEqual(result.returncode,1)
        self.assertFalse(any(a[0]=='tf' for a in trace))
        self.assertTrue(any(x['phase']=='unclassified-baseline-default-redaction' and x['status']=='FAIL' for x in ledger))

    def test_invalid_local_ownership_fails_before_any_privileged_export(self):
        changes = [
            ('work/import-1password/terraform.tfstate', lambda d: d['resources'][0]['instances'][0]['attributes'].update(id='different')),
            ('work/import-1password/terraform.tfstate', lambda d: d['resources'][0]['instances'][0]['attributes'].update(name='not-owned')),
            ('work/import-1password/terraform.tfstate', lambda d: d['resources'][0]['instances'][0]['attributes'].update(type='postgres')),
            ('work/integration-1password/main.tf.json', lambda d: d['resource']['adaptive_resource']['test'].update(service_account='not-approved')),
            ('inventory/integration-cases.json', lambda d: d['cases'][0]['attributes'].update(service_account='not-approved')),
            ('cli-origin/import-1password.json', lambda d: d['configuration'].update(service_account='not-approved')),
        ]
        for filename, alter in changes:
            def corrupt(run):
                path=run/filename
                doc=json.loads(path.read_text()); alter(doc); path.write_text(json.dumps(doc))
            with self.subTest(filename=filename):
                result, trace, ledger = self.execute(corrupt=corrupt)
                self.assertEqual(result.returncode,1)
                self.assertFalse(any('--include-secrets' in a or a[0]=='tf' for a in trace))
                self.assertTrue(any(x['phase']=='unclassified-ownership-invalid' and x['status']=='FAIL' for x in ledger))

    def test_symlinked_source_is_rejected_before_privileged_read(self):
        def corrupt(run):
            path=run/'work/integration-1password/terraform.tfstate'
            target=run/'source-state-copy.json'
            path.rename(target)
            path.symlink_to(target)
        result, trace, ledger = self.execute(corrupt=corrupt)
        self.assertEqual(result.returncode,1)
        self.assertFalse(any('--include-secrets' in a or a[0]=='tf' for a in trace))

    def test_unsafe_paths_have_zero_writes_and_zero_process_calls(self):
        for relative in ['evidence', 'ledger.jsonl', 'work', 'inventory', 'cli-origin',
                         'work/import-1password/main.tf.json', 'work/import-1password/terraform.tfstate']:
            def corrupt(run):
                path = run / relative
                target = run.parent / ('credential-canary-' + path.name)
                path.rename(target)
                path.symlink_to(target, target_is_directory=target.is_dir())
            with self.subTest(relative=relative):
                self.execute('unsafe', corrupt=corrupt)
        for relative, mode in [('evidence', 0o755), ('ledger.jsonl', 0o644),
                               ('work/import-1password', 0o770)]:
            with self.subTest(relative=relative, mode=mode):
                self.execute('unsafe', corrupt=lambda run: (run / relative).chmod(mode))

    def test_source_inventory_link_is_rejected_before_evidence(self):
        def corrupt(run):
            path = run.parent / 'testing/live/integration-cases.json'
            target = run.parent / 'credential-canary-inventory.json'
            path.rename(target); path.symlink_to(target)
        self.execute('unsafe', corrupt=corrupt)

    def test_unsafe_run_ancestor_and_nonregular_record_have_zero_mutations(self):
        self.execute('unsafe', corrupt=lambda run: run.parent.chmod(0o777))
        self.execute('unsafe', corrupt=lambda run: os.chmod(run, 0o755))
        def linked_ancestor(run):
            source = run.parent / 'run-real'
            run.rename(source)
            run.symlink_to(source, target_is_directory=True)
        self.execute('unsafe', corrupt=linked_ancestor)
        def nonregular_input(run):
            path = run / 'work/import-1password/terraform.tfstate'
            path.rename(run / 'original-state.json')
            path.mkdir(mode=0o700)
        self.execute('unsafe', corrupt=nonregular_input)

    def test_python_optimization_cannot_disable_ownership_checks(self):
        def corrupt(run):
            path=run/'cli-origin/import-1password.id'
            path.write_text('33333333-3333-3333-3333-333333333333\n')
        result, trace, ledger = self.execute(corrupt=corrupt,extra_env={'PYTHONOPTIMIZE':'1'})
        self.assertEqual(result.returncode,1)
        self.assertFalse(any('--include-secrets' in a or a[0]=='tf' for a in trace))

    def test_missing_source_api_identity_blocks_privileged_read(self):
        def corrupt(run):
            path = run/'work/integration-1password/terraform.tfstate'
            doc = json.loads(path.read_text())
            del doc['resources'][0]['instances'][0]['attributes']['id']
            path.write_text(json.dumps(doc))
        result, trace, ledger = self.execute(corrupt=corrupt)
        self.assertEqual(result.returncode,1)
        self.assertFalse(any('--include-secrets' in a or a[0]=='tf' for a in trace))
        self.assertTrue(any(x['status']=='FAIL' for x in ledger[1:]))


if __name__ == '__main__':
    unittest.main()
