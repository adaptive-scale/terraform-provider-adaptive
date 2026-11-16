import importlib.util
import pathlib
import tempfile
import unittest

PATH = pathlib.Path(__file__).with_name('source_capture.py')

class SourceCaptureTests(unittest.TestCase):
    def module(self):
        self.assertTrue(PATH.exists(), 'source provenance helper is missing')
        spec = importlib.util.spec_from_file_location('source_capture', PATH)
        assert spec is not None and spec.loader is not None
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        return module

    def test_uncommitted_source_changes_change_fingerprint(self):
        module = self.module()
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            (root / 'go.mod').write_text('module unit-only\n')
            (root / 'main.go').write_text('package main\n')
            before = module.capture(root)
            (root / 'main.go').write_text('package main\n// changed\n')
            after = module.capture(root)
            self.assertNotEqual(before['sha256'], after['sha256'])
            self.assertEqual(set(after['files']), {'go.mod', 'main.go'})
            self.assertNotIn('package main', str(after))

    def test_does_not_capture_credential_files_or_generated_binary(self):
        module = self.module()
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            (root / 'go.mod').write_text('module unit-only\n')
            (root / '.env').write_text('unit-only-not-a-real-token')
            (root / 'terraform-provider-adaptive').write_text('unit-binary')
            (root / 'internal').mkdir()
            (root / 'internal' / 'fixture.go').write_text('package fixture\n')
            result = module.capture(root)
            self.assertEqual(set(result['files']), {'go.mod', 'internal/fixture.go'})
            self.assertNotIn('unit-only-not-a-real-token', str(result))

    def test_symlinked_source_input_is_rejected_before_read(self):
        module = self.module()
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            (root / 'go.mod').write_text('module unit-only\n')
            (root / '.env').write_text('unit-only-not-a-real-token')
            (root / 'main.go').symlink_to(root / '.env')
            with self.assertRaisesRegex(ValueError, 'symlink'):
                module.capture(root)

if __name__ == '__main__':
    unittest.main()
