import unittest
import tempfile
from pathlib import Path
from secret_audit import visible_fields
import secret_audit

class SecretAuditTests(unittest.TestCase):
    def test_detects_only_sensitive_canaries_without_returning_values(self):
        got = visible_fields({'password': 'synthetic-password', 'host': 'db.invalid'}, {'password'}, 'password: synthetic-password\nhost: db.invalid')
        self.assertEqual(got, ['password'])
        self.assertNotIn('synthetic-password', repr(got))
    def test_redacted_export_is_not_visible(self):
        self.assertEqual(visible_fields({'password':'synthetic-password'}, {'password'}, 'password: <REDACTED>'), [])
    def test_nested_json_private_key_canary_detected(self):
        self.assertEqual(visible_fields({'key_file':'{"private_key":"synthetic-key-canary"}'}, {'key_file'}, 'private_key: synthetic-key-canary'), ['key_file'])

    def test_original_and_auth_cohorts_are_audited_without_unrelated_configs(self):
        with tempfile.TemporaryDirectory() as directory:
            run = Path(directory)
            for case in ('integration-postgres', 'auth-google-delegated', 'unrelated'):
                path = run / 'work' / case / 'main.tf.json'
                path.parent.mkdir(parents=True)
                path.touch()
            self.assertEqual([p.parent.name for p in secret_audit.fixture_configs(run)],
                             ['auth-google-delegated', 'integration-postgres'])

if __name__ == '__main__':
    unittest.main()
