#!/usr/bin/env bash
# Phase 1.5 end-to-end test. It assumes the K8s/Compose stack is running and
# gateway/admin are reachable through BASE_URL/ADMIN_URL (port-forward is fine).
set -euo pipefail

BASE="${BASE_URL:-http://127.0.0.1:18080}"
ADMIN="${ADMIN_URL:-http://127.0.0.1:18081}"
SUFFIX="$(date +%s)"
json() { python3 -c "import json,sys; d=json.load(sys.stdin); print(eval(sys.argv[1], {'d':d}))" "$1"; }
need() { [ -n "${1:-}" ] || { echo "missing $2"; exit 1; }; }

echo "== admin login =="
ADMIN_TOKEN="$(curl -fsS -X POST "$BASE/api/v1/auth/login" -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}' | json "d['access_token']")"
need "$ADMIN_TOKEN" admin_token
AH="Authorization: Bearer $ADMIN_TOKEN"

echo "== create category/brand =="
CID="$(curl -fsS -X POST "$ADMIN/api/admin/v1/categories" -H "$AH" -H 'Content-Type: application/json' -d "{\"category\":{\"name\":\"E2E Cat $SUFFIX\",\"parent_id\":0,\"sort\":99,\"status\":\"ENABLED\"}}" | json "d['id']")"
BID="$(curl -fsS -X POST "$ADMIN/api/admin/v1/brands" -H "$AH" -H 'Content-Type: application/json' -d "{\"brand\":{\"name\":\"E2E Brand $SUFFIX\",\"status\":\"ENABLED\"}}" | json "d['id']")"
need "$CID" category
need "$BID" brand

echo "== create product/SKU with image and attribute =="
PRODUCT="$(curl -fsS -X POST "$ADMIN/api/admin/v1/products" -H "$AH" -H 'Content-Type: application/json' -d "{\"category_id\":$CID,\"brand_id\":$BID,\"name\":\"E2E Widget $SUFFIX\",\"subtitle\":\"phase15\",\"description\":\"e2e\",\"main_image\":\"https://example.invalid/main.png\",\"images\":[{\"url\":\"https://example.invalid/1.png\",\"sort\":1}],\"attributes\":[{\"name\":\"color\",\"value\":\"red\",\"sort\":1}],\"skus\":[{\"sku_code\":\"E2E-$SUFFIX\",\"specs_json\":\"{\\\"color\\\":\\\"red\\\"}\",\"price_cent\":1234}]}")"
PID="$(echo "$PRODUCT" | json "d['id']")"
SKU="$(echo "$PRODUCT" | json "d['skus'][0]['id']")"
need "$PID" product
need "$SKU" sku

echo "== initialize inventory and publish =="
curl -fsS -X POST "$ADMIN/api/admin/v1/inventory/adjust" -H "$AH" -H 'Content-Type: application/json' \
  -d "{\"sku_id\":$SKU,\"warehouse_id\":1,\"delta\":10,\"reason\":\"e2e init\"}" >/dev/null
curl -fsS -X PUT "$ADMIN/api/admin/v1/products/$PID/status" -H "$AH" -H 'Content-Type: application/json' -d '{"status":"ON_SALE"}' >/dev/null
sleep 2

echo "== C-end search filters =="
SEARCH="$(curl -fsS "$BASE/api/v1/products?keyword=E2E&brand_id=$BID&min_price_cent=1000&max_price_cent=2000&sort=price_asc")"
FOUND="$(echo "$SEARCH" | json "d['total']")"
[ "$FOUND" -ge 1 ] || { echo "search did not find created product"; exit 1; }
echo "$SEARCH" | python3 -c 'import json,sys; d=json.load(sys.stdin); p=d["products"][0]; assert p["images"] and p["attributes"] and p["skus"][0]["stock"]>0'

echo "== user login/address/cart =="
DEMO_TOKEN="$(curl -fsS -X POST "$BASE/api/v1/auth/login" -H 'Content-Type: application/json' -d '{"username":"demo","password":"user123"}' | json "d['access_token']")"
need "$DEMO_TOKEN" demo_token
UH="Authorization: Bearer $DEMO_TOKEN"
AID="$(curl -fsS -X POST "$BASE/api/v1/addresses" -H "$UH" -H 'Content-Type: application/json' \
  -d "{\"receiver\":\"E2E $SUFFIX\",\"phone\":\"13800000000\",\"province\":\"Shanghai\",\"city\":\"Shanghai\",\"district\":\"Pudong\",\"detail\":\"E2E $SUFFIX\",\"is_default\":true}" | json "d['id']")"
need "$AID" address
curl -fsS -X POST "$BASE/api/v1/cart/items" -H "$UH" -H 'Content-Type: application/json' -d "{\"sku_id\":$SKU,\"quantity\":2}" >/dev/null

echo "== preview/place/pay =="
QUOTE="$(curl -fsS -X POST "$BASE/api/v1/checkout/preview" -H "$UH" -H 'Content-Type: application/json' -d "{\"items\":[{\"sku_id\":$SKU,\"quantity\":2}],\"address_id\":$AID}")"
QID="$(echo "$QUOTE" | json "d['quote_id']")"
PAY_CENT="$(echo "$QUOTE" | json "d['pay_cent']")"
PLACE="$(curl -fsS -X POST "$BASE/api/v1/checkout/place" -H "$UH" -H 'Content-Type: application/json' \
  -H "Idempotency-Key: e2e-$SUFFIX" -d "{\"quote_id\":\"$QID\",\"address_id\":$AID,\"client_request_id\":\"e2e-$SUFFIX\"}")"
ORDER="$(echo "$PLACE" | json "d['order']['order_no']")"
PAYMENT="$(echo "$PLACE" | json "d['payment']['payment_no']")"
need "$ORDER" order
curl -fsS -X POST "$BASE/api/v1/payments/$PAYMENT/pay" -H "$UH" -H 'Content-Type: application/json' \
  -d "{\"amount_cent\":$PAY_CENT,\"event_id\":\"e2e-pay-$SUFFIX\"}" >/dev/null

echo "== ship/receive =="
curl -fsS -X POST "$ADMIN/api/admin/v1/shipments" -H "$AH" -H 'Content-Type: application/json' \
  -d "{\"order_no\":\"$ORDER\",\"carrier\":\"SF\",\"tracking_no\":\"SF-$SUFFIX\",\"ship\":true}" >/dev/null
curl -fsS -X POST "$BASE/api/v1/orders/$ORDER/receive" -H "$UH" -H 'Content-Type: application/json' -d '{}' >/dev/null

echo "== return/refund =="
AS="$(curl -fsS -X POST "$BASE/api/v1/after-sales" -H "$UH" -H 'Content-Type: application/json' \
  -d "{\"order_no\":\"$ORDER\",\"type\":\"RETURN_REFUND\",\"reason\":\"e2e return\",\"amount_cent\":1234,\"items\":[{\"sku_id\":$SKU,\"quantity\":1}]}")"
ASNO="$(echo "$AS" | json "d['after_sale_no']")"
need "$ASNO" after_sale
curl -fsS -X POST "$ADMIN/api/admin/v1/after-sales/$ASNO/approve" -H "$AH" -H 'Content-Type: application/json' -d '{"approved":true}' >/dev/null
curl -fsS -X POST "$BASE/api/v1/after-sales/$ASNO/return-tracking" -H "$UH" -H 'Content-Type: application/json' \
  -d "{\"carrier\":\"SF\",\"tracking_no\":\"RET-$SUFFIX\"}" >/dev/null
curl -fsS -X POST "$ADMIN/api/admin/v1/after-sales/$ASNO/receive" -H "$AH" -H 'Content-Type: application/json' -d '{"condition":"SELLABLE"}' >/dev/null
curl -fsS -X POST "$ADMIN/api/admin/v1/after-sales/$ASNO/refund" -H "$AH" -H 'Content-Type: application/json' -d '{}' >/dev/null

echo "== verify projections =="
DETAIL=""
for _ in $(seq 1 20); do
  DETAIL="$(curl -fsS "$BASE/api/v1/orders/$ORDER" -H "$UH")"
  if echo "$DETAIL" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d.get("refunded_cent")==1234 and d.get("status")=="PARTIALLY_REFUNDED"' 2>/dev/null; then
    break
  fi
  sleep 0.5
done
echo "$DETAIL" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d.get("refunded_cent")==1234 and d.get("status")=="PARTIALLY_REFUNDED", d'
REFUNDS="$(curl -fsS "$ADMIN/api/admin/v1/payments/refunds?order_no=$ORDER" -H "$AH")"
echo "$REFUNDS" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["total"]==1 and d["refunds"][0]["amount_cent"]==1234, d'
STOCK="$(curl -fsS "$BASE/api/v1/skus/$SKU")"
echo "$STOCK" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["stock"]==9, d'

echo "== phase15 e2e passed =="
