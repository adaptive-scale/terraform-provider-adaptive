import tempfile
import unittest
from pathlib import Path
from guard import validate_configuration, validate_plan, saved_plan_argument

class GuardTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name)
        self.config = {'terraform':{'required_providers':{'adaptive':{'source':'adaptive-scale/adaptive'}}}, 'provider':{'adaptive':{'workspace_url':'http://localhost:8080'}}, 'resource':{'adaptive_group':{'test':{'name':'tflive-test-group'}}}}
    def test_rejects_nonlocal_provider_and_extra_configuration(self):
        self.config['provider']['adaptive']['workspace_url'] = 'https://other.invalid'
        with self.assertRaises(ValueError): validate_configuration(self.directory, self.config, 'tflive-test')
        self.config['provider']['adaptive']['workspace_url'] = 'http://localhost:8080'
        (self.directory/'extra.tf').write_text('')
        with self.assertRaises(ValueError): validate_configuration(self.directory, self.config, 'tflive-test')
    def test_accepts_owned_configuration(self):
        validate_configuration(self.directory, self.config, 'tflive-test')
    def test_delete_rejected_except_explicit_destroy(self):
        plan = self.plan(['delete'])
        with self.assertRaises(ValueError): validate_plan(plan, 'tflive-test', False)
        validate_plan(plan, 'tflive-test', True)
    def test_foreign_name_rejected_even_for_destroy(self):
        plan = self.plan(['delete'])
        plan['resource_changes'][0]['change']['before']['name']='unowned'
        with self.assertRaises(ValueError): validate_plan(plan, 'tflive-test', True)
    def test_provider_in_saved_plan_must_be_literal_local(self):
        plan = self.plan(['update'])
        plan['configuration']['provider_config']['adaptive']['expressions']['workspace_url']={'references':['var.url']}
        with self.assertRaises(ValueError): validate_plan(plan, 'tflive-test', False)
    def test_refresh_only_cannot_be_overridden_or_combined_with_saved_plan(self):
        for arguments in [
            ['apply', '-refresh-only', '-refresh-only=false', '-auto-approve'],
            ['apply', '-refresh-only=false', '-auto-approve'],
            ['apply', '-refresh-only', 'destroy.tfplan'],
            ['apply', '-config=elsewhere', 'saved.tfplan'],
        ]:
            with self.assertRaises(ValueError): saved_plan_argument(arguments)
        self.assertIsNone(saved_plan_argument(['apply', '-refresh-only', '-auto-approve']))
        self.assertEqual(saved_plan_argument(['apply', '-input=false', '-auto-approve', 'saved.tfplan']), 'saved.tfplan')
    @staticmethod
    def plan(actions):
        return {'configuration':{'provider_config':{'adaptive':{'full_name':'registry.terraform.io/adaptive-scale/adaptive','expressions':{'workspace_url':{'constant_value':'http://localhost:8080'}}}}},'resource_changes':[{'type':'adaptive_group','change':{'actions':actions,'before':{'name':'tflive-test-group'},'after':None if actions==['delete'] else {'name':'tflive-test-group'}}}]}

if __name__ == '__main__': unittest.main()
