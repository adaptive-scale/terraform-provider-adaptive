#!/bin/bash
# Explicit stages only. There is deliberately no automatic destroy stage.
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
STAGE=${1:-help}
RUN=${2:-}
case "$STAGE" in
 init) exec bash "$ROOT/testing/live/run.sh" init ;;
 baseline) exec bash "$ROOT/testing/live/run.sh" baseline "$RUN" ;;
 resources) exec bash "$ROOT/testing/live/integrations.sh" "$RUN" ;;
 imports) exec bash "$ROOT/testing/live/import-resources.sh" "$RUN" ;;
 auth-modes) exec bash "$ROOT/testing/live/auth-modes.sh" "$RUN" ;;
 field-updates) exec python3 "$ROOT/testing/live/field-updates.py" "$RUN" --execute ;;
 sync) exec bash "$ROOT/testing/live/sync.sh" "$RUN" ;;
 family-imports) exec bash "$ROOT/testing/live/family-imports.sh" "$RUN" "${3:-$(dirname "$ROOT")/inventorize}" "${4:-all}" ;;
 secrets) exec bash "$ROOT/testing/live/secrets.sh" "$RUN" ;;
 runtime-prepare) exec bash "$ROOT/testing/live/runtime-prepare.sh" "$RUN" "${3:-k8s-local}" ;;
 runtime-checks) exec bash "$ROOT/testing/live/runtime-checks.sh" "$RUN" ;;
 runtime-policies) exec bash "$ROOT/testing/live/runtime-policies.sh" setup "$RUN" "${3:-k8s-local}" "${4:-$(dirname "$ROOT")/inventorize}" ;;
 endpoint-policies) exec bash "$ROOT/testing/live/endpoints.sh" "$RUN" ;;
 dataprotection) exec bash "$ROOT/testing/live/dataprotection.sh" "$RUN" ;;
 negative-imports) exec bash "$ROOT/testing/live/negative-imports.sh" "$RUN" ;;
 families|verify-families)
  BACKEND=${3:-$(dirname "$ROOT")/inventorize}
  PREFIX=$(jq -er .prefix "$RUN/manifest.json")
  MODE=all; test "$STAGE" != verify-families || MODE=verify
  exec bash "$ROOT/testing/live/families.sh" "$RUN" "$BACKEND" "$MODE" "$PREFIX-postgres-cli"
  ;;
 report)
  python3 "$ROOT/testing/live/secret_audit.py" "$RUN"
  exec python3 "$ROOT/testing/live/report.py" "$RUN"
  ;;
 destroy-plan)
  bash "$ROOT/testing/live/destroy-generated.sh" plan "$RUN"
  bash "$ROOT/testing/live/destroy-orphans.sh" plan "$RUN"
  if test -f "$RUN/family-imports/ownership.jsonl"; then
    bash "$ROOT/testing/live/destroy-family-orphans.sh" plan "$RUN"
  fi
  exec bash "$ROOT/testing/live/destroy.sh" plan "$RUN"
  ;;
 *)
  printf '%s\n' 'Stages: init, baseline, resources, imports, auth-modes, field-updates, family-imports, secrets, runtime-prepare, runtime-checks, runtime-policies, endpoint-policies, dataprotection, sync, negative-imports, families, verify-families, report, destroy-plan.' 'Use an explicit RUN path for every stage after init. No stage applies destroy.' 'Additional auth-mode and field-specific runners have their own case/result manifests; see README.md.'
  ;;
esac
