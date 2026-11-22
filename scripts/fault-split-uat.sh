#!/usr/bin/env bash
# Fault injection for split checkout on the isolated UAT stack only.
# It scales UAT NATS/Fulfillment to zero and restores them in a trap.
# Caller supplies gateway/admin port-forwards through BASE/ADMIN.
set -euo pipefail
NS="${UAT_NS:-mall-uat}"
DATA_NS="${UAT_DATA_NS:-mall-uat-data}"
BASE="${BASE:-http://127.0.0.1:18090}"
ADMIN="${ADMIN:-http://127.0.0.1:18091}"
SKU_A="${SPLIT_SKU_A:-10001}"
SKU_B="${SPLIT_SKU_B:-10002}"

case "$NS" in
  mall-uat*) ;;
  *) echo "refusing fault injection outside an isolated mall-uat namespace" >&2; exit 1 ;;
esac

json() { python3 -c "import json,sys; d=json.load(sys.stdin); print(eval(sys.argv[1], {'d':d}))" "$1"; }
restore() {
  kubectl -n "$DATA_NS" scale statefulset/nats --replicas=1 >/dev/null 2>&1 || true
  kubectl -n "$NS" scale deployment/fulfillment --replicas=1 >/dev/null 2>&1 || true
  kubectl -n "$NS" scale deployment/order --replicas=1 >/dev/null 2>&1 || true
  kubectl -n "$DATA_NS" rollout status statefulset/nats --timeout=180s >/dev/null 2>&1 || true
  kubectl -n "$NS" rollout status deployment/fulfillment --timeout=180s >/dev/null 2>&1 || true
  kubectl -n "$NS" rollout status deployment/order --timeout=180s >/dev/null 2>&1 || true
}
trap restore EXIT INT TERM

UT="$(curl -fsS -X POST "$BASE/api/v1/auth/login" -H 'Content-Type: application/json' -d '{"username":"demo","password":"user123"}' | json "d['access_token']")"
AT="$(curl -fsS -X POST "$BASE/api/v1/auth/login" -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}' | json "d['access_token']")"
UH="Authorization: Bearer $UT"; AH="Authorization: Bearer $AT"

order_count() {
  kubectl -n "$DATA_NS" exec mysql-0 -- mysql -uroot -proot -N -B mall_order     -e 'SELECT COUNT(*) FROM orders' 2>/dev/null
}

create_pay() { # suffix -> order
  local q id pay o pn order
  q="$(curl -fsS -X POST "$BASE/api/v1/checkout/preview" -H "$UH" -H 'Content-Type: application/json' \
    -d "{\"items\":[{\"sku_id\":$SKU_A,\"quantity\":1},{\"sku_id\":$SKU_B,\"quantity\":1}],\"address_id\":1}")"
  [ "$(echo "$q" | json "len(d['groups'])")" = "2" ] || { echo "need split stock in UAT"; exit 1; }
  id="$(echo "$q" | json "d['quote_id']")"; pay="$(echo "$q" | json "d['pay_cent']")"
  o="$(curl -fsS -X POST "$BASE/api/v1/checkout/place" -H "$UH" -H 'Content-Type: application/json' \
    -H "Idempotency-Key: fault-$1-$RANDOM" \
    -d "{\"quote_id\":\"$id\",\"address_id\":1,\"receiver_name\":\"Fault\",\"receiver_phone\":\"13800000000\",\"receiver_address\":\"Shanghai\"}")"
  order="$(echo "$o" | json "d['order']['order_no']")"; pn="$(echo "$o" | json "d['payment']['payment_no']")"
  curl -fsS -X POST "$BASE/api/v1/payments/$pn/pay" -H "$UH" -H 'Content-Type: application/json' \
    -d "{\"amount_cent\":$pay,\"event_id\":\"fault-$pn\"}" >/dev/null
  echo "$order"
}
shipment_count() {
  curl -fsS "$ADMIN/api/admin/v1/shipments?order_no=$1&page=1&page_size=10" -H "$AH" | json "len(d['shipments'])"
}
wait_shipments() { # order expected
  for _ in $(seq 1 80); do
    [ "$(shipment_count "$1")" = "$2" ] && return 0
    sleep 1
  done
  echo "order $1 did not get $2 shipments; got $(shipment_count "$1")" >&2
  return 1
}

echo "== Order outage: place fails closed, no order row =="
kubectl -n "$NS" scale deployment/order --replicas=0 >/dev/null
kubectl -n "$NS" wait --for=delete pod -l app.kubernetes.io/name=order --timeout=120s >/dev/null
BEFORE=$(order_count)
Q=$(curl -fsS -X POST "$BASE/api/v1/checkout/preview" -H "$UH" -H 'Content-Type: application/json' \
  -d "{\"items\":[{\"sku_id\":$SKU_A,\"quantity\":1},{\"sku_id\":$SKU_B,\"quantity\":1}],\"address_id\":1}")
QID=$(echo "$Q" | json "d['quote_id']")
CODE=$(curl -sS -o /tmp/order-down.$$ -w '%{http_code}' -X POST "$BASE/api/v1/checkout/place" -H "$UH" \
  -H 'Content-Type: application/json' -H "Idempotency-Key: fault-order-down-$RANDOM" \
  -d "{\"quote_id\":\"$QID\",\"address_id\":1,\"receiver_name\":\"Down\",\"receiver_phone\":\"13800000000\",\"receiver_address\":\"Shanghai\"}")
[ "$CODE" = "200" ] && { echo "place unexpectedly succeeded while Order was down"; exit 1; }
AFTER=$(order_count)
[ "$BEFORE" = "$AFTER" ] || { echo "order row changed while Order was down: $BEFORE -> $AFTER"; exit 1; }
echo "place failed closed with HTTP $CODE; order count unchanged ($AFTER)"
kubectl -n "$NS" scale deployment/order --replicas=1 >/dev/null
kubectl -n "$NS" rollout status deployment/order --timeout=180s >/dev/null

echo "== NATS outage: outbox/auto-shipment recovery =="
kubectl -n "$DATA_NS" scale statefulset/nats --replicas=0 >/dev/null
kubectl -n "$DATA_NS" wait --for=delete pod -l app.kubernetes.io/name=nats --timeout=120s >/dev/null
O1=$(create_pay nats)
sleep 2
[ "$(shipment_count "$O1")" = "0" ] || { echo "shipment appeared while NATS was down"; exit 1; }
echo "order $O1 paid while NATS was down; no auto shipment yet"
kubectl -n "$DATA_NS" scale statefulset/nats --replicas=1 >/dev/null
kubectl -n "$DATA_NS" rollout status statefulset/nats --timeout=180s >/dev/null
wait_shipments "$O1" 2
echo "order $O1 auto shipments recovered after NATS returned"

echo "== Fulfillment outage: durable order.paid delivery =="
kubectl -n "$NS" scale deployment/fulfillment --replicas=0 >/dev/null
kubectl -n "$NS" wait --for=delete pod -l app.kubernetes.io/name=fulfillment --timeout=120s >/dev/null
O2=$(create_pay fulfillment)
sleep 2
echo "order $O2 paid while Fulfillment was down; admin shipment query is unavailable"
kubectl -n "$NS" scale deployment/fulfillment --replicas=1 >/dev/null
kubectl -n "$NS" rollout status deployment/fulfillment --timeout=180s >/dev/null
wait_shipments "$O2" 2
echo "order $O2 auto shipments recovered after Fulfillment returned"

echo "split fault injection passed"
