#!/usr/bin/env python3
"""Fingerprint local provider Go sources without reading credentials or state."""
import argparse
import hashlib
import json
import os
from pathlib import Path


def capture(root):
    root = Path(root).resolve()
    candidates = list(root.glob('*.go')) + list((root / 'internal').rglob('*.go'))
    candidates += [root / name for name in ('go.mod', 'go.sum') if (root / name).exists()]
    files = {}
    for path in sorted(candidates):
        if path.is_symlink():
            raise ValueError('symlinked provider source input is not allowed')
        relative = str(path.relative_to(root))
        files[relative] = hashlib.sha256(path.read_bytes()).hexdigest()
    serialized = json.dumps(files, sort_keys=True, separators=(',', ':')).encode()
    return {'sha256': hashlib.sha256(serialized).hexdigest(), 'files': files,
            'scope': 'Local provider Go sources (including tests), go.mod and go.sum; hashes only, not credential files or a dependency supply-chain attestation'}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('root', type=Path)
    parser.add_argument('destination', type=Path)
    args = parser.parse_args()
    os.umask(0o077)
    result = capture(args.root)
    with args.destination.open('x') as output:
        json.dump(result, output, indent=2)
        output.write('\n')
    print(result['sha256'])


if __name__ == '__main__':
    main()
