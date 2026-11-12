#!/usr/bin/env bash
# S2 smoke: admin activity + one qualification, idempotent retry and status.
set -euo pipefail
GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:18080}"
ADMIN_URL="${ADMIN_URL:-http://127.0.0.1:18081}"
SKU_ID="${SKU_ID:-}"
ADMIN_USER="${ADMIN_USER:-admin}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin123}"
USER_USER="${USER_USER:-demo}"
USER_PASSWORD="${USER_PASSWORD:-user123}"

json() { python3 -c "import json,sys; d=json.load(sys.stdin); print($1)"; }
ADMIN_TOKEN="$(curl -fsS -X POST "$GATEWAY_URL/api/v1/auth/login" -H 'Content-Type: application/json' -d "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASSWORD\"}" | json "d['access_token']")"
USER_TOKEN="$(curl -fsS -X POST "$GATEWAY_URL/api/v1/auth/login" -H 'Content-Type: application/json' -d "{\"username\":\"$USER_USER\",\"password\":\"$USER_PASSWORD\"}" | json "d['access_token']")"
AH="Authorization: Bearer $ADMIN_TOKEN"
UH="Authorization: Bearer $USER_TOKEN"
if [ -z "$SKU_ID" ]; then
  SKU_ID="$(curl -fsS "$ADMIN_URL/api/admin/v1/products?page=1&page_size=100" -H "$AH" | python3 -c "import json,sys; d=json.load(sys.stdin); print(next(s['id'] for p in d['products'] for s in p.get('skus',[])))")"
fi
NOW="$(date +%s)"; END="$((NOW+3600))"
echo "== restock sku $SKU_ID"
curl -fsS -X POST "$ADMIN_URL/api/admin/v1/inventory/adjust" -H "$AH" -H 'Content-Type: application/json' -d "{\"sku_id\":$SKU_ID,\"warehouse_id\":1,\"delta\":10,\"reason\":\"S2 seckill smoke\"}" >/dev/null
CAMPAIGN="$(curl -fsS -X POST "$ADMIN_URL/api/admin/v1/campaigns" -H "$AH" -H 'Content-Type: application/json' -d "{\"code\":\"S2-SMOKE-$(date +%s)\",\"name\":\"S2 smoke\",\"type\":\"SECKILL\",\"status\":\"ACTIVE\",\"start_at_unix\":$((NOW-60)),\"end_at_unix\":$END,\"priority\":100,\"budget_cent\":0,\"per_user_limit\":1,\"stack_mode\":\"PRIORITY\"}")"
CAMPAIGN_ID="$(echo "$CAMPAIGN" | json "d['id']")"
ACTIVITY="$(curl -fsS -X POST "$ADMIN_URL/api/admin/v1/seckill/activities" -H "$AH" -H 'Content-Type: application/json' -d "{\"campaign_id\":$CAMPAIGN_ID,\"sku_id\":$SKU_ID,\"warehouse_id\":1,\"name\":\"S2 smoke $NOW\",\"price_cent\":100,\"stock_total\":1,\"per_user_limit\":1,\"start_at_unix\":$((NOW-60)),\"end_at_unix\":$END}")"
ACTIVITY_ID="$(echo "$ACTIVITY" | json "d['id']")"
curl -fsS -X POST "$ADMIN_URL/api/admin/v1/seckill/activities/$ACTIVITY_ID/publish" -H "$AH" -H 'Content-Type: application/json' -d '{}' >/dev/null
echo "== reserve (first)"
FIRST="$(curl -fsS -X POST "$GATEWAY_URL/api/v1/seckill/activities/$ACTIVITY_ID/reserve" -H "$UH" -H 'Content-Type: application/json' -d '{}')"
echo "$FIRST"
TOKEN="$(echo "$FIRST" | json "d['token']")"
echo "== reserve (idempotent)"
SECOND="$(curl -fsS -X POST "$GATEWAY_URL/api/v1/seckill/activities/$ACTIVITY_ID/reserve" -H "$UH" -H 'Content-Type: application/json' -d '{}')"
echo "$SECOND"
[ "$TOKEN" = "$(echo "$SECOND" | json "d['token']")" ]
echo "== qualification"
curl -fsS "$GATEWAY_URL/api/v1/seckill/qualifications/$TOKEN" -H "$UH"; echo
echo "== activity stock"
curl -fsS "$ADMIN_URL/api/admin/v1/seckill/activities/$ACTIVITY_ID/stock" -H "$AH"; echo
echo "== seckill S2 smoke passed (activity $ACTIVITY_ID)"
