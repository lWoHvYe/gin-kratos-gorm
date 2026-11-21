#!/usr/bin/env bash
# Rehearse the C migration and restore rollback on disposable MySQL instances.
#
# It never touches the live cluster. It restores pre-C dumps, runs the same
# mall/migrate image used by Helm, checks group/warehouse invariants, then
# restores the dumps into a second instance to prove the supported rollback.
#
#   BACKUP_DIR=/tmp/mall-predeploy \
#   MIGRATE_IMAGE=mall/migrate:dev ./scripts/rehearse-c-migration.sh
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BACKUP_DIR="${BACKUP_DIR:-/tmp/mall-predeploy}"
MYSQL_IMAGE="${MYSQL_IMAGE:-mysql:8.4}"
MIGRATE_IMAGE="${MIGRATE_IMAGE:-mall/migrate:dev}"
BASE_PORT="${BASE_PORT:-13311}"
N1="mall-c-rehearsal-$$"; N2="mall-c-rollback-$$"

cleanup() {
  docker rm -f "$N1" "$N2" >/dev/null 2>&1 || true
}
trap cleanup EXIT

need() { command -v "$1" >/dev/null || { echo "missing $1" >&2; exit 1; }; }
need docker
[ -f "$BACKUP_DIR/mall_order.sql" ] || { echo "missing pre-C backups in $BACKUP_DIR" >&2; exit 1; }
docker image inspect "$MIGRATE_IMAGE" >/dev/null 2>&1 || { echo "migrate image $MIGRATE_IMAGE not found; build it first" >&2; exit 1; }
docker image inspect "$MYSQL_IMAGE" >/dev/null 2>&1 || { echo "mysql image $MYSQL_IMAGE not found" >&2; exit 1; }

start_mysql() { # name port
  docker run -d --rm --name "$1" -e MYSQL_ROOT_PASSWORD=root \
    -p "127.0.0.1:$2:3306" "$MYSQL_IMAGE" >/dev/null
  for _ in $(seq 1 90); do
    docker exec "$1" mysql -uroot -proot -e 'SELECT 1' >/dev/null 2>&1 && return 0
    sleep 1
  done
  echo "MySQL $1 did not become ready" >&2
  docker logs "$1" | tail -30 >&2
  exit 1
}

restore() { # container
  local c="$1"
  docker exec -i "$c" mysql -uroot -proot -e "CREATE DATABASE IF NOT EXISTS mall_order; CREATE DATABASE IF NOT EXISTS mall_aftersales; CREATE DATABASE IF NOT EXISTS mall_inventory; CREATE DATABASE IF NOT EXISTS mall_pricing; CREATE DATABASE IF NOT EXISTS mall_fulfillment;"
  while IFS='|' read -r db file; do
    docker exec -i "$c" mysql -uroot -proot "$db" < "$BACKUP_DIR/$file"
  done <<'MAP'
mall_order|mall_order.sql
mall_aftersales|mall_aftersales.sql
mall_inventory|mall_inventory.sql
mall_pricing|mall_pricing_c2b.sql
mall_fulfillment|mall_fulfillment.sql
MAP
}

q() { # container database sql
  docker exec "$1" mysql -uroot -proot -N -B "$2" -e "$3"
}
assert_eq() { # label expected actual
  if [ "$2" != "$3" ]; then
    echo "FAIL $1: expected $2, got $3" >&2
    exit 1
  fi
  echo "OK   $1 = $3"
}

echo "== start migration clone on 127.0.0.1:$BASE_PORT =="
start_mysql "$N1" "$BASE_PORT"
restore "$N1"
echo "== run $MIGRATE_IMAGE =="
docker run --rm --add-host=host.docker.internal:host-gateway \
  -e MYSQL_HOST=host.docker.internal -e MYSQL_PORT="$BASE_PORT" \
  -e MYSQL_USER=root -e MYSQL_PASSWORD=root "$MIGRATE_IMAGE" | tail -25

echo "== C ledger =="
while IFS='|' read -r db name; do
  assert_eq "ledger $name" 1 "$(q "$N1" "$db" "SELECT COUNT(*) FROM schema_migrations WHERE filename='$name'" 2>/dev/null || true)"
done <<'MAP'
mall_order|order/0011_fulfillment_groups.sql
mall_order|order/0012_group_shipping_permits.sql
mall_inventory|inventory/0016_reservation_warehouse_keys.sql
mall_pricing|pricing/0010_quote_warehouse.sql
mall_aftersales|aftersales/0011_group_items.sql
mall_fulfillment|fulfillment/0003_group_shipments.sql
MAP

echo "== order invariants =="
assert_eq "orders without group" 0 "$(q "$N1" mall_order "SELECT COUNT(*) FROM orders o LEFT JOIN order_fulfillment_groups g ON g.order_no=o.order_no WHERE g.id IS NULL")"
assert_eq "items without matching group/warehouse" 0 "$(q "$N1" mall_order "SELECT COUNT(*) FROM order_items i LEFT JOIN order_fulfillment_groups g ON g.order_no=i.order_no AND g.group_no=i.fulfillment_group_no WHERE g.id IS NULL OR g.warehouse_id<>i.warehouse_id")"
assert_eq "orphan permits" 0 "$(q "$N1" mall_order "SELECT COUNT(*) FROM order_shipping_permits p LEFT JOIN order_fulfillment_groups g ON g.order_no=p.order_no AND g.group_no=p.fulfillment_group_no WHERE g.id IS NULL")"

echo "== after-sales/pricing/fulfillment invariants =="
assert_eq "after-sale items missing group" 0 "$(q "$N1" mall_aftersales "SELECT COUNT(*) FROM after_sale_items WHERE warehouse_id IS NULL OR group_no=''")"
assert_eq "restock tasks missing group" 0 "$(q "$N1" mall_aftersales "SELECT COUNT(*) FROM after_sale_restock_tasks WHERE warehouse_id IS NULL OR group_no=''")"
assert_eq "quote lines without group" 0 "$(q "$N1" mall_pricing "SELECT COUNT(*) FROM quote_lines l LEFT JOIN quote_groups g ON g.quote_id=l.quote_id AND g.group_no=l.group_no WHERE g.id IS NULL")"
assert_eq "duplicate group shipments" 0 "$(q "$N1" mall_fulfillment "SELECT COUNT(*) FROM (SELECT order_no,fulfillment_group_no FROM shipments GROUP BY order_no,fulfillment_group_no HAVING COUNT(*)>1) d")"

echo "== schema guards =="
assert_eq "inventory reservation warehouse index" 1 "$(q "$N1" mall_inventory "SELECT COUNT(DISTINCT index_name) FROM information_schema.statistics WHERE table_schema='mall_inventory' AND table_name='stock_reservations' AND index_name='uk_reservations_order_sku_warehouse'")"
assert_eq "fulfillment group shipment index" 1 "$(q "$N1" mall_fulfillment "SELECT COUNT(DISTINCT index_name) FROM information_schema.statistics WHERE table_schema='mall_fulfillment' AND table_name='shipments' AND index_name='uk_shipments_order_group'")"

echo "== rollback clone: restore pre-C dumps =="
start_mysql "$N2" "$((BASE_PORT + 1))"
restore "$N2"
assert_eq "rollback order_items group column" 0 "$(q "$N2" mall_order "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='mall_order' AND table_name='order_items' AND column_name='fulfillment_group_no'")"
assert_eq "rollback order groups table" 0 "$(q "$N2" mall_order "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='mall_order' AND table_name='order_fulfillment_groups'")"
for db in mall_order mall_aftersales mall_inventory mall_pricing mall_fulfillment; do
  assert_eq "rollback C ledger rows ($db)" 0 "$(q "$N2" "$db" "SELECT COUNT(*) FROM schema_migrations WHERE filename IN ('order/0011_fulfillment_groups.sql','order/0012_group_shipping_permits.sql','inventory/0016_reservation_warehouse_keys.sql','pricing/0010_quote_warehouse.sql','aftersales/0011_group_items.sql','fulfillment/0003_group_shipments.sql')")"
done

echo "C migration/rollback rehearsal passed on disposable MySQL"
