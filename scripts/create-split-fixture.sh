#!/usr/bin/env bash
# Create two fresh split-test SKUs, stocked in warehouse 1 and warehouse 2.
# Intended for an isolated UAT/dev stack; it does not reuse demo SKUs.
set -euo pipefail
BASE="${BASE:-http://127.0.0.1:18080}"
ADMIN="${ADMIN:-http://127.0.0.1:18081}"
OUT="${FIXTURE_ENV:-/tmp/split-fixture.env}"
SUFFIX="$(date +%s)"
json() { python3 -c "import json,sys; d=json.load(sys.stdin); print(eval(sys.argv[1], {'d':d}))" "$1"; }

AT="$(curl -fsS -X POST "$BASE/api/v1/auth/login" -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' | json "d['access_token']")"
AH="Authorization: Bearer $AT"
CID="$(curl -fsS -X POST "$ADMIN/api/admin/v1/categories" -H "$AH" -H 'Content-Type: application/json' \
  -d "{\"category\":{\"name\":\"Split Fixture $SUFFIX\",\"parent_id\":0,\"sort\":99,\"status\":\"ENABLED\"}}" | json "d['id']")"
BID="$(curl -fsS -X POST "$ADMIN/api/admin/v1/brands" -H "$AH" -H 'Content-Type: application/json' \
  -d "{\"brand\":{\"name\":\"Split Fixture $SUFFIX\",\"status\":\"ENABLED\"}}" | json "d['id']")"

create_product() { # code price
  curl -fsS -X POST "$ADMIN/api/admin/v1/products" -H "$AH" -H 'Content-Type: application/json' \
    -d "{\"category_id\":$CID,\"brand_id\":$BID,\"name\":\"Split Fixture $1\",\"subtitle\":\"fixture\",\"description\":\"split fixture\",\"main_image\":\"https://example.invalid/main.png\",\"images\":[{\"url\":\"https://example.invalid/1.png\",\"sort\":1}],\"attributes\":[{\"name\":\"fixture\",\"value\":\"split\",\"sort\":1}],\"skus\":[{\"sku_code\":\"$1\",\"specs_json\":\"{\\\"fixture\\\":\\\"split\\\"}\",\"price_cent\":$2}]}"
}
P1="$(create_product "SPLIT-A-$SUFFIX" 1000)"
P2="$(create_product "SPLIT-B-$SUFFIX" 1200)"
SKU_A="$(echo "$P1" | json "d['skus'][0]['id']")"; PID_A="$(echo "$P1" | json "d['id']")"
SKU_B="$(echo "$P2" | json "d['skus'][0]['id']")"; PID_B="$(echo "$P2" | json "d['id']")"
for pair in "$SKU_A:1" "$SKU_B:2"; do
  sku="${pair%:*}"; warehouse="${pair#*:}"
  curl -fsS -X POST "$ADMIN/api/admin/v1/inventory/adjust" -H "$AH" -H 'Content-Type: application/json' \
    -d "{\"sku_id\":$sku,\"warehouse_id\":$warehouse,\"delta\":10,\"reason\":\"split fixture\"}" >/dev/null
done
for pid in "$PID_A" "$PID_B"; do
  curl -fsS -X PUT "$ADMIN/api/admin/v1/products/$pid/status" -H "$AH" -H 'Content-Type: application/json' \
    -d '{"status":"ON_SALE"}' >/dev/null
done
cat > "$OUT" <<ENV
SPLIT_SKU_A=$SKU_A
SPLIT_SKU_B=$SKU_B
ENV
echo "split fixture: SKU_A=$SKU_A (warehouse 1), SKU_B=$SKU_B (warehouse 2)"
echo "wrote $OUT"
