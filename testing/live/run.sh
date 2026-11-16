#!/bin/bash
# Resumable local-only acceptance runner. Never destroys automatically.
set -euo pipefail
umask 077
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
ACTION=${1:-help}
RUN=${2:-}
BASE="$HOME/IdeaProjects/.terraform-provider-adaptive-live"

die() { printf '%s\n' "$*" >&2; exit 1; }
local_identity() {
  test -z "${ADAPTIVE_SVC_TOKEN:-}" || die 'Unset ADAPTIVE_SVC_TOKEN: this runner requires the saved local CLI context.'
  test -z "${ADAPTIVE_URL:-}" || die 'Unset ADAPTIVE_URL: verify the actual saved CLI context, not an environment override.'
  test -z "${ADAPTIVE_PROFILE:-}" || die 'Unset ADAPTIVE_PROFILE: provider uses the default CLI token file.'
  adaptive whoami -o json | jq -e '.URL == "http://localhost:8080" and .CredentialSource == "saved context"' >/dev/null
  curl --fail --silent --max-time 10 http://localhost:8080/ >/dev/null
}
check_run() {
  test -n "$RUN" && test -f "$RUN/manifest.json" || die 'Supply an existing absolute run directory.'
  test "$(cd "$RUN/.." && pwd)" = "$BASE" || die 'Run must be an immediate child of the private run base.'
  jq -e '.url == "http://localhost:8080" and (.prefix | startswith("tflive-"))' "$RUN/manifest.json" >/dev/null
  test "$(shasum -a 256 "$RUN/bin/terraform-provider-adaptive" | cut -d ' ' -f 1)" = "$(jq -r .binary_sha256 "$RUN/manifest.json")" || die 'Pinned binary checksum mismatch.'
  local_identity
}
ledger() {
  jq -nc --arg at "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" --arg case "$1" --arg phase "$2" --arg status "$3" --arg evidence "$4" --argjson exit "$5" --arg config_sha256 "${ACTIVE_CONFIG_SHA:-}" --arg snapshot "${ACTIVE_SNAPSHOT:-}" --argjson arguments "${ACTIVE_ARGUMENTS:-[]}" '{at:$at,case:$case,phase:$phase,status:$status,exit:$exit,evidence:$evidence,config_sha256:$config_sha256,configuration_snapshot:$snapshot,arguments:$arguments}' >> "$RUN/ledger.jsonl"
}
validate_fixture() {
  local fixture=$1 family=$2 attrs=$3 prefix
  [[ "$fixture" =~ ^[a-z0-9][a-z0-9_-]*$ ]] || die 'Fixture identifier must be a simple lowercase filename.'
  [[ "$family" =~ ^adaptive_(resource|endpoint|authorization|group|script|schedule|dataprotection)$ ]] || die 'Unsupported fixture family.'
  prefix=$(jq -r .prefix "$RUN/manifest.json")
  jq -e --arg p "$prefix-" '((.name // .resource // "") | startswith($p))' <<< "$attrs" >/dev/null || die 'Fixture object name/binding is not owned by this run.'
}
run_tf() {
  local fixture=$1 phase=$2 expected=$3
  shift 3
  [[ "$fixture" =~ ^[a-z0-9][a-z0-9_-]*$ ]] || die 'Invalid fixture identifier.'
  [[ "$phase" =~ ^[a-z0-9][a-z0-9_-]*$ ]] || die 'Invalid phase identifier.'
  test -f "$RUN/work/$fixture/main.tf.json" || die "Unknown fixture: $fixture"
  local config="$RUN/work/$fixture/main.tf.json" family attrs
  jq -e '.provider.adaptive.workspace_url == "http://localhost:8080" and (.provider.adaptive.service_token == null) and (.terraform.backend == null) and (.resource | length == 1)' "$config" >/dev/null || die 'Unsafe provider/backend/resource configuration.'
  family=$(jq -r '.resource | keys[0]' "$config")
  test "$(jq --arg f "$family" '.resource[$f] | length' "$config")" = 1 || die 'One object per fixture is required.'
  attrs=$(jq -c --arg f "$family" '.resource[$f].test' "$config")
  validate_fixture "$fixture" "$family" "$attrs"
  python3 "$ROOT/testing/live/guard.py" command "$RUN" "$fixture" "$@" >/dev/null
  local sequence log rc status
  sequence=$(date -u '+%Y%m%dT%H%M%S')-$$-$RANDOM
  log="$RUN/evidence/$fixture-$phase-$sequence.txt"
  ACTIVE_SNAPSHOT="$RUN/evidence/$fixture-$phase-$sequence.tf.json"
  cp "$config" "$ACTIVE_SNAPSHOT"
  ACTIVE_CONFIG_SHA=$(shasum -a 256 "$ACTIVE_SNAPSHOT" | cut -d ' ' -f 1)
  ACTIVE_ARGUMENTS=$(printf '%s\n' "$@" | jq -R . | jq -s .)
  ledger "$fixture" "$phase" STARTED "$log" -1
  set +e
  env -u ADAPTIVE_SVC_TOKEN -u ADAPTIVE_URL -u ADAPTIVE_PROFILE -u TF_LOG -u TF_LOG_PATH -u TF_REATTACH_PROVIDERS -u TF_WORKSPACE -u TF_DATA_DIR -u TF_CLI_ARGS -u TF_CLI_ARGS_plan -u TF_CLI_ARGS_apply -u TF_CLI_ARGS_validate -u TF_CLI_ARGS_import -u TF_CLI_ARGS_show TF_CLI_CONFIG_FILE="$RUN/terraform.rc" TF_IN_AUTOMATION=1 CHECKPOINT_DISABLE=1 terraform -chdir="$RUN/work/$fixture" "$@" -no-color >"$log" 2>&1
  rc=$?
  set -e
  status=FAIL
  test "$rc" = "$expected" && status=PASS
  ledger "$fixture" "$phase" "$status" "$log" "$rc"
  printf '%s %s %s (exit %s; expected %s)\n' "$fixture" "$phase" "$status" "$rc" "$expected"
  test "$status" = PASS
}
make_fixture() {
  local fixture=$1 family=$2 attrs=$3
  validate_fixture "$fixture" "$family" "$attrs"
  mkdir -p "$RUN/work/$fixture"
  test ! -e "$RUN/work/$fixture/main.tf.json" || die "Fixture already exists: $fixture"
  jq -n --arg family "$family" --argjson attrs "$attrs" '{terraform:{required_providers:{adaptive:{source:"adaptive-scale/adaptive"}}},provider:{adaptive:{workspace_url:"http://localhost:8080"}},resource:{($family):{test:$attrs}}}' > "$RUN/work/$fixture/main.tf.json"
}
case "$ACTION" in
  init)
    local_identity
    test -z "$RUN" || die 'init generates a unique run directory; do not supply a run path.'
    mkdir -p "$BASE"
    RUN="$BASE/$(date -u '+%Y%m%dT%H%M%SZ')-$$"
    mkdir -m 700 "$RUN" "$RUN/bin" "$RUN/evidence" "$RUN/work" "$RUN/inventory"
    source_before=$(python3 "$ROOT/testing/live/source_capture.py" "$ROOT" "$RUN/inventory/provider-source-before-build.json")
    (cd "$ROOT" && go build -o "$RUN/bin/terraform-provider-adaptive" .)
    source_after=$(python3 "$ROOT/testing/live/source_capture.py" "$ROOT" "$RUN/inventory/provider-source.json")
    test "$source_before" = "$source_after" || die 'Provider sources changed during the build; no live fixtures were created. Start a new run after writers finish.'
    sha=$(shasum -a 256 "$RUN/bin/terraform-provider-adaptive" | cut -d ' ' -f 1)
    jq -n --arg url http://localhost:8080 --arg run "$RUN" --arg prefix "tflive-$(date -u '+%Y%m%d%H%M%S')-$$" --arg commit "$(git -C "$ROOT" rev-parse HEAD)" --arg branch "$(git -C "$ROOT" branch --show-current)" --arg binary_sha256 "$sha" --arg provider_source_sha256 "$source_after" --arg terraform "$(terraform version -json | jq -r .terraform_version)" '{url:$url,run:$run,prefix:$prefix,commit:$commit,branch:$branch,binary_sha256:$binary_sha256,provider_source_sha256:$provider_source_sha256,terraform:$terraform,destruction:"MANUAL HOLD"}' > "$RUN/manifest.json"
    printf 'provider_installation {\n dev_overrides {\n "adaptive-scale/adaptive" = "%s/bin"\n }\n direct {}\n}\n' "$RUN" > "$RUN/terraform.rc"
    cp "$ROOT/testing/live/destroy.sh" "$ROOT/testing/live/destroy-infra.sh" "$ROOT/testing/live/destroy-orphans.sh" "$ROOT/testing/live/destroy-generated.sh" "$ROOT/testing/live/orphan_plan.py" "$ROOT/testing/live/guard.py" "$RUN/"
    printf '%s\n' "$ROOT" > "$RUN/repository-path"
    git -C "$ROOT" status --short > "$RUN/inventory/initial-git-status.txt"
    mkdir -p "$RUN/work/schema"
    printf '%s\n' '{"terraform":{"required_providers":{"adaptive":{"source":"adaptive-scale/adaptive"}}}}' > "$RUN/work/schema/main.tf.json"
    TF_CLI_CONFIG_FILE="$RUN/terraform.rc" CHECKPOINT_DISABLE=1 terraform -chdir="$RUN/work/schema" providers schema -json > "$RUN/inventory/schema.json"
    jq '.provider_schemas["registry.terraform.io/adaptive-scale/adaptive"].resource_schemas | to_entries | map({family:.key,attributes:(.value.block.attributes | to_entries | map({name:.key,required:(.value.required//false),optional:(.value.optional//false),computed:(.value.computed//false),sensitive:(.value.sensitive//false)})),nested_blocks:((.value.block.block_types//{})|keys)})' "$RUN/inventory/schema.json" > "$RUN/inventory/families.json"
    test "$(jq length "$RUN/inventory/families.json")" = 7
    printf '%s\n' "$RUN"
    ;;
  fixture)
    check_run
    test "$#" = 5 || die 'fixture RUN CASE FAMILY ATTRIBUTES_JSON'
    make_fixture "$3" "$4" "$5"
    ;;
  tf)
    check_run
    test "$#" -ge 6 || die 'tf RUN CASE PHASE EXPECTED_EXIT TERRAFORM_SUBCOMMAND [args]'
    fixture=$3; phase=$4; expected=$5; shift 5
    case "$1" in destroy|state) die 'Use explicit destroy.sh; state surgery requires a separately reviewed command.' ;; esac
    run_tf "$fixture" "$phase" "$expected" "$@"
    ;;
  readback)
    check_run
    test "$#" = 5 || die 'readback RUN CASE EXPORT_KIND EXACT_NAME'
    fixture=$3; kind=$4; name=$5
    prefix=$(jq -r .prefix "$RUN/manifest.json")
    case "$name" in "$prefix"-*) ;; *) die 'Readback name must be run-owned.' ;; esac
    log="$RUN/evidence/$fixture-readback-$(date -u '+%Y%m%dT%H%M%S')-$RANDOM.yaml"
    set +e
    adaptive admin export "$kind" --name "$name" > "$log" 2>&1
    rc=$?
    set -e
    status=FAIL
    if test "$rc" = 0 && test -s "$log" && /usr/bin/grep -Fq "$name" "$log"; then status=PASS; fi
    ledger "$fixture" readback "$status" "$log" "$rc"
    printf '%s readback %s\n' "$fixture" "$status"
    test "$status" = PASS
    ;;
  baseline)
    check_run
    log="$RUN/evidence/make-test-build-$(date -u '+%Y%m%dT%H%M%SZ')-$$.txt"
    set +e
    (cd "$ROOT" && make test && make build) > "$log" 2>&1
    rc=$?
    set -e
    status=FAIL; test "$rc" = 0 && status=PASS
    ledger baseline make-test-build "$status" "$log" "$rc"
    test "$rc" = 0
    ;;
  report)
    check_run
    jq -s '{records:length,by_status:(group_by(.status)|map({status:.[0].status,count:length})),failures:map(select(.status!="PASS"))}' "$RUN/ledger.jsonl" > "$RUN/report.json"
    jq . "$RUN/report.json"
    ;;
  *)
    printf '%s\n' 'Usage: bash testing/live/run.sh init | baseline RUN | fixture RUN CASE FAMILY ATTRS_JSON | tf RUN CASE PHASE EXPECTED_EXIT COMMAND [args] | readback RUN CASE EXPORT_KIND EXACT_NAME | report RUN' 'Fixtures are retained. Cleanup is only through the explicit destroy.sh.'
    ;;
esac
