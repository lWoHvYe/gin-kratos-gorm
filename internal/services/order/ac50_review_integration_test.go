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
	// AC-69/O19: ordinary after-sales has no refund_applications row and the
	// review must never create one (the mirror is the AfterSales task).
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_applications WHERE refund_id = 'R-R50'`); got != 0 {
		t.Fatalf("after-sales review created %d refund_applications rows", got)
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
	ac50SeedPermit(t, db, "ORD-RR51", permitConsumed)
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

// T3 (AC-50/AC-70, ADR-0007 Amd1 AC-50 Addendum 0.6 §3.4): a CONSUMED permit
// with a partial amount is classified only from Order-owned group facts.
// CancelIfCreated is never called, PARTIAL leaves zero rows, and the
// unclassifiable case freezes one RESERVED commitment with a review.

func ac50SeedOrderStatus(t *testing.T, db *sql.DB, orderNo string, paid int64, status string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO orders
		(order_no,user_id,status,total_cent,pay_cent,paid_cent,settlement_payment_no,expire_at,applied_promotions_json)
		VALUES (?,1,?,?,?,?,'PAY-AC50',NOW(3),'[]')`, orderNo, status, paid, paid, paid); err != nil {
		t.Fatalf("seed order %s: %v", orderNo, err)
	}
	if _, err := db.Exec(`INSERT INTO order_items
		(order_no,line_no,sku_id,name,quantity,warehouse_id,fulfillment_group_no,line_paid_cent,unit_paid_cent)
		VALUES (?,1,1,'x',1,1,'G1',?,?)`, orderNo, paid, paid); err != nil {
		t.Fatalf("seed order item %s: %v", orderNo, err)
	}
}

func ac50SeedGroup(t *testing.T, db *sql.DB, orderNo, status string, freight int64) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO order_fulfillment_groups
		(group_no, order_no, seq, warehouse_id, status, refund_state, reservation_state,
		 freight_cent, freight_share_cent, version)
		VALUES ('G1', ?, 1, 1, ?, 'NONE', 'HELD', ?, ?, 1)`, orderNo, status, freight, freight); err != nil {
		t.Fatalf("seed group %s: %v", orderNo, err)
	}
}

func TestAC50ReviewPartialUnshipped(t *testing.T) {
	// O4a: non-CONSUMED partial (unshipped) is rejected before any write.
	db := orderTestDB(t)
	ff := &fakeFulfillmentPort{status: "CANCELLED"}
	svc := ac50Service(db, &fakePaymentPort{captured: 10000}, ff)
	ctx := context.Background()
	ac50SeedOrder(t, db, "ORD-PA", 10000)
	if _, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-PA", OrderNo: "ORD-PA", GroupNo: "G1", AmountCent: 5000, Source: "aftersale",
	}); err == nil {
		t.Fatal("partial unshipped: expected PARTIAL_REFUND_REQUIRES_FULL_GROUP")
	} else if !strings.Contains(apperr.From(err).Message, "partial refund of an unshipped group") {
		t.Fatalf("error = %v, want the PARTIAL business error", err)
	}
	for _, table := range []string{"order_refund_admissions", "order_refund_commitments"} {
		if got := countRows(t, db, `SELECT COUNT(*) FROM `+table+` WHERE refund_id = 'R-PA'`); got != 0 {
			t.Fatalf("O4a wrote %s: %d", table, got)
		}
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-PA'`); got != 0 {
		t.Fatalf("O4a wrote %d outbox events", got)
	}
	if ff.cancelCalls.Load() != 0 {
		t.Fatalf("O4a probed CancelIfCreated: %d", ff.cancelCalls.Load())
	}
}

func TestAC50ReviewPartialConsumedUnshipped(t *testing.T) {
	// O4b: CONSUMED + partial + projection unshipped: zero writes, no probe
	// (a real CREATED row would remain untouched because the fake is never
	// called).
	db := orderTestDB(t)
	ff := &fakeFulfillmentPort{status: "CANCELLED"}
	svc := ac50Service(db, &fakePaymentPort{captured: 10000}, ff)
	ctx := context.Background()
	ac50SeedOrder(t, db, "ORD-PB", 10000)
	ac50SeedGroup(t, db, "ORD-PB", "ALLOCATED", 0)
	ac50SeedPermit(t, db, "ORD-PB", permitConsumed)
	if _, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-PB", OrderNo: "ORD-PB", GroupNo: "G1", AmountCent: 5000, Source: "aftersale",
	}); err == nil {
		t.Fatal("partial consumed unshipped: expected PARTIAL")
	} else if !strings.Contains(apperr.From(err).Message, "partial refund of an unshipped group") {
		t.Fatalf("error = %v, want PARTIAL", err)
	}
	for _, table := range []string{"order_refund_admissions", "order_refund_commitments"} {
		if got := countRows(t, db, `SELECT COUNT(*) FROM `+table+` WHERE refund_id = 'R-PB'`); got != 0 {
			t.Fatalf("O4b wrote %s: %d", table, got)
		}
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-PB'`); got != 0 {
		t.Fatalf("O4b wrote %d outbox events", got)
	}
	if ff.cancelCalls.Load() != 0 {
		t.Fatalf("O4b probed CancelIfCreated: %d (must not classify via cancel)", ff.cancelCalls.Load())
	}
}

func TestAC50ReviewPartialShipped(t *testing.T) {
	// O4c: CONSUMED + partial + projection SHIPPED => POST, one commitment,
	// no probe, no release.
	db := orderTestDB(t)
	ff := &fakeFulfillmentPort{status: "CANCELLED"}
	svc := ac50Service(db, &fakePaymentPort{captured: 10000}, ff)
	ctx := context.Background()
	ac50SeedOrder(t, db, "ORD-PC", 10000)
	ac50SeedGroup(t, db, "ORD-PC", "SHIPPED", 0)
	ac50SeedPermit(t, db, "ORD-PC", permitConsumed)
	reply, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-PC", OrderNo: "ORD-PC", GroupNo: "G1", AmountCent: 5000, Source: "aftersale",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetResult() != refundResultPost {
		t.Fatalf("result = %q, want POST", reply.GetResult())
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitments WHERE refund_id = 'R-PC' AND state = 'RESERVED'`); got != 1 {
		t.Fatalf("commitments = %d, want 1", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-PC' AND event_type = 'order.refund_admitted'`); got != 1 {
		t.Fatalf("refund_admitted events = %d, want 1 terminal", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-PC' AND event_type = ?`, events.RefundReleaseEventType); got != 0 {
		t.Fatalf("release events = %d, want 0", got)
	}
	if ff.cancelCalls.Load() != 0 {
		t.Fatalf("partial shipped probed CancelIfCreated: %d", ff.cancelCalls.Load())
	}
}

func TestAC50ReviewPartialUnclassifiable(t *testing.T) {
	// O4d: CONSUMED + partial + projection missing + orders SHIPPED => REVIEW
	// with exactly one RESERVED commitment and no probe/outbox.
	db := orderTestDB(t)
	ff := &fakeFulfillmentPort{status: "CANCELLED"}
	svc := ac50Service(db, &fakePaymentPort{captured: 10000}, ff)
	ctx := context.Background()
	ac50SeedOrderStatus(t, db, "ORD-PD", 10000, statusShipped)
	ac50SeedPermit(t, db, "ORD-PD", permitConsumed)
	_, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-PD", OrderNo: "ORD-PD", GroupNo: "G1", AmountCent: 5000, Source: "aftersale",
	})
	ae := requireReviewDetails(t, err, "R-PD", refundReasonPartialUnclassifiable)
	if got, _ := ae.Details["reason"].(string); got != refundReasonPartialUnclassifiable {
		t.Fatalf("reason = %q, want %q", got, refundReasonPartialUnclassifiable)
	}
	result, state, _ := ac50Admission(t, db, "R-PD")
	if result != refundReviewRequired || state != refundPending {
		t.Fatalf("admission = %s/%s, want REVIEW/PENDING", result, state)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitments WHERE refund_id = 'R-PD' AND state = 'RESERVED'`); got != 1 {
		t.Fatalf("RESERVED commitments = %d, want 1 (frozen quota)", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-PD'`); got != 0 {
		t.Fatalf("partial REVIEW wrote %d outbox events, want 0", got)
	}
	if ff.cancelCalls.Load() != 0 {
		t.Fatalf("partial REVIEW probed CancelIfCreated: %d", ff.cancelCalls.Load())
	}
}

func TestAC50ReviewPartialPreseedUnshipped(t *testing.T) {
	// O4e: an existing PENDING partial admission is reclassified as PARTIAL
	// without new writes; the placeholder/commitment stay untouched.
	db := orderTestDB(t)
	ff := &fakeFulfillmentPort{status: "CANCELLED"}
	svc := ac50Service(db, &fakePaymentPort{captured: 10000}, ff)
	ctx := context.Background()
	ac50SeedOrder(t, db, "ORD-PE", 10000)
	ac50SeedGroup(t, db, "ORD-PE", "ALLOCATED", 0)
	ac50SeedPermit(t, db, "ORD-PE", permitConsumed)
	if _, err := db.Exec(`INSERT INTO order_refund_admissions
		(refund_id, order_no, group_no, result, state, amount_cent, version)
		VALUES ('R-PE','ORD-PE','G1',?,?,5000,3)`, refundResultPost, refundPending); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO order_refund_commitments
		(order_no,group_no,refund_id,amount_cent,state,version) VALUES ('ORD-PE','G1','R-PE',5000,'RESERVED',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-PE", OrderNo: "ORD-PE", GroupNo: "G1", AmountCent: 5000, Source: "aftersale",
	}); err == nil {
		t.Fatal("preseed partial: expected PARTIAL")
	} else if !strings.Contains(apperr.From(err).Message, "partial refund of an unshipped group") {
		t.Fatalf("error = %v, want PARTIAL", err)
	}
	result, state, version := ac50Admission(t, db, "R-PE")
	if result != refundResultPost || state != refundPending || version != 3 {
		t.Fatalf("preseed admission changed: %s/%s v%d", result, state, version)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitments WHERE refund_id = 'R-PE'`); got != 1 {
		t.Fatalf("preseed commitments = %d, want 1", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-PE'`); got != 0 {
		t.Fatalf("preseed partial wrote %d outbox events", got)
	}
	if ff.cancelCalls.Load() != 0 {
		t.Fatalf("preseed partial probed CancelIfCreated: %d", ff.cancelCalls.Load())
	}
}

func TestAC50ReviewWholeGroupProjectionShipped(t *testing.T) {
	// O5c: whole-group CONSUMED with projection SHIPPED writes POST directly,
	// without probing or cancelling the waybill.
	db := orderTestDB(t)
	ff := &fakeFulfillmentPort{status: "CANCELLED"}
	svc := ac50Service(db, &fakePaymentPort{captured: 10000}, ff)
	ctx := context.Background()
	ac50SeedOrder(t, db, "ORD-PF", 10000)
	ac50SeedGroup(t, db, "ORD-PF", "SHIPPED", 0)
	ac50SeedPermit(t, db, "ORD-PF", permitConsumed)
	reply, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-PF", OrderNo: "ORD-PF", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetResult() != refundResultPost || reply.GetState() != refundResultPost {
		t.Fatalf("admission = %s/%s, want POST/POST", reply.GetResult(), reply.GetState())
	}
	if ff.cancelCalls.Load() != 0 {
		t.Fatalf("whole-group shipped probed CancelIfCreated: %d", ff.cancelCalls.Load())
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-PF' AND event_type = ?`, events.RefundReleaseEventType); got != 0 {
		t.Fatalf("release events = %d, want 0", got)
	}
}

func TestAC50ReviewLegacyUnclassifiable(t *testing.T) {
	// O10b: legacy without a permit row + projection missing + orders SHIPPED
	// => REVIEW with the legacy reason, no PRE, no probe.
	db := orderTestDB(t)
	ff := &fakeFulfillmentPort{status: "CANCELLED"}
	svc := ac50Service(db, &fakePaymentPort{captured: 10000}, ff)
	ctx := context.Background()
	ac50SeedOrderStatus(t, db, "ORD-PG", 10000, statusShipped)
	_, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-PG", OrderNo: "ORD-PG", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
	})
	ae := requireReviewDetails(t, err, "R-PG", refundReasonLegacyShipmentUnknown)
	if got, _ := ae.Details["reason"].(string); got != refundReasonLegacyShipmentUnknown {
		t.Fatalf("reason = %q, want %q", got, refundReasonLegacyShipmentUnknown)
	}
	var permitState string
	if err := db.QueryRow(`SELECT state FROM order_shipping_permits WHERE order_no = 'ORD-PG'`).Scan(&permitState); err != nil {
		t.Fatal(err)
	}
	if permitState == permitRevoked || permitState == permitConsumed {
		t.Fatalf("legacy REVIEW changed the permit to %q", permitState)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitments WHERE refund_id = 'R-PG' AND state = 'RESERVED'`); got != 1 {
		t.Fatalf("RESERVED commitments = %d, want 1", got)
	}
	if ff.cancelCalls.Load() != 0 {
		t.Fatalf("legacy unclassifiable probed CancelIfCreated: %d", ff.cancelCalls.Load())
	}
}

func TestAC50ReviewZeroTransientNoWrites(t *testing.T) {
	// O7b: zero + TRANSIENT_ERROR stays retryable with zero writes; O21: it
	// contributes no REVIEW row (the AC-72 gauge only counts those rows).
	db := orderTestDB(t)
	payments := &fakePaymentPort{}
	ff := &fakeFulfillmentPort{status: "TRANSIENT_ERROR"}
	svc := ac50Service(db, payments, ff)
	ctx := context.Background()
	ac50SeedZeroOrder(t, db, "ORD-PH")
	ac50SeedPermit(t, db, "ORD-PH", permitConsumed)
	if _, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-PH", OrderNo: "ORD-PH", GroupNo: "G1", AmountCent: 0, Source: refundSourceSystemZero,
	}); err == nil {
		t.Fatal("zero transient: expected Unavailable")
	} else if !apperr.IsCode(err, apperr.CodeUnavailable) {
		t.Fatalf("error = %v, want UNAVAILABLE", err)
	}
	for _, table := range []string{"order_refund_admissions", "refund_applications", "order_refund_commitments"} {
		if got := countRows(t, db, `SELECT COUNT(*) FROM `+table+` WHERE refund_id = 'R-PH'`); got != 0 {
			t.Fatalf("zero transient wrote %s: %d", table, got)
		}
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_admissions WHERE result = 'REVIEW_REQUIRED'`); got != 0 {
		t.Fatalf("zero transient added %d REVIEW rows (AC-72 backlog)", got)
	}
	if payments.getCalls != 0 || len(payments.refundCalls) != 0 {
		t.Fatalf("zero transient consulted Payment: get=%d refund=%d", payments.getCalls, len(payments.refundCalls))
	}
}

func TestAC50ReviewZeroNoWaybillThenCancelled(t *testing.T) {
	// O8: the first zero probe (NO_WAYBILL) writes nothing; once a real
	// CANCELLED row exists the same refund_id converges to PRE + DONE, and a
	// replay stays idempotent.
	db := orderTestDB(t)
	payments := &fakePaymentPort{}
	ff := &fakeFulfillmentPort{status: probeStatusNoWaybill}
	svc := ac50Service(db, payments, ff)
	ctx := context.Background()
	ac50SeedZeroOrder(t, db, "ORD-PI")
	ac50SeedPermit(t, db, "ORD-PI", permitConsumed)
	req := &pb.AdmitRefundRequest{
		RefundId: "R-PI", OrderNo: "ORD-PI", GroupNo: "G1", AmountCent: 0, Source: refundSourceSystemZero,
	}
	if _, err := svc.AdmitRefund(ctx, req); err == nil {
		t.Fatal("zero no-waybill: expected REFUND_ADMISSION_CONFLICT")
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_admissions WHERE refund_id = 'R-PI'`); got != 0 {
		t.Fatalf("zero no-waybill persisted %d admissions", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_applications WHERE refund_id = 'R-PI'`); got != 0 {
		t.Fatalf("zero no-waybill persisted %d applications", got)
	}
	ff.setStatus("CANCELLED")
	reply, err := svc.AdmitRefund(ctx, req)
	if err != nil {
		t.Fatalf("zero after real cancel: %v", err)
	}
	if reply.GetResult() != refundResultPre || reply.GetState() != refundApplicationDone {
		t.Fatalf("zero converged = %s/%s, want PRE/DONE", reply.GetResult(), reply.GetState())
	}
	again, err := svc.AdmitRefund(ctx, req)
	if err != nil || again.GetState() != refundApplicationDone {
		t.Fatalf("zero replay = (%v, %v)", again, err)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_applications WHERE refund_id = 'R-PI'`); got != 1 {
		t.Fatalf("zero replay duplicated application: %d", got)
	}
	if payments.getCalls != 0 || len(payments.refundCalls) != 0 {
		t.Fatalf("zero path consulted Payment: get=%d refund=%d", payments.getCalls, len(payments.refundCalls))
	}
}

func TestTCCReviewTx2BatchGateRollsBack(t *testing.T) {
	// T1c hardening: a REVIEW Tx2 whose batch/app mirror matches 0 rows must
	// fail closed and roll back the admission decision instead of committing a
	// review without the human hold.
	ctx := context.Background()
	svc, _, ff, refundID := tccReviewFixture(t, "ORD-T1C-GATE", "PAY-T1C-GATE")
	db := svc.db
	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-T1C-GATE", PaymentNo: "PAY-T1C-GATE", PaidCent: 10000,
	}); err == nil {
		t.Fatal("expected MANUAL_HOLD first")
	}
	batch := loadTccBatchRow(t, db, "ORD-T1C-GATE", "PAY-T1C-GATE")
	// Put the batch in a non-resumable state and reset the review decision; the
	// Tx2 admission CAS still matches but the batch mirror must be 0 rows.
	if _, err := db.Exec(`UPDATE refund_batches SET state = ? WHERE batch_id = ?`, refundBatchPaying, batch.BatchID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE order_refund_admissions SET result = ?, state = ? WHERE refund_id = ?`,
		refundPending, refundPending, refundID); err != nil {
		t.Fatal(err)
	}
	cancelBefore := ff.cancelCalls.Load()
	_, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: refundID, OrderNo: "ORD-T1C-GATE", GroupNo: "G1",
		AmountCent: 10000, Source: refundApplicationSourceTCC, BatchId: batch.BatchID,
	})
	if err == nil {
		t.Fatal("batch gate: expected a fail-closed error")
	}
	if !strings.Contains(err.Error(), "batch state changed while persisting review") {
		t.Fatalf("error = %v, want the batch mirror gate", err)
	}
	result, state, _ := ac50Admission(t, db, refundID)
	if result != refundPending || state != refundPending {
		t.Fatalf("Tx2 committed without the batch hold: %s/%s", result, state)
	}
	var batchState string
	if err := db.QueryRow(`SELECT state FROM refund_batches WHERE batch_id = ?`, batch.BatchID).Scan(&batchState); err != nil {
		t.Fatal(err)
	}
	if batchState != refundBatchPaying {
		t.Fatalf("batch state changed to %q, want PAYING (rollback)", batchState)
	}
	if ff.cancelCalls.Load() != cancelBefore+1 {
		t.Fatalf("probe calls = %d, want %d", ff.cancelCalls.Load(), cancelBefore+1)
	}
}

// requireReviewDetails asserts the stable conflict fields for a review whose
// reason is not CONSUMED_WITHOUT_SHIPMENT (partial/legacy).
func requireReviewDetails(t *testing.T, err error, refundID, reason string) *apperr.Error {
	t.Helper()
	if err == nil {
		t.Fatalf("refund %s: expected REFUND_ADMISSION_CONFLICT", refundID)
	}
	ae := apperr.From(err)
	if ae == nil || ae.Code != apperr.CodeFailedPrecondition || !strings.Contains(ae.Message, "REFUND_ADMISSION_CONFLICT") {
		t.Fatalf("refund %s: error = %v", refundID, err)
	}
	want := map[string]string{
		"result": refundReviewRequired, "state": refundPending,
		"reason": reason, "ticket": "RVW:" + refundID, "manual_queue": manualQueueNowb,
	}
	for key, wantValue := range want {
		if got, _ := ae.Details[key].(string); got != wantValue {
			t.Fatalf("refund %s: details[%q] = %q, want %q", refundID, key, got, wantValue)
		}
	}
	return ae
}
