#!/bin/bash
# Scoped discovery attempt. A scheduler acknowledgment is NOT sync completion.
# Watches only the collector pod for this run's PostgreSQL resource, then disables
# future discovery scheduling. No fixture or collector pod is deleted by this script.
set -euo pipefail
umask 077
RUN=${1:?absolute isolated run directory required}
PREFIX=$(jq -er .prefix "$RUN/manifest.json")
NAME="$PREFIX-runtime-postgres"
STATE="$RUN/work/runtime-postgres/terraform.tfstate"
jq -e --arg n "$NAME" '.resources[0].instances[0].attributes.name==$n' "$STATE" >/dev/null
ID=$(jq -er '.resources[0].instances[0].attributes.id' "$STATE")
[[ "$ID" =~ ^[0-9a-f-]{36}$ ]]
test -z "${ADAPTIVE_SVC_TOKEN:-}" && test -z "${ADAPTIVE_URL:-}" && test -z "${ADAPTIVE_PROFILE:-}"
adaptive whoami -o json | jq -e '.URL=="http://localhost:8080" and .CredentialSource=="saved context"' >/dev/null
STAMP=$(date -u '+%Y%m%dT%H%M%SZ')-$$
mkdir -p "$RUN/sync"
jq -n --arg r "$NAME" '{type:"DataDiscoverySchedule",configuration:{resource:$r,cluster:"k8s-local",frequency:"24h",frequencyType:"interval",enabled:true,classificationEnabled:false}}' > "$RUN/sync/discovery-enabled.json"
jq '.configuration.enabled=false' "$RUN/sync/discovery-enabled.json" > "$RUN/sync/discovery-disabled.json"
record() { jq -nc --arg c sync-runtime-postgres --arg p "$1" --arg s "$2" --arg e "$3" --argjson rc "$4" '{case:$c,phase:$p,status:$s,exit:$rc,evidence:$e}' >> "$RUN/ledger.jsonl"; }
# Server-side watch, not a polling loop. No pod spec, secret, or row data is saved.
kubectl --context kind-endpoint-cluster get pods -n default --field-selector "metadata.name=adaptive-system-sync-$ID" --watch --output-watch-events -o json --request-timeout=330s 2> "$RUN/sync/watch-$STAMP.stderr" | jq --unbuffered -c 'select(.object.kind=="Pod")|{event:.type,pod:.object.metadata.name,uid:.object.metadata.uid,created:.object.metadata.creationTimestamp,phase:.object.status.phase}' > "$RUN/sync/watch-$STAMP.jsonl" &
WATCH=$!
set +e
adaptive admin apply -f "$RUN/sync/discovery-enabled.json" > "$RUN/sync/enable-$STAMP.txt" 2>&1
RC=$?
set -e
STATUS=FAIL; test "$RC" != 0 || STATUS=ACKNOWLEDGED
record discovery-enable "$STATUS" "$RUN/sync/enable-$STAMP.txt" "$RC"
# Even a failed response can follow persistence; always apply the explicit disable
# below rather than leaving a recurring test schedule enabled.
if test "$RC" = 0; then
 set +e; wait "$WATCH"; WATCH_RC=$?; set -e
else
 kill "$WATCH" 2>/dev/null || true
 set +e; wait "$WATCH"; WATCH_RC=$?; set -e
fi
set +e
adaptive admin apply -f "$RUN/sync/discovery-disabled.json" > "$RUN/sync/disable-$STAMP.txt" 2>&1
DISABLE_RC=$?
set -e
STATUS=FAIL; test "$DISABLE_RC" != 0 || STATUS=ACKNOWLEDGED
record discovery-disable "$STATUS" "$RUN/sync/disable-$STAMP.txt" "$DISABLE_RC"
if test -s "$RUN/sync/watch-$STAMP.jsonl"; then
 record sync-worker-observed PASS "$RUN/sync/watch-$STAMP.jsonl" 0
else
 record sync-worker-observed BLOCKED "$RUN/sync/watch-$STAMP.jsonl" "$WATCH_RC"
fi
# The CLI has no discovery export/status reader. Pod deletion also occurs on
# failure, so neither a watch event nor a 200 response establishes completion.
record sync-completion BLOCKED "$RUN/sync/watch-$STAMP.jsonl" 1
printf '%s\n' 'Discovery attempt recorded; future scheduling disable requested. Completion/metadata persistence remains unverified, not PASS.'
test "$RC" = 0 && test "$DISABLE_RC" = 0
