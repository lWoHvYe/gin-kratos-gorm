#!/usr/bin/env bash
# R0.1 real-migration verification for PAY-SETTLE-01 Phase 0-R.
#
# Proves, on a disposable fresh MySQL 8.4 instance and with the same
# mall/migrate image the Helm job uses:
#   * the real migrations/order/0017..0022 + migrations/fulfillment/0010..0011
#     files apply from scratch and are recorded in the checksum ledger;
#   * order/0018 can be replayed after a successful run and after an interrupted
#     run (atomic ALTER committed, later statements missing) and converges;
#   * fulfillment/0011 enforces chk_switch_settled_expiry, is re-runnable, and
#     fails closed on an existing settled row with NULL expires_at.
#
# Nothing here is inline replacement DDL: every schema change is read from the
# real migration files. The scratch databases are only used to re-create the
# pre-migration state for the re-entry/CHECK scenarios.
#
# Usage: ./scripts/verify-r01-migrations.sh
#   MYSQL_IMAGE=mysql:8.4 MIGRATE_IMAGE=mall/migrate:dev BASE_PORT=13319
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MYSQL_IMAGE="${MYSQL_IMAGE:-mysql:8.4}"
MIGRATE_IMAGE="${MIGRATE_IMAGE:-mall/migrate:dev}"
BASE_PORT="${BASE_PORT:-13319}"
NAME="mall-r01-verify-$$"
TMP_MIGRATIONS=""

MIGRATION_FILES=(
  "order/0017_refund_batch_ledger.sql"
  "order/0018_refund_commitment_composition.sql"
  "order/0019_legacy_payment_resolution.sql"
  "order/0020_permit_state_check.sql"
  "order/0021_zero_application_check.sql"
  "order/0022_r04_checks.sql"
  "fulfillment/0010_settlement_shadow_switch.sql"
  "fulfillment/0011_switch_expiry_check.sql"
)

cleanup() {
  docker rm -f "$NAME" >/dev/null 2>&1 || true
  [ -n "$TMP_MIGRATIONS" ] && rm -rf "$TMP_MIGRATIONS"
}
trap cleanup EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }
assert_eq() { # label expected actual
  if [ "$2" != "$3" ]; then
    fail "$1: expected [$2], got [$3]"
  fi
  echo "OK   $1 = $3"
}
assert_error() { # label expected-fragment command...
  local label="$1" want="$2"; shift 2
  local out
  if out="$("$@" 2>&1)"; then
    fail "$label: expected an error containing [$want], command succeeded"
  fi
  case "$out" in
    *"$want"*) echo "OK   $label (rejected: $want)";;
    *) fail "$label: error did not contain [$want]: $out";;
  esac
}
sha() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d' ' -f1
  else shasum -a 256 "$1" | cut -d' ' -f1
  fi
}
check_clause() { # database table constraint -> lowercased CHECK_CLAUSE
  q "$1" "SELECT LOWER(cc.CHECK_CLAUSE)
    FROM information_schema.CHECK_CONSTRAINTS cc
    JOIN information_schema.TABLE_CONSTRAINTS tc
      ON tc.CONSTRAINT_SCHEMA = cc.CONSTRAINT_SCHEMA
     AND tc.CONSTRAINT_NAME = cc.CONSTRAINT_NAME
    WHERE cc.CONSTRAINT_SCHEMA = '$1' AND tc.TABLE_NAME = '$2' AND cc.CONSTRAINT_NAME = '$3'"
}
assert_clause_has() { # label database table constraint fragment
  local label="$1" db="$2" table="$3" name="$4" want="$5" clause
  clause="$(check_clause "$db" "$table" "$name")"
  case "$clause" in
    *"$want"*) echo "OK   $label (CHECK_CLAUSE contains $want)";;
    *) fail "$label: CHECK_CLAUSE [$clause] does not contain [$want]";;
  esac
}
assert_clause_lacks() { # label database table constraint fragment
  local label="$1" db="$2" table="$3" name="$4" unwanted="$5" clause
  clause="$(check_clause "$db" "$table" "$name")"
  case "$clause" in
    *"$unwanted"*) fail "$label: CHECK_CLAUSE [$clause] must not contain [$unwanted]";;
    *) echo "OK   $label (CHECK_CLAUSE omits $unwanted)";;
  esac
}
mysql_exec() { docker exec -e MYSQL_PWD=root "$NAME" mysql -uroot "$@"; }
mysql_file() { # database file
  docker exec -i -e MYSQL_PWD=root "$NAME" mysql -uroot "$1" < "$2"
}
q() { # database sql (empty database = no default schema)
  if [ -n "$1" ]; then mysql_exec -N -B "$1" -e "$2"
  else mysql_exec -N -B -e "$2"; fi
}
count() { q "$1" "$2"; }

need() { command -v "$1" >/dev/null || fail "missing $1"; }
need docker
git -C "$ROOT" rev-parse --verify HEAD >/dev/null
docker image inspect "$MYSQL_IMAGE" >/dev/null 2>&1 || fail "missing $MYSQL_IMAGE"
docker image inspect "$MIGRATE_IMAGE" >/dev/null 2>&1 || fail "missing $MIGRATE_IMAGE"

echo "== R0.1 HEAD binding =="
echo "HEAD             $(git -C "$ROOT" rev-parse HEAD)"
echo "MYSQL_IMAGE      $MYSQL_IMAGE"
echo "MIGRATE_IMAGE    $MIGRATE_IMAGE"
if [ -n "$(git -C "$ROOT" status --porcelain)" ]; then
  echo "WORKTREE         dirty (WIP evidence; commit binds HEAD)"
  git -C "$ROOT" status --porcelain
else
  echo "WORKTREE         clean"
fi
echo "== migration checksums (sha256 of the files under test) =="
for f in "${MIGRATION_FILES[@]}"; do
  printf '%s  %s\n' "$(sha "$ROOT/migrations/$f")" "$f"
done

echo "== start disposable MySQL 8.4 on 127.0.0.1:$BASE_PORT =="
docker run -d --rm --name "$NAME" -e MYSQL_ROOT_PASSWORD=root \
  -p "127.0.0.1:$BASE_PORT:3306" "$MYSQL_IMAGE" >/dev/null
for _ in $(seq 1 90); do
  mysql_exec -e 'SELECT 1' >/dev/null 2>&1 && break
  sleep 1
done
mysql_exec -e 'SELECT 1' >/dev/null 2>&1 || { docker logs "$NAME" | tail -30 >&2; fail "MySQL did not become ready"; }
echo "MySQL version: $(q '' 'SELECT VERSION()')"

echo
echo "== fresh apply through the real mall/migrate runner =="
docker run --rm --add-host=host.docker.internal:host-gateway \
  -e MYSQL_HOST=host.docker.internal -e MYSQL_PORT="$BASE_PORT" \
  -e MYSQL_USER=root -e MYSQL_PASSWORD=root \
  -e MIGRATION_LOCK_WAIT_SECONDS=60 \
  -v "$ROOT/migrations:/migrations:ro" "$MIGRATE_IMAGE"

echo
echo "== ledger checksum binding (order + fulfillment) =="
for f in "${MIGRATION_FILES[@]}"; do
  case "$f" in
    order/*) db=mall_order;;
    fulfillment/*) db=mall_fulfillment;;
  esac
  assert_eq "ledger $f" "$(sha "$ROOT/migrations/$f")" "$(q "$db" "SELECT checksum FROM schema_migrations WHERE filename='$f'")"
done

echo
echo "== 0017-0022 + fulfillment/0010-0011 objects =="
assert_eq "0017 refund_batches" 1 "$(count mall_order "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='mall_order' AND table_name='refund_batches'")"
assert_eq "0017 refund_applications" 1 "$(count mall_order "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='mall_order' AND table_name='refund_applications'")"
assert_eq "0018 header columns" 4 "$(count mall_order "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='mall_order' AND table_name='order_refund_commitments' AND column_name IN ('goods_cent','freight_cent','composition_known','composition_source')")"
assert_eq "0018 commitment lines" 1 "$(count mall_order "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='mall_order' AND table_name='order_refund_commitment_lines'")"
assert_eq "0018 releases" 1 "$(count mall_order "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='mall_order' AND table_name='refund_commitment_releases'")"
assert_eq "0019 legacy_payment_resolution" 1 "$(count mall_order "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='mall_order' AND table_name='legacy_payment_resolution'")"
assert_eq "0019 order_refund_sequences" 1 "$(count mall_order "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='mall_order' AND table_name='order_refund_sequences'")"
assert_eq "0020 chk_permit_state" 1 "$(count mall_order "SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA='mall_order' AND TABLE_NAME='order_shipping_permits' AND CONSTRAINT_NAME='chk_permit_state' AND CONSTRAINT_TYPE='CHECK'")"
assert_eq "0021 chk_refund_app_system_zero" 1 "$(count mall_order "SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA='mall_order' AND TABLE_NAME='refund_applications' AND CONSTRAINT_NAME='chk_refund_app_system_zero' AND CONSTRAINT_TYPE='CHECK'")"
assert_clause_has "0021 zero-amount semantics" mall_order refund_applications chk_refund_app_system_zero "system_zero"
assert_clause_has "0021 zero-amount semantics" mall_order refund_applications chk_refund_app_system_zero "amount_cent"
assert_eq "0022 chk_refund_app_group_discriminator" 1 "$(count mall_order "SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA='mall_order' AND TABLE_NAME='refund_applications' AND CONSTRAINT_NAME='chk_refund_app_group_discriminator' AND CONSTRAINT_TYPE='CHECK'")"
assert_eq "0022 chk_lpr_expiry_24h" 1 "$(count mall_order "SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA='mall_order' AND TABLE_NAME='legacy_payment_resolution' AND CONSTRAINT_NAME='chk_lpr_expiry_24h' AND CONSTRAINT_TYPE='CHECK'")"
# Name equality is not semantics: read CHECK_CLAUSE. The discriminator must not
# copy the 0017 source enum, and the 24h window must be equality, not a bound.
assert_clause_has "0022 discriminator NULL check" mall_order refund_applications chk_refund_app_group_discriminator "group_no"
assert_clause_has "0022 discriminator NULL check" mall_order refund_applications chk_refund_app_group_discriminator "is null"
assert_clause_lacks "0022 discriminator no enum copy" mall_order refund_applications chk_refund_app_group_discriminator "system_zero"
assert_clause_lacks "0022 discriminator no enum copy" mall_order refund_applications chk_refund_app_group_discriminator "system_shortage"
assert_clause_lacks "0022 discriminator no enum copy" mall_order refund_applications chk_refund_app_group_discriminator "aftersale"
assert_clause_lacks "0022 discriminator no enum copy" mall_order refund_applications chk_refund_app_group_discriminator "tcc"
assert_clause_has "0022 24h equality" mall_order legacy_payment_resolution chk_lpr_expiry_24h "interval 24 hour"
assert_clause_lacks "0022 24h equality" mall_order legacy_payment_resolution chk_lpr_expiry_24h ">="
assert_eq "fulfillment/0010 shadow diffs" 1 "$(count mall_fulfillment "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='mall_fulfillment' AND table_name='settlement_shadow_diffs'")"
assert_eq "fulfillment/0010 switch approvals" 1 "$(count mall_fulfillment "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='mall_fulfillment' AND table_name='settlement_switch_approvals'")"
assert_eq "fulfillment/0011 chk_switch_settled_expiry" 1 "$(count mall_fulfillment "SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA='mall_fulfillment' AND TABLE_NAME='settlement_switch_approvals' AND CONSTRAINT_NAME='chk_switch_settled_expiry' AND CONSTRAINT_TYPE='CHECK'")"

echo
echo "== 0020 permits fail closed on GRANTED =="
assert_error "0020 rejects GRANTED" "3819" mysql_exec mall_order -e \
  "INSERT INTO order_shipping_permits (order_no, fulfillment_group_no, state) VALUES ('ORD-R01-PERMIT','G1','GRANTED')"
mysql_exec mall_order -e \
  "INSERT INTO order_shipping_permits (order_no, fulfillment_group_no, state) VALUES ('ORD-R01-PERMIT','G1','NONE')" >/dev/null
assert_eq "0020 accepts NONE" 1 "$(count mall_order "SELECT COUNT(*) FROM order_shipping_permits WHERE order_no='ORD-R01-PERMIT' AND state='NONE'")"

echo
echo "== 0021/0022 fail closed on dirty values, accept the exact boundaries =="
assert_error "0021 rejects nonzero system_zero" "3819" mysql_exec mall_order -e \
  "INSERT INTO refund_applications (refund_id, source, order_no, group_no, non_effective_capture, amount_cent, goods_cent, freight_cent)
   VALUES ('APP-R04-ZERO-BAD','system_zero','ORD-R04-ZERO','G1',0,100,100,0)"
mysql_exec mall_order -e \
  "INSERT INTO refund_applications (refund_id, source, order_no, group_no, non_effective_capture, amount_cent, goods_cent, freight_cent)
   VALUES ('APP-R04-ZERO-OK','system_zero','ORD-R04-ZERO','G1',0,0,0,0)" >/dev/null
assert_eq "0021 accepts zero system_zero" 1 "$(count mall_order "SELECT COUNT(*) FROM refund_applications WHERE refund_id='APP-R04-ZERO-OK'")"
assert_error "0022 rejects payment with group" "3819" mysql_exec mall_order -e \
  "INSERT INTO refund_applications (refund_id, source, order_no, group_no, non_effective_capture, amount_cent, goods_cent, freight_cent)
   VALUES ('APP-R04-PAY-BAD','payment','ORD-R04-PAY','G1',1,100,100,0)"
mysql_exec mall_order -e \
  "INSERT INTO refund_applications (refund_id, source, order_no, group_no, non_effective_capture, amount_cent, goods_cent, freight_cent)
   VALUES ('APP-R04-PAY-OK','payment','ORD-R04-PAY',NULL,1,100,100,0)" >/dev/null
assert_eq "0022 accepts payment-level row" 1 "$(count mall_order "SELECT COUNT(*) FROM refund_applications WHERE refund_id='APP-R04-PAY-OK'")"
assert_error "0022 rejects group row without group" "3819" mysql_exec mall_order -e \
  "INSERT INTO refund_applications (refund_id, source, order_no, group_no, non_effective_capture, amount_cent, goods_cent, freight_cent)
   VALUES ('APP-R04-GROUP-BAD','aftersale','ORD-R04-GROUP',NULL,0,100,90,10)"
mysql_exec mall_order -e \
  "INSERT INTO refund_applications (refund_id, source, order_no, group_no, non_effective_capture, amount_cent, goods_cent, freight_cent)
   VALUES ('APP-R04-GROUP-OK','aftersale','ORD-R04-GROUP','G1',0,100,90,10)" >/dev/null
assert_eq "0022 accepts group row" 1 "$(count mall_order "SELECT COUNT(*) FROM refund_applications WHERE refund_id='APP-R04-GROUP-OK'")"
mysql_exec mall_order -e \
  "INSERT INTO legacy_payment_resolution (order_no, state, entered_at, expires_at)
   VALUES ('ORD-R04-LPR-24H','OPEN',NOW(3),DATE_ADD(NOW(3),INTERVAL 24 HOUR))" >/dev/null
assert_eq "0022 accepts exactly 24h" 1 "$(count mall_order "SELECT COUNT(*) FROM legacy_payment_resolution WHERE order_no='ORD-R04-LPR-24H'")"
assert_error "0022 rejects 48h" "3819" mysql_exec mall_order -e \
  "INSERT INTO legacy_payment_resolution (order_no, state, entered_at, expires_at)
   VALUES ('ORD-R04-LPR-48H','OPEN',NOW(3),DATE_ADD(NOW(3),INTERVAL 48 HOUR))"
assert_error "0022 rejects 24h+1s" "3819" mysql_exec mall_order -e \
  "INSERT INTO legacy_payment_resolution (order_no, state, entered_at, expires_at)
   VALUES ('ORD-R04-LPR-24H1S','OPEN',NOW(3),DATE_ADD(NOW(3),INTERVAL 24 HOUR) + INTERVAL 1 SECOND)"
assert_error "0022 rejects 25h" "3819" mysql_exec mall_order -e \
  "INSERT INTO legacy_payment_resolution (order_no, state, entered_at, expires_at)
   VALUES ('ORD-R04-LPR-25H','OPEN',NOW(3),DATE_ADD(NOW(3),INTERVAL 25 HOUR))"
assert_error "0022 rejects 23h" "chk_lpr_expiry" mysql_exec mall_order -e \
  "INSERT INTO legacy_payment_resolution (order_no, state, entered_at, expires_at)
   VALUES ('ORD-R04-LPR-23H','OPEN',NOW(3),DATE_ADD(NOW(3),INTERVAL 23 HOUR))"
# Replay the real 0022 file: conditional ADD must not fail or duplicate.
mysql_file mall_order "$ROOT/migrations/order/0022_r04_checks.sql" >/dev/null
assert_eq "0022 replay keeps one discriminator" 1 "$(count mall_order "SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA='mall_order' AND TABLE_NAME='refund_applications' AND CONSTRAINT_NAME='chk_refund_app_group_discriminator'")"
assert_eq "0022 replay keeps one 24h CHECK" 1 "$(count mall_order "SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA='mall_order' AND TABLE_NAME='legacy_payment_resolution' AND CONSTRAINT_NAME='chk_lpr_expiry_24h'")"

echo
echo "== 0022 partial failure: first ALTER committed, second blocked, repair converges =="
q '' "DROP DATABASE IF EXISTS r04_dirty; CREATE DATABASE r04_dirty" >/dev/null
mysql_file r04_dirty "$ROOT/migrations/order/0017_refund_batch_ledger.sql" >/dev/null
mysql_file r04_dirty "$ROOT/migrations/order/0019_legacy_payment_resolution.sql" >/dev/null
# Both rows are legal before 0022: payment with a group (0022-1) and a 48h
# deadline (0022-2). The WIP migration accepted both; the accepted contract
# must reject them.
q r04_dirty "INSERT INTO refund_applications (refund_id, source, order_no, group_no, non_effective_capture, amount_cent, goods_cent, freight_cent)
  VALUES ('APP-R04-DIRTY','payment','ORD-R04-DIRTY','G1',1,100,100,0)" >/dev/null
q r04_dirty "INSERT INTO legacy_payment_resolution (order_no, state, entered_at, expires_at)
  VALUES ('ORD-R04-DIRTY-48H','OPEN',NOW(3),DATE_ADD(NOW(3),INTERVAL 48 HOUR))" >/dev/null
set +e
dirty_out="$(mysql_file r04_dirty "$ROOT/migrations/order/0022_r04_checks.sql" 2>&1)"
dirty_rc=$?
set -e
[ "$dirty_rc" -ne 0 ] || fail "0022 should reject the dirty group discriminator row"
case "$dirty_out" in
  *"chk_refund_app_group_discriminator"*) echo "OK   0022 blocked by the dirty discriminator row";;
  *) fail "0022 failed without the discriminator diagnostic: $dirty_out";;
esac
assert_eq "blocked 0022 added no discriminator" 0 "$(count r04_dirty "SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA='r04_dirty' AND CONSTRAINT_NAME='chk_refund_app_group_discriminator'")"
q r04_dirty "DELETE FROM refund_applications WHERE refund_id='APP-R04-DIRTY'" >/dev/null
set +e
dirty2_out="$(mysql_file r04_dirty "$ROOT/migrations/order/0022_r04_checks.sql" 2>&1)"
dirty2_rc=$?
set -e
[ "$dirty2_rc" -ne 0 ] || fail "0022 should now fail on the 48h dirty row"
case "$dirty2_out" in
  *"chk_lpr_expiry_24h"*) echo "OK   0022 blocked by the 48h dirty row (first ALTER committed)";;
  *) fail "0022 failed without the 24h diagnostic: $dirty2_out";;
esac
assert_eq "partial 0022 kept the discriminator" 1 "$(count r04_dirty "SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA='r04_dirty' AND CONSTRAINT_NAME='chk_refund_app_group_discriminator'")"
assert_eq "partial 0022 did not add the 24h CHECK" 0 "$(count r04_dirty "SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA='r04_dirty' AND CONSTRAINT_NAME='chk_lpr_expiry_24h'")"
q r04_dirty "UPDATE legacy_payment_resolution SET expires_at=DATE_ADD(entered_at,INTERVAL 24 HOUR) WHERE order_no='ORD-R04-DIRTY-48H'" >/dev/null
mysql_file r04_dirty "$ROOT/migrations/order/0022_r04_checks.sql" >/dev/null
assert_eq "repaired 0022 adds the 24h CHECK" 1 "$(count r04_dirty "SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA='r04_dirty' AND CONSTRAINT_NAME='chk_lpr_expiry_24h'")"
assert_eq "repaired 0022 has 0 blocking rows" 0 "$(count r04_dirty "SELECT COUNT(*) FROM legacy_payment_resolution WHERE expires_at <> DATE_ADD(entered_at, INTERVAL 24 HOUR)")"

echo
echo "== 0018 re-entry: full replay is a no-op =="
mysql_file mall_order "$ROOT/migrations/order/0018_refund_commitment_composition.sql" >/dev/null
assert_eq "0018 replay keeps 4 columns" 4 "$(count mall_order "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='mall_order' AND table_name='order_refund_commitments' AND column_name IN ('goods_cent','freight_cent','composition_known','composition_source')")"

echo
echo "== 0018 partial failure: atomic ALTER committed, then replay converges =="
q '' "DROP DATABASE IF EXISTS r01_reentry; CREATE DATABASE r01_reentry" >/dev/null
q r01_reentry "CREATE TABLE order_refund_commitments LIKE mall_order.order_refund_commitments" >/dev/null
# Back to the pre-0018 shape: the new columns/tables were never created.
q r01_reentry "ALTER TABLE order_refund_commitments
  DROP COLUMN goods_cent, DROP COLUMN freight_cent,
  DROP COLUMN composition_known, DROP COLUMN composition_source" >/dev/null
q r01_reentry "INSERT INTO order_refund_commitments (order_no, group_no, refund_id, amount_cent)
  VALUES ('ORD-R01-LEGACY','G1','REF-R01-LEGACY',500)" >/dev/null
# Replay exactly the prefix of the real file through the ALTER (crash window).
sed -n '1,/DEALLOCATE PREPARE c05_commitment_columns;/p' \
  "$ROOT/migrations/order/0018_refund_commitment_composition.sql" | \
  docker exec -i -e MYSQL_PWD=root "$NAME" mysql -uroot r01_reentry >/dev/null
assert_eq "partial run added header columns" 4 "$(count r01_reentry "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='r01_reentry' AND table_name='order_refund_commitments' AND column_name IN ('goods_cent','freight_cent','composition_known','composition_source')")"
# Re-run the real file: UPDATE and the two CREATE TABLE IF NOT EXISTS must finish.
mysql_file r01_reentry "$ROOT/migrations/order/0018_refund_commitment_composition.sql" >/dev/null
assert_eq "replay created lines table" 1 "$(count r01_reentry "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='r01_reentry' AND table_name='order_refund_commitment_lines'")"
assert_eq "replay created releases table" 1 "$(count r01_reentry "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='r01_reentry' AND table_name='refund_commitment_releases'")"
assert_eq "legacy backfill" "500|0|0|legacy" "$(q r01_reentry "SELECT goods_cent,freight_cent,composition_known,composition_source FROM order_refund_commitments WHERE refund_id='REF-R01-LEGACY'" | tr '\t' '|')"
mysql_file r01_reentry "$ROOT/migrations/order/0018_refund_commitment_composition.sql" >/dev/null
assert_eq "second replay keeps backfill" "500|0|0|legacy" "$(q r01_reentry "SELECT goods_cent,freight_cent,composition_known,composition_source FROM order_refund_commitments WHERE refund_id='REF-R01-LEGACY'" | tr '\t' '|')"

echo
echo "== 0018 partial failure: one existing column is not duplicated =="
q '' "DROP DATABASE IF EXISTS r01_partial; CREATE DATABASE r01_partial" >/dev/null
q r01_partial "CREATE TABLE order_refund_commitments LIKE mall_order.order_refund_commitments" >/dev/null
q r01_partial "ALTER TABLE order_refund_commitments
  DROP COLUMN goods_cent, DROP COLUMN freight_cent,
  DROP COLUMN composition_known, DROP COLUMN composition_source" >/dev/null
q r01_partial "ALTER TABLE order_refund_commitments ADD COLUMN goods_cent BIGINT NOT NULL DEFAULT 0" >/dev/null
mysql_file r01_partial "$ROOT/migrations/order/0018_refund_commitment_composition.sql" >/dev/null
assert_eq "partial column set converges to 4" 4 "$(count r01_partial "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='r01_partial' AND table_name='order_refund_commitments' AND column_name IN ('goods_cent','freight_cent','composition_known','composition_source')")"

echo
echo "== 0011 CHECK semantics on the applied schema =="
assert_error "settled NULL expires rejected" "3819" mysql_exec mall_fulfillment -e \
  "INSERT INTO settlement_switch_approvals (approval_id,target_mode,approved_by,approved_at,evidence_ref,order_paid_admission_retired_at,order_paid_subject_removal_date,status)
   VALUES ('A-R01-NULL','settled','dir',NOW(3),'ev',NOW(3),CURRENT_DATE,'APPROVED')"
mysql_exec mall_fulfillment -e \
  "INSERT INTO settlement_switch_approvals (approval_id,target_mode,approved_by,approved_at,evidence_ref,expires_at,order_paid_admission_retired_at,order_paid_subject_removal_date,status)
   VALUES ('A-R01-OK','settled','dir',NOW(3),'ev',DATE_ADD(NOW(3),INTERVAL 30 DAY),NOW(3),CURRENT_DATE,'APPROVED')" >/dev/null
assert_eq "settled with expires accepted" 1 "$(count mall_fulfillment "SELECT COUNT(*) FROM settlement_switch_approvals WHERE approval_id='A-R01-OK'")"
mysql_exec mall_fulfillment -e \
  "INSERT INTO settlement_switch_approvals (approval_id,target_mode,approved_by,approved_at,evidence_ref,status)
   VALUES ('A-R01-SHADOW','shadow','dir',NOW(3),'ev','APPROVED')" >/dev/null
assert_eq "shadow NULL expires accepted" 1 "$(count mall_fulfillment "SELECT COUNT(*) FROM settlement_switch_approvals WHERE approval_id='A-R01-SHADOW'")"
# Replay the real 0011 file: conditional ADD must not fail on the existing CHECK.
mysql_file mall_fulfillment "$ROOT/migrations/fulfillment/0011_switch_expiry_check.sql" >/dev/null
assert_eq "0011 replay keeps one CHECK" 1 "$(count mall_fulfillment "SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA='mall_fulfillment' AND TABLE_NAME='settlement_switch_approvals' AND CONSTRAINT_NAME='chk_switch_settled_expiry'")"

echo
echo "== 0011 fail-closed on a pre-existing settled row with NULL expires_at =="
q '' "DROP DATABASE IF EXISTS r01_switch_pre; CREATE DATABASE r01_switch_pre" >/dev/null
mysql_file r01_switch_pre "$ROOT/migrations/fulfillment/0010_settlement_shadow_switch.sql" >/dev/null
q r01_switch_pre "INSERT INTO settlement_switch_approvals (approval_id,target_mode,approved_by,approved_at,evidence_ref,order_paid_admission_retired_at,order_paid_subject_removal_date,status)
  VALUES ('A-R01-BAD','settled','dir',NOW(3),'ev',NOW(3),CURRENT_DATE,'APPROVED')" >/dev/null
set +e
blocked_out="$(mysql_file r01_switch_pre "$ROOT/migrations/fulfillment/0011_switch_expiry_check.sql" 2>&1)"
blocked_rc=$?
set -e
[ "$blocked_rc" -ne 0 ] || fail "0011 should fail closed on a violating row"
case "$blocked_out" in
  *"d27 precheck"*) echo "OK   0011 fail-closed diagnostic: d27 precheck";;
  *) fail "0011 blocked without the d27 precheck diagnostic: $blocked_out";;
esac
assert_eq "blocked run adds no CHECK" 0 "$(count r01_switch_pre "SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA='r01_switch_pre' AND CONSTRAINT_NAME='chk_switch_settled_expiry'")"
assert_eq "blocked run keeps the row" 1 "$(count r01_switch_pre "SELECT COUNT(*) FROM settlement_switch_approvals WHERE approval_id='A-R01-BAD'")"
# Director re-sign path: repair the row, then the same real file applies.
q r01_switch_pre "UPDATE settlement_switch_approvals SET expires_at=DATE_ADD(NOW(3),INTERVAL 30 DAY) WHERE approval_id='A-R01-BAD'" >/dev/null
mysql_file r01_switch_pre "$ROOT/migrations/fulfillment/0011_switch_expiry_check.sql" >/dev/null
assert_eq "after re-sign 0011 adds the CHECK" 1 "$(count r01_switch_pre "SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA='r01_switch_pre' AND CONSTRAINT_NAME='chk_switch_settled_expiry'")"

echo
echo "== runner replay: checksum-stable ledger, no file replayed =="
docker run --rm --add-host=host.docker.internal:host-gateway \
  -e MYSQL_HOST=host.docker.internal -e MYSQL_PORT="$BASE_PORT" \
  -e MYSQL_USER=root -e MYSQL_PASSWORD=root \
  -e MIGRATION_LOCK_WAIT_SECONDS=60 \
  -v "$ROOT/migrations:/migrations:ro" "$MIGRATE_IMAGE" | grep -E 'order/0017|order/0018|order/0019|order/0020|order/0021|order/0022|fulfillment/0010|fulfillment/0011|complete'

echo
echo "== runner-level partial failure: FAILED attempt refuses replay, operator repair converges =="
# Rebuild mall_order up to order/0017 with the real runner, then simulate a run
# that died after the atomic 0018 ALTER. The runner's attempt ledger must refuse
# to replay the FAILED file; after the documented operator repair (inspect and
# delete the FAILED attempt row) the same real 0018 must converge.
q '' "DROP DATABASE IF EXISTS mall_order" >/dev/null
TMP_MIGRATIONS="$(mktemp -d)"
mkdir -p "$TMP_MIGRATIONS/order"
for f in "$ROOT"/migrations/order/*.sql; do
  b="$(basename "$f")"
  if [[ "$b" < "0018_refund_commitment_composition.sql" ]]; then
    cp "$f" "$TMP_MIGRATIONS/order/"
  fi
done
docker run --rm --add-host=host.docker.internal:host-gateway \
  -e MYSQL_HOST=host.docker.internal -e MYSQL_PORT="$BASE_PORT" \
  -e MYSQL_USER=root -e MYSQL_PASSWORD=root \
  -e MIGRATION_LOCK_WAIT_SECONDS=60 \
  -v "$TMP_MIGRATIONS:/migrations:ro" "$MIGRATE_IMAGE" >/dev/null
# The real 0018 prefix represents the committed ALTER before the process died.
sed -n '1,/DEALLOCATE PREPARE c05_commitment_columns;/p' \
  "$ROOT/migrations/order/0018_refund_commitment_composition.sql" | \
  docker exec -i -e MYSQL_PWD=root "$NAME" mysql -uroot mall_order >/dev/null
c05_sum="$(sha "$ROOT/migrations/order/0018_refund_commitment_composition.sql")"
mysql_exec mall_order -e "INSERT INTO schema_migration_attempts
  (filename, checksum, status, owner, error, started_at, updated_at)
  VALUES ('order/0018_refund_commitment_composition.sql', '$c05_sum', 'FAILED', 'r01-sim', 'simulated partial failure', NOW(3), NOW(3))" >/dev/null
set +e
runner_out="$(docker run --rm --add-host=host.docker.internal:host-gateway \
  -e MYSQL_HOST=host.docker.internal -e MYSQL_PORT="$BASE_PORT" \
  -e MYSQL_USER=root -e MYSQL_PASSWORD=root \
  -e MIGRATION_LOCK_WAIT_SECONDS=60 \
  -v "$ROOT/migrations:/migrations:ro" "$MIGRATE_IMAGE" 2>&1)"
runner_rc=$?
set -e
[ "$runner_rc" -ne 0 ] || fail "runner should refuse a FAILED 0018 attempt"
case "$runner_out" in
  *"previous FAILED attempt"*) echo "OK   runner refuses replay after a FAILED 0018 attempt";;
  *) fail "runner failed without the FAILED-attempt diagnostic: $runner_out";;
esac
assert_eq "refused run records no 0018 ledger row" 0 "$(count mall_order "SELECT COUNT(*) FROM schema_migrations WHERE filename='order/0018_refund_commitment_composition.sql'")"
# Documented operator repair: inspect the attempt row, then delete only it.
mysql_exec mall_order -e "DELETE FROM schema_migration_attempts WHERE filename='order/0018_refund_commitment_composition.sql' AND status='FAILED'" >/dev/null
docker run --rm --add-host=host.docker.internal:host-gateway \
  -e MYSQL_HOST=host.docker.internal -e MYSQL_PORT="$BASE_PORT" \
  -e MYSQL_USER=root -e MYSQL_PASSWORD=root \
  -e MIGRATION_LOCK_WAIT_SECONDS=60 \
  -v "$ROOT/migrations:/migrations:ro" "$MIGRATE_IMAGE" >/dev/null
assert_eq "operator repair records 0018 ledger row" "$c05_sum" "$(q mall_order "SELECT checksum FROM schema_migrations WHERE filename='order/0018_refund_commitment_composition.sql'")"
assert_eq "operator repair created lines table" 1 "$(count mall_order "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='mall_order' AND table_name='order_refund_commitment_lines'")"
assert_eq "operator repair created releases table" 1 "$(count mall_order "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='mall_order' AND table_name='refund_commitment_releases'")"

echo
echo "R0.1 real migration verification passed"
