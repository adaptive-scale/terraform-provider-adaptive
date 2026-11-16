#!/bin/bash
# Two operational children: Adaptive in a private PTY, then native psql.
# ENDPOINT SQL_FILE PORT_OR_auto REPORT_TSV [EXPECTED_SQLSTATE|-]
set +x
set -eu
base=$(cd "$(dirname "$0")" && pwd)
command -v expect >/dev/null 2>&1 || { printf 'Runner requires Expect for terminal-safe capture\n' >&2; exit 2; }
exec expect -f "$base/postgres-client-session.exp" "$@"
