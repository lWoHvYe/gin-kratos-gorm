#!/usr/bin/env bash
# Targeted split-order concurrency checks (not a load test).
# Requires two SKUs stocked in different warehouses and split enabled.
set -euo pipefail
BASE="${BASE:-http://127.0.0.1:18080}"
ADMIN="${ADMIN:-http://127.0.0.1:18081}"
SKU_A="${SPLIT_SKU_A:-10179}"
SKU_B="${SPLIT_SKU_B:-10178}"
json() { python3 -c "import json,sys; d=json.load(sys.stdin); print(eval(sys.argv[1], {'d':d}))" "$1"; }
code() { curl -sS -o /tmp/split-conc-body.$$ -w '%{http_code}' "$@"; }

UT="$(curl -fsS -X POST "$BASE/api/v1/auth/login" -H 'Content-Type: application/json' -d '{"username":"demo","password":"user123"}' | json "d['access_token']")"
AT="$(curl -fsS -X POST "$BASE/api/v1/auth/login" -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}' | json "d['access_token']")"
UH="Authorization: Bearer $UT"; AH="Authorization: Bearer $AT"

create_order() { # suffix -> order_no payment_no pay_cent
  local sfx="$1"
  local q id pay o pn
  q="$(curl -fsS -X POST "$BASE/api/v1/checkout/preview" -H "$UH" -H 'Content-Type: application/json' \
    -d "{\"items\":[{\"sku_id\":$SKU_A,\"quantity\":1},{\"sku_id\":$SKU_B,\"quantity\":1}],\"address_id\":1}")"
  [ "$(echo "$q" | json "len(d['groups'])")" = "2" ] || { echo "expected two groups; prepare split stock"; exit 1; }
  id="$(echo "$q" | json "d['quote_id']")"; pay="$(echo "$q" | json "d['pay_cent']")"
  o="$(curl -fsS -X POST "$BASE/api/v1/checkout/place" -H "$UH" -H 'Content-Type: application/json' \
    -H "Idempotency-Key: conc-$sfx-$RANDOM" \
    -d "{\"quote_id\":\"$id\",\"address_id\":1,\"receiver_name\":\"Conc\",\"receiver_phone\":\"13800000000\",\"receiver_address\":\"Shanghai\"}")"
  echo "$(echo "$o" | json "d['order']['order_no']") $(echo "$o" | json "d['payment']['payment_no']") $pay"
}

wait_status() { # order target
  for _ in $(seq 1 60); do
    local d; d="$(curl -fsS "$BASE/api/v1/orders/$1" -H "$UH")"
    [ "$(echo "$d" | json "d['status']")" = "$2" ] && return 0
    sleep 0.5
  done
  echo "order $1 did not reach $2" >&2; return 1
}

shipments() { curl -fsS "$ADMIN/api/admin/v1/shipments?order_no=$1&page=1&page_size=10" -H "$AH"; }
wait_shipments() { # order count
  for _ in $(seq 1 40); do
    [ "$(shipments "$1" | json "len(d['shipments'])")" = "$2" ] && return 0
    sleep 0.5
  done
  echo "order $1 did not get $2 auto shipments" >&2; return 1
}
ship_no() { shipments "$1" | python3 -c 'import json,sys; g=sys.argv[1]; print(next(s["shipment_no"] for s in json.load(sys.stdin)["shipments"] if s["fulfillment_group_no"]==g))' "$2"; }

echo "== duplicate payment event is idempotent =="
read -r O1 P1 PAY1 <<<"$(create_order dup)"
BODY="{\"amount_cent\":$PAY1,\"event_id\":\"conc-dup-$P1\"}"
C1=$(curl -sS -o /tmp/pay1.$$ -w '%{http_code}' -X POST "$BASE/api/v1/payments/$P1/pay" -H "$UH" -H 'Content-Type: application/json' -d "$BODY")
[ "$C1" = "200" ] || { echo "first pay failed: $C1 body=$BODY response=$(cat /tmp/pay1.$$)"; exit 1; }
C2=$(curl -sS -o /tmp/pay2.$$ -w '%{http_code}' -X POST "$BASE/api/v1/payments/$P1/pay" -H "$UH" -H 'Content-Type: application/json' -d "$BODY")
case "$C2" in 200|409|422) ;; *) echo "duplicate pay unexpected: $C2 $(cat /tmp/pay2.$$)"; exit 1;; esac
rm -f /tmp/pay1.$$ /tmp/pay2.$$
wait_status "$O1" ALLOCATED
D="$(curl -fsS "$BASE/api/v1/orders/$O1" -H "$UH")"
[ "$(echo "$D" | json "d['paid_cent']")" = "$PAY1" ] || { echo "paid_cent changed after duplicate pay"; exit 1; }
echo "order $O1 paid_cent=$PAY1"

echo "== G1 and G2 ship concurrently, then duplicate ship =="
wait_shipments "$O1" 2
S1=$(ship_no "$O1" G1); S2=$(ship_no "$O1" G2)
( curl -fsS -X POST "$ADMIN/api/admin/v1/shipments/$S1/ship" -H "$AH" -H 'Content-Type: application/json' -d '{}' >/dev/null ) & A=$!
( curl -fsS -X POST "$ADMIN/api/admin/v1/shipments/$S2/ship" -H "$AH" -H 'Content-Type: application/json' -d '{}' >/dev/null ) & B=$!
wait $A; wait $B
wait_status "$O1" SHIPPED
curl -fsS -X POST "$ADMIN/api/admin/v1/shipments/$S1/ship" -H "$AH" -H 'Content-Type: application/json' -d '{}' >/dev/null
echo "order $O1 SHIPPED; duplicate ship accepted"

echo "== unshipped G2 refund cancels its draft parcel =="
read -r O3 P3 PAY3 <<<"$(create_order cancel)"
curl -fsS -X POST "$BASE/api/v1/payments/$P3/pay" -H "$UH" -H 'Content-Type: application/json' \
  -d "{\"amount_cent\":$PAY3,\"event_id\":\"conc-cancel-$P3\"}" >/dev/null
wait_shipments "$O3" 2
S31=$(ship_no "$O3" G1); S32=$(ship_no "$O3" G2)
AS3=$(curl -fsS -X POST "$BASE/api/v1/after-sales" -H "$UH" -H 'Content-Type: application/json' \
  -d "{\"order_no\":\"$O3\",\"type\":\"REFUND_ONLY\",\"reason\":\"cancel draft\",\"amount_cent\":100,\"items\":[{\"sku_id\":$SKU_B,\"quantity\":1,\"warehouse_id\":2,\"group_no\":\"G2\"}]}")
AS3NO=$(echo "$AS3" | json "d['after_sale_no']")
curl -fsS -X POST "$ADMIN/api/admin/v1/after-sales/$AS3NO/approve" -H "$AH" -H 'Content-Type: application/json' -d '{"approved":true}' >/dev/null
curl -fsS -X POST "$ADMIN/api/admin/v1/after-sales/$AS3NO/refund" -H "$AH" -H 'Content-Type: application/json' -d '{}' >/dev/null
for _ in $(seq 1 40); do
  ST=$(shipments "$O3" | python3 -c 'import json,sys; g=sys.argv[1]; print(next((s["status"] for s in json.load(sys.stdin)["shipments"] if s["fulfillment_group_no"]==g),""))' G2)
  [ "$ST" = CANCELLED ] && break
  sleep 0.5
done
[ "$ST" = CANCELLED ] || { echo "draft G2 shipment was not cancelled: $ST"; exit 1; }
echo "G2 draft shipment CANCELLED"
curl -fsS -X POST "$ADMIN/api/admin/v1/shipments/$S31/ship" -H "$AH" -H 'Content-Type: application/json' -d '{}' >/dev/null
wait_status "$O3" SHIPPED
echo "order $O3 G1 shipped after G2 draft cancellation"

echo "== G2 refund/ship race leaves G1 shippable =="
read -r O2 P2 PAY2 <<<"$(create_order race)"
curl -fsS -X POST "$BASE/api/v1/payments/$P2/pay" -H "$UH" -H 'Content-Type: application/json' \
  -d "{\"amount_cent\":$PAY2,\"event_id\":\"conc-race-$P2\"}" >/dev/null
wait_shipments "$O2" 2
S21=$(ship_no "$O2" G1); S22=$(ship_no "$O2" G2)
AS=$(curl -fsS -X POST "$BASE/api/v1/after-sales" -H "$UH" -H 'Content-Type: application/json' \
  -d "{\"order_no\":\"$O2\",\"type\":\"REFUND_ONLY\",\"reason\":\"race\",\"amount_cent\":100,\"items\":[{\"sku_id\":$SKU_B,\"quantity\":1,\"warehouse_id\":2,\"group_no\":\"G2\"}]}")
ASNO=$(echo "$AS" | json "d['after_sale_no']")
curl -fsS -X POST "$ADMIN/api/admin/v1/after-sales/$ASNO/approve" -H "$AH" -H 'Content-Type: application/json' -d '{"approved":true}' >/dev/null
( curl -sS -o /dev/null -w '%{http_code}' -X POST "$ADMIN/api/admin/v1/after-sales/$ASNO/refund" -H "$AH" -H 'Content-Type: application/json' -d '{}' > /tmp/refund-code.$$ ) & R=$!
( curl -sS -o /dev/null -w '%{http_code}' -X POST "$ADMIN/api/admin/v1/shipments/$S22/ship" -H "$AH" -H 'Content-Type: application/json' -d '{}' > /tmp/ship-code.$$ ) & S=$!
wait $R; wait $S
REFUND_CODE=$(cat /tmp/refund-code.$$); SHIP_CODE=$(cat /tmp/ship-code.$$); rm -f /tmp/refund-code.$$ /tmp/ship-code.$$
[ "$REFUND_CODE" = "200" ] || { echo "refund failed: $REFUND_CODE"; exit 1; }
echo "race: refund=$REFUND_CODE ship=$SHIP_CODE (either winner is valid)"
curl -fsS -X POST "$ADMIN/api/admin/v1/shipments/$S21/ship" -H "$AH" -H 'Content-Type: application/json' -d '{}' >/dev/null
wait_status "$O2" SHIPPED
D="$(curl -fsS "$BASE/api/v1/orders/$O2" -H "$UH")"
echo "order $O2 after G2 refund: status=$(echo "$D" | json "d['status']") refund=$(echo "$D" | json "d['refund_state']")"

echo "split concurrency checks passed"
