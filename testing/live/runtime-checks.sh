#!/bin/bash
# Repeatable runtime checks; SQL asserts the owned database without printing rows.
set -euo pipefail
umask 077
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
RUN=${1:?absolute isolated run directory required}
PREFIX=$(jq -er .prefix "$RUN/manifest.json")
adaptive whoami -o json | jq -e '.URL=="http://localhost:8080" and .CredentialSource=="saved context"' >/dev/null
python3 "$ROOT/testing/live/guard.py" config "$RUN" endpoint-cli >/dev/null
python3 "$ROOT/testing/live/guard.py" config "$RUN" endpoint-client >/dev/null
STAMP=$(date -u '+%Y%m%dT%H%M%SZ')-$$
record() { jq -nc --arg at "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" --arg c "$1" --arg p "$2" --arg e "$3" --argjson rc "$4" '{at:$at,case:$c,phase:$p,status:(if $rc==0 then "PASS" else "FAIL" end),exit:$rc,evidence:$e}' >> "$RUN/ledger.jsonl"; }
set +e
adaptive exec "$PREFIX-postgres-cli" --timeout 60s -f "$ROOT/testing/live/postgres-smoke.sql" > "$RUN/evidence/cli-exec-$STAMP.txt" 2>&1
CLI_RC=$?
set -e
record endpoint-cli runtime-exec-smoke "$RUN/evidence/cli-exec-$STAMP.txt" "$CLI_RC"
set +e
bash "$ROOT/testing/live/native/check-postgres-client.sh" "$PREFIX-postgres-client" "$ROOT/testing/live/postgres-smoke.sql" auto "$RUN/evidence/native-client-$STAMP.tsv"
CLIENT_RC=$?
set -e
record endpoint-client runtime-privileged-smoke "$RUN/evidence/native-client-$STAMP.tsv" "$CLIENT_RC"
printf 'Exec smoke exit=%s; native-client smoke exit=%s. No objects destroyed.\n' "$CLI_RC" "$CLIENT_RC"
test "$CLI_RC" = 0 && test "$CLIENT_RC" = 0
