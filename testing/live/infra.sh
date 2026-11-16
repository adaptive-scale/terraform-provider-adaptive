#!/bin/bash
# Reusable owned PostgreSQL target on the already-existing local kind network.
set -euo pipefail
umask 077
RUN=${1:?absolute isolated run directory required}
PREFIX=$(jq -er .prefix "$RUN/manifest.json")
NAME="$PREFIX-runtime-postgres-db"
jq -e '.url=="http://localhost:8080"' "$RUN/manifest.json" >/dev/null
adaptive whoami -o json | jq -e '.URL=="http://localhost:8080"' >/dev/null
kubectl --context kind-endpoint-cluster get node endpoint-cluster-control-plane -o json | jq -e 'any(.status.conditions[]; .type=="Ready" and .status=="True")' >/dev/null
mkdir -p "$RUN/infra"
# This is an intentionally synthetic fixture password, never an existing secret.
if test ! -f "$RUN/infra/postgres.env"; then
  printf '%s\n' POSTGRES_USER=tf_live POSTGRES_DB=tf_live_test POSTGRES_PASSWORD=tf-live-runtime-synthetic-password > "$RUN/infra/postgres.env"
fi
if docker container inspect "$NAME" >/dev/null 2>&1; then
  test "$(docker inspect "$NAME" --format '{{ index .Config.Labels "adaptive.terraform.run" }}')" = "$PREFIX"
else
  # Record cleanup identity before the side effect so an interrupted start is recoverable.
  jq -n --arg n "$NAME" --arg p "$PREFIX" '{docker_containers:[$n],ownership_label:$p,network:"kind",cleanup:"only after explicit user validation"}' > "$RUN/infra/ownership.json"
  docker run --detach --pull=never --name "$NAME" --network kind --label "adaptive.terraform.run=$PREFIX" --env-file "$RUN/infra/postgres.env" --health-cmd='pg_isready -U tf_live -d tf_live_test' --health-interval=2s --health-retries=30 postgres:16-alpine > "$RUN/infra/postgres-container.id"
fi
for attempt in $(seq 1 30); do
  status=$(docker inspect "$NAME" --format '{{.State.Health.Status}}')
  test "$status" != unhealthy
  test "$status" = healthy && break
  sleep 2
done
test "$(docker inspect "$NAME" --format '{{.State.Health.Status}}')" = healthy
IP=$(docker inspect "$NAME" --format '{{(index .NetworkSettings.Networks "kind").IPAddress}}')
jq -n --arg ip "$IP" --arg n "$NAME" '{host:$ip,port:"5432",username:"tf_live",database:"tf_live_test",container:$n}' > "$RUN/infra/postgres-target.json"
# This changes only the new test database, never the platform database.
docker exec -i "$NAME" psql -v ON_ERROR_STOP=1 -U tf_live -d tf_live_test > "$RUN/evidence/seed-postgres.txt" <<'SQL'
CREATE TABLE IF NOT EXISTS public.customers (id integer PRIMARY KEY, name text, email text, ssn text, phone text, card_number text, aadhar text);
INSERT INTO public.customers VALUES (1, 'Synthetic Person', 'fixture@example.invalid', '111-22-3333', '5551234567', '4111111111111111', '123456789012') ON CONFLICT (id) DO NOTHING;
CREATE TABLE IF NOT EXISTS public.out_of_scope (id integer PRIMARY KEY, marker text);
INSERT INTO public.out_of_scope VALUES (1, 'synthetic-private-marker') ON CONFLICT (id) DO NOTHING;
SELECT count(*) AS fixture_rows FROM public.customers;
SQL
printf 'Owned PostgreSQL fixture healthy; metadata=%s\n' "$RUN/infra/postgres-target.json"
