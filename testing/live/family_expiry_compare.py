"""Pure schedule export comparator; never rewrites the supplied evidence.

Only expires_at is an absolute instant. Calendar-pattern strings and timezone
remain exact. Unknown offsets (-00:00) and leap-second spellings are rejected:
this policy requires a known, representable aware instant, not guessed time.
"""
import datetime
import re
from typing import Any

POLICY = 'schedule-expiry-aware-instant-v1'
_RFC3339 = re.compile(
    r'([0-9]{4})-([0-9]{2})-([0-9]{2})[Tt]([0-9]{2}):([0-9]{2}):([0-9]{2})'
    r'(?:\.([0-9]+))?([Zz]|[+-][0-9]{2}:[0-9]{2})\Z'
)


def _instant(value):
    if not isinstance(value, str):
        raise ValueError('expiry must be an RFC3339 string')
    match = _RFC3339.fullmatch(value)
    if not match:
        raise ValueError('expiry must be strict RFC3339 with a known offset')
    year, month, day, hour, minute, second = map(int, match.groups()[:6])
    date = datetime.datetime(year, month, day, hour, minute, second)
    fraction, offset = match.groups()[6:]
    offset_seconds = 0
    if offset not in ('Z', 'z'):
        oh, om = map(int, offset[1:].split(':'))
        if oh > 23 or om > 59 or offset == '-00:00':
            raise ValueError('invalid or unknown RFC3339 offset')
        offset_seconds = (oh * 3600 + om * 60) * (1 if offset[0] == '+' else -1)
    # Integer seconds plus an arbitrary-precision fractional string: datetime's
    # microseconds and float timestamps would erase submicrosecond differences.
    seconds = date.toordinal() * 86400 + hour * 3600 + minute * 60 + second - offset_seconds
    return seconds, (fraction or '').rstrip('0')


def _exact(actual, expected):
    if type(actual) is not type(expected):
        return False
    if isinstance(expected, dict):
        return actual.keys() == expected.keys() and all(_exact(actual[k], v) for k, v in expected.items())
    if isinstance(expected, list):
        return len(actual) == len(expected) and all(_exact(a, e) for a, e in zip(actual, expected))
    return actual == expected


def compare_schedule(actual, expected):
    """Return an explicit semantic verdict, retaining raw operands at caller.

    The sole absence equivalence is missing expires_at versus empty string,
    used by the supported clear/export-omission contract. Explicit null never
    establishes a clear, and missing/nonempty loss never establishes equality.
    """
    result: dict[str, Any] = dict(policy=POLICY, literal_byte_proof=False, observations=[], equal=False)
    if not isinstance(actual, dict) or not isinstance(expected, dict):
        return result
    other_actual = {k: v for k, v in actual.items() if k != 'expires_at'}
    other_expected = {k: v for k, v in expected.items() if k != 'expires_at'}
    if not _exact(other_actual, other_expected):
        return result
    missing = object()
    a, e = actual.get('expires_at', missing), expected.get('expires_at', missing)
    if a is missing and e is missing:
        result['equal'] = True
        return result
    if (a is missing or a == '') and (e is missing or e == ''):
        result['equal'] = True
        if a is missing or e is missing:
            result['observations'].append('expires_at_empty_missing_clear_contract')
        return result
    try:
        result['equal'] = _instant(a) == _instant(e)
    except (ValueError, TypeError):
        return result
    if result['equal'] and a != e:
        result['observations'].append('expires_at_spelling_differs')
    return result
