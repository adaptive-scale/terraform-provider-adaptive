"""Exercise stage dispatch without invoking any live command."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


class CampaignDispatch(unittest.TestCase):
    def test_runtime_policies_forwards_explicit_setup_scope(self):
        root = Path(__file__).resolve().parents[2]
        with tempfile.TemporaryDirectory(dir=os.environ['TMPDIR']) as tmp:
            stub = Path(tmp) / 'bash'
            stub.write_text('#!/usr/bin/env python3\nimport json,sys\nprint(json.dumps(sys.argv[1:]))\n')
            stub.chmod(0o700)
            env = dict(os.environ, PATH=tmp + os.pathsep + os.environ['PATH'])
            result = subprocess.run(
                ['/bin/bash', str(root / 'testing/live/campaign.sh'), 'runtime-policies',
                 '/owned/fresh-run', 'approved-cluster', '/approved/backend-worktree'],
                env=env, text=True, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertTrue(result.stdout.startswith('['), 'runtime-policies stage was not dispatched')
            self.assertEqual(json.loads(result.stdout), [
                str(root / 'testing/live/runtime-policies.sh'), 'setup',
                '/owned/fresh-run', 'approved-cluster', '/approved/backend-worktree'])


if __name__ == '__main__':
    unittest.main()
