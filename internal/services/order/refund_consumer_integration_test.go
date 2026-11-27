package order

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"strings"
	"testing"
)

// T1.4b1 evidence: `payment.refunded` is the only writer that closes a group
// refund (commitment RESERVED -> SETTLED, application DONE) and the only writer
// that can close a TCC batch PAYING -> DONE. It never writes the lifecycle
// status and never rewrites a terminal/human-owned state.

func seedTccBatchRow(t *testing.T, db *sql.DB, batchID, orderNo, paymentNo, state string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO refund_batches
		(batch_id, source, order_no, payment_no, attempt_seq, state, version)
		VALUES (?, ?, ?, ?, 1, ?, 1)`,
		batchID, refundBatchSourceTCC, orderNo, paymentNo, state); err != nil {
		t.Fatal(err)
	}
}

func seedTccApplicationRow(t *testing.T, db *sql.DB, refundID, batchID, orderNo, groupNo, state, result string, amount, goods, freight int64) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO refund_applications
		(refund_id, batch_id, source, order_no, group_no, non_effective_capture,
		 amount_cent, goods_cent, freight_cent, state, admission_result, version)
		VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?, ?, ?, 1)`,
		refundID, batchID, refundApplicationSourceTCC, orderNo, groupNo, amount, goods, freight, state, result); err != nil {
		t.Fatal(err)
	}
}

func consumerTestService(t *testing.T, db *sql.DB) *Service {
	t.Helper()
	return New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
}

func setOrderRefundState(t *testing.T, db *sql.DB, orderNo, state string) {
	t.Helper()
	if _, err := db.Exec(`UPDATE orders SET refund_state = ? WHERE order_no = ?`, state, orderNo); err != nil {
		t.Fatal(err)
	}
}

// T1.4b1: a refunded fact settles the commitment, closes the application and,
// once every group of the batch is DONE, moves the batch to DONE. A replay is a
// no-op (no second projection event, no second state transition).
func TestPaymentRefundedSettlesCommitmentApplicationAndBatch(t *testing.T) {
	db := orderTestDB(t)
	svc := consumerTestService(t, db)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-CONV", 10000, 10000, 0, 0, "PAY-CONV")
	setOrderRefundState(t, db, "ORD-CONV", refundStateRefunding)
	batchID := tccBatchID("PAY-CONV", 1)
	seedTccBatchRow(t, db, batchID, "ORD-CONV", "PAY-CONV", refundBatchPaying)
	refundID := tccGroupRefundID("PAY-CONV", "G1")
	seedTccApplicationRow(t, db, refundID, batchID, "ORD-CONV", "G1",
		refundApplicationProcessing, refundResultPre, 10000, 10000, 0)
	seedKnownTccCommitment(t, db, "ORD-CONV", "G1", refundID, 10000, 10000, 0)

	event := paymentRefundedEvent{
		PaymentNo: "PAY-CONV", OrderNo: "ORD-CONV", RefundID: refundID,
		AmountCent: 10000, RefundedCent: 10000, OrderRefundedCent: 10000, CapturedCent: 10000,
	}
	if err := applyPaymentRefunded(ctx, svc, event); err != nil {
		t.Fatal(err)
	}
	if state, amount := loadCommitmentState(t, db, refundID); state != commitmentSettled || amount != 10000 {
		t.Fatalf("commitment = %s/%d, want SETTLED/10000", state, amount)
	}
	var commitmentVersion int64
	if err := db.QueryRow(`SELECT version FROM order_refund_commitments WHERE refund_id = ?`, refundID).
		Scan(&commitmentVersion); err != nil {
		t.Fatal(err)
	}
	if appState, _, _, _, _ := loadTccApplication(t, ctx, db, refundID); appState != refundApplicationDone {
		t.Fatalf("application = %s, want DONE", appState)
	}
	if batch := loadTccBatchRow(t, db, "ORD-CONV", "PAY-CONV"); batch.State != refundBatchDone {
		t.Fatalf("batch = %s, want DONE", batch.State)
	}
	var status, refundStatus, refundState string
	var refunded int64
	if err := db.QueryRow(`SELECT status, refund_status, refund_state, refunded_cent
		FROM orders WHERE order_no = 'ORD-CONV'`).
		Scan(&status, &refundStatus, &refundState, &refunded); err != nil {
		t.Fatal(err)
	}
	if status != statusPaid || refundStatus != statusRefunded || refundState != refundStateFull || refunded != 10000 {
		t.Fatalf("order projection = %s/%s/%s/%d, want PAID/REFUNDED/FULL/10000",
			status, refundStatus, refundState, refunded)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events
		WHERE aggregate_id = 'ORD-CONV' AND event_type = 'order.refund_projected'`); got != 1 {
		t.Fatalf("order.refund_projected = %d, want 1", got)
	}

	// Replay: nothing changes, including the commitment version and outbox.
	if err := applyPaymentRefunded(ctx, svc, event); err != nil {
		t.Fatal(err)
	}
	var versionAfter int64
	if err := db.QueryRow(`SELECT version FROM order_refund_commitments WHERE refund_id = ?`, refundID).
		Scan(&versionAfter); err != nil {
		t.Fatal(err)
	}
	if versionAfter != commitmentVersion {
		t.Fatalf("replayed event changed commitment version %d -> %d", commitmentVersion, versionAfter)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events
		WHERE aggregate_id = 'ORD-CONV' AND event_type = 'order.refund_projected'`); got != 1 {
		t.Fatalf("replayed event appended a projection event (%d)", got)
	}
	if batch := loadTccBatchRow(t, db, "ORD-CONV", "PAY-CONV"); batch.State != refundBatchDone {
		t.Fatalf("batch after replay = %s, want DONE", batch.State)
	}
}

// T1.4b1: an out-of-order (already covered) cumulative amount must NOT skip the
// ledger convergence; otherwise a commitment would stay RESERVED forever.
func TestPaymentRefundedOutOfOrderStillSettlesLedger(t *testing.T) {
	db := orderTestDB(t)
	svc := consumerTestService(t, db)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-OOO", 10000, 10000, 0, 5000, "PAY-OOO")
	if _, err := db.Exec(`UPDATE orders SET refunded_cent = 5000, refund_status = ?,
		refund_state = ? WHERE order_no = 'ORD-OOO'`, statusPartiallyRefunded, refundStatePartial); err != nil {
		t.Fatal(err)
	}
	batchID := tccBatchID("PAY-OOO", 1)
	seedTccBatchRow(t, db, batchID, "ORD-OOO", "PAY-OOO", refundBatchPaying)
	refundID := tccGroupRefundID("PAY-OOO", "G1")
	seedTccApplicationRow(t, db, refundID, batchID, "ORD-OOO", "G1",
		refundApplicationProcessing, refundResultPost, 2000, 2000, 0)
	seedKnownTccCommitment(t, db, "ORD-OOO", "G1", refundID, 2000, 2000, 0)

	// The event's cumulative amount (2000) is older than the projected 5000.
	if err := applyPaymentRefunded(ctx, svc, paymentRefundedEvent{
		PaymentNo: "PAY-OOO", OrderNo: "ORD-OOO", RefundID: refundID,
		AmountCent: 2000, RefundedCent: 2000, OrderRefundedCent: 2000, CapturedCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	if state, _ := loadCommitmentState(t, db, refundID); state != commitmentSettled {
		t.Fatalf("commitment = %s, want SETTLED even for an out-of-order event", state)
	}
	if appState, _, _, _, _ := loadTccApplication(t, ctx, db, refundID); appState != refundApplicationDone {
		t.Fatalf("application = %s, want DONE", appState)
	}
	if batch := loadTccBatchRow(t, db, "ORD-OOO", "PAY-OOO"); batch.State != refundBatchDone {
		t.Fatalf("batch = %s, want DONE", batch.State)
	}
	var refunded int64
	if err := db.QueryRow(`SELECT refunded_cent FROM orders WHERE order_no = 'ORD-OOO'`).Scan(&refunded); err != nil {
		t.Fatal(err)
	}
	if refunded != 5000 {
		t.Fatalf("order refunded_cent = %d, want monotonic 5000", refunded)
	}
}

// T1.4b1: non-effective capture refunds (duplicate/over-under capture, payment
// cancel) are outside the order captured/refunded facts and commitments.
func TestPaymentRefundedIgnoresNonEffectiveCapture(t *testing.T) {
	db := orderTestDB(t)
	svc := consumerTestService(t, db)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-NONEF", 10000, 10000, 0, 0, "PAY-NONEF")
	setOrderRefundState(t, db, "ORD-NONEF", refundStateRefunding)
	batchID := tccBatchID("PAY-NONEF", 1)
	seedTccBatchRow(t, db, batchID, "ORD-NONEF", "PAY-NONEF", refundBatchPaying)
	refundID := tccGroupRefundID("PAY-NONEF", "G1")
	seedTccApplicationRow(t, db, refundID, batchID, "ORD-NONEF", "G1",
		refundApplicationProcessing, refundResultPre, 10000, 10000, 0)
	seedKnownTccCommitment(t, db, "ORD-NONEF", "G1", refundID, 10000, 10000, 0)

	if err := applyPaymentRefunded(ctx, svc, paymentRefundedEvent{
		PaymentNo: "PAY-NONEF", OrderNo: "ORD-NONEF", RefundID: refundID,
		AmountCent: 10000, RefundedCent: 10000, OrderRefundedCent: 10000, CapturedCent: 10000,
		NonEffectiveCapture: true,
	}); err != nil {
		t.Fatal(err)
	}
	if state, _ := loadCommitmentState(t, db, refundID); state != commitmentReserved {
		t.Fatalf("non-effective event settled the commitment: %s", state)
	}
	if appState, _, _, _, _ := loadTccApplication(t, ctx, db, refundID); appState != refundApplicationProcessing {
		t.Fatalf("non-effective event changed the application: %s", appState)
	}
	if batch := loadTccBatchRow(t, db, "ORD-NONEF", "PAY-NONEF"); batch.State != refundBatchPaying {
		t.Fatalf("non-effective event changed the batch: %s", batch.State)
	}
	var refunded int64
	if err := db.QueryRow(`SELECT refunded_cent FROM orders WHERE order_no = 'ORD-NONEF'`).Scan(&refunded); err != nil {
		t.Fatal(err)
	}
	if refunded != 0 {
		t.Fatalf("non-effective refund entered orders.refunded_cent: %d", refunded)
	}
}

// T1.4b1: an unknown refund_id has no commitment/application in Order. It must
// not be guessed onto a group, but the order amount projection still applies
// (payment-level/legacy refunds) and the event is a no-op for the ledger.
func TestPaymentRefundedUnknownRefundIDProjectsOrderOnly(t *testing.T) {
	db := orderTestDB(t)
	svc := consumerTestService(t, db)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-UNKNOWN", 10000, 10000, 0, 0, "PAY-UNKNOWN")

	if err := applyPaymentRefunded(ctx, svc, paymentRefundedEvent{
		PaymentNo: "PAY-UNKNOWN", OrderNo: "ORD-UNKNOWN", RefundID: "pay:PAY-UNKNOWN:cancel",
		AmountCent: 1000, RefundedCent: 1000, OrderRefundedCent: 1000, CapturedCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	var refundStatus, refundState string
	var refunded int64
	if err := db.QueryRow(`SELECT refund_status, refund_state, refunded_cent
		FROM orders WHERE order_no = 'ORD-UNKNOWN'`).
		Scan(&refundStatus, &refundState, &refunded); err != nil {
		t.Fatal(err)
	}
	if refundStatus != statusPartiallyRefunded || refundState != refundStatePartial || refunded != 1000 {
		t.Fatalf("order projection = %s/%s/%d, want PARTIALLY_REFUNDED/PARTIAL/1000",
			refundStatus, refundState, refunded)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitments WHERE order_no = 'ORD-UNKNOWN'`); got != 0 {
		t.Fatalf("unknown refund_id created %d commitments", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_applications WHERE order_no = 'ORD-UNKNOWN'`); got != 0 {
		t.Fatalf("unknown refund_id created %d applications", got)
	}
}

// T1.4b1: while a durable TCC batch is paying, a partial refund projection must
// keep orders.refund_state=REFUNDING (the T1.2/T1.3 resume contract) and must
// only move the batch to DONE after the last group is DONE.
func TestPaymentRefundedKeepsRefundingWhileTccBatchPays(t *testing.T) {
	db := orderTestDB(t)
	svc := consumerTestService(t, db)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-PARTIAL", 10000, 10000, 500, 0, "PAY-PARTIAL")
	setOrderRefundState(t, db, "ORD-PARTIAL", refundStateRefunding)
	batchID := tccBatchID("PAY-PARTIAL", 1)
	seedTccBatchRow(t, db, batchID, "ORD-PARTIAL", "PAY-PARTIAL", refundBatchPaying)
	for _, group := range []struct {
		no      string
		amount  int64
		goods   int64
		freight int64
	}{{"G1", 5800, 5500, 300}, {"G2", 4200, 4000, 200}} {
		refundID := tccGroupRefundID("PAY-PARTIAL", group.no)
		seedTccApplicationRow(t, db, refundID, batchID, "ORD-PARTIAL", group.no,
			refundApplicationProcessing, refundResultPre, group.amount, group.goods, group.freight)
		seedKnownTccCommitment(t, db, "ORD-PARTIAL", group.no, refundID, group.amount, group.goods, group.freight)
	}

	if err := applyPaymentRefunded(ctx, svc, paymentRefundedEvent{
		PaymentNo: "PAY-PARTIAL", OrderNo: "ORD-PARTIAL", RefundID: tccGroupRefundID("PAY-PARTIAL", "G1"),
		AmountCent: 5800, RefundedCent: 5800, OrderRefundedCent: 5800, CapturedCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	var refundState, refundStatus string
	var refunded int64
	if err := db.QueryRow(`SELECT refund_state, refund_status, refunded_cent
		FROM orders WHERE order_no = 'ORD-PARTIAL'`).
		Scan(&refundState, &refundStatus, &refunded); err != nil {
		t.Fatal(err)
	}
	if refundState != refundStateRefunding {
		t.Fatalf("refund_state after a partial TCC group = %s, want REFUNDING (batch still PAYING)", refundState)
	}
	if refundStatus != statusPartiallyRefunded || refunded != 5800 {
		t.Fatalf("amount projection = %s/%d, want PARTIALLY_REFUNDED/5800", refundStatus, refunded)
	}
	if batch := loadTccBatchRow(t, db, "ORD-PARTIAL", "PAY-PARTIAL"); batch.State != refundBatchPaying {
		t.Fatalf("batch = %s, want PAYING while G2 is unfinished", batch.State)
	}
	if state, _ := loadCommitmentState(t, db, tccGroupRefundID("PAY-PARTIAL", "G2")); state != commitmentReserved {
		t.Fatalf("G2 commitment = %s, want RESERVED", state)
	}

	if err := applyPaymentRefunded(ctx, svc, paymentRefundedEvent{
		PaymentNo: "PAY-PARTIAL", OrderNo: "ORD-PARTIAL", RefundID: tccGroupRefundID("PAY-PARTIAL", "G2"),
		AmountCent: 4200, RefundedCent: 10000, OrderRefundedCent: 10000, CapturedCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	if batch := loadTccBatchRow(t, db, "ORD-PARTIAL", "PAY-PARTIAL"); batch.State != refundBatchDone {
		t.Fatalf("batch after the last group = %s, want DONE", batch.State)
	}
	if err := db.QueryRow(`SELECT refund_state, refund_status, refunded_cent
		FROM orders WHERE order_no = 'ORD-PARTIAL'`).
		Scan(&refundState, &refundStatus, &refunded); err != nil {
		t.Fatal(err)
	}
	if refundState != refundStateFull || refundStatus != statusRefunded || refunded != 10000 {
		t.Fatalf("final projection = %s/%s/%d, want FULL/REFUNDED/10000", refundState, refundStatus, refunded)
	}
}

func TestPaymentRefundedBeforeCaptureRejected(t *testing.T) {
	db := orderTestDB(t)
	svc := consumerTestService(t, db)
	if _, err := db.Exec(`INSERT INTO orders
		(order_no,user_id,status,total_cent,pay_cent,paid_cent,settlement_payment_no,
		 refunded_cent,refund_status,refund_state,expire_at,applied_promotions_json)
		VALUES ('ORD-OOO-EARLY',1,'PENDING_PAYMENT',10000,0,0,'',0,'','NONE',NOW(3),'[]')`); err != nil {
		t.Fatal(err)
	}
	err := applyPaymentRefunded(context.Background(), svc, paymentRefundedEvent{
		PaymentNo: "PAY-OOO-EARLY", OrderNo: "ORD-OOO-EARLY", RefundID: "R-OOO-EARLY",
		AmountCent: 2000, RefundedCent: 2000, CapturedCent: 10000,
	})
	if err == nil || !strings.Contains(err.Error(), "EVENT_OUT_OF_ORDER") {
		t.Fatalf("early refund error = %v", err)
	}
	if got := countRows(t, db, `SELECT refunded_cent FROM orders WHERE order_no = 'ORD-OOO-EARLY'`); got != 0 {
		t.Fatalf("early refund projected %d", got)
	}
}
