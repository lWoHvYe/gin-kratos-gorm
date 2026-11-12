#!/usr/bin/env bash
# S3 smoke: qualification -> JetStream worker -> seckill order -> payment.
set -euo pipefail
GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:18080}"
ADMIN_URL="${ADMIN_URL:-http://127.0.0.1:18081}"
SKU_ID="${SKU_ID:-}"
json() { python3 -c "import json,sys; d=json.load(sys.stdin); print($1)"; }

ADMIN_TOKEN="$(curl -fsS -X POST "$GATEWAY_URL/api/v1/auth/login" -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}' | json "d['access_token']")"
USER_TOKEN="$(curl -fsS -X POST "$GATEWAY_URL/api/v1/auth/login" -H 'Content-Type: application/json' -d '{"username":"demo","password":"user123"}' | json "d['access_token']")"
AH="Authorization: Bearer $ADMIN_TOKEN"; UH="Authorization: Bearer $USER_TOKEN"
if [ -z "$SKU_ID" ]; then
  SKU_ID="$(curl -fsS "$ADMIN_URL/api/admin/v1/products?page=1&page_size=100" -H "$AH" | python3 -c "import json,sys; d=json.load(sys.stdin); print(next(s['id'] for p in d['products'] for s in p.get('skus',[])))")"
fi
NOW="$(date +%s)"; END="$((NOW+3600))"
curl -fsS -X POST "$ADMIN_URL/api/admin/v1/inventory/adjust" -H "$AH" -H 'Content-Type: application/json' -d "{\"sku_id\":$SKU_ID,\"warehouse_id\":1,\"delta\":10,\"reason\":\"S3 seckill smoke\"}" >/dev/null
CAMPAIGN="$(curl -fsS -X POST "$ADMIN_URL/api/admin/v1/campaigns" -H "$AH" -H 'Content-Type: application/json' -d "{\"code\":\"S3-SMOKE-$(date +%s)\",\"name\":\"S3 smoke\",\"type\":\"SECKILL\",\"status\":\"ACTIVE\",\"start_at_unix\":$((NOW-60)),\"end_at_unix\":$END,\"priority\":100,\"budget_cent\":0,\"per_user_limit\":1,\"stack_mode\":\"PRIORITY\"}")"
CAMPAIGN_ID="$(echo "$CAMPAIGN" | json "d['id']")"
ACTIVITY="$(curl -fsS -X POST "$ADMIN_URL/api/admin/v1/seckill/activities" -H "$AH" -H 'Content-Type: application/json' -d "{\"campaign_id\":$CAMPAIGN_ID,\"sku_id\":$SKU_ID,\"warehouse_id\":1,\"name\":\"S3 smoke $NOW\",\"price_cent\":100,\"stock_total\":1,\"per_user_limit\":1,\"start_at_unix\":$((NOW-60)),\"end_at_unix\":$END}")"
ACTIVITY_ID="$(echo "$ACTIVITY" | json "d['id']")"
curl -fsS -X POST "$ADMIN_URL/api/admin/v1/seckill/activities/$ACTIVITY_ID/publish" -H "$AH" -H 'Content-Type: application/json' -d '{}' >/dev/null
FIRST="$(curl -fsS -X POST "$GATEWAY_URL/api/v1/seckill/activities/$ACTIVITY_ID/reserve" -H "$UH" -H 'Content-Type: application/json' -d '{"address_id":1,"receiver_name":"Demo","receiver_phone":"13800000000","receiver_address":"Shanghai"}')"
TOKEN="$(echo "$FIRST" | json "d['token']")"
QUAL=""
for _ in $(seq 1 20); do
  QUAL="$(curl -fsS "$GATEWAY_URL/api/v1/seckill/qualifications/$TOKEN" -H "$UH")"
  STATUS="$(echo "$QUAL" | json "d['status']")"
  [ "$STATUS" = "ORDERED" ] && break
  sleep 1
done
echo "$QUAL"
[ "$(echo "$QUAL" | json "d['status']")" = "ORDERED" ] || { echo "qualification was not ordered"; exit 1; }
ORDER_NO="$(echo "$QUAL" | json "d['order_no']")"; PAYMENT_NO="$(echo "$QUAL" | json "d['payment_no']")"
echo "order=$ORDER_NO payment=$PAYMENT_NO"
curl -fsS -X POST "$GATEWAY_URL/api/v1/payments/$PAYMENT_NO/pay" -H "$UH" -H 'Content-Type: application/json' -d '{"amount_cent":100}' >/dev/null
for _ in $(seq 1 20); do
  ORDER="$(curl -fsS "$GATEWAY_URL/api/v1/orders/$ORDER_NO" -H "$UH")"
  STATUS="$(echo "$ORDER" | json "d['status']")"
  [ "$STATUS" != "PENDING_PAYMENT" ] && break
  sleep 1
done
echo "$ORDER"
[ "$(echo "$ORDER" | json "d['status']")" != "PENDING_PAYMENT" ] || { echo "order did not leave PENDING_PAYMENT"; exit 1; }
echo "== seckill S3 smoke passed (activity $ACTIVITY_ID)"
