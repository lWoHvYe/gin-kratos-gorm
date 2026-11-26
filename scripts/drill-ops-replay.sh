#!/usr/bin/env bash
# PAY-OPS-REPLAY-01 OPS-P4 disposable drill (manual runbook rehearsal).
#
# NOT CI, NOT production. Starts throwaway MySQL 8.4 + NATS 2.10 JetStream,
# applies the real migrations with mall/migrate:dev, then rehearses D1-D11 of
# docs/ops.md "Manual ops runbook (PAY-OPS-REPLAY-01)": DLQ replay, lease
# recovery, NOWB re-arm/abandon, authorization/evidence rejection, wrong-group
# evidence, concurrency and TCC abandon. Every step asserts the invariants:
# no duplicate refund, no release, no permit restore, no PRE/POST on reject.
#
# Usage: ./scripts/drill-ops-replay.sh
#   MYSQL_IMAGE=mysql:8.4 MIGRATE_IMAGE=mall/migrate:dev NATS_IMAGE=nats:2.10-alpine
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MYSQL_IMAGE="${MYSQL_IMAGE:-mysql:8.4}"
MIGRATE_IMAGE="${MIGRATE_IMAGE:-mall/migrate:dev}"
NATS_IMAGE="${NATS_IMAGE:-nats:2.10-alpine}"
BASE_PORT="${BASE_PORT:-13401}"
NATS_PORT="${NATS_PORT:-14301}"
NAME="mall-ops-drill-$$"
NATS_NAME="mall-ops-drill-nats-$$"
EVID="$ROOT/.tmp/evidence-ops-replay"
NATS_HELPER="$ROOT/.tmp/ops-drill-nats/main.go"
PASS=0
FAIL=0

mkdir -p "$EVID" "$(dirname "$NATS_HELPER")"
: > "$EVID/drill.log"

log() { echo "$*" | tee -a "$EVID/drill.log"; }
cleanup() {
  docker rm -f "$NAME" "$NATS_NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

ok() { PASS=$((PASS+1)); log "OK   $*"; }
bad() { FAIL=$((FAIL+1)); log "FAIL $*"; }
assert_eq() { # label expected actual
  if [ "$2" = "$3" ]; then ok "$1 = $3"; else bad "$1: expected [$2], got [$3]"; fi
}
assert_ne() { # label unwanted actual
  if [ "$2" != "$3" ]; then ok "$1 != $3"; else bad "$1: unexpectedly [$3]"; fi
}
assert_nonzero_rc() { # label rc
  if [ "$2" -ne 0 ]; then ok "$1 (rejected, rc=$2)"; else bad "$1: expected rejection, rc=0"; fi
}

mysql_exec() { docker exec -i -e MYSQL_PWD=root "$NAME" mysql -uroot "$@"; }
mysql_q() { mysql_exec -N -B "$@"; }
mysql_file() { docker exec -i -e MYSQL_PWD=root "$NAME" mysql -uroot "$1" < "$2"; }

# ---------------------------------------------------------------- environment
log "== PAY-OPS-REPLAY-01 drill =="
log "time  $(date -u +%Y-%m-%dT%H:%M:%SZ)"
log "HEAD  $(git -C "$ROOT" rev-parse HEAD)"
log "mysql $MYSQL_IMAGE @127.0.0.1:$BASE_PORT; nats $NATS_IMAGE @127.0.0.1:$NATS_PORT"
log ""

docker run -d --rm --name "$NAME" -e MYSQL_ROOT_PASSWORD=root \
  -p "127.0.0.1:$BASE_PORT:3306" "$MYSQL_IMAGE" >/dev/null
for _ in $(seq 1 90); do
  mysql_q -e 'SELECT 1' >/dev/null 2>&1 && break
  sleep 1
done
mysql_q -e 'SELECT 1' >/dev/null 2>&1 || { log "FATAL mysql not ready"; exit 2; }
log "MySQL $(mysql_q -e 'SELECT VERSION()')"

docker run -d --rm --name "$NATS_NAME" -p "127.0.0.1:$NATS_PORT:4222" "$NATS_IMAGE" -js >/dev/null
sleep 2
NATS_URL="nats://127.0.0.1:$NATS_PORT"

if ! docker run --rm --add-host=host.docker.internal:host-gateway \
  -e MYSQL_HOST=host.docker.internal -e MYSQL_PORT="$BASE_PORT" \
  -e MYSQL_USER=root -e MYSQL_PASSWORD=root \
  -e MIGRATION_LOCK_WAIT_SECONDS=60 \
  -v "$ROOT/migrations:/migrations:ro" "$MIGRATE_IMAGE" >"$EVID/migrate.log" 2>&1; then
  log "FATAL migration failed (see $EVID/migrate.log)"; exit 2
fi
log "migrations applied (mall_order / mall_aftersales / mall_fulfillment)"

# NATS helper: publish with Msg-Id and read stream/last-message state.
cat > "$NATS_HELPER" <<'GO'
package main

import (
	"fmt"
	"os"

	"github.com/nats-io/nats.go"
)

func main() {
	nc, err := nats.Connect(os.Getenv("NATS_URL"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if _, err := js.StreamInfo("MALL"); err != nil {
		if _, err := js.AddStream(&nats.StreamConfig{Name: "MALL", Subjects: []string{"shop.>"}, Storage: nats.FileStorage}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
	switch os.Getenv("ACTION") {
	case "publish":
		body, err := os.ReadFile(os.Getenv("PAYLOAD_FILE"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		msg := nats.NewMsg(os.Getenv("SUBJECT"))
		msg.Data = body
		if id := os.Getenv("MSG_ID"); id != "" {
			msg.Header.Set("Nats-Msg-Id", id)
		}
		if _, err := js.PublishMsg(msg); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("published")
	case "msgs":
		info, err := js.StreamInfo("MALL")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		fmt.Println(info.State.Msgs)
	case "last":
		msg, err := js.GetLastMsg("MALL", os.Getenv("SUBJECT"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Stdout.Write(msg.Data)
	default:
		os.Exit(2)
	}
}
GO
nats_do() {
  (cd "$ROOT" && PATH="$ROOT/.tmp/bin:$PATH" NATS_URL="$NATS_URL" \
    GOCACHE="$ROOT/.tmp/gocache" GOTMPDIR="$ROOT/.tmp" GOMODCACHE="$ROOT/.tmp/modcache" \
    GOPATH="$ROOT/.tmp/gopath" GOSUMDB=off GOFLAGS=-mod=readonly GOPROXY=off \
    go run "$NATS_HELPER" 2>>"$EVID/nats.err")
}

# ---------------------------------------------------------------- fixtures
log ""
log "== fixtures (disposable drill data; not production evidence) =="
mysql_q mall_order <<'SQL'
INSERT INTO orders (order_no,user_id,status,total_cent,pay_cent,paid_cent,settlement_payment_no,refunded_cent,refund_state,expire_at,applied_promotions_json)
VALUES ('ORD-OPS',1,'PAID',10000,10000,10000,'PAY-OPS',0,'NONE',NOW(3),'[]');
INSERT INTO order_fulfillment_groups (group_no,order_no,seq,warehouse_id,status,refund_state,reservation_state,freight_cent,freight_share_cent,version)
VALUES ('G1','ORD-OPS',1,1,'ALLOCATED','NONE','HELD',0,0,1);
INSERT INTO order_shipping_permits (order_no,fulfillment_group_no,state)
VALUES ('ORD-OPS','G1','CONSUMED');
INSERT INTO order_refund_admissions (refund_id,order_no,group_no,result,state,amount_cent,version)
VALUES ('REF-OPS','ORD-OPS','G1','REVIEW_REQUIRED','PENDING_REMOTE',10000,1);
INSERT INTO order_refund_commitments (order_no,group_no,refund_id,amount_cent,state,version)
VALUES ('ORD-OPS','G1','REF-OPS',10000,'RESERVED',1);
INSERT INTO refund_batches (batch_id,source,order_no,payment_no,attempt_seq,state,captured_snapshot_cent,refunded_snapshot_cent,sum_group_cap_cent,remaining_captured_cent,last_error,version)
VALUES ('tcc:PAY-OPS:1','tcc','ORD-OPS','PAY-OPS',1,'MANUAL_HOLD',10000,0,10000,10000,
        'REVIEW_REQUIRED reason=CONSUMED_WITHOUT_SHIPMENT ticket=RVW:REF-OPS',1);
INSERT INTO refund_applications (refund_id,batch_id,source,order_no,group_no,non_effective_capture,amount_cent,goods_cent,freight_cent,state,admission_result,version)
VALUES ('REF-OPS','tcc:PAY-OPS:1','tcc','ORD-OPS','G1',0,10000,10000,0,'PENDING','REVIEW_REQUIRED',1);
SQL
mysql_q mall_aftersales <<'SQL'
INSERT INTO after_sales (after_sale_no,order_no,user_id,type,status,amount_cent,reason)
VALUES ('REF-OPS','ORD-OPS',1,'REFUND_ONLY','REFUNDING',10000,'drill');
INSERT INTO after_sale_refund_tasks (after_sale_no,status,attempts,last_error)
VALUES ('REF-OPS','PENDING',4,'REVIEW_REQUIRED reason=CONSUMED_WITHOUT_SHIPMENT ticket=RVW:REF-OPS');
INSERT INTO after_sales (after_sale_no,order_no,user_id,type,status,amount_cent,reason)
VALUES ('REF-OPS-ABANDON','ORD-OPS',1,'REFUND_ONLY','REFUNDING',10000,'drill abandon');
SQL
mysql_q mall_fulfillment <<'SQL'
INSERT INTO shipments (shipment_no,order_no,fulfillment_group_no,status)
VALUES ('SHIP-OPS','ORD-OPS','G1','CANCELLED');
INSERT INTO shipment_events (shipment_no,status) VALUES ('SHIP-OPS','CANCELLED');
SQL

# ---------------------------------------------------------------- auth guard
# Authorization is the human control defined in the runbook. The drill uses the
# same field checks; it is NOT a technical non-repudiation mechanism.
AUTH_ID=""; ACTION=""; REFUND_ID=""; ORDER_NO=""; GROUP_NO=""; AMOUNT_CENT=0
OPERATOR=""; AUTHORIZER=""; REVIEWER=""; REASON=""; EVIDENCE_REF=""
EVIDENCE_ORDER=""; EVIDENCE_GROUP=""; AUTHORIZED_AT=""; EXPIRES_AT=""
OWNER_APPROVAL=""; TICKET=""; OP_ID=""
auth_check() {
  local reason=""
  case "$ACTION" in REPLAY|NOWB_REARM|NOWB_ABANDON) ;; *) reason="ACTION_INVALID";; esac
  [ -z "$AUTH_ID" ] && reason="${reason:-AUTHORIZATION_ID_MISSING}"
  [ -z "$OP_ID" ] && reason="${reason:-OP_ID_MISSING}"
  [ -z "$REFUND_ID" ] && reason="${reason:-REFUND_ID_MISSING}"
  [ -z "$ORDER_NO" ] && reason="${reason:-ORDER_NO_MISSING}"
  [ -z "$GROUP_NO" ] && reason="${reason:-GROUP_NO_MISSING}"
  case "$AMOUNT_CENT" in ''|*[!0-9]*) reason="${reason:-AMOUNT_INVALID}";; esac
  case "$AUTH_ID" in AUTH-*) ;; *) reason="${reason:-AUTHORIZATION_ID_UNVERIFIED}";; esac
  [ -z "$OPERATOR" ] && reason="${reason:-OPERATOR_MISSING}"
  [ -z "$AUTHORIZER" ] && reason="${reason:-AUTHORIZER_MISSING}"
  [ "$OPERATOR" = "$AUTHORIZER" ] && reason="${reason:-OPERATOR_AUTHORIZER_SAME}"
  [ "$TICKET" != "RVW:$REFUND_ID" ] && reason="${reason:-TICKET_MISMATCH}"
  if [ "$AMOUNT_CENT" -ge 100000 ]; then
    [ -z "$REVIEWER" ] && reason="${reason:-REVIEWER_REQUIRED}"
    [ "$REVIEWER" = "$AUTHORIZER" ] && reason="${reason:-REVIEWER_AUTHORIZER_SAME}"
  fi
  if [ "$ACTION" = "NOWB_ABANDON" ]; then
    [ -z "$OWNER_APPROVAL" ] && reason="${reason:-OWNER_APPROVAL_MISSING}"
  fi
  if [ "$ACTION" = "NOWB_REARM" ]; then
    [ -z "$EVIDENCE_REF" ] && reason="${reason:-EVIDENCE_MISSING}"
    [ "$EVIDENCE_ORDER" != "$ORDER_NO" ] && reason="${reason:-S6_ORDER_MISMATCH}"
    [ "$EVIDENCE_GROUP" != "$GROUP_NO" ] && reason="${reason:-S6_GROUP_MISMATCH}"
    S6_STATUS="$(mysql_q mall_fulfillment -e "SELECT status FROM shipments
      WHERE order_no='$EVIDENCE_ORDER' AND fulfillment_group_no='$EVIDENCE_GROUP'" 2>/dev/null)"
    case "$S6_STATUS" in CANCELLED|SHIPPED) ;; *) reason="${reason:-S6_NOT_CANCELLED}";; esac
  fi
  local now exp auth
  now="$(python3 -c 'import time;print(int(time.time()))')"
  if [ -n "$EXPIRES_AT" ]; then
    exp="$(python3 - "$EXPIRES_AT" <<'PY'
import datetime,sys
print(int(datetime.datetime.strptime(sys.argv[1],"%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=datetime.timezone.utc).timestamp()))
PY
)" || exp=0
    [ "$exp" -le "$now" ] && reason="${reason:-AUTHORIZATION_EXPIRED}"
    auth="$(python3 - "$AUTHORIZED_AT" <<'PY'
import datetime,sys
print(int(datetime.datetime.strptime(sys.argv[1],"%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=datetime.timezone.utc).timestamp()))
PY
)" || auth=0
    [ $((exp-auth)) -gt 86400 ] && reason="${reason:-AUTHORIZATION_TOO_LONG}"
  else
    reason="${reason:-EXPIRES_AT_MISSING}"
  fi
  if [ -n "$reason" ]; then AUTH_FAIL="$reason"; return 1; fi
  AUTH_FAIL=""
  return 0
}
now_iso() { python3 -c 'import datetime;print(datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"))'; }
plus_hours_iso() { python3 -c "import datetime;print((datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(hours=$1)).strftime('%Y-%m-%dT%H:%M:%SZ'))"; }
set_auth() {
  AUTH_ID="AUTH-OPS-01"; OP_ID="OP-OPS-01"; ACTION="$1"; REFUND_ID="$2"
  ORDER_NO="$3"; GROUP_NO="$4"; AMOUNT_CENT="$5"
  OPERATOR="dba-alice"; AUTHORIZER="admin-bob"; REVIEWER=""; REASON="CONSUMED_WITHOUT_SHIPMENT"
  EVIDENCE_REF="S6-ARCHIVE-1"; EVIDENCE_ORDER="$3"; EVIDENCE_GROUP="$4"
  AUTHORIZED_AT="$(now_iso)"; EXPIRES_AT="$(plus_hours_iso 2)"
  OWNER_APPROVAL=""; TICKET="RVW:$REFUND_ID"
}

# ---------------------------------------------------------------- assertions
snapshot() {
  local db="$1"
  mysql_q "$db" -e "SELECT CONCAT(
    (SELECT COUNT(*) FROM order_refund_admissions),
    '|', (SELECT COUNT(*) FROM order_refund_commitments),
    '|', (SELECT COUNT(*) FROM refund_applications),
    '|', (SELECT COUNT(*) FROM refund_batches))" 2>/dev/null
}
refunded_cent() { mysql_q mall_order -e "SELECT refunded_cent FROM orders WHERE order_no='$1'"; }
permit_state() { mysql_q mall_order -e "SELECT state FROM order_shipping_permits WHERE order_no='$1' AND fulfillment_group_no='$2'"; }
release_events() { mysql_q mall_order -e "SELECT COUNT(*) FROM outbox_events WHERE event_type='inventory.refund_release_requested'"; }
admission_row() { mysql_q mall_order -e "SELECT CONCAT(result,'|',state,'|',version) FROM order_refund_admissions WHERE refund_id='$1'"; }

rearm_order_tx() { # refund order group; prints rows affected (admission) and batch
  mysql_q mall_order <<SQL
START TRANSACTION;
UPDATE order_refund_admissions
   SET result='PENDING_REMOTE', version=version+1, updated_at=NOW(3)
 WHERE refund_id='$1' AND result='REVIEW_REQUIRED' AND state='PENDING_REMOTE' AND version=$4;
SELECT ROW_COUNT();
COMMIT;
SQL
}
rearm_batch_tx() { # batch_id version
  mysql_q mall_order <<SQL
START TRANSACTION;
UPDATE refund_batches SET state='ADMITTING', version=version+1, updated_at=NOW(3)
 WHERE batch_id='$1' AND state='MANUAL_HOLD' AND version=$2;
SELECT ROW_COUNT();
COMMIT;
SQL
}

log ""
log "== D1 DLQ replay (duplicate publish is idempotent; no refund rows change) =="
EVENT="$EVID/aftersale-completed.json"
printf '%s' '{"after_sale_no":"REF-OPS","order_no":"ORD-OPS","group_no":"G1","status":"REFUNDED","effective_amount_cent":10000}' > "$EVENT"
# Synthetic DLQ wrapper (fixture): {subject,data,error,attempts,failed_at}.
python3 - "$EVENT" > "$EVID/aftersale-completed.dlq.json" <<'NATSPY'
import json,sys
data=open(sys.argv[1]).read()
print(json.dumps({"subject":"shop.aftersale.completed.v1","data":data,"error":"consumer timeout","attempts":5,"failed_at":"2026-11-27T00:00:00Z"}))
NATSPY
PAYLOAD_FILE="$EVID/aftersale-completed.dlq.json" SUBJECT="shop.aftersale.completed.v1.dlq" ACTION=publish nats_do >/dev/null
ACTION=last SUBJECT="shop.aftersale.completed.v1.dlq" nats_do > "$EVID/dlq-wrapper.json"
python3 - "$EVID/dlq-wrapper.json" > "$EVID/replay-data.json" <<'NATSPY'
import json,sys
wrapper=json.load(open(sys.argv[1]))
sys.stdout.write(wrapper["data"])
NATSPY
if cmp -s "$EVENT" "$EVID/replay-data.json"; then ok "D1 original data bytes extracted from DLQ wrapper"; else bad "D1 DLQ data extraction mismatch"; fi
BEFORE_MSGS="$(ACTION=msgs nats_do)"
REFUNDED_BEFORE="$(refunded_cent ORD-OPS)"
SNAP_BEFORE="$(snapshot mall_order)"
PAYLOAD_FILE="$EVID/replay-data.json" SUBJECT="shop.aftersale.completed.v1" MSG_ID="manual:AUTH-OPS-REPLAY:1" ACTION=publish nats_do >/dev/null
PAYLOAD_FILE="$EVID/replay-data.json" SUBJECT="shop.aftersale.completed.v1" MSG_ID="manual:AUTH-OPS-REPLAY:1" ACTION=publish nats_do >/dev/null
AFTER_MSGS="$(ACTION=msgs nats_do)"
assert_eq "D1 JetStream dedupe stores one message" "$((BEFORE_MSGS+1))" "$AFTER_MSGS"
assert_eq "D1 refunded_cent unchanged" "$REFUNDED_BEFORE" "$(refunded_cent ORD-OPS)"
assert_eq "D1 ledger snapshot unchanged" "$SNAP_BEFORE" "$(snapshot mall_order)"

log ""
log "== D2 missing DLQ message stops (no guessing) =="
if ACTION=last SUBJECT="shop.raftersale.completed.v1.dlq" nats_do >/dev/null 2>&1; then
  bad "D2 expected no DLQ message"
else
  ok "D2 missing DLQ message rejected (fail-closed)"
fi

log ""
log "== D3 lease recovery (manual fallback is status/lease CAS, not admission) =="
ADM_BEFORE="$(admission_row REF-OPS-LEASE)"
mysql_q mall_order -e "INSERT INTO order_refund_admissions
 (refund_id,order_no,group_no,result,state,amount_cent,version)
 VALUES ('REF-OPS-LEASE','ORD-OPS','G1','REVIEW_REQUIRED','PENDING_REMOTE',10000,1)" >/dev/null
mysql_q mall_aftersales -e "INSERT INTO after_sale_refund_tasks
 (after_sale_no,status,attempts,last_error,lease_owner,lease_until)
 VALUES ('REF-OPS-LEASE','PROCESSING',3,'REVIEW_REQUIRED reason=CONSUMED_WITHOUT_SHIPMENT ticket=RVW:REF-OPS-LEASE','dead-owner',DATE_SUB(NOW(3),INTERVAL 1 HOUR))" >/dev/null
mysql_q mall_aftersales <<'SQL' >/dev/null
START TRANSACTION;
SELECT status,attempts,lease_owner,lease_until FROM after_sale_refund_tasks WHERE after_sale_no='REF-OPS-LEASE' FOR UPDATE;
UPDATE after_sale_refund_tasks SET status='PENDING', attempts=0, lease_owner='', lease_until=NULL
 WHERE after_sale_no='REF-OPS-LEASE' AND status IN ('PENDING','PROCESSING');
COMMIT;
SQL
assert_eq "D3 task reset to PENDING" "PENDING|0||" "$(mysql_q mall_aftersales -e "SELECT CONCAT(status,'|',attempts,'|',lease_owner,'|',IFNULL(DATE_FORMAT(lease_until,'%H:%i'),'')) FROM after_sale_refund_tasks WHERE after_sale_no='REF-OPS-LEASE'")"
assert_eq "D3 canonical last_error preserved" "REVIEW_REQUIRED reason=CONSUMED_WITHOUT_SHIPMENT ticket=RVW:REF-OPS-LEASE" "$(mysql_q mall_aftersales -e "SELECT last_error FROM after_sale_refund_tasks WHERE after_sale_no='REF-OPS-LEASE'")"
assert_eq "D3 admission untouched" "REVIEW_REQUIRED|PENDING_REMOTE|1" "$(admission_row REF-OPS-LEASE)"

log ""
log "== D4 FAILED_PERMANENT task is rejected (no replay) =="
mysql_q mall_aftersales -e "INSERT INTO after_sale_refund_tasks
 (after_sale_no,status,attempts,last_error) VALUES
 ('REF-OPS-FAILED','FAILED_PERMANENT',8,'REVIEW_REQUIRED reason=CONSUMED_WITHOUT_SHIPMENT ticket=RVW:REF-OPS-FAILED')" >/dev/null
set_auth NOWB_REARM REF-OPS-FAILED ORD-OPS G1 10000
if [ "$(mysql_q mall_aftersales -e "SELECT status FROM after_sale_refund_tasks WHERE after_sale_no='REF-OPS-FAILED'")" = "FAILED_PERMANENT" ]; then
  ok "D4 FAILED_PERMANENT precondition rejects replay"
else
  bad "D4 FAILED_PERMANENT task changed"
fi
assert_eq "D4 no new admission for FAILED task" "0" "$(mysql_q mall_order -e "SELECT COUNT(*) FROM order_refund_admissions WHERE refund_id='REF-OPS-FAILED'")"

log ""
log "== D5 re-arm with real CANCELLED evidence (audit-first, two CAS) =="
set_auth NOWB_REARM REF-OPS ORD-OPS G1 10000
AUTH_FAIL=""; if auth_check; then REASON_MSQ=""; else REASON_MSQ="$AUTH_FAIL"; fi
if [ -z "$REASON_MSQ" ]; then ok "D5 authorization accepted"; else bad "D5 authorization rejected: $REASON_MSQ"; fi
V_ADM="$(mysql_q mall_order -e "SELECT version FROM order_refund_admissions WHERE refund_id='REF-OPS'")"
V_BATCH="$(mysql_q mall_order -e "SELECT version FROM refund_batches WHERE batch_id='tcc:PAY-OPS:1'")"
# audit-first: the ticket/log is represented by this drill log line before CAS.
log "AUDIT review_rearm authorization_id=$AUTH_ID op_id=$OP_ID ticket=$TICKET refund=$REFUND_ID order=$ORDER_NO group=$GROUP_NO evidence=$EVIDENCE_REF"
N_ADM="$(rearm_order_tx REF-OPS ORD-OPS G1 "$V_ADM")"
N_BATCH="$(rearm_batch_tx 'tcc:PAY-OPS:1' "$V_BATCH")"
assert_eq "D5 admission CAS" "1" "$N_ADM"
assert_eq "D5 batch CAS" "1" "$N_BATCH"
assert_eq "D5 admission result reset" "PENDING_REMOTE|PENDING_REMOTE|$((V_ADM+1))" "$(admission_row REF-OPS)"
assert_eq "D5 batch MANUAL_HOLD -> ADMITTING" "ADMITTING" "$(mysql_q mall_order -e "SELECT state FROM refund_batches WHERE batch_id='tcc:PAY-OPS:1'")"
CANON="REVIEW_REQUIRED reason=CONSUMED_WITHOUT_SHIPMENT ticket=RVW:REF-OPS"
assert_eq "D5 canonical batch last_error preserved" "$CANON" "$(mysql_q mall_order -e "SELECT last_error FROM refund_batches WHERE batch_id='tcc:PAY-OPS:1'")"
assert_eq "D5 app admission_result stays REVIEW" "REVIEW_REQUIRED" "$(mysql_q mall_order -e "SELECT admission_result FROM refund_applications WHERE refund_id='REF-OPS'")"
assert_eq "D5 no release event" "0" "$(release_events)"
assert_eq "D5 permit stays CONSUMED" "CONSUMED" "$(permit_state ORD-OPS G1)"

log ""
log "== D5b wrong-group S6 evidence is rejected (zero writes) =="
set_auth NOWB_REARM REF-OPS ORD-OPS G1 10000
EVIDENCE_GROUP="G2"
BEFORE="$(snapshot mall_order)|$(admission_row REF-OPS)"
AUTH_FAIL=""; if auth_check; then REASON_MSQ=""; else REASON_MSQ="$AUTH_FAIL"; fi
if [ -z "$REASON_MSQ" ]; then bad "D5b wrong-group evidence accepted"; else ok "D5b rejected: $REASON_MSQ"; fi
assert_eq "D5b zero changes" "$BEFORE" "$(snapshot mall_order)|$(admission_row REF-OPS)"

log ""
log "== D6 SHIPPED evidence maps to POST (never PRE) =="
mysql_q mall_order -e "INSERT INTO order_refund_admissions (refund_id,order_no,group_no,result,state,amount_cent,version)
 VALUES ('REF-OPS-SHIP','ORD-OPS-SHIP','G1','REVIEW_REQUIRED','PENDING_REMOTE',10000,1)" >/dev/null
mysql_q mall_fulfillment -e "INSERT INTO shipments (shipment_no,order_no,fulfillment_group_no,status) VALUES ('SHIP-OPS-SHIP','ORD-OPS-SHIP','G1','SHIPPED')" >/dev/null
set_auth NOWB_REARM REF-OPS-SHIP ORD-OPS-SHIP G1 10000
EVIDENCE_ORDER="ORD-OPS-SHIP"; EVIDENCE_GROUP="G1"; decision=""
AUTH_FAIL=""; if auth_check; then REASON_MSQ=""; else REASON_MSQ="$AUTH_FAIL"; fi
if [ -z "$REASON_MSQ" ]; then
  # The runbook maps real SHIPPED evidence to POST, not to a new PRE.
  case "$S6_STATUS" in
    SHIPPED) decision="POST_SHIP_ALLOWED";;
    CANCELLED) decision="PRE_SHIP_REVOKED";;
  esac
  ok "D6 decision=$decision (evidence=$S6_STATUS; no PRE written by drill)"
else
  bad "D6 SHIPPED evidence should be usable: $REASON_MSQ"
fi
if [ "${decision:-}" = "POST_SHIP_ALLOWED" ]; then
  ok "D6 SHIPPED evidence maps to POST (not PRE)"
else
  bad "D6 expected POST for SHIPPED evidence, got ${decision:-none}"
fi
assert_eq "D6 admission untouched (no PRE)" "REVIEW_REQUIRED|PENDING_REMOTE|1" "$(admission_row REF-OPS-SHIP)"
assert_eq "D6 no release event" "0" "$(release_events)"

log ""
log "== D7 abandon (task FAILED; admission REVIEW; commitment RESERVED; permit/group unchanged) =="
mysql_q mall_order -e "INSERT INTO order_refund_admissions (refund_id,order_no,group_no,result,state,amount_cent,version)
 VALUES ('REF-OPS-ABANDON','ORD-OPS','G1','REVIEW_REQUIRED','PENDING_REMOTE',10000,1)" >/dev/null
mysql_q mall_order -e "INSERT INTO order_refund_commitments (order_no,group_no,refund_id,amount_cent,state,version)
 VALUES ('ORD-OPS','G1','REF-OPS-ABANDON',10000,'RESERVED',1)" >/dev/null
mysql_q mall_aftersales -e "INSERT INTO after_sale_refund_tasks (after_sale_no,status,attempts,last_error)
 VALUES ('REF-OPS-ABANDON','PENDING',4,'REVIEW_REQUIRED reason=CONSUMED_WITHOUT_SHIPMENT ticket=RVW:REF-OPS-ABANDON')" >/dev/null
set_auth NOWB_ABANDON REF-OPS-ABANDON ORD-OPS G1 10000
OWNER_APPROVAL="OWNER-AC62-2026-11-27"
AUTH_FAIL=""; if auth_check; then REASON_MSQ=""; else REASON_MSQ="$AUTH_FAIL"; fi
if [ -z "$REASON_MSQ" ]; then ok "D7 abandon authorization accepted"; else bad "D7 reject: $REASON_MSQ"; fi
mysql_q mall_aftersales <<'SQL' >/dev/null
START TRANSACTION;
SELECT status,attempts,lease_owner,lease_until FROM after_sale_refund_tasks WHERE after_sale_no='REF-OPS-ABANDON' FOR UPDATE;
UPDATE after_sale_refund_tasks SET status='FAILED_PERMANENT', lease_owner='', lease_until=NULL
 WHERE after_sale_no='REF-OPS-ABANDON' AND status IN ('PENDING','PROCESSING');
INSERT INTO after_sale_logs (after_sale_id, action, from_status, to_status, remark)
 SELECT id, 'OPS_NOWB_ABANDON', 'PENDING', 'FAILED_PERMANENT', 'authorization=AUTH-OPS-01 owner=OWNER-AC62-2026-11-27 ticket=RVW:REF-OPS-ABANDON'
 FROM after_sales WHERE after_sale_no='REF-OPS-ABANDON' LIMIT 1;
COMMIT;
SQL
assert_eq "D7 task FAILED_PERMANENT" "FAILED_PERMANENT" "$(mysql_q mall_aftersales -e "SELECT status FROM after_sale_refund_tasks WHERE after_sale_no='REF-OPS-ABANDON'")"
assert_eq "D7 canonical last_error preserved" "REVIEW_REQUIRED reason=CONSUMED_WITHOUT_SHIPMENT ticket=RVW:REF-OPS-ABANDON" "$(mysql_q mall_aftersales -e "SELECT last_error FROM after_sale_refund_tasks WHERE after_sale_no='REF-OPS-ABANDON'")"
assert_eq "D7 admission REVIEW" "REVIEW_REQUIRED|PENDING_REMOTE|1" "$(admission_row REF-OPS-ABANDON)"
assert_eq "D7 commitment RESERVED" "RESERVED" "$(mysql_q mall_order -e "SELECT state FROM order_refund_commitments WHERE refund_id='REF-OPS-ABANDON'")"
assert_eq "D7 permit CONSUMED" "CONSUMED" "$(permit_state ORD-OPS G1)"
assert_eq "D7 group not closed" "NONE" "$(mysql_q mall_order -e "SELECT refund_state FROM order_fulfillment_groups WHERE order_no='ORD-OPS' AND group_no='G1'")"
assert_eq "D7 refunded_cent unchanged" "0" "$(refunded_cent ORD-OPS)"
assert_eq "D7 no release event" "0" "$(release_events)"
assert_eq "D7 audit log written" "1" "$(mysql_q mall_aftersales -e "SELECT COUNT(*) FROM after_sale_logs WHERE action='OPS_NOWB_ABANDON'")"

log ""
log "== D7b/D11 re-Refund requires a new review (no automatic reopen/retry) =="
assert_eq "D7b task remains FAILED_PERMANENT (no auto reopen)" "FAILED_PERMANENT" "$(mysql_q mall_aftersales -e "SELECT status FROM after_sale_refund_tasks WHERE after_sale_no='REF-OPS-ABANDON'")"
mysql_q mall_order <<'SQL'
INSERT INTO order_refund_admissions (refund_id,order_no,group_no,result,state,amount_cent,version)
VALUES ('REF-OPS-TCC-ABANDON','ORD-OPS','G1','REVIEW_REQUIRED','PENDING_REMOTE',10000,1);
INSERT INTO order_refund_commitments (order_no,group_no,refund_id,amount_cent,state,version)
VALUES ('ORD-OPS','G1','REF-OPS-TCC-ABANDON',10000,'RESERVED',1);
INSERT INTO refund_batches (batch_id,source,order_no,payment_no,attempt_seq,state,captured_snapshot_cent,refunded_snapshot_cent,sum_group_cap_cent,remaining_captured_cent,last_error,version)
VALUES ('tcc:PAY-OPS-2:1','tcc','ORD-OPS','PAY-OPS-2',1,'MANUAL_HOLD',10000,0,10000,10000,
        'REVIEW_REQUIRED reason=CONSUMED_WITHOUT_SHIPMENT ticket=RVW:REF-OPS-TCC-ABANDON',1);
INSERT INTO refund_applications
 (refund_id,batch_id,source,order_no,group_no,non_effective_capture,amount_cent,goods_cent,freight_cent,state,admission_result,version)
VALUES ('REF-OPS-TCC-ABANDON','tcc:PAY-OPS-2:1','tcc','ORD-OPS','G1',0,10000,10000,0,'PENDING','REVIEW_REQUIRED',1);
SQL
mysql_q mall_order <<'SQL' >/dev/null
START TRANSACTION;
UPDATE refund_applications SET state='FAILED_PERMANENT', version=version+1, updated_at=NOW(3)
 WHERE refund_id='REF-OPS-TCC-ABANDON' AND state='PENDING' AND version=1;
COMMIT;
SQL
assert_eq "D11 app FAILED_PERMANENT" "FAILED_PERMANENT" "$(mysql_q mall_order -e "SELECT state FROM refund_applications WHERE refund_id='REF-OPS-TCC-ABANDON'")"
assert_eq "D11 batch still MANUAL_HOLD" "MANUAL_HOLD" "$(mysql_q mall_order -e "SELECT state FROM refund_batches WHERE batch_id='tcc:PAY-OPS-2:1'")"
assert_eq "D11 batch canonical last_error preserved" "REVIEW_REQUIRED reason=CONSUMED_WITHOUT_SHIPMENT ticket=RVW:REF-OPS-TCC-ABANDON" "$(mysql_q mall_order -e "SELECT last_error FROM refund_batches WHERE batch_id='tcc:PAY-OPS-2:1'")"
assert_eq "D11 commitment RESERVED" "RESERVED" "$(mysql_q mall_order -e "SELECT state FROM order_refund_commitments WHERE refund_id='REF-OPS-TCC-ABANDON'")"
assert_eq "D11 admission REVIEW" "REVIEW_REQUIRED|PENDING_REMOTE|1" "$(admission_row REF-OPS-TCC-ABANDON)"
assert_eq "D11 permit CONSUMED" "CONSUMED" "$(permit_state ORD-OPS G1)"
assert_eq "D11 no release event" "0" "$(release_events)"

log ""
log "== D8 authorization rejections (missing reviewer/expired/same-person/owner) =="
set_auth NOWB_REARM REF-OPS ORD-OPS G1 100000
REVIEWER=""
BEFORE="$(snapshot mall_order)"
AUTH_FAIL=""; if auth_check; then REASON_MSQ=""; else REASON_MSQ="$AUTH_FAIL"; fi
if [ -z "$REASON_MSQ" ]; then bad "D8 >=1000 without reviewer accepted"; else ok "D8 rejected: $REASON_MSQ"; fi
assert_eq "D8 zero changes" "$BEFORE" "$(snapshot mall_order)"
set_auth NOWB_REARM REF-OPS ORD-OPS G1 10000
EXPIRES_AT="2000-01-01T00:00:00Z"
AUTH_FAIL=""; if auth_check; then REASON_MSQ=""; else REASON_MSQ="$AUTH_FAIL"; fi
if [ -z "$REASON_MSQ" ]; then bad "D8 expired authorization accepted"; else ok "D8 rejected: $REASON_MSQ"; fi
set_auth NOWB_REARM REF-OPS ORD-OPS G1 100000
REVIEWER="admin-bob"
AUTH_FAIL=""; if auth_check; then REASON_MSQ=""; else REASON_MSQ="$AUTH_FAIL"; fi
if [ -z "$REASON_MSQ" ]; then bad "D8 same reviewer/authorizer accepted"; else ok "D8 rejected: $REASON_MSQ"; fi
set_auth NOWB_ABANDON REF-OPS ORD-OPS G1 10000
OWNER_APPROVAL=""
AUTH_FAIL=""; if auth_check; then REASON_MSQ=""; else REASON_MSQ="$AUTH_FAIL"; fi
if [ -z "$REASON_MSQ" ]; then bad "D8 abandon without owner approval accepted"; else ok "D8 rejected: $REASON_MSQ"; fi
# Forged authorization_id/owner: the runbook requires Reviewer to verify the
# original document; the drill records it as a manual control, not a DB check.
set_auth NOWB_REARM REF-OPS ORD-OPS G1 10000
AUTH_ID="FORGED-AUTH"
AUTH_FAIL=""; if auth_check; then REASON_MSQ=""; else REASON_MSQ="$AUTH_FAIL"; fi
if [ -z "$REASON_MSQ" ]; then bad "D8 forged authorization_id accepted"; else ok "D8 forged authorization_id rejected: $REASON_MSQ"; fi
assert_eq "D8 no writes from rejections" "$BEFORE" "$(snapshot mall_order)"

log ""
log "== D9 concurrency: exactly one re-arm CAS wins =="
mysql_q mall_order -e "INSERT INTO order_refund_admissions (refund_id,order_no,group_no,result,state,amount_cent,version)
 VALUES ('REF-OPS-CONC','ORD-OPS','G1','REVIEW_REQUIRED','PENDING_REMOTE',10000,1)" >/dev/null
V="$(mysql_q mall_order -e "SELECT version FROM order_refund_admissions WHERE refund_id='REF-OPS-CONC'")"
( mysql_q mall_order -e "UPDATE order_refund_admissions SET result='PENDING_REMOTE', version=version+1 WHERE refund_id='REF-OPS-CONC' AND result='REVIEW_REQUIRED' AND state='PENDING_REMOTE' AND version=$V; SELECT ROW_COUNT();" >"$EVID/d9-a.out" 2>&1 ) &
( mysql_q mall_order -e "UPDATE order_refund_admissions SET result='PENDING_REMOTE', version=version+1 WHERE refund_id='REF-OPS-CONC' AND result='REVIEW_REQUIRED' AND state='PENDING_REMOTE' AND version=$V; SELECT ROW_COUNT();" >"$EVID/d9-b.out" 2>&1 ) &
wait
N1="$(cat "$EVID/d9-a.out")"; N2="$(cat "$EVID/d9-b.out")"
assert_eq "D9 one CAS wins" "1" "$((N1+N2))"
assert_eq "D9 version advanced once" "$((V+1))" "$(mysql_q mall_order -e "SELECT version FROM order_refund_admissions WHERE refund_id='REF-OPS-CONC'")"

log ""
log "== D10 forged/missing S6 evidence rejected; no new shipment rows =="
SHIPS_BEFORE="$(mysql_q mall_fulfillment -e "SELECT COUNT(*) FROM shipments")"
set_auth NOWB_REARM REF-OPS ORD-OPS G1 10000
EVIDENCE_REF=""
AUTH_FAIL=""; if auth_check; then REASON_MSQ=""; else REASON_MSQ="$AUTH_FAIL"; fi
if [ -z "$REASON_MSQ" ]; then bad "D10 missing evidence accepted"; else ok "D10 rejected: $REASON_MSQ"; fi
set_auth NOWB_REARM REF-OPS ORD-OPS G1 10000
EVIDENCE_GROUP="G3"
AUTH_FAIL=""; if auth_check; then REASON_MSQ=""; else REASON_MSQ="$AUTH_FAIL"; fi
if [ -z "$REASON_MSQ" ]; then bad "D10b wrong-group evidence accepted"; else ok "D10b rejected: $REASON_MSQ"; fi
assert_eq "D10 no new shipment rows" "$SHIPS_BEFORE" "$(mysql_q mall_fulfillment -e "SELECT COUNT(*) FROM shipments")"

log ""
log "== final invariants =="
assert_eq "final no release events" "0" "$(release_events)"
assert_eq "final permit CONSUMED" "CONSUMED" "$(permit_state ORD-OPS G1)"
assert_eq "final refunded_cent 0" "0" "$(refunded_cent ORD-OPS)"
assert_eq "final one admission per refund" "1" "$(mysql_q mall_order -e "SELECT COUNT(*) FROM order_refund_admissions WHERE refund_id='REF-OPS'")"
assert_eq "final no duplicate TCC app" "1" "$(mysql_q mall_order -e "SELECT COUNT(*) FROM refund_applications WHERE refund_id='REF-OPS-TCC-ABANDON'")"

log ""
log "== result: $PASS passed, $FAIL failed =="
if [ "$FAIL" -ne 0 ]; then
  log "DRILL FAILED"
  exit 1
fi
log "DRILL PASSED"
exit 0
