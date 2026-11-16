#!/bin/bash
# Safety assertions against an existing isolated run; never applies resources.
set -euo pipefail
umask 077
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
RUN=${1:?absolute isolated run directory required}
LOG="$RUN/evidence/runner-safety-$(date -u '+%Y%m%dT%H%M%SZ')-$$.txt"
bash "$ROOT/testing/live/run.sh" tf "$RUN" resource-postgres safety-preflight 0 validate > "$LOG" 2>&1
if bash "$ROOT/testing/live/run.sh" fixture "$RUN" safety-unowned adaptive_group '{"name":"not-owned-by-this-run"}' >> "$LOG" 2>&1; then
  printf '%s\n' 'FAIL: fixture command accepted a non-owned object name.'
  exit 1
fi
/usr/bin/grep -Fq 'Fixture object name/binding is not owned by this run.' "$LOG"
if bash "$ROOT/testing/live/destroy.sh" apply "$RUN" WRONG-PREFIX >> "$LOG" 2>&1; then
  printf '%s\n' 'FAIL: destroy accepted incorrect confirmation.'
  exit 1
fi
/usr/bin/grep -Fq 'Manual validation hold: supply the exact run prefix to authorize deletion.' "$LOG"
if bash "$ROOT/testing/live/run.sh" tf "$RUN" resource-postgres unsafe-delete-flag 1 apply -destroy -auto-approve >> "$LOG" 2>&1; then
 printf '%s\n' 'FAIL: normal runner accepted apply -destroy.' >&2
 exit 1
fi
/usr/bin/grep -Fq 'fixture guard rejected unsafe or unreadable configuration/plan' "$LOG"
printf '%s\n' 'PASS: successful preflight; exact non-owned-name rejection; wrong destroy confirmation; apply -destroy blocked.'
