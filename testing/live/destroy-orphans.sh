#!/bin/bash
# Explicit orphan reconciliation, separate from Terraform's state-managed destroy.
set -euo pipefail
umask 077
MODE=${1:?plan or apply required}
RUN=${2:?absolute isolated run directory required}
CONFIRM=${3:-}
HERE=$(cd "$(dirname "$0")" && pwd)
PREFIX=$(jq -er '.prefix | select(startswith("tflive-"))' "$RUN/manifest.json")
test "$(cd "$RUN/.." && pwd)" = "$HOME/IdeaProjects/.terraform-provider-adaptive-live"
case "$MODE" in plan) ;; apply) test "$CONFIRM" = "$PREFIX" || { printf '%s\n' 'Wrong manual-validation confirmation.' >&2; exit 1; } ;; *) exit 1 ;; esac
test -z "${ADAPTIVE_SVC_TOKEN:-}" && test -z "${ADAPTIVE_URL:-}" && test -z "${ADAPTIVE_PROFILE:-}"
adaptive whoami -o json | jq -e '.URL=="http://localhost:8080" and .CredentialSource=="saved context"' >/dev/null
STAMP=$(date -u '+%Y%m%dT%H%M%SZ')-$$
DIR="$RUN/destroy/orphans-$STAMP"
mkdir -p "$DIR"
adaptive admin list-resources > "$DIR/before-listing.txt"
python3 "$HERE/orphan_plan.py" "$RUN" "$DIR/before-listing.txt" "$DIR/plan.json"
if test "$MODE" = apply; then
 jq -e 'all(.entries[];.status!="ORPHAN_UNVERIFIED")' "$DIR/plan.json" >/dev/null || { printf '%s\n' 'Unverified orphan identity; record and independently verify its ID before deletion.' >&2; exit 1; }
 while IFS= read -r row; do
  fixture=$(jq -r .fixture <<< "$row")
  doc=$(jq -r .document <<< "$row")
  adaptive admin delete -f "$doc" > "$DIR/$fixture-delete.txt" 2>&1
  adaptive admin list-resources > "$DIR/$fixture-after-listing.txt"
  python3 "$HERE/orphan_plan.py" "$RUN" "$DIR/$fixture-after-listing.txt" "$DIR/$fixture-after-plan.json" >/dev/null
  name=$(jq -r .name <<< "$row")
  jq -e --arg n "$name" 'any(.entries[];.name==$n and .status=="ABSENT")' "$DIR/$fixture-after-plan.json" >/dev/null
 done < <(jq -c '.entries[]|select(.status=="ORPHAN_VERIFIED")' "$DIR/plan.json")
fi
printf 'Orphan reconciliation mode=%s; evidence=%s\n' "$MODE" "$DIR"
printf '%s\n' 'State-managed objects were not deleted by this stage. Masking authorization/view residue requires its separately scoped cleanup.'
