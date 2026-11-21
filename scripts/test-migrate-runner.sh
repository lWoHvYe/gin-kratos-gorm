#!/usr/bin/env bash
# Exercise the migration runner's lease and attempt ledger with disposable
# MySQL. Requires Docker and a built mall/migrate:dev image.
set -euo pipefail
IMAGE="${MIGRATE_IMAGE:-mall/migrate:dev}"
MYSQL_IMAGE="${MYSQL_IMAGE:-mysql:8.4}"
PORT="${BASE_PORT:-13316}"
NAME="mall-migrate-runner-test-$$"
TMP="$(mktemp -d)"
cleanup() { docker rm -f "$NAME" >/dev/null 2>&1 || true; rm -rf "$TMP"; }
trap cleanup EXIT
docker image inspect "$IMAGE" >/dev/null 2>&1 || { echo "missing $IMAGE" >&2; exit 1; }

docker run -d --rm --name "$NAME" -e MYSQL_ROOT_PASSWORD=root \
  -p "127.0.0.1:$PORT:3306" "$MYSQL_IMAGE" >/dev/null
for _ in $(seq 1 90); do
  docker exec "$NAME" mysql -uroot -proot -e 'SELECT 1' >/dev/null 2>&1 && break
  sleep 1
done
docker exec "$NAME" mysql -uroot -proot -e 'SELECT 1' >/dev/null

mkdir -p "$TMP/order"
cat > "$TMP/order/0001_ok.sql" <<'SQL'
CREATE TABLE IF NOT EXISTS runner_smoke (id BIGINT PRIMARY KEY);
SQL
cat > "$TMP/order/0002_fail.sql" <<'SQL'
CREATE TABLE IF NOT EXISTS runner_partial (id BIGINT PRIMARY KEY);
THIS IS NOT VALID SQL;
SQL

run_migrate() {
  docker run --rm --add-host=host.docker.internal:host-gateway \
    -e MYSQL_HOST=host.docker.internal -e MYSQL_PORT="$PORT" \
    -e MYSQL_USER=root -e MYSQL_PASSWORD=root \
    -e MIGRATION_LOCK_WAIT_SECONDS=20 \
    -v "$TMP:/migrations:ro" "$IMAGE" "$@"
}
q() { docker exec "$NAME" mysql -uroot -proot -N -B mall_order -e "$1"; }

echo "== first run must fail and mark FAILED =="
if run_migrate >"$TMP/first.log" 2>&1; then
  echo "expected migration failure" >&2; exit 1
fi
assert() { [ "$1" = "$2" ] || { echo "expected $3=$2, got $1" >&2; exit 1; }; }
assert "$(q "SELECT status FROM schema_migration_attempts WHERE filename='order/0002_fail.sql'")" FAILED "attempt status"
assert "$(q "SELECT COUNT(*) FROM schema_migrations WHERE filename='order/0002_fail.sql'")" 0 "failed ledger"
assert "$(q "SELECT COUNT(*) FROM schema_migrations WHERE filename='order/0001_ok.sql'")" 1 "first ledger"

echo "== second run must fail closed, not replay 0002 =="
if run_migrate >"$TMP/second.log" 2>&1; then
  echo "expected failed-attempt refusal" >&2; exit 1
fi
grep -q "previous FAILED attempt" "$TMP/second.log"

echo "== operator repair: remove failed attempt and apply a fixed 0002 =="
docker exec "$NAME" mysql -uroot -proot mall_order -e "DELETE FROM schema_migration_attempts WHERE filename='order/0002_fail.sql'"
rm "$TMP/order/0002_fail.sql"
cat > "$TMP/order/0002_ok.sql" <<'SQL'
CREATE TABLE IF NOT EXISTS runner_fixed (id BIGINT PRIMARY KEY);
SQL
run_migrate >"$TMP/third.log" 2>&1
assert "$(q "SELECT status FROM schema_migration_attempts WHERE filename='order/0002_ok.sql'")" APPLIED "fixed attempt"
assert "$(q "SELECT COUNT(*) FROM schema_migrations")" 2 "final ledger"

echo "== idempotent replay =="
run_migrate >"$TMP/fourth.log" 2>&1
grep -q "already applied" "$TMP/fourth.log"

echo "== concurrent runners share one lease =="
cat > "$TMP/order/0003_slow.sql" <<'SQL'
SELECT SLEEP(3);
SQL
run_migrate >"$TMP/conc1.log" 2>&1 & P1=$!
run_migrate >"$TMP/conc2.log" 2>&1 & P2=$!
set +e
wait "$P1"; R1=$?
wait "$P2"; R2=$?
set -e
[ "$R1" = 0 ] && [ "$R2" = 0 ] || { echo "concurrent migration failed: $R1/$R2" >&2; cat "$TMP/conc1.log" "$TMP/conc2.log"; exit 1; }
grep -q "waiting for migration lease" "$TMP/conc1.log" "$TMP/conc2.log"
assert "$(q "SELECT COUNT(*) FROM schema_migrations WHERE filename='order/0003_slow.sql'")" 1 "slow ledger"

echo "== stale lease is taken over =="
docker exec "$NAME" mysql -uroot -proot -e "INSERT INTO mall_migrate.schema_migration_lock (name,owner,expires_at)
  VALUES ('mall_migrate','stale',DATE_SUB(NOW(3), INTERVAL 1 MINUTE))
  ON DUPLICATE KEY UPDATE owner='stale', expires_at=DATE_SUB(NOW(3), INTERVAL 1 MINUTE)"
run_migrate >"$TMP/stale.log" 2>&1
grep -q "complete" "$TMP/stale.log"

echo "migration runner test passed"
