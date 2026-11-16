#!/bin/bash
# Run only AFTER Terraform teardown and manual validation.
set -euo pipefail
RUN=${1:?absolute isolated run directory required}
CONFIRM=${2:?exact run prefix required}
PREFIX=$(jq -er '.prefix | select(startswith("tflive-"))' "$RUN/manifest.json")
test "$CONFIRM" = "$PREFIX" || { printf '%s\n' 'Manual validation hold: wrong prefix.' >&2; exit 1; }
test "$(cd "$RUN/.." && pwd)" = "$HOME/IdeaProjects/.terraform-provider-adaptive-live"
if test -f "$RUN/infra/ownership.json"; then
  test "$(jq -r .ownership_label "$RUN/infra/ownership.json")" = "$PREFIX"
  for state in "$RUN"/work/*/terraform.tfstate; do
    test -f "$state" || continue
    jq -e '[.resources[]? | select(.type=="adaptive_endpoint" or .type=="adaptive_dataprotection" or .type=="adaptive_resource") | .instances[]?] | length==0' "$state" >/dev/null || { printf '%s\n' 'Terraform resources remain; refusing infrastructure cleanup.' >&2; exit 1; }
  done
  while IFS= read -r name; do
    [[ "$name" == "$PREFIX-"* ]]
    if docker container inspect "$name" >/dev/null 2>&1; then
      test "$(docker inspect "$name" --format '{{ index .Config.Labels "adaptive.terraform.run" }}')" = "$PREFIX"
      docker rm -f "$name"
      if docker container inspect "$name" >/dev/null 2>&1; then printf '%s\n' 'Container still present.' >&2; exit 1; fi
    fi
  done < <(jq -r '.docker_containers[]' "$RUN/infra/ownership.json")
fi
printf '%s\n' 'Owned target infrastructure removed; private evidence retained.'
