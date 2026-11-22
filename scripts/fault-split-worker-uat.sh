#!/usr/bin/env bash
# Restart the after-sales worker between ReceiveReturn and restock, then verify
# the durable task drains and the G2 refund completes. UAT only.
set -euo pipefail
NS="${UAT_NS:-mall-uat}"
DATA_NS="${UAT_DATA_NS:-mall-uat-data}"
BASE="${BASE:-http://127.0.0.1:18090}"
ADMIN="${ADMIN:-http://127.0.0.1:18091}"
SKU_A="${SPLIT_SKU_A:?set SPLIT_SKU_A}"
SKU_B="${SPLIT_SKU_B:?set SPLIT_SKU_B}"
case "$NS" in mall-uat*) ;; *) echo "refusing outside mall-uat" >&2; exit 1;; esac
json() { python3 -c "import json,sys; d=json.load(sys.stdin); print(eval(sys.argv[1], {'d':d}))" "$1"; }
restore() { kubectl -n "$NS" scale deployment/aftersales --replicas=1 >/dev/null 2>&1 || true; }
trap restore EXIT INT TERM
q() { kubectl -n "$DATA_NS" exec mysql-0 -- mysql -uroot -proot -N -B "$1" -e "$2" 2>/dev/null; }
UT="$(curl -fsS -X POST "$BASE/api/v1/auth/login" -H 'Content-Type: application/json' -d '{"username":"demo","password":"user123"}' | json "d['access_token']")"
AT="$(curl -fsS -X POST "$BASE/api/v1/auth/login" -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}' | json "d['access_token']")"
UH="Authorization: Bearer $UT"; AH="Authorization: Bearer $AT"
Q="$(curl -fsS -X POST "$BASE/api/v1/checkout/preview" -H "$UH" -H 'Content-Type: application/json' -d "{\"items\":[{\"sku_id\":$SKU_A,\"quantity\":1},{\"sku_id\":$SKU_B,\"quantity\":1}],\"address_id\":1}")"
QID="$(echo "$Q" | json "d['quote_id']")"; PAY="$(echo "$Q" | json "d['pay_cent']")"
P="$(curl -fsS -X POST "$BASE/api/v1/checkout/place" -H "$UH" -H 'Content-Type: application/json' -H "Idempotency-Key: worker-$RANDOM" -d "{\"quote_id\":\"$QID\",\"address_id\":1,\"receiver_name\":\"Worker\",\"receiver_phone\":\"13800000000\",\"receiver_address\":\"Shanghai\"}")"
ORDER="$(echo "$P" | json "d['order']['order_no']")"; PN="$(echo "$P" | json "d['payment']['payment_no']")"
curl -fsS -X POST "$BASE/api/v1/payments/$PN/pay" -H "$UH" -H 'Content-Type: application/json' -d "{\"amount_cent\":$PAY,\"event_id\":\"worker-$PN\"}" >/dev/null
for _ in $(seq 1 40); do
  COUNT="$(curl -fsS "$ADMIN/api/admin/v1/shipments?order_no=$ORDER&page=1&page_size=10" -H "$AH" | json "len(d['shipments'])")"
  [ "$COUNT" = "2" ] && break; sleep .5
done
S2="$(curl -fsS "$ADMIN/api/admin/v1/shipments?order_no=$ORDER&page=1&page_size=10" -H "$AH" | python3 -c 'import json,sys; print(next(s["shipment_no"] for s in json.load(sys.stdin)["shipments"] if s["fulfillment_group_no"]=="G2"))')"
curl -fsS -X POST "$ADMIN/api/admin/v1/shipments/$S2/ship" -H "$AH" -H 'Content-Type: application/json' -d '{}' >/dev/null
for _ in $(seq 1 40); do
  GS="$(curl -fsS "$BASE/api/v1/orders/$ORDER" -H "$UH" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(next((g["status"] for g in d.get("fulfillment_groups",[]) if g["group_no"]=="G2"),""))')"
  [ "$GS" = "SHIPPED" ] && break
  sleep .5
done
[ "$GS" = "SHIPPED" ] || { echo "G2 did not reach SHIPPED: $GS"; exit 1; }
AS="$(curl -fsS -X POST "$BASE/api/v1/after-sales" -H "$UH" -H 'Content-Type: application/json' -d "{\"order_no\":\"$ORDER\",\"type\":\"RETURN_REFUND\",\"reason\":\"worker restart\",\"amount_cent\":100,\"items\":[{\"sku_id\":$SKU_B,\"quantity\":1,\"warehouse_id\":2,\"group_no\":\"G2\"}]}")"
ASNO="$(echo "$AS" | json "d['after_sale_no']")"
curl -fsS -X POST "$ADMIN/api/admin/v1/after-sales/$ASNO/approve" -H "$AH" -H 'Content-Type: application/json' -d '{"approved":true}' >/dev/null
curl -fsS -X POST "$BASE/api/v1/after-sales/$ASNO/return-tracking" -H "$UH" -H 'Content-Type: application/json' -d '{"carrier":"SF","tracking_no":"WORKER"}' >/dev/null
curl -fsS -X POST "$ADMIN/api/admin/v1/after-sales/$ASNO/receive" -H "$AH" -H 'Content-Type: application/json' -d '{"condition":"SELLABLE"}' >/dev/null
# Kill the worker immediately after the durable task is committed.
kubectl -n "$NS" scale deployment/aftersales --replicas=0 >/dev/null
kubectl -n "$NS" wait --for=delete pod -l app.kubernetes.io/name=aftersales --timeout=120s >/dev/null
ST="$(q mall_aftersales "SELECT status FROM after_sale_restock_tasks WHERE after_sale_no='$ASNO' LIMIT 1")"
[ "$ST" != "DONE" ] || { echo "worker already completed before restart; inconclusive"; exit 1; }
echo "after-sales worker stopped with restock task status=$ST"
kubectl -n "$NS" scale deployment/aftersales --replicas=1 >/dev/null
kubectl -n "$NS" rollout status deployment/aftersales --timeout=180s >/dev/null
for _ in $(seq 1 60); do
  ST="$(q mall_aftersales "SELECT status FROM after_sale_restock_tasks WHERE after_sale_no='$ASNO' LIMIT 1")"
  [ "$ST" = "DONE" ] && break; sleep 1
done
[ "$ST" = "DONE" ] || { echo "restock task did not converge: $ST"; exit 1; }
curl -fsS -X POST "$ADMIN/api/admin/v1/after-sales/$ASNO/refund" -H "$AH" -H 'Content-Type: application/json' -d '{}' >/dev/null
for _ in $(seq 1 40); do
  ST="$(q mall_aftersales "SELECT status FROM after_sales WHERE after_sale_no='$ASNO'")"
  [ "$ST" = "REFUNDED" ] && break; sleep .5
done
[ "$ST" = "REFUNDED" ] || { echo "after-sale did not refund: $ST"; exit 1; }
GS="$(q mall_order "SELECT refund_state FROM order_fulfillment_groups WHERE order_no='$ORDER' AND group_no='G2'")"
[ "$GS" = "REFUNDED" ] || { echo "group refund state = $GS"; exit 1; }
echo "worker restart recovered: restock=DONE refund=REFUNDED group=REFUNDED"
