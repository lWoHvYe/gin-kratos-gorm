#!/usr/bin/env bash
# S1 smoke: campaign + seckill activity + inventory bucket + Redis warmup + pause.
set -euo pipefail

GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:18080}"
ADMIN_URL="${ADMIN_URL:-http://127.0.0.1:18081}"
ADMIN_USER="${ADMIN_USER:-admin}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin123}"
SKU_ID="${SKU_ID:-}"
STOCK="${STOCK:-100}"

json() { python3 -c "import json,sys; d=json.load(sys.stdin); print($1)"; }

TOKEN="$(curl -fsS -X POST "$GATEWAY_URL/api/v1/auth/login" -H 'Content-Type: application/json' \
  -d "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASSWORD\"}" | json "d['access_token']")"
AUTH="Authorization: Bearer $TOKEN"
if [ -z "$SKU_ID" ]; then
  SKU_ID="$(curl -fsS "$ADMIN_URL/api/admin/v1/products?page=1&page_size=100" -H "$AUTH" | python3 -c "import json,sys; d=json.load(sys.stdin); print(next(s['id'] for p in d['products'] for s in p.get('skus',[])))")"
fi
NOW="$(date +%s)"
END="$((NOW + 3600))"

echo "== restock sku $SKU_ID"
curl -fsS -X POST "$ADMIN_URL/api/admin/v1/inventory/adjust" -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"sku_id\":$SKU_ID,\"warehouse_id\":1,\"delta\":$STOCK,\"reason\":\"S1 seckill smoke\"}" >/dev/null

echo "== create campaign"
CAMPAIGN="$(curl -fsS -X POST "$ADMIN_URL/api/admin/v1/campaigns" -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"code\":\"S1-CAMP-$(date +%s)\",\"name\":\"S1 seckill smoke\",\"type\":\"SECKILL\",\"status\":\"ACTIVE\",\"start_at_unix\":$((NOW-60)),\"end_at_unix\":$END,\"priority\":100,\"budget_cent\":0,\"per_user_limit\":1,\"stack_mode\":\"PRIORITY\"}")"
CAMPAIGN_ID="$(echo "$CAMPAIGN" | json "d['id']")"

echo "== create activity"
ACTIVITY="$(curl -fsS -X POST "$ADMIN_URL/api/admin/v1/seckill/activities" -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"campaign_id\":$CAMPAIGN_ID,\"sku_id\":$SKU_ID,\"warehouse_id\":1,\"name\":\"S1 smoke $NOW\",\"price_cent\":100,\"stock_total\":$STOCK,\"per_user_limit\":1,\"start_at_unix\":$((NOW-60)),\"end_at_unix\":$END}")"
ACTIVITY_ID="$(echo "$ACTIVITY" | json "d['id']")"

echo "== publish"
curl -fsS -X POST "$ADMIN_URL/api/admin/v1/seckill/activities/$ACTIVITY_ID/publish" -H "$AUTH" -H 'Content-Type: application/json' -d '{}' >/dev/null
STOCK_REPLY="$(curl -fsS "$ADMIN_URL/api/admin/v1/seckill/activities/$ACTIVITY_ID/stock" -H "$AUTH")"
echo "$STOCK_REPLY"
echo "$STOCK_REPLY" | python3 -c "import json,sys; d=json.load(sys.stdin); assert d['total']==$STOCK and d['available']==$STOCK and d['status']=='ALLOCATED', d"

echo "== pause/release"
curl -fsS -X POST "$ADMIN_URL/api/admin/v1/seckill/activities/$ACTIVITY_ID/pause" -H "$AUTH" -H 'Content-Type: application/json' -d '{}' >/dev/null
PAUSED_STOCK="$(curl -fsS "$ADMIN_URL/api/admin/v1/seckill/activities/$ACTIVITY_ID/stock" -H "$AUTH")"
echo "$PAUSED_STOCK"
echo "$PAUSED_STOCK" | python3 -c "import json,sys; d=json.load(sys.stdin); assert d.get('available',0)==0 and d['status']=='RELEASED', d"

echo "== idempotent pause"
curl -fsS -X POST "$ADMIN_URL/api/admin/v1/seckill/activities/$ACTIVITY_ID/pause" -H "$AUTH" -H 'Content-Type: application/json' -d '{}' >/dev/null

echo "== seckill S1 smoke passed (activity $ACTIVITY_ID)"
