package order

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/events"
)

// AC-50 (SPEC-0002 v1.11 §4, AC-50/AC-69/AC-70; ADR-0007 Amd1 AC-50 Addendum
// 0.6 §3.1/§3.5): a CONSUMED permit with no waybill row is not a legal AC-50
// branch. Order persists result=REVIEW_REQUIRED,state=PENDING_REMOTE and
// returns the stable REFUND_ADMISSION_CONFLICT with structured details; it must
// not write PRE/POST, release inventory, or call Payment.Refund.

func ac50Service(db *sql.DB, payments paymentPort, ff fulfillmentPort) *Service {
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(payments, ff)
	return svc
}

func ac50SeedOrder(t *testing.T, db *sql.DB, orderNo string, paid int64) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO orders
		(order_no,user_id,status,total_cent,pay_cent,paid_cent,settlement_payment_no,expire_at,applied_promotions_json)
		VALUES (?,1,'PAID',?,?,?,'PAY-AC50',NOW(3),'[]')`, orderNo, paid, paid, paid); err != nil {
		t.Fatalf("seed order %s: %v", orderNo, err)
	}
	if _, err := db.Exec(`INSERT INTO order_items
		(order_no,line_no,sku_id,name,quantity,warehouse_id,fulfillment_group_no,line_paid_cent,unit_paid_cent)
		VALUES (?,1,1,'x',1,1,'G1',?,?)`, orderNo, paid, paid); err != nil {
		t.Fatalf("seed order item %s: %v", orderNo, err)
	}
}

func ac50SeedZeroOrder(t *testing.T, db *sql.DB, orderNo string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO orders
		(order_no,user_id,status,total_cent,pay_cent,paid_cent,settlement_payment_no,expire_at,applied_promotions_json)
		VALUES (?,1,'PAID',100,0,0,CONCAT('zero:',?),NOW(3),'[]')`, orderNo, orderNo); err != nil {
		t.Fatalf("seed zero order %s: %v", orderNo, err)
	}
	if _, err := db.Exec(`INSERT INTO order_items
		(order_no,line_no,sku_id,name,quantity,warehouse_id,fulfillment_group_no,line_paid_cent,unit_paid_cent)
		VALUES (?,1,1,'x',1,1,'G1',0,0)`, orderNo); err != nil {
		t.Fatalf("seed zero order item %s: %v", orderNo, err)
	}
}

func ac50SeedPermit(t *testing.T, db *sql.DB, orderNo, state string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO order_shipping_permits (order_no, fulfillment_group_no, state)
		VALUES (?,?,?)`, orderNo, "G1", state); err != nil {
		t.Fatalf("seed permit %s: %v", orderNo, err)
	}
}

func ac50Admission(t *testing.T, db *sql.DB, refundID string) (result, state string, version int64) {
	t.Helper()
	if err := db.QueryRow(`SELECT result, state, version FROM order_refund_admissions WHERE refund_id = ?`, refundID).
		Scan(&result, &state, &version); err != nil {
		t.Fatalf("load admission %s: %v", refundID, err)
	}
	return result, state, version
}

func ac50RequireReviewConflict(t *testing.T, err error, refundID string) *apperr.Error {
	t.Helper()
	if err == nil {
		t.Fatalf("refund %s: expected REFUND_ADMISSION_CONFLICT, got success", refundID)
	}
	ae := apperr.From(err)
	if ae == nil || ae.Code != apperr.CodeFailedPrecondition {
		t.Fatalf("refund %s: code = %v, want FAILED_PRECONDITION (%v)", refundID, ae, err)
	}
	if !strings.Contains(ae.Message, "REFUND_ADMISSION_CONFLICT") {
		t.Fatalf("refund %s: message = %q, want REFUND_ADMISSION_CONFLICT", refundID, ae.Message)
	}
	want := map[string]string{
		"result":       refundReviewRequired,
		"state":        refundPending,
		"reason":       refundReasonConsumedWithoutShipment,
		"ticket":       "RVW:" + refundID,
		"manual_queue": manualQueueNowb,
	}
	for key, wantValue := range want {
		if got, _ := ae.Details[key].(string); got != wantValue {
			t.Fatalf("refund %s: details[%q] = %q, want %q (details=%v)", refundID, key, got, wantValue, ae.Details)
		}
	}
	return ae
}

// O1: whole-group CONSUMED + no waybill row persists REVIEW_REQUIRED only.
func TestAC50ReviewMapping(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	ff := &fakeFulfillmentPort{status: probeStatusNoWaybill}
	svc := ac50Service(db, payments, ff)
	ctx := context.Background()
	ac50SeedOrder(t, db, "ORD-R50", 10000)
	ac50SeedPermit(t, db, "ORD-R50", permitConsumed)

	_, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-R50", OrderNo: "ORD-R50", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
	})
	ae := ac50RequireReviewConflict(t, err, "R-R50")
	if got, _ := ae.Details["probe_status"].(string); got != probeStatusNoWaybill {
		t.Fatalf("probe_status details = %q, want %q", got, probeStatusNoWaybill)
	}

	result, state, version := ac50Admission(t, db, "R-R50")
	if result != refundReviewRequired || state != refundPending || version != 2 {
		t.Fatalf("admission = %s/%s v%d, want REVIEW_REQUIRED/PENDING_REMOTE v2", result, state, version)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitments
		WHERE refund_id = 'R-R50' AND state = 'RESERVED'`); got != 1 {
		t.Fatalf("RESERVED commitments = %d, want 1", got)
	}
	var permitState string
	if err := db.QueryRow(`SELECT state FROM order_shipping_permits WHERE order_no = 'ORD-R50'`).Scan(&permitState); err != nil {
		t.Fatal(err)
	}
	if permitState != permitConsumed {
		t.Fatalf("permit = %q, want CONSUMED", permitState)
	}
	// Tx1 wrote exactly one placeholder order.refund_admitted; the REVIEW Tx2
	// writes no outbox and no release.
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events
		WHERE aggregate_id = 'ORD-R50' AND event_type = 'order.refund_admitted'`); got != 1 {
		t.Fatalf("refund_admitted events = %d, want 1 placeholder", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events
		WHERE aggregate_id = 'ORD-R50' AND event_type = ?`, events.RefundReleaseEventType); got != 0 {
		t.Fatalf("inventory release events = %d, want 0", got)
	}
	if got := len(payments.refundCalls); got != 0 {
		t.Fatalf("Payment.Refund calls = %d, want 0", got)
	}
	if ff.cancelCalls.Load() != 1 {
		t.Fatalf("cancel probe calls = %d, want 1", ff.cancelCalls.Load())
	}
}

// O7: zero-amount CONSUMED + no waybill row returns the conflict without any
// persistence (no admission/S19/DONE, no REVIEW marker) and without Payment.
func TestAC50ReviewZeroNotPersisted(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{}
	ff := &fakeFulfillmentPort{status: probeStatusNoWaybill}
	svc := ac50Service(db, payments, ff)
	ctx := context.Background()
	ac50SeedZeroOrder(t, db, "ORD-RZ50")
	ac50SeedPermit(t, db, "ORD-RZ50", permitConsumed)

	_, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-RZ50", OrderNo: "ORD-RZ50", GroupNo: "G1", AmountCent: 0, Source: refundSourceSystemZero,
	})
	if err == nil {
		t.Fatal("zero-amount no-waybill: expected REFUND_ADMISSION_CONFLICT")
	}
	ae := apperr.From(err)
	if ae == nil || ae.Code != apperr.CodeFailedPrecondition || !strings.Contains(ae.Message, "REFUND_ADMISSION_CONFLICT") {
		t.Fatalf("zero conflict = %v, want FAILED_PRECONDITION REFUND_ADMISSION_CONFLICT", err)
	}
	if got, _ := ae.Details["review_persisted"].(bool); got {
		t.Fatalf("zero conflict claims review_persisted=true: %v", ae.Details)
	}
	for _, table := range []string{"order_refund_admissions", "refund_applications", "order_refund_commitments"} {
		if got := countRows(t, db, `SELECT COUNT(*) FROM `+table+` WHERE refund_id = 'R-RZ50'`); got != 0 {
			t.Fatalf("zero conflict wrote %s: %d rows", table, got)
		}
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-RZ50'`); got != 0 {
		t.Fatalf("zero conflict wrote %d outbox events", got)
	}
	var permitState string
	if err := db.QueryRow(`SELECT state FROM order_shipping_permits WHERE order_no = 'ORD-RZ50'`).Scan(&permitState); err != nil {
		t.Fatal(err)
	}
	if permitState != permitConsumed {
		t.Fatalf("zero conflict changed permit to %q", permitState)
	}
	if payments.getCalls != 0 || len(payments.refundCalls) != 0 {
		t.Fatalf("zero conflict consulted Payment: get=%d refund=%d", payments.getCalls, len(payments.refundCalls))
	}
}

// O10: legacy (no permit row) keeps the AC-50 NONE->REVOKED PRE branch and
// never probes; a legacy CONSUMED permit still obeys the same mapping.
func TestAC50ReviewLegacy(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	ff := &fakeFulfillmentPort{status: probeStatusNoWaybill}
	svc := ac50Service(db, payments, ff)
	ctx := context.Background()

	// Legacy order without a permit row: whole-group refund => PRE, no probe.
	ac50SeedOrder(t, db, "ORD-RL50", 10000)
	reply, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-RL50", OrderNo: "ORD-RL50", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
	})
	if err != nil {
		t.Fatalf("legacy PRE: %v", err)
	}
	if reply.GetResult() != refundResultPre {
		t.Fatalf("legacy result = %q, want PRE", reply.GetResult())
	}
	if ff.cancelCalls.Load() != 0 {
		t.Fatalf("legacy NONE probe called CancelIfCreated %d times, want 0", ff.cancelCalls.Load())
	}
	var permitState string
	if err := db.QueryRow(`SELECT state FROM order_shipping_permits WHERE order_no = 'ORD-RL50'`).Scan(&permitState); err != nil {
		t.Fatal(err)
	}
	if permitState != permitRevoked {
		t.Fatalf("legacy permit = %q, want REVOKED", permitState)
	}

	// Legacy CONSUMED permit + no waybill row => the same REVIEW decision.
	ac50SeedOrder(t, db, "ORD-RL51", 10000)
	ac50SeedPermit(t, db, "ORD-RL51", permitConsumed)
	if _, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-RL51", OrderNo: "ORD-RL51", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
	}); err == nil {
		t.Fatal("legacy consumed no-waybill: expected REFUND_ADMISSION_CONFLICT")
	} else {
		ac50RequireReviewConflict(t, err, "R-RL51")
	}

	// Legacy CONSUMED permit + already-shipped parcel => POST.
	ac50SeedOrder(t, db, "ORD-RL52", 10000)
	ac50SeedPermit(t, db, "ORD-RL52", permitConsumed)
	ff.setStatus("ALREADY_SHIPPED")
	reply, err = svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-RL52", OrderNo: "ORD-RL52", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
	})
	if err != nil {
		t.Fatalf("legacy shipped: %v", err)
	}
	if reply.GetResult() != refundResultPost {
		t.Fatalf("legacy shipped result = %q, want POST", reply.GetResult())
	}
}

// O2/O2b/O2c: the persisted review is the idempotency anchor. A replay returns
// the same conflict without probing or reading Payment, even when Payment is
// unavailable.
func TestAC50ReviewReplay(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	ff := &fakeFulfillmentPort{status: probeStatusNoWaybill}
	svc := ac50Service(db, payments, ff)
	ctx := context.Background()
	ac50SeedOrder(t, db, "ORD-RR50", 10000)
	ac50SeedPermit(t, db, "ORD-RR50", permitConsumed)

	req := &pb.AdmitRefundRequest{
		RefundId: "R-RR50", OrderNo: "ORD-RR50", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
	}
	if _, err := svc.AdmitRefund(ctx, req); err == nil {
		t.Fatal("first admission: expected conflict")
	}
	_, _, version := ac50Admission(t, db, "R-RR50")
	cancelBefore, getBefore := ff.cancelCalls.Load(), payments.getCalls

	if _, err := svc.AdmitRefund(ctx, req); err == nil {
		t.Fatal("replay: expected the same conflict")
	} else {
		ac50RequireReviewConflict(t, err, "R-RR50")
	}
	if ff.cancelCalls.Load() != cancelBefore {
		t.Fatalf("replay re-probed: cancel calls %d -> %d", cancelBefore, ff.cancelCalls.Load())
	}
	if payments.getCalls != getBefore {
		t.Fatalf("replay read Payment: get calls %d -> %d", getBefore, payments.getCalls)
	}
	if _, _, after := ac50Admission(t, db, "R-RR50"); after != version {
		t.Fatalf("replay changed admission version %d -> %d", version, after)
	}

	// O2c: a preseeded review row must answer before any Payment read, even when
	// Payment is down.
	ac50SeedOrder(t, db, "ORD-RR51", 10000)
	if _, err := db.Exec(`INSERT INTO order_refund_admissions
		(refund_id, order_no, group_no, result, state, amount_cent, version)
		VALUES ('R-RR51','ORD-RR51','G1',?,?,10000,1)`, refundReviewRequired, refundPending); err != nil {
		t.Fatal(err)
	}
	downPayments := &fakePaymentPort{getErr: errors.New("payment unavailable")}
	downFF := &fakeFulfillmentPort{status: probeStatusNoWaybill}
	downSvc := ac50Service(db, downPayments, downFF)
	if _, err := downSvc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-RR51", OrderNo: "ORD-RR51", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
	}); err == nil {
		t.Fatal("preseeded review: expected conflict")
	} else {
		ac50RequireReviewConflict(t, err, "R-RR51")
	}
	if downPayments.getCalls != 0 || downFF.cancelCalls.Load() != 0 {
		t.Fatalf("preseeded review probed: get=%d cancel=%d", downPayments.getCalls, downFF.cancelCalls.Load())
	}
}
