#!/bin/bash
set -u
umask 077
base=$(cd "$(dirname "$0")" && pwd)
work=$(mktemp -d "${TMPDIR:-$HOME}/adaptive-shell-tests.XXXXXX") || exit 1
trap 'rm -rf "$work"' EXIT
mkdir "$work/bin"
# Local process fixtures only; never customer acceptance evidence.
cat > "$work/bin/adaptive" <<'FIXTURE'
#!/bin/bash
printf '%s\n' "$$" > "$FIXTURE_DIR/connect.pid"
if [ "$FIXTURE_MODE" = require_tty ] && ! [ -t 1 ]; then exit 96; fi
if [ "$FIXTURE_MODE" = auto ] && [ "$#" != 2 ]; then exit 95; fi
printf 'user = fixture_user\npassword = FIXTURE_PASSWORD\ndbname = fixture_db\n?application_name=fixture_application\n=> started local service at port 15432\n'
if [ "$FIXTURE_MODE" != no_listener ]; then : > "$FIXTURE_DIR/ready"; fi
exec sleep 30
FIXTURE
cat > "$work/bin/nc" <<'FIXTURE'
#!/bin/bash
: > "$FIXTURE_DIR/active-probe-called"
exit 99
FIXTURE
cat > "$work/bin/psql" <<'FIXTURE'
#!/bin/bash
[ -f "$FIXTURE_DIR/ready" ] || exit 97
[ "$PGPASSWORD" = FIXTURE_PASSWORD ] || exit 98
[ "${PGAPPNAME:-}" = fixture_application ] || exit 96
if [ -n "${PGOPTIONS:-}" ]; then
  printf 'psql: FATAL: unsupported startup parameter: options\n' >&2
  exit 2
fi
case "$*" in *FIXTURE_PASSWORD*) exit 99;; esac
: > "$FIXTURE_DIR/psql-called"
printf '%s\n' "$$" > "$FIXTURE_DIR/query.pid"
case "$FIXTURE_MODE" in
  query_timeout) exec sleep 30;;
  deny) printf 'ERROR:  42501\n' >&2; exit 3;;
  wrong_error) printf 'ERROR:  42601\n' >&2; exit 3;;
  proxy_redacted) printf 'E2E_MASK_BEGIN\n[REDACTED]\nE2E_MASK_END\n'; exit 0;;
  proxy_unmasked) printf 'E2E_MASK_BEGIN\nRAW_FIXTURE_EMAIL@example.invalid\nE2E_MASK_END\n'; exit 0;;
  proxy_empty) printf 'E2E_MASK_BEGIN\nE2E_MASK_END\n'; exit 0;;
  proxy_empty_second) printf 'E2E_MASK_BEGIN\n[REDACTED]\nE2E_MASK_END\nE2E_MASK_BEGIN\nE2E_MASK_END\n'; exit 0;;
  proxy_policy_ok) printf 'E2E_DENY_BEGIN\nERROR:  42501\nE2E_DENY_END\n'; exit 0;;
  proxy_policy_allow) printf 'E2E_DENY_BEGIN\nE2E_DENY_END\n'; exit 0;;
  proxy_policy_wrong) printf 'E2E_DENY_BEGIN\nERROR:  42601\nE2E_DENY_END\n'; exit 0;;
  proxy_policy_read_denied) printf 'ERROR:  42501\nE2E_DENY_BEGIN\nERROR:  42501\nE2E_DENY_END\n'; exit 0;;
esac
printf 'DO_NOT_PERSIST_DATABASE_VALUES\n'
FIXTURE
cat > "$work/bin/lsof" <<'FIXTURE'
#!/bin/bash
if [ "$FIXTURE_MODE" = busy ] || [ -f "$FIXTURE_DIR/ready" ]; then printf '1\n'; exit 0; fi
exit 1
FIXTURE
chmod 700 "$work/bin/"*
printf 'SELECT 1;\n' > "$work/check.sql"
passed=0
failed=0
run_case() {
  name=$1; mode=$2; expected=$3; want=$4; port=${5:-15432}; assertion=${6:-none}
  mkdir "$work/$name"
  args=(fixture "$work/check.sql" "$port" "$work/$name/report.tsv" "$expected")
  if [ "$assertion" != none ]; then args+=("$assertion"); fi
  FIXTURE_DIR="$work/$name" FIXTURE_MODE="$mode" PATH="$work/bin:$PATH" \
  ADAPTIVE_BIN="$work/bin/adaptive" PSQL_BIN="$work/bin/psql" CONNECT_TIMEOUT=2 QUERY_TIMEOUT=2 \
    /bin/bash "$base/check-postgres-client.sh" "${args[@]}" \
    > "$work/$name/output" 2>&1
  code=$?
  good=true
  if [ "$want" = pass ]; then [ "$code" = 0 ] || good=false
  else [ "$code" != 0 ] || good=false; fi
  if [ -f "$work/$name/report.tsv" ]; then
    while IFS= read -r line; do
      case "$line" in *FIXTURE_PASSWORD*|*DO_NOT_PERSIST_DATABASE_VALUES*|*RAW_FIXTURE_EMAIL*) good=false;; esac
    done < "$work/$name/report.tsv"
    IFS=$'\t' read -r e phase status state cleanup seconds reason < "$work/$name/report.tsv"
    [ "$cleanup" = OK ] || good=false
    case "$mode" in
      allow|require_tty|auto) [ "$phase:$status:$state" = query:PASS:- ] || good=false;;
      deny) [ "$phase:$status:$state" = query:PASS:42501 ] || good=false;;
      wrong_error) [ "$phase:$status:$state:$reason" = query:FAIL:42601:expected_denial_not_observed ] || good=false;;
      no_listener) [ "$phase:$reason" = forward_readiness:local_listener_timeout ] || good=false;;
      busy) [ "$phase:$reason" = preflight:port_already_in_use ] || good=false;;
      query_timeout) [ "$phase:$reason" = query:query_timeout ] || good=false;;
      proxy_redacted) [ "$phase:$status:$reason" = query:PASS:proxy_redaction_verified ] || good=false;;
      proxy_unmasked) [ "$phase:$status:$reason" = query:FAIL:mask_not_applied ] || good=false;;
      proxy_empty|proxy_empty_second) [ "$phase:$status:$reason" = query:FAIL:empty_mask_sample ] || good=false;;
      proxy_policy_ok) [ "$phase:$status:$reason" = query:PASS:proxy_policy_verified ] || good=false;;
      proxy_policy_allow) [ "$phase:$status:$reason" = query:FAIL:expected_denial_not_observed ] || good=false;;
      proxy_policy_wrong) [ "$phase:$status:$state:$reason" = query:FAIL:42601:unexpected_sqlstate ] || good=false;;
      proxy_policy_read_denied) [ "$phase:$status:$state:$reason" = query:FAIL:42501:allowed_query_denied ] || good=false;;
    esac
  else good=false; fi
  for child in connect query; do
    if [ -f "$work/$name/$child.pid" ]; then
      read -r pid < "$work/$name/$child.pid"
      if kill -0 "$pid" 2>/dev/null; then good=false; kill -TERM "$pid" 2>/dev/null || true; fi
    fi
  done
  [ ! -e "$work/$name/active-probe-called" ] || good=false
  while IFS= read -r line; do
    case "$line" in *FIXTURE_PASSWORD*|*RAW_FIXTURE_EMAIL*|*DO_NOT_PERSIST_DATABASE_VALUES*) good=false;; esac
  done < "$work/$name/output"
  if [ "$mode" = no_listener ] && [ -f "$work/$name/psql-called" ]; then good=false; fi
  if [ "$good" = true ]; then printf 'PASS %s\n' "$name"; passed=$((passed+1))
  else
    printf 'FAIL %s\n' "$name"; failed=$((failed+1))
    if [ -r "$work/$name/report.tsv" ]; then
      printf '  phase=%s result=%s SQLSTATE=%s cleanup=%s reason=%s\n' "$phase" "$status" "$state" "$cleanup" "$reason"
    fi
  fi
}
run_case allowed allow - pass
run_case banner_not_ready no_listener - fail
run_case expected_denial deny 42501 pass
run_case wrong_error wrong_error 42501 fail
run_case occupied_port busy - fail
run_case connection_stdout_is_tty require_tty - pass
run_case default_command_auto_port auto - pass auto
run_case query_timeout query_timeout - fail
run_case proxy_redacted proxy_redacted - pass 15432 proxy-redact
run_case proxy_unmasked proxy_unmasked - fail 15432 proxy-redact
run_case proxy_empty proxy_empty - fail 15432 proxy-redact
run_case proxy_empty_second proxy_empty_second - fail 15432 proxy-redact
run_case proxy_policy_ok proxy_policy_ok - pass 15432 proxy-policy
run_case proxy_policy_allow proxy_policy_allow - fail 15432 proxy-policy
run_case proxy_policy_wrong proxy_policy_wrong - fail 15432 proxy-policy
run_case proxy_policy_read_denied proxy_policy_read_denied - fail 15432 proxy-policy
printf 'Offline fixtures: %s passed, %s failed\n' "$passed" "$failed"
[ "$failed" = 0 ]
