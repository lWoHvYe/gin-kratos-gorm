#!/usr/bin/env bash
# Happy-path smoke test for the mall + DTM stack.
#
# Requires: curl, python3, and a running `make run-all` stack.
# This is a manual smoke test, not part of `make test` (no integration test gate).
set -euo pipefail

BASE="${BASE_URL:-http://127.0.0.1:8080}"
ADMIN_BASE="${ADMIN_URL:-http://127.0.0.1:8081}"

json() { python3 -c "import json,sys; d=json.load(sys.stdin); print(eval(sys.argv[1], {'d':d}))" "$1"; }

echo "== login demo =="
LOGIN="$(curl -fsS -X POST "$BASE/api/v1/auth/login" -H 'Content-Type: application/json' -d '{"username":"demo","password":"user123"}')"
TOKEN="$(printf '%s' "$LOGIN" | json "d['access_token']")"
[ -n "$TOKEN" ] || { echo "login failed: $LOGIN"; exit 1; }
AUTH="Authorization: Bearer $TOKEN"

echo "== list products =="
PRODUCTS="$(curl -fsS "$BASE/api/v1/products?page=1&page_size=10")"
SKU_ID="$(printf '%s' "$PRODUCTS" | python3 -c '
import json,sys
d=json.load(sys.stdin)
for p in d.get("products", []):
    for sku in p.get("skus", []):
        if sku.get("stock", 0) > 0:
            print(sku["id"]); raise SystemExit
raise SystemExit("no purchasable sku found")
')"
echo "sku_id=$SKU_ID"

echo "== checkout preview =="
QUOTE="$(curl -fsS -X POST "$BASE/api/v1/checkout/preview" -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"items\":[{\"sku_id\":$SKU_ID,\"quantity\":1}],\"address_id\":1}")"
QUOTE_ID="$(printf '%s' "$QUOTE" | json "d['quote_id']")"
PAY_CENT="$(printf '%s' "$QUOTE" | json "d['pay_cent']")"
echo "quote_id=$QUOTE_ID pay_cent=$PAY_CENT"

echo "== place order (DTM saga) =="
PLACE="$(curl -fsS -X POST "$BASE/api/v1/checkout/place" -H "$AUTH" -H 'Content-Type: application/json' \
  -H "Idempotency-Key: smoke-$(date +%s)" \
  -d "{\"quote_id\":\"$QUOTE_ID\",\"address_id\":1,\"receiver_name\":\"Smoke\",\"receiver_phone\":\"13800000000\",\"receiver_address\":\"Shanghai\"}")"
ORDER_NO="$(printf '%s' "$PLACE" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["order"]["order_no"])')"
PAYMENT_NO="$(printf '%s' "$PLACE" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["payment"]["payment_no"])')"
echo "order_no=$ORDER_NO payment_no=$PAYMENT_NO"

echo "== mock pay callback =="
curl -fsS -X POST "$BASE/api/v1/payments/$PAYMENT_NO/pay" -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"amount_cent\":$PAY_CENT,\"event_id\":\"smoke-event-$PAYMENT_NO\"}" >/dev/null
echo "paid"

echo "== order detail =="
curl -fsS "$BASE/api/v1/orders/$ORDER_NO" -H "$AUTH" | python3 -m json.tool | head -40

echo "== DTM smoke passed =="
