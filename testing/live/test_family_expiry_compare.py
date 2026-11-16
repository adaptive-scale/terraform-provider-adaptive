"""Offline tests; no live command, private run, or backend is invoked."""
import ast
import contextlib
import copy
import io
import json
import os
import pathlib
import tempfile
import time
import unittest
from typing import Any

HERE = pathlib.Path(__file__).resolve().parent


def harness_functions(namespace: dict[str, Any], *names):
    source = (HERE / 'families.sh').read_text().split("<<'PY'\n", 1)[1].rsplit('\nPY', 1)[0]
    tree = ast.parse(source)
    selected: list[ast.stmt] = [node for node in tree.body if isinstance(node, ast.FunctionDef) and node.name in names]
    assert len(selected) == len(names)
    exec(compile(ast.Module(body=selected, type_ignores=[]), 'families.sh', 'exec'), namespace)
    return namespace


class ScheduleReadbackTests(unittest.TestCase):
    def test_schedule_integration_rejects_loss_and_keeps_other_gates_literal(self):
        from family_expiry_compare import compare_schedule
        utc = '2030-12-31T00:00:00Z'
        cases = [({'expiresAt': utc}, {}, False),
                 ({}, {'expires_at': utc}, False),
                 ({}, {'expires_at': ''}, True),
                 ({'expiresAt': None}, {'expires_at': ''}, False),
                 ({'expiresAt': 'invalid'}, {'expires_at': 'invalid'}, False),
                 ({'expiresAt': False}, {'expires_at': utc}, False),
                 ({'expiresAt': 1}, {'expires_at': utc}, False),
                 ({'expiresAt': '2030-12-31T00:00:01Z'}, {'expires_at': utc}, False),
                 ({'expiresAt': utc, 'timezone': 'Etc/UTC'}, {'expires_at': utc, 'timezone': 'UTC'}, False),
                 ({'expiresAt': utc, 'pattern': {'specificDates': ['2030-12-01T05:30:00+05:30']}},
                  {'expires_at': utc, 'specific_dates': ['2030-12-01T00:00:00Z']}, False)]
        with tempfile.TemporaryDirectory(dir=os.environ.get('TMPDIR')) as tmp:
            root = pathlib.Path(tmp)
            (root / 'export.yaml').write_text('offline fixture')
            (root / 'ledger.jsonl').write_text(json.dumps(dict(case='schedule', phase='readback', evidence=str(root / 'export.yaml'))) + '\n')
            for exported, configured, expected_good in cases:
                rows = []
                namespace: dict[str, Any] = dict(OUT=root, RUN=root, RUNNER=root / 'never-executed', json=json,
                                 time=time, copy=copy, pathlib=pathlib, compare_schedule=compare_schedule,
                                 command=lambda *args: (True, root / 'command.txt'),
                                 yaml_parse=lambda text: {'configuration': dict(name='owned', **exported)},
                                 record=lambda case, phase, status, evidence, **kw: rows.append(dict(status=status, evidence=evidence, **kw)) or status == 'PASS')
                harness_functions(namespace, 'compare', 'readback')
                with self.subTest(exported=exported, configured=configured):
                    self.assertEqual(namespace['readback']('schedule', 'schedules', dict(name='owned', **configured), 'update'), expected_good)
                    self.assertEqual(rows[-1]['comparison_policy'], 'schedule-expiry-aware-instant-v1')
            # An identical expiry-looking string on another family stays literal.
            self.assertFalse(namespace['compare']('other', 'literal', {'expires_at': utc},
                                                {'expires_at': '2030-12-31T05:30:00+05:30'}))

    def test_offset_equivalence_through_real_readback(self):
        with tempfile.TemporaryDirectory(dir=os.environ.get('TMPDIR')) as tmp:
            root = pathlib.Path(tmp)
            (root / 'ledger.jsonl').write_text(json.dumps(dict(case='schedule', phase='readback', evidence=str(root / 'export.yaml'))) + '\n')
            (root / 'export.yaml').write_text('offline fixture')
            rows = []
            namespace: dict[str, Any] = dict(OUT=root, RUN=root, RUNNER=root / 'not-executed', json=json,
                             time=time, copy=copy, pathlib=pathlib,
                             command=lambda *args: (True, root / 'command.txt'),
                             yaml_parse=lambda text: {'configuration': {'name': 'owned', 'expiresAt': '2030-12-31T05:30:00+05:30'}},
                             record=lambda case, phase, status, evidence, **kw: rows.append(dict(status=status, evidence=evidence, **kw)) or status == 'PASS')
            from family_expiry_compare import compare_schedule
            namespace['compare_schedule'] = compare_schedule
            harness_functions(namespace, 'compare', 'readback')
            self.assertTrue(namespace['readback']('schedule', 'schedules', {'name': 'owned', 'expires_at': '2030-12-31T00:00:00Z'}, 'create'))
            artifact = json.loads(pathlib.Path(rows[-1]['evidence']).read_text())
            self.assertEqual(artifact['actual']['expires_at'], '2030-12-31T05:30:00+05:30')
            self.assertEqual(artifact['expected']['expires_at'], '2030-12-31T00:00:00Z')
            self.assertEqual(artifact['comparison']['policy'], 'schedule-expiry-aware-instant-v1')
            self.assertIn('expires_at_spelling_differs', artifact['comparison']['observations'])


class ExpiryComparatorTests(unittest.TestCase):
    def test_strict_expiry_and_precision_boundaries(self):
        from family_expiry_compare import compare_schedule
        utc = '2030-12-31T00:00:00Z'
        passing = [('2030-12-31T05:30:00+05:30', utc),
                   ('2030-12-30T19:00:00-05:00', utc),
                   ('2030-12-31t00:00:00z', utc),
                   ('2030-12-31T00:00:00.123456789000Z', '2030-12-31T05:30:00.123456789+05:30'),
                   ('2030-12-31T00:00:00.000000000Z', utc), ('', '')]
        for a, e in passing:
            with self.subTest(actual=a, expected=e):
                self.assertTrue(compare_schedule({'expires_at': a}, {'expires_at': e})['equal'])
        invalid = [None, False, True, 0, 1, [], {}, '', '2030-12-31T00:00:01Z',
                   '2030-12-31T00:00:00', '2030-12-31 00:00:00Z', '2030-12-31T00:00:00+0530',
                   '2030-12-31T00:00:00+24:00', '2030-12-31T00:00:00+00:60',
                   '2030-12-31T00:00:00-00:00', '2030-02-30T00:00:00Z',
                   '2030-12-31T24:00:00Z', '2030-12-31T00:00:60Z', utc + '\n',
                   '2030-12-31T00:00:00.000000001Z']
        for a in invalid:
            with self.subTest(actual=a):
                self.assertFalse(compare_schedule({'expires_at': a}, {'expires_at': utc})['equal'])
        # Equal invalid operands must not sneak through an initial equality check.
        for a in (None, True, 1, 'not-a-date', '2030-12-31T00:00:00'):
            self.assertFalse(compare_schedule({'expires_at': a}, {'expires_at': a})['equal'])
        self.assertFalse(compare_schedule({'expires_at': '2030-12-31T00:00:00.1234567891Z'},
                                          {'expires_at': '2030-12-31T00:00:00.1234567892Z'})['equal'])

    def test_missing_clear_and_other_fields_exact(self):
        from family_expiry_compare import compare_schedule
        for a, e in (({}, {}), ({}, {'expires_at': ''}), ({'expires_at': ''}, {})):
            self.assertTrue(compare_schedule(a, e)['equal'])
        for a, e in (({}, {'expires_at': '2030-12-31T00:00:00Z'}),
                     ({'expires_at': None}, {}), ({'expires_at': None}, {'expires_at': ''}),
                     ({'expires_at': ''}, {'expires_at': None})):
            self.assertFalse(compare_schedule(a, e)['equal'])
        baseline = dict(name='owned', timezone='UTC', start_hour=9, all_day=False,
                        specific_dates=['2030-12-01T00:00:00Z'],
                        expires_at='2030-12-31T00:00:00Z')
        for field, value in dict(name='different', timezone='Etc/UTC', start_hour=10,
                                 all_day=0, specific_dates=['2030-12-01T05:30:00+05:30']).items():
            actual = dict(baseline, **{field: value})
            self.assertFalse(compare_schedule(actual, baseline)['equal'], field)
        untouched = copy.deepcopy(baseline)
        compare_schedule(baseline, untouched)
        self.assertEqual(baseline, untouched)


class ReportTests(unittest.TestCase):
    def test_versioned_record_derived_reports_preserve_legacy_failures(self):
        import collections
        import datetime
        import sys
        with tempfile.TemporaryDirectory(dir=os.environ.get('TMPDIR')) as tmp:
            root = pathlib.Path(tmp)
            records = root / 'results.jsonl'
            rows = [dict(case='cohort-auth-cockroachdb', phase='create', status='PASS'),
                    dict(case='schedule', phase='exact-readback-create', status='FAIL'),
                    dict(case='schedule', phase='exact-readback-create', status='PASS')]
            raw = ''.join(json.dumps(row) + '\n' for row in rows)
            records.write_text(raw)
            for name in ('findings.json', 'report.json', 'fixtures.tsv'):
                (root / name).write_text('immutable legacy')
            source = (HERE / 'families.sh').read_text().split("<<'PY'\n", 1)[1].rsplit('\nPY', 1)[0]
            start = source.index('rows=[json.loads(x) for x in RECORDS.read_text()')
            namespace = dict(RECORDS=records, RUN=root, OUT=root, json=json, collections=collections,
                             TYPES=['cockroachdb'], PATTERNS=['custom'], source={},
                             datetime=datetime, time=time, sys=sys, state=lambda case: {})
            harness_functions(namespace, 'versioned_output', 'record_findings')
            with contextlib.redirect_stdout(io.StringIO()), self.assertRaises(SystemExit) as exit_context:
                exec(compile(source[start:], 'families.sh-report', 'exec'), namespace)
            self.assertEqual(exit_context.exception.code, 1)
            for name in ('findings.json', 'report.json', 'fixtures.tsv'):
                self.assertEqual((root / name).read_text(), 'immutable legacy')
            self.assertEqual(records.read_text(), raw)
            reports = list(root.glob('report-v2-*.json'))
            self.assertEqual(len(reports), 1)
            report = json.loads(reports[0].read_text())
            self.assertEqual(report['failures'], [rows[1]])
            findings = json.loads(next(root.glob('findings-v2-*.json')).read_text())
            self.assertEqual(findings['historical_nonpass_records'], [rows[1]])
            self.assertEqual(findings['latest_by_case_phase'][1]['status'], 'PASS')
            self.assertEqual(findings['outcome_basis'], 'recorded outcomes only; no product-defect inference')
            self.assertIn('runtime_enforcement_not_tested', findings['boundary_labels'])
            # A second generation must append another report, not replace one.
            with contextlib.redirect_stdout(io.StringIO()), self.assertRaises(SystemExit):
                exec(compile(source[start:], 'families.sh-report', 'exec'), namespace)
            self.assertEqual(len(list(root.glob('report-v2-*.json'))), 2)


if __name__ == '__main__':
    unittest.main()
