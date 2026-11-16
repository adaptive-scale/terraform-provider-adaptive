#!/usr/bin/env python3
"""Plan cleanup from run ownership, exact CLI listing IDs, and retained states."""
import json
from pathlib import Path
import re
import sys


def classify(remote_id, recorded_id, managed_ids):
    if remote_id is None:
        return 'ABSENT'
    if recorded_id is not None and remote_id != recorded_id:
        raise ValueError('recorded and remote object identities disagree')
    if remote_id in managed_ids:
        return 'MANAGED'
    return 'ORPHAN_VERIFIED' if recorded_id else 'ORPHAN_UNVERIFIED'


def main(runtext, listingtext, outputtext):
    run = Path(runtext).resolve()
    manifest = json.loads((run/'manifest.json').read_text())
    prefix = manifest['prefix']
    if manifest['url'] != 'http://localhost:8080' or not prefix.startswith('tflive-'):
        raise ValueError('not an owned local run')
    live = {}
    for line in Path(listingtext).read_text().splitlines():
        columns = line.split('│')
        if len(columns) < 4:
            continue
        name, slug = columns[1].strip(), columns[3].strip()
        match = re.fullmatch(r'/resources/([a-f0-9-]{36})', slug)
        if not name.startswith(prefix+'-') or not match:
            continue
        if name in live:
            raise ValueError('ambiguous owned resource name')
        live[name] = match.group(1)
    managed = set()
    for state in (run/'work').glob('*/terraform.tfstate'):
        for resource in json.loads(state.read_text()).get('resources', []):
            if resource['type'] != 'adaptive_resource':
                continue
            for instance in resource.get('instances', []):
                identity = instance.get('attributes', {}).get('id')
                if identity:
                    managed.add(identity)
    owned = {}
    ownership = run/'cli-origin/ownership.jsonl'
    for line in ownership.read_text().splitlines() if ownership.exists() else []:
        row = json.loads(line)
        if not row['name'].startswith(prefix+'-'):
            raise ValueError('non-owned CLI origin name')
        docpath = Path(row['document']).resolve()
        if run/'cli-origin' not in docpath.parents:
            raise ValueError('CLI document outside run')
        doc = json.loads(docpath.read_text())
        if doc.get('type') != 'resource' or doc.get('configuration', {}).get('name') != row['name']:
            raise ValueError('CLI document identity mismatch')
        if not re.fullmatch(r'[a-z0-9][a-z0-9_-]*', row['fixture']):
            raise ValueError('unsafe fixture identifier')
        idpath = run/'cli-origin'/(row['fixture']+'.id')
        recorded = idpath.read_text().strip() if idpath.exists() else None
        if recorded and not re.fullmatch(r'[a-f0-9-]{36}', recorded):
            raise ValueError('invalid recorded ID format')
        remote = live.get(row['name'])
        owned[row['name']] = dict(name=row['name'], document=str(docpath), fixture=row['fixture'], remote_id=remote,
                                  recorded_id=recorded, status=classify(remote, recorded, managed))
    result = {'prefix':prefix,'entries':list(owned.values()),'scope':'CLI-origin integrations only; generated authorization cleanup and Kubernetes residue are separate.'}
    output = Path(outputtext)
    output.write_text(json.dumps(result, indent=2)+'\n')
    output.chmod(0o600)
    counts = {status:sum(x['status']==status for x in owned.values()) for status in ('MANAGED','ABSENT','ORPHAN_VERIFIED','ORPHAN_UNVERIFIED')}
    print(json.dumps(counts))


if __name__ == '__main__':
    try:
        main(*sys.argv[1:])
    except (ValueError, KeyError, OSError, TypeError):
        print('orphan planning failed closed; inspect ownership metadata', file=sys.stderr)
        raise SystemExit(1)
