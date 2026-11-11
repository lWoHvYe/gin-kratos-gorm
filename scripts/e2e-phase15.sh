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

# Keep repeat runs isolated: pause older E2E campaigns so their rules do not
# stack into this run's quote.
CAMPAIGNS="$(curl -fsS "$ADMIN/api/admin/v1/campaigns" -H "$AH")"
echo "$CAMPAIGNS" | python3 -c '
import json,sys
for c in json.load(sys.stdin).get("campaigns", []):
    if c.get("code","").startswith("E2E-CAMP-") and c.get("status")=="ACTIVE":
        print(c["id"])
' | while read -r old_id; do
  curl -fsS -X POST "$ADMIN/api/admin/v1/campaigns/$old_id/status" -H "$AH" -H 'Content-Type: application/json' -d '{"status":"PAUSED"}' >/dev/null
done

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

echo "== campaign/promotion =="
NOW="$(date +%s)"
CAMPAIGN="$(curl -fsS -X POST "$ADMIN/api/admin/v1/campaigns" -H "$AH" -H 'Content-Type: application/json' \
  -d "{\"code\":\"E2E-CAMP-$SUFFIX\",\"name\":\"E2E Campaign\",\"type\":\"NORMAL\",\"status\":\"ACTIVE\",\"start_at_unix\":$((NOW-3600)),\"end_at_unix\":$((NOW+7200)),\"priority\":10,\"budget_cent\":100000,\"stack_mode\":\"PRIORITY\"}")"
CAMPAIGN_ID="$(echo "$CAMPAIGN" | json "d['id']")"
PROMOTION="$(curl -fsS -X POST "$ADMIN/api/admin/v1/promotions" -H "$AH" -H 'Content-Type: application/json' \
  -d "{\"campaign_id\":$CAMPAIGN_ID,\"code\":\"E2E-PROMO-$SUFFIX\",\"name\":\"E2E 5 off\",\"type\":\"FIXED_AMOUNT\",\"status\":\"ACTIVE\",\"priority\":100,\"scope_json\":\"{\\\"type\\\":\\\"SKU\\\",\\\"ids\\\":[$SKU]}\",\"condition_expr\":\"subtotal_cent >= 1000\",\"effect_json\":\"{\\\"type\\\":\\\"FIXED_AMOUNT\\\",\\\"amount_cent\\\":500}\",\"max_discount_cent\":500,\"budget_cent\":100000,\"allow_with_coupon\":true,\"start_at_unix\":$((NOW-3600)),\"end_at_unix\":$((NOW+7200))}")"
PROMOTION_ID="$(echo "$PROMOTION" | json "d['id']")"
need "$CAMPAIGN_ID" campaign
need "$PROMOTION_ID" promotion

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
CART="$(curl -fsS "$BASE/api/v1/cart" -H "$UH")"
echo "$CART" | python3 -c 'import json,sys; [print(i["sku_id"]) for i in json.load(sys.stdin).get("items",[])]' | while read -r old_sku; do
  curl -fsS -X DELETE "$BASE/api/v1/cart/items" -H "$UH" -H 'Content-Type: application/json' -d "{\"sku_id\":$old_sku}" >/dev/null
done
curl -fsS -X POST "$BASE/api/v1/cart/items" -H "$UH" -H 'Content-Type: application/json' -d "{\"sku_id\":$SKU,\"quantity\":2}" >/dev/null

echo "== preview/place/pay =="
# No explicit items: price the selected cart lines so the cart stock/quantity
# path is exercised end to end.
QUOTE="$(curl -fsS -X POST "$BASE/api/v1/checkout/preview" -H "$UH" -H 'Content-Type: application/json' -d "{\"address_id\":$AID}")"
QID="$(echo "$QUOTE" | json "d['quote_id']")"
PAY_CENT="$(echo "$QUOTE" | json "d['pay_cent']")"
PROMO_DISCOUNT="$(echo "$QUOTE" | json "d['promotion_discount_cent']")"
[ "$PROMO_DISCOUNT" -ge 500 ] || { echo "promotion not applied: $QUOTE"; exit 1; }
PLACE="$(curl -fsS -X POST "$BASE/api/v1/checkout/place" -H "$UH" -H 'Content-Type: application/json' \
  -H "Idempotency-Key: e2e-$SUFFIX" -d "{\"quote_id\":\"$QID\",\"address_id\":$AID,\"client_request_id\":\"e2e-$SUFFIX\"}")"
ORDER="$(echo "$PLACE" | json "d['order']['order_no']")"
PAYMENT="$(echo "$PLACE" | json "d['payment']['payment_no']")"
need "$ORDER" order
curl -fsS -X POST "$BASE/api/v1/payments/$PAYMENT/pay" -H "$UH" -H 'Content-Type: application/json' \
  -d "{\"amount_cent\":$PAY_CENT,\"event_id\":\"e2e-pay-$SUFFIX\"}" >/dev/null

RESERVATIONS="$(curl -fsS "$ADMIN/api/admin/v1/promotion-reservations?order_no=$ORDER" -H "$AH")"
echo "$RESERVATIONS" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["total"]>=1 and any(r["status"]=="CONFIRMED" for r in d["reservations"]), d'

echo "== ship/receive =="
curl -fsS -X POST "$ADMIN/api/admin/v1/shipments" -H "$AH" -H 'Content-Type: application/json' \
  -d "{\"order_no\":\"$ORDER\",\"carrier\":\"SF\",\"tracking_no\":\"SF-$SUFFIX\",\"ship\":true}" >/dev/null
curl -fsS -X POST "$BASE/api/v1/orders/$ORDER/receive" -H "$UH" -H 'Content-Type: application/json' -d '{}' >/dev/null
# Shipment -> order status is an async projection. After-sales deliberately
# requires a shipped/completed order, so wait for the projection to converge.
for _ in $(seq 1 20); do
  STATUS="$(curl -fsS "$BASE/api/v1/orders/$ORDER" -H "$UH" | json "d['status']")"
  [ "$STATUS" = "COMPLETED" ] && break
  sleep 0.5
done
[ "$STATUS" = "COMPLETED" ] || { echo "order did not reach COMPLETED: $STATUS"; exit 1; }

echo "== return/refund =="
# A return may not invent quantity that was never ordered.
BAD_STATUS="$(curl -sS -o /tmp/e2e-bad-return.json -w '%{http_code}' -X POST "$BASE/api/v1/after-sales" -H "$UH" -H 'Content-Type: application/json' \
  -d "{\"order_no\":\"$ORDER\",\"type\":\"RETURN_REFUND\",\"reason\":\"bad\",\"amount_cent\":1234,\"items\":[{\"sku_id\":$SKU,\"quantity\":99}]}")"
case "$BAD_STATUS" in 2*) echo "invalid return was accepted"; cat /tmp/e2e-bad-return.json; exit 1;; esac
AS_AMOUNT=$((PAY_CENT / 2))
[ "$AS_AMOUNT" -gt 0 ] || AS_AMOUNT=1
AS="$(curl -fsS -X POST "$BASE/api/v1/after-sales" -H "$UH" -H 'Content-Type: application/json' \
  -d "{\"order_no\":\"$ORDER\",\"type\":\"RETURN_REFUND\",\"reason\":\"e2e return\",\"amount_cent\":$AS_AMOUNT,\"items\":[{\"sku_id\":$SKU,\"quantity\":1}]}")"
ASNO="$(echo "$AS" | json "d['after_sale_no']")"
need "$ASNO" after_sale
curl -fsS -X POST "$ADMIN/api/admin/v1/after-sales/$ASNO/approve" -H "$AH" -H 'Content-Type: application/json' -d '{"approved":true}' >/dev/null
curl -fsS -X POST "$BASE/api/v1/after-sales/$ASNO/return-tracking" -H "$UH" -H 'Content-Type: application/json' \
  -d "{\"carrier\":\"SF\",\"tracking_no\":\"RET-$SUFFIX\"}" >/dev/null
curl -fsS -X POST "$ADMIN/api/admin/v1/after-sales/$ASNO/receive" -H "$AH" -H 'Content-Type: application/json' -d '{"condition":"SELLABLE"}' >/dev/null
AS_DONE="$(curl -fsS -X POST "$ADMIN/api/admin/v1/after-sales/$ASNO/refund" -H "$AH" -H 'Content-Type: application/json' -d '{}')"
echo "$AS_DONE" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["status"]=="REFUNDED" and d.get("refund_no",""), d'

echo "== verify projections =="
DETAIL=""
for _ in $(seq 1 20); do
  DETAIL="$(curl -fsS "$BASE/api/v1/orders/$ORDER" -H "$UH")"
  if echo "$DETAIL" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d.get("refunded_cent")==int(sys.argv[1]) and d.get("refund_status")=="PARTIALLY_REFUNDED"' "$AS_AMOUNT" 2>/dev/null; then
    break
  fi
  sleep 0.5
done
echo "$DETAIL" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d.get("refunded_cent")==int(sys.argv[1]) and d.get("refund_status")=="PARTIALLY_REFUNDED", d' "$AS_AMOUNT"
REFUNDS="$(curl -fsS "$ADMIN/api/admin/v1/payments/refunds?order_no=$ORDER" -H "$AH")"
echo "$REFUNDS" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["total"]==1 and d["refunds"][0]["amount_cent"]==int(sys.argv[1]), d' "$AS_AMOUNT"
STOCK=""
for _ in $(seq 1 20); do
  STOCK="$(curl -fsS "$BASE/api/v1/skus/$SKU")"
  if echo "$STOCK" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["stock"]==9' 2>/dev/null; then break; fi
  sleep 0.5
done
echo "$STOCK" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["stock"]==9, d'

echo "== phase15 e2e passed =="
