#!/usr/bin/env python3
"""Offline tests only: never invoke the live CLI or Terraform."""
import json
import os
from pathlib import Path
import subprocess
import unittest

SCRIPT = Path(__file__).with_name('runtime-policies.sh')
PREFIX = 'tflive-20261105010203-12345'


class RuntimePolicies(unittest.TestCase):
    def run_script(self, *args):
        return subprocess.run(['bash', str(SCRIPT), *map(str, args)], text=True, capture_output=True)

    def template(self, mask=''):
        result = self.run_script('template', PREFIX, 'k8s-local', 'existing@example.test', mask)
        self.assertEqual(result.returncode, 0, result.stderr)
        return json.loads(result.stdout)

    def test_native_select_policy_and_client_binding(self):
        doc = self.template()
        auth = doc['authorization-postgres']
        self.assertEqual(auth['name'], PREFIX + '-authorization-postgres')
        self.assertEqual(auth['resource_type'], 'postgres')
        # JSON is a YAML subset; the real consumer yaml.Unmarshal accepts it.
        policy = json.loads(auth['permissions'])
        self.assertEqual(policy, {'allow': [{'database': 'tf_live_test', 'schema': ['public'], 'objects': ['customers'], 'privileges': ['SELECT']}]})
        endpoint = doc['endpoint-readonly']
        self.assertEqual(endpoint['authorization'], auth['name'])
        self.assertEqual(endpoint['resource'], PREFIX + '-runtime-postgres')
        self.assertEqual(endpoint['type'], 'client')
        self.assertEqual(endpoint['users'], ['existing@example.test'])
        self.assertEqual(endpoint['cluster'], 'k8s-local')
        self.assertFalse(endpoint['is_jit_enabled'])
        self.assertFalse(endpoint['auto_approval'])
        self.assertEqual(endpoint['jit_approvers'], [])
        self.assertNotIn('endpoint-masked', doc)

    def test_mask_uses_exact_generated_authorization(self):
        doc = self.template('masked_' + PREFIX + '-runtime-postgres')
        self.assertEqual(doc['endpoint-masked']['authorization'], 'masked_' + PREFIX + '-runtime-postgres')
        self.assertEqual(doc['endpoint-masked']['resource'], PREFIX + '-runtime-postgres')
        for mask in ('masked_other', PREFIX + '-cohort-auth', 'masked_' + PREFIX + '-postgres'):
            self.assertNotEqual(self.run_script('template', PREFIX, 'k8s-local', 'existing@example.test', mask).returncode, 0)

    def test_bad_prefix_and_legacy_run_rejected_before_external_commands(self):
        for prefix in ('other', PREFIX + '-suffix', 'tflive-../../escape'):
            self.assertNotEqual(self.run_script('template', prefix, 'k8s-local', 'existing@example.test', '').returncode, 0)
        old = Path.home() / 'IdeaProjects/.terraform-provider-adaptive-live/20261104T042306Z-96512'
        result = self.run_script('check-run', old)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('legacy', result.stderr.lower())

    def test_source_pin_required_and_exact_run_binding(self):
        import tempfile
        scratch = Path(os.environ.get('TMPDIR', str(Path.home() / '.hermes/cache/scratch')))
        with tempfile.TemporaryDirectory(dir=scratch) as tmp:
            run = Path(tmp).resolve()
            manifest = dict(prefix=PREFIX, url='http://localhost:8080', run=str(run))
            (run / 'manifest.json').write_text(json.dumps(manifest))
            result = self.run_script('check-run', run)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('source-pinned', result.stderr)
            manifest['provider_source_sha256'] = 'a' * 64
            (run / 'manifest.json').write_text(json.dumps(manifest))
            result = self.run_script('check-run', run)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('private run base', result.stderr)

    def test_state_refuses_tainted_partial_or_wrong_bindings(self):
        import tempfile
        expected = self.template()['endpoint-readonly']
        with tempfile.TemporaryDirectory(dir=os.environ['TMPDIR']) as tmp:
            p = Path(tmp) / 'state.json'
            attrs = dict(expected, id='owned-endpoint-id')
            state: dict = dict(resources=[dict(mode='managed', type='adaptive_endpoint', name='test', instances=[dict(attributes=attrs)])])
            p.write_text(json.dumps(state))
            result = self.run_script('inspect-state', p, 'adaptive_endpoint', json.dumps(expected))
            self.assertEqual(result.returncode, 0, result.stderr)
            for field, value in [('authorization', 'unowned'), ('resource', PREFIX + '-other'), ('users', ['other@example.test'])]:
                attrs[field] = value
                p.write_text(json.dumps(state))
                self.assertNotEqual(self.run_script('inspect-state', p, 'adaptive_endpoint', json.dumps(expected)).returncode, 0)
                attrs[field] = expected[field]
            state['resources'][0]['instances'][0]['status'] = 'tainted'
            p.write_text(json.dumps(state))
            result = self.run_script('inspect-state', p, 'adaptive_endpoint', json.dumps(expected))
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('tainted', result.stderr)
            del state['resources'][0]['instances'][0]['status']
            attrs['id'] = ''
            p.write_text(json.dumps(state))
            self.assertNotEqual(self.run_script('inspect-state', p, 'adaptive_endpoint', json.dumps(expected)).returncode, 0)

    def test_independent_exports_verify_native_shapes_not_just_names(self):
        import tempfile
        expected = self.template()
        with tempfile.TemporaryDirectory(dir=os.environ['TMPDIR']) as tmp:
            p = Path(tmp) / 'export.yaml'
            auth = expected['authorization-postgres']
            document: dict = dict(type='authorization', configuration=dict(name=auth['name'], integration_type='postgres', auth=auth['permissions']))
            p.write_text(json.dumps(document))
            result = self.run_script('compare-export', 'authorizations', json.dumps(auth), p)
            self.assertEqual(result.returncode, 0, result.stderr)
            document['configuration']['auth'] = '{"allow":[]}'
            p.write_text(json.dumps(document))
            self.assertNotEqual(self.run_script('compare-export', 'authorizations', json.dumps(auth), p).returncode, 0)
            endpoint = expected['endpoint-readonly']
            document: dict = dict(type='endpoint', configuration=dict(name=endpoint['name'], access_type='client', integration_name=endpoint['resource'], authorization=endpoint['authorization'], cluster='k8s-local', jit=dict(is_jit_enabled=False)))
            p.write_text(json.dumps(document))
            self.assertEqual(self.run_script('compare-export', 'endpoints', json.dumps(endpoint), p).returncode, 0)
            document['configuration']['authorization'] = 'privileged-wrong-binding'
            p.write_text(json.dumps(document))
            self.assertNotEqual(self.run_script('compare-export', 'endpoints', json.dumps(endpoint), p).returncode, 0)

    def masking_export(self) -> tuple[dict, dict]:
        resource = PREFIX + '-runtime-postgres'
        policy = dict(scoped=True, masks=[dict(database_name='tf_live_test', configuration=[
            dict(schema='public', table_name='customers', masked_columns=[
                dict(column_name='synthetic_mask_' + str(i), masking_type='email')
                for i in range(11)])])])
        attrs = dict(resource=resource, scoped=True)
        document = dict(type='dataprotection', configuration=dict(
            resource_name=resource, configuration=json.dumps(policy)))
        return attrs, document

    def compare_text(self, text, kind='dataprotection', attrs=None):
        import tempfile
        if attrs is None:
            attrs, _ = self.masking_export()
        with tempfile.TemporaryDirectory(dir=os.environ['TMPDIR']) as tmp:
            path = Path(tmp) / 'synthetic-export.yaml'
            path.write_text(text)
            result = self.run_script('compare-export', kind, json.dumps(attrs), path)
            self.assertEqual(path.read_text(), text, 'Readback evidence must stay immutable')
            return result

    def orphan_warning(self, resource_id='245cd856-2b7c-47ba-b570-0640a4f0ec6e'):
        # export_documents.go:174 + export_docs.go:730-734; emitted before YAML.
        return ('warning: skipped "' + resource_id + '": the resource this policy belongs '
                'to no longer exists; the policy is orphaned and cannot be exported or applied\n')

    def test_dataprotection_accepts_only_known_leading_orphan_diagnostic(self):
        attrs, document = self.masking_export()
        # Exercise native YAML framing, with the nested policy retained as text.
        text = ('type: dataprotection\nconfiguration:\n  resource_name: ' + attrs['resource']
                + '\n  configuration: ' + json.dumps(document['configuration']['configuration']) + '\n')
        for prefix in ('', self.orphan_warning(), self.orphan_warning() + self.orphan_warning(
                '00000000-0000-0000-0000-000000000001')):
            with self.subTest(prefix=prefix):
                result = self.compare_text(prefix + text)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertIn('Exact independent configuration readback verified', result.stdout)
                self.assertEqual(result.stderr, prefix, 'Known diagnostics must remain visible')

    def test_resource_redaction_note_is_exact_trailing_diagnostic_only(self):
        attrs = dict(name=PREFIX + '-runtime-postgres')
        document = dict(type='resource', configuration=dict(
            name=attrs['name'], type='postgres', databaseName='tf_live_test'))
        text = json.dumps(document) + '\n'
        note = ('\nnote: secret values are shown as <REDACTED>. Fill them in before applying, '
                'or re-run with --include-secrets.\n')
        result = self.compare_text(text + note, kind='resources', attrs=attrs)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stderr, note)
        for invalid in (note + text, text + note + note,
                        text + note.replace('secret values', 'other values'),
                        text + note + 'error: denied\n'):
            self.assertNotEqual(self.compare_text(invalid, kind='resources', attrs=attrs).returncode, 0)
        mask_attrs, mask_document = self.masking_export()
        self.assertNotEqual(self.compare_text(json.dumps(mask_document) + '\n' + note,
                                             attrs=mask_attrs).returncode, 0)

    def test_export_rejects_other_diagnostics_even_when_valid_yaml(self):
        attrs, document = self.masking_export()
        text = ('type: dataprotection\nconfiguration:\n  resource_name: ' + str(attrs['resource'])
                + '\n  configuration: ' + json.dumps(document['configuration']['configuration']) + '\n')
        for noise in ('warning: unrelated warning\n', 'error: permission denied\n', 'noise: unexpected\n'):
            with self.subTest(noise=noise):
                self.assertNotEqual(self.compare_text(noise + text).returncode, 0)

    def test_orphan_exception_is_prefix_only_and_keeps_all_semantic_gates(self):
        import copy
        attrs, document = self.masking_export()
        text = json.dumps(document) + '\n'
        warning = self.orphan_warning()
        invalid = [
            '', warning, warning + 'configuration: [\n', warning + 'null\n',
            warning + text + '---\n' + text, warning + text + '---\n',
            warning + '--- !ruby/hash\n' + text,
            warning + 'type: dataprotection\nconfiguration: &c {}\ncopy: *c\n',
            text + warning, '---\n' + warning + text,
            ' ' + warning + text,
            warning.replace('warning:', 'error:') + text,
            warning.replace('orphaned', 'unreadable') + text,
            warning.replace('applied\n', 'applied .\n') + text,
            self.orphan_warning('not-a-uuid') + text,
            self.orphan_warning(attrs['resource']) + text,
            warning.rstrip('\n') + text,
            warning + 'warning: unrelated\n' + text,
        ]
        mutations = [
            ('kind', lambda d: d.update(type='resource')),
            ('owned identity', lambda d: d['configuration'].update(resource_name=PREFIX + '-other')),
        ]
        for label, mutate in mutations:
            bad = copy.deepcopy(document)
            mutate(bad)
            invalid.append(warning + json.dumps(bad))
        for field, value in (('scoped', False), ('masks', [])):
            bad = copy.deepcopy(document)
            policy = json.loads(bad['configuration']['configuration'])
            policy[field] = value
            bad['configuration']['configuration'] = json.dumps(policy)
            invalid.append(warning + json.dumps(bad))
        for target, value in (('database_name', 'postgres'), ('schema', 'private'),
                              ('table_name', 'other'), ('masked_columns', [])):
            bad = copy.deepcopy(document)
            policy = json.loads(bad['configuration']['configuration'])
            mask = policy['masks'][0]
            (mask if target == 'database_name' else mask['configuration'][0])[target] = value
            bad['configuration']['configuration'] = json.dumps(policy)
            invalid.append(warning + json.dumps(bad))
        for index, bad in enumerate(invalid):
            with self.subTest(index=index):
                self.assertNotEqual(self.compare_text(bad).returncode, 0)
        expected = self.template()
        auth = expected['authorization-postgres']
        endpoint = expected['endpoint-readonly']
        other_exports = [
            ('resources', dict(name=attrs['resource']), dict(type='resource', configuration=dict(
                name=attrs['resource'], type='postgres', databaseName='tf_live_test'))),
            ('authorizations', auth, dict(type='authorization', configuration=dict(
                name=auth['name'], integration_type='postgres', auth=auth['permissions']))),
            ('endpoints', endpoint, dict(type='endpoint', configuration=dict(
                name=endpoint['name'], access_type='client', integration_name=endpoint['resource'],
                authorization=endpoint['authorization'], cluster=endpoint['cluster'],
                jit=dict(is_jit_enabled=False)))),
        ]
        for kind, binding, exported in other_exports:
            with self.subTest(kind=kind):
                clean = json.dumps(exported) + '\n'
                self.assertEqual(self.compare_text(clean, kind=kind, attrs=binding).returncode, 0)
                self.assertNotEqual(self.compare_text(warning + clean, kind=kind, attrs=binding).returncode, 0)

    def test_retained_fixture_never_replans_or_reapplies(self):
        import tempfile
        with tempfile.TemporaryDirectory(dir=os.environ['TMPDIR']) as tmp:
            run = Path(tmp)
            directory = run / 'work/endpoint-readonly'
            directory.mkdir(parents=True)
            attrs = self.template()['endpoint-readonly']
            (directory / 'main.tf.json').write_text(json.dumps(dict(resource=dict(adaptive_endpoint=dict(test=attrs)))))
            (directory / 'terraform.tfstate').write_text(json.dumps(dict(resources=[dict(mode='managed', type='adaptive_endpoint', name='test', instances=[dict(attributes=dict(attrs, id='retained-id'))])])) )
            command = '''source "$1"
RUN=$2; MODE=setup; OUT="$RUN"; PREFIX=$3
record() { printf '%s\\n' "$*"; }
readback() { return 0; }
ready() { return 0; }
R=/must-not-call-runner
ensure_case endpoint-readonly adaptive_endpoint "$4" endpoints
'''
            result = subprocess.run(['bash', '-c', command, 'offline-test', str(SCRIPT), str(run), PREFIX, json.dumps(attrs)], text=True, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn('retained', result.stdout)

    def test_failed_create_retains_id_and_never_retries(self):
        import tempfile
        with tempfile.TemporaryDirectory(dir=os.environ['TMPDIR']) as tmp:
            run = Path(tmp)
            (run / 'evidence').mkdir()
            attrs = self.template()['endpoint-readonly']
            runner = run / 'fake-runner.sh'
            runner.write_text('''#!/bin/bash
exec python3 - "$@" <<'PY'
import json,pathlib,sys
args=sys.argv[1:]; run=pathlib.Path(args[1]); case=args[2]; d=run/'work'/case
with (run/'events').open('a') as f: f.write(' '.join(args[:1]+args[3:4])+'\\n')
if args[0]=='fixture':
 d.mkdir(parents=True); (d/'main.tf.json').write_text(json.dumps({'resource':{args[3]:{'test':json.loads(args[4])}}}))
elif args[3]=='runtime-inspect-plan':
 p=run/'evidence/plan.json'; p.write_text(json.dumps({'resource_changes':[{'type':'adaptive_endpoint','address':'adaptive_endpoint.test','change':{'actions':['create']}}]}))
 with (run/'ledger.jsonl').open('a') as f: f.write(json.dumps(dict(case=case,phase=args[3],evidence=str(p)))+'\\n')
elif args[3]=='create-apply':
 attrs=json.loads((d/'main.tf.json').read_text())['resource']['adaptive_endpoint']['test']; attrs['id']='failed-but-retained-id'
 (d/'terraform.tfstate').write_text(json.dumps({'resources':[{'mode':'managed','name':'test','type':'adaptive_endpoint','instances':[{'status':'tainted','attributes':attrs}]}]}))
 sys.exit(1)
PY
''')
            command = '''source "$1"
RUN=$2; OUT="$RUN/evidence"; MODE=setup; R="$RUN/fake-runner.sh"
record() { printf '%s\\n' "$*"; }
adaptive() { return 0; }
readback() { printf 'readback\\n' >> "$RUN/events"; }
ready() { printf 'ready\\n' >> "$RUN/events"; }
set +e
ensure_case endpoint-readonly adaptive_endpoint "$3" endpoints
first=$?
ensure_case endpoint-readonly adaptive_endpoint "$3" endpoints
second=$?
test "$first" = 1 && test "$second" = 1
'''
            result = subprocess.run(['bash', '-c', command, 'offline-test', str(SCRIPT), str(run), json.dumps(attrs)], text=True, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            events = (run / 'events').read_text().splitlines()
            self.assertEqual(events.count('tf create-apply'), 1)
            self.assertEqual(events.count('readback'), 2)
            self.assertEqual(events.count('ready'), 2)
            state = json.loads((run / 'work/endpoint-readonly/terraform.tfstate').read_text())
            self.assertEqual(state['resources'][0]['instances'][0]['attributes']['id'], 'failed-but-retained-id')
            self.assertIn('retained-state BLOCKED', result.stdout)

    def test_native_backend_yaml_consumer_accepts_list_rejects_scalar(self):
        import re
        import tempfile
        checkout = os.environ.get('INVENTORIZE_CHECKOUT')
        if not checkout:
            self.skipTest('Set INVENTORIZE_CHECKOUT to the Inventorize checkout root to test the backend YAML contract')
        backend = Path(checkout) / 'inventorize-app'
        postgres = backend / 'internal/service/integrations/postgres'
        try:
            source = (postgres / 'authorization.go').read_text(encoding='utf-8')
            consumer = (postgres / 'authorization_allow_deny.go').read_text(encoding='utf-8')
        except OSError as exc:
            self.fail(f'Cannot read Inventorize PostgreSQL source: {exc}. '
                      'Set INVENTORIZE_CHECKOUT to the Inventorize checkout root (the parent of inventorize-app).')
        type_blocks = []
        for name in ('FilterVal', 'Opt'):
            match = re.search(r'type ' + name + r' struct\s*\{[^}]+\}', source)
            self.assertIsNotNone(match, 'Missing native declaration: ' + name)
            assert match is not None
            type_blocks.append(match.group())
        declarations = '\n'.join(type_blocks)
        self.assertIn('yaml.Unmarshal([]byte(yamlFilterConfig), &filterConfig)', consumer)
        policy = self.template()['authorization-postgres']['permissions']
        with tempfile.TemporaryDirectory(dir=os.environ['TMPDIR']) as tmp:
            main = Path(tmp) / 'main.go'
            main.write_text('package main\nimport ("fmt"; "os"; "gopkg.in/yaml.v3")\n' + declarations + '''
func main() {
 var f FilterVal
 if err := yaml.Unmarshal([]byte(os.Args[1]), &f); err != nil { panic(err) }
 if len(f.Allow)!=1 || len(f.Allow[0].Schema)!=1 || f.Allow[0].Schema[0]!="public" { panic("wrong scope") }
 var bad FilterVal
 if yaml.Unmarshal([]byte("allow:\\n- database: tf_live_test\\n  schema: public\\n  objects: [customers]\\n  privileges: [SELECT]\\n"), &bad)==nil { panic("scalar accepted") }
 fmt.Println("NATIVE_SCHEMA_LIST_PASS SCALAR_REJECTED")
}
''')
            result = subprocess.run(['go', 'run', '-mod=vendor', str(main), policy], cwd=backend, text=True, capture_output=True, timeout=120)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout.strip(), 'NATIVE_SCHEMA_LIST_PASS SCALAR_REJECTED')

    def test_source_pinned_manifest_checks_binary_inventory_and_run(self):
        import hashlib
        import runpy
        import tempfile
        capture = runpy.run_path(str(SCRIPT.with_name('source_capture.py')))['capture']
        snapshot = capture(SCRIPT.parents[2])
        with tempfile.TemporaryDirectory(dir=os.environ['TMPDIR']) as tmp:
            home = Path(tmp).resolve()
            run = home / 'IdeaProjects/.terraform-provider-adaptive-live/unit-only'
            (run / 'inventory').mkdir(parents=True)
            (run / 'bin').mkdir()
            binary = b'offline unit-test binary stand-in; never executable'
            (run / 'bin/terraform-provider-adaptive').write_bytes(binary)
            (run / 'inventory/provider-source.json').write_text(json.dumps(snapshot))
            manifest = dict(prefix=PREFIX, url='http://localhost:8080', run=str(run), provider_source_sha256=snapshot['sha256'], binary_sha256=hashlib.sha256(binary).hexdigest())
            env = dict(os.environ, HOME=str(home))
            def check():
                (run / 'manifest.json').write_text(json.dumps(manifest))
                return subprocess.run(['bash', str(SCRIPT), 'check-run', str(run)], env=env, text=True, capture_output=True)
            result = check()
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout.strip(), PREFIX)
            manifest['binary_sha256'] = '0' * 64
            self.assertNotEqual(check().returncode, 0)
            manifest['binary_sha256'] = hashlib.sha256(binary).hexdigest()
            manifest['provider_source_sha256'] = '0' * 64
            self.assertNotEqual(check().returncode, 0)
            manifest['provider_source_sha256'] = snapshot['sha256']
            manifest['run'] = str(run) + '-other'
            self.assertNotEqual(check().returncode, 0)

    def test_runtime_resource_and_mask_policy_must_match_exact_database(self):
        import tempfile
        with tempfile.TemporaryDirectory(dir=os.environ['TMPDIR']) as tmp:
            p = Path(tmp) / 'export.yaml'
            resource = dict(name=PREFIX + '-runtime-postgres', type='postgres', database_name='tf_live_test')
            document: dict = dict(type='resource', configuration=dict(name=resource['name'], type='postgres', databaseName='tf_live_test'))
            p.write_text(json.dumps(document))
            self.assertEqual(self.run_script('compare-export', 'resources', json.dumps(resource), p).returncode, 0)
            document['configuration']['databaseName'] = 'postgres'
            p.write_text(json.dumps(document))
            self.assertNotEqual(self.run_script('compare-export', 'resources', json.dumps(resource), p).returncode, 0)
            policy = dict(scoped=True, masks=[dict(database_name='tf_live_test', configuration=[dict(schema='public', table_name='customers', masked_columns=[dict(column_name='mask_email', masking_type='email')])])])
            dp = dict(resource=resource['name'], scoped=True)
            document: dict = dict(type='dataprotection', configuration=dict(resource_name=resource['name'], configuration=json.dumps(policy)))
            p.write_text(json.dumps(document))
            self.assertEqual(self.run_script('compare-export', 'dataprotection', json.dumps(dp), p).returncode, 0)
            for field, value in [('resource_name', PREFIX + '-other'), ('configuration', json.dumps(dict(scoped=False, masks=policy['masks'])))]:
                document['configuration'][field] = value
                p.write_text(json.dumps(document))
                self.assertNotEqual(self.run_script('compare-export', 'dataprotection', json.dumps(dp), p).returncode, 0)
                document['configuration'][field] = resource['name'] if field == 'resource_name' else json.dumps(policy)


if __name__ == '__main__':
    unittest.main(verbosity=2)
