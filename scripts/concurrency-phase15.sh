#!/usr/bin/env bash
# Verify that two simultaneous checkouts cannot oversell a single unit.
# Requires the gateway/admin BFFs to be reachable (port-forward is fine).
set -euo pipefail

BASE="${BASE_URL:-http://127.0.0.1:18080}"
ADMIN="${ADMIN_URL:-http://127.0.0.1:18081}"
SUFFIX="$(date +%s)"
json() { python3 -c "import json,sys; d=json.load(sys.stdin); print(eval(sys.argv[1], {'d':d}))" "$1"; }
need() { [ -n "${1:-}" ] || { echo "missing $2"; exit 1; }; }

A="$(curl -fsS -X POST "$BASE/api/v1/auth/login" -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}' | json "d['access_token']")"
U="$(curl -fsS -X POST "$BASE/api/v1/auth/login" -H 'Content-Type: application/json' -d '{"username":"demo","password":"user123"}' | json "d['access_token']")"
AH="Authorization: Bearer $A"
UH="Authorization: Bearer $U"

CID="$(curl -fsS -X POST "$ADMIN/api/admin/v1/categories" -H "$AH" -H 'Content-Type: application/json' -d "{\"category\":{\"name\":\"Race Cat $SUFFIX\",\"status\":\"ENABLED\"}}" | json "d['id']")"
BID="$(curl -fsS -X POST "$ADMIN/api/admin/v1/brands" -H "$AH" -H 'Content-Type: application/json' -d "{\"brand\":{\"name\":\"Race Brand $SUFFIX\",\"status\":\"ENABLED\"}}" | json "d['id']")"
PRODUCT="$(curl -fsS -X POST "$ADMIN/api/admin/v1/products" -H "$AH" -H 'Content-Type: application/json' -d "{\"category_id\":$CID,\"brand_id\":$BID,\"name\":\"Race Widget $SUFFIX\",\"description\":\"race\",\"skus\":[{\"sku_code\":\"RACE-$SUFFIX\",\"specs_json\":\"{}\",\"price_cent\":100}]}")"
PID="$(echo "$PRODUCT" | json "d['id']")"
SKU="$(echo "$PRODUCT" | json "d['skus'][0]['id']")"
need "$SKU" sku
need "$PID" product
curl -fsS -X POST "$ADMIN/api/admin/v1/inventory/adjust" -H "$AH" -H 'Content-Type: application/json' \
  -d "{\"sku_id\":$SKU,\"warehouse_id\":1,\"delta\":1,\"reason\":\"race test\"}" >/dev/null
curl -fsS -X PUT "$ADMIN/api/admin/v1/products/$PID/status" -H "$AH" -H 'Content-Type: application/json' -d '{"status":"ON_SALE"}' >/dev/null
AID="$(curl -fsS "$BASE/api/v1/addresses" -H "$UH" | python3 -c 'import json,sys; a=json.load(sys.stdin).get("addresses") or []; print(a[0]["id"] if a else "")')"
if [ -z "$AID" ]; then
  AID="$(curl -fsS -X POST "$BASE/api/v1/addresses" -H "$UH" -H 'Content-Type: application/json' -d "{\"receiver\":\"Race $SUFFIX\",\"phone\":\"13800000000\",\"province\":\"\",\"city\":\"\",\"district\":\"\",\"detail\":\"race\",\"is_default\":true}" | json "d['id']")"
fi
need "$AID" address

Q1="$(curl -fsS -X POST "$BASE/api/v1/checkout/preview" -H "$UH" -H 'Content-Type: application/json' -d "{\"items\":[{\"sku_id\":$SKU,\"quantity\":1}],\"address_id\":$AID}")"
Q2="$(curl -fsS -X POST "$BASE/api/v1/checkout/preview" -H "$UH" -H 'Content-Type: application/json' -d "{\"items\":[{\"sku_id\":$SKU,\"quantity\":1}],\"address_id\":$AID}")"
QID1="$(echo "$Q1" | json "d['quote_id']")"
QID2="$(echo "$Q2" | json "d['quote_id']")"

place() {
  local qid="$1" out="$2"
  curl -sS -o "$out" -w '%{http_code}' -X POST "$BASE/api/v1/checkout/place" -H "$UH" -H 'Content-Type: application/json' \
    -H "Idempotency-Key: race-$SUFFIX-$qid" \
    -d "{\"quote_id\":\"$qid\",\"address_id\":$AID,\"client_request_id\":\"race-$SUFFIX-$qid\"}" >/tmp/race-status-$(basename "$out")
}
place "$QID1" /tmp/race-order-1.json & p1=$!
place "$QID2" /tmp/race-order-2.json & p2=$!
wait "$p1" "$p2"

SUCCESS=0
for f in /tmp/race-order-1.json /tmp/race-order-2.json; do
  if grep -q '"order_no"' "$f"; then SUCCESS=$((SUCCESS + 1)); fi
done
echo "successful orders: $SUCCESS"
[ "$SUCCESS" -eq 1 ] || { echo "expected exactly one successful order"; cat /tmp/race-order-1.json /tmp/race-order-2.json; exit 1; }
STOCK="$(curl -fsS "$ADMIN/api/admin/v1/inventory/stocks?sku_id=$SKU&warehouse_id=1" -H "$AH")"
echo "stock: $STOCK"
echo "$STOCK" | python3 -c 'import json,sys; d=json.load(sys.stdin)["stocks"][0]; assert d["on_hand"]==1 and d.get("available",0)==0 and d["reserved"]==1, d'

echo "== concurrency check passed =="
