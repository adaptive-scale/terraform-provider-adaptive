#!/usr/bin/env python3
"""Audit ONLY run-owned synthetic credentials; never emits credential values."""
import argparse
import json
from pathlib import Path


def visible_fields(attributes, sensitive_names, text):
    result = []
    for name in sorted(sensitive_names):
        value = attributes.get(name)
        if not isinstance(value, str) or len(value) < 8:
            continue
        candidates = [value]
        try:
            document = json.loads(value)
        except (ValueError, TypeError):
            document = None
        if isinstance(document, dict):
            candidates += [v for k, v in document.items()
                           if ('private' in k.lower() or 'secret' in k.lower())
                           and isinstance(v, str) and len(v) >= 8]
        if any(v in text for v in candidates):
            result.append(name)
    return result


def fixture_configs(run):
    return sorted(path for pattern in ('integration-*/main.tf.json', 'auth-*/main.tf.json')
                  for path in (run / 'work').glob(pattern))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('run', type=Path)
    args = parser.parse_args()
    run = args.run.resolve()
    manifest = json.loads((run / 'manifest.json').read_text())
    if manifest['url'] != 'http://localhost:8080' or not manifest['prefix'].startswith('tflive-'):
        raise SystemExit('not an owned local run')
    schema = json.loads((run / 'inventory/schema.json').read_text())
    block = schema['provider_schemas']['registry.terraform.io/adaptive-scale/adaptive']['resource_schemas']['adaptive_resource']['block']
    sensitive = {k for k, v in block['attributes'].items() if v.get('sensitive')}
    ledger = [json.loads(s) for s in (run / 'ledger.jsonl').read_text().splitlines()]
    rows = []
    for config in fixture_configs(run):
        fixture = config.parent.name
        attrs = json.loads(config.read_text())['resource']['adaptive_resource']['test']
        if not attrs['name'].startswith(manifest['prefix'] + '-'):
            raise SystemExit('non-owned fixture encountered')
        case_rows = [r for r in ledger if r['case'] == fixture]
        exports = [Path(r['evidence']) for r in case_rows if r['phase'] == 'readback' and r['status'] == 'PASS']
        export = next((p for p in reversed(exports) if p.is_file()), None)
        logs = [Path(r['evidence']) for r in case_rows if r['phase'] != 'readback' and Path(r['evidence']).suffix == '.txt']
        human_text = '\n'.join(p.read_text(errors='replace') for p in logs if p.is_file())
        state_path = config.parent / 'terraform.tfstate'
        state_text = state_path.read_text() if state_path.is_file() else ''
        rows.append({
            'fixture': fixture, 'integration': attrs['type'],
            'configured_sensitive_fields': sorted(sensitive & attrs.keys()),
            'sensitive_canary_visible_in_cli_export': visible_fields(attrs, sensitive, export.read_text(errors='replace')) if export else [],
            'sensitive_canary_visible_in_ordinary_tf_logs': visible_fields(attrs, sensitive, human_text),
            'sensitive_canary_persisted_in_state': visible_fields(attrs, sensitive, state_text),
            'state_permissions_private': (state_path.stat().st_mode & 0o077) == 0 if state_path.is_file() else None,
            'export_evidence': str(export) if export else None,
        })
    output = {
        'scope': 'Original integration and alternate-auth synthetic scalar inputs only; no nested RDP audit or cryptographic guarantee. Counts are schema-sensitive matches, NOT confirmed credential-leak counts. AWS/Keyspaces service-account identifiers and noncredential URIs can legitimately be visible; see FINDINGS.md for classification.',
        'fixtures': rows,
        'counts': {
            'fixtures': len(rows),
            'ordinary_log_exposure_fixtures': sum(bool(r['sensitive_canary_visible_in_ordinary_tf_logs']) for r in rows),
            'export_visible_sensitive_fixtures': sum(bool(r['sensitive_canary_visible_in_cli_export']) for r in rows),
            'state_canary_present_fixtures': sum(bool(r['sensitive_canary_persisted_in_state']) for r in rows),
            'nonprivate_state_fixtures': sum(r['state_permissions_private'] is False for r in rows),
        },
    }
    path = run / 'secret-audit.json'
    path.write_text(json.dumps(output, indent=2) + '\n')
    path.chmod(0o600)
    print(output['scope'])
    print(json.dumps(output['counts'], indent=2))
    print('Sensitive fields visible in CLI exports (names only):')
    print(json.dumps([{ 'integration': r['integration'], 'fields': r['sensitive_canary_visible_in_cli_export'] } for r in rows if r['sensitive_canary_visible_in_cli_export']], indent=2))


if __name__ == '__main__':
    main()
