#!/usr/bin/env bash
# Cross-warehouse split checkout/shipment E2E.
#
# Preconditions: a dev stack with WAREHOUSE_SPLIT_ENABLED=true and
# WAREHOUSE_ALLOCATION_SHADOW=true, two purchasable SKUs whose stock is
# available in different warehouses, and gateway/admin port-forwards:
#
#   BASE=http://127.0.0.1:18080 ADMIN=http://127.0.0.1:18081 \
#     SPLIT_SKU_A=10179 SPLIT_SKU_B=10178 ./scripts/e2e-warehouse-split.sh
set -euo pipefail
BASE="${BASE:-http://127.0.0.1:18080}"
ADMIN="${ADMIN:-http://127.0.0.1:18081}"
SKU_A="${SPLIT_SKU_A:-10179}"
SKU_B="${SPLIT_SKU_B:-10178}"
json() { python3 -c "import json,sys; d=json.load(sys.stdin); print(eval(sys.argv[1], {'d':d}))" "$1"; }

UT="$(curl -fsS -X POST "$BASE/api/v1/auth/login" -H 'Content-Type: application/json' \
  -d '{"username":"demo","password":"user123"}' | json "d['access_token']")"
AT="$(curl -fsS -X POST "$BASE/api/v1/auth/login" -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' | json "d['access_token']")"
UH="Authorization: Bearer $UT"; AH="Authorization: Bearer $AT"

QUOTE="$(curl -fsS -X POST "$BASE/api/v1/checkout/preview" -H "$UH" -H 'Content-Type: application/json' \
  -d "{\"items\":[{\"sku_id\":$SKU_A,\"quantity\":1},{\"sku_id\":$SKU_B,\"quantity\":1}],\"address_id\":1}")"
GROUP_COUNT="$(echo "$QUOTE" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(len(d.get("groups",[])))')"
[ "$GROUP_COUNT" = "2" ] || { echo "expected 2 quote groups, got $GROUP_COUNT; prepare stock in two warehouses"; echo "$QUOTE"; exit 1; }
QID="$(echo "$QUOTE" | json "d['quote_id']")"; PAY="$(echo "$QUOTE" | json "d['pay_cent']")"

PLACE="$(curl -fsS -X POST "$BASE/api/v1/checkout/place" -H "$UH" -H 'Content-Type: application/json' \
  -H "Idempotency-Key: split-e2e-$(date +%s%N)" \
  -d "{\"quote_id\":\"$QID\",\"address_id\":1,\"receiver_name\":\"Split E2E\",\"receiver_phone\":\"13800000000\",\"receiver_address\":\"Shanghai\"}")"
ORDER="$(echo "$PLACE" | json "d['order']['order_no']")"; PN="$(echo "$PLACE" | json "d['payment']['payment_no']")"
curl -fsS -X POST "$BASE/api/v1/payments/$PN/pay" -H "$UH" -H 'Content-Type: application/json' \
  -d "{\"amount_cent\":$PAY,\"event_id\":\"split-e2e-$PN\"}" >/dev/null

wait_order() { # target
  for _ in $(seq 1 60); do
    D="$(curl -fsS "$BASE/api/v1/orders/$ORDER" -H "$UH")"
    [ "$(echo "$D" | json "d['status']")" = "$1" ] && { echo "$D"; return 0; }
    sleep 0.5
  done
  echo "order did not reach $1"; curl -fsS "$BASE/api/v1/orders/$ORDER" -H "$UH"; exit 1
}
D="$(wait_order ALLOCATED)"
echo "$D" | python3 -c 'import json,sys; d=json.load(sys.stdin); g=d.get("fulfillment_groups",[]); assert len(g)==2, g; print("groups", [(x["group_no"],x["warehouse_id"],x["status"]) for x in g])'

SHIPMENTS="$(curl -fsS "$ADMIN/api/admin/v1/shipments?order_no=$ORDER&page=1&page_size=10" -H "$AH")"
echo "$SHIPMENTS" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert len(d.get("shipments",[]))==2, d; print("shipments", [(s["fulfillment_group_no"],s["status"]) for s in d["shipments"]])'
for SNO in $(echo "$SHIPMENTS" | python3 -c 'import json,sys; print(" ".join(s["shipment_no"] for s in json.load(sys.stdin)["shipments"]))'); do
  curl -fsS -X POST "$ADMIN/api/admin/v1/shipments/$SNO/ship" -H "$AH" -H 'Content-Type: application/json' -d '{}' >/dev/null
done
wait_order SHIPPED >/dev/null
for SNO in $(echo "$SHIPMENTS" | python3 -c 'import json,sys; print(" ".join(s["shipment_no"] for s in json.load(sys.stdin)["shipments"]))'); do
  curl -fsS -X POST "$BASE/api/v1/orders/$ORDER/receive" -H "$UH" -H 'Content-Type: application/json' \
    -d "{\"shipment_no\":\"$SNO\"}" >/dev/null
done
D="$(wait_order COMPLETED)"
echo "$D" | python3 -c 'import json,sys; d=json.load(sys.stdin); print("split E2E complete", d["order_no"], d["status"], d.get("shipment_status"))'
