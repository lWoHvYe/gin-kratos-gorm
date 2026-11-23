package order

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"

	"google.golang.org/grpc"
)

// T1.2 / T1.3 evidence (SPEC-0002 v1.8 AC-51/AC-65, ADR-0007 Amendment 1
// §4.1.2/§6.1/§6.2). Every test runs against a real MySQL database
// (TEST_MYSQL_DSN) because the guarantees under test are transactional: the
// order lock, the Σ recheck with one snapshot, the permit CAS and the durable
// refund_batches/refund_applications ledger.

// countingPaymentPort is the concurrency-safe Payment stand-in for the race
// test; fakePaymentPort's counters are not safe for parallel use.
type countingPaymentPort struct {
	mu       sync.Mutex
	captured int64
	refunded int64
	getCalls int
}

func (f *countingPaymentPort) GetIntent(context.Context, *pb.GetIntentRequest, ...grpc.CallOption) (*pb.Intent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getCalls++
	return &pb.Intent{PaymentNo: "PAY-1", CapturedCent: f.captured, RefundedCent: f.refunded}, nil
}

func (f *countingPaymentPort) ListOrderIntents(context.Context, *pb.ListOrderIntentsRequest, ...grpc.CallOption) (*pb.IntentList, error) {
	return &pb.IntentList{}, nil
}

func (f *countingPaymentPort) setRefunded(cent int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refunded = cent
}

func seedTccPermit(t *testing.T, db *sql.DB, orderNo, groupNo, state, shipmentNo string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO order_shipping_permits
		(order_no, fulfillment_group_no, state, shipment_no, after_sale_no, version)
		VALUES (?, ?, ?, ?, '', 1)`, orderNo, groupNo, state, shipmentNo); err != nil {
		t.Fatal(err)
	}
}

func setTccGroupStatus(t *testing.T, db *sql.DB, orderNo, groupNo, status string) {
	t.Helper()
	if _, err := db.Exec(`UPDATE order_fulfillment_groups SET status = ? WHERE order_no = ? AND group_no = ?`,
		status, orderNo, groupNo); err != nil {
		t.Fatal(err)
	}
}

func countRows(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var count int
	if err := db.QueryRow(query, args...).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

type tccBatchRow struct {
	BatchID       string
	State         string
	AttemptSeq    int
	CapturedCent  int64
	RefundedCent  int64
	SumCapCent    int64
	RemainingCent int64
}

func loadTccBatchRow(t *testing.T, db *sql.DB, orderNo, paymentNo string) tccBatchRow {
	t.Helper()
	var row tccBatchRow
	if err := db.QueryRow(`SELECT batch_id, state, attempt_seq, captured_snapshot_cent,
		refunded_snapshot_cent, sum_group_cap_cent, remaining_captured_cent
		FROM refund_batches WHERE source = 'tcc' AND order_no = ? AND payment_no = ?`,
		orderNo, paymentNo).Scan(&row.BatchID, &row.State, &row.AttemptSeq, &row.CapturedCent,
		&row.RefundedCent, &row.SumCapCent, &row.RemainingCent); err != nil {
		t.Fatal(err)
	}
	return row
}

// assertNoTccStateWrites proves a fail-closed path changed no refund state.
// The refund_batches ledger is asserted separately because a Σ mismatch is
// allowed (and required) to leave one PRECHECK_FAILED audit batch (R-1).
func assertNoTccStateWrites(t *testing.T, db *sql.DB, orderNo string, versionBefore int64) {
	t.Helper()
	var version int64
	var refundState string
	if err := db.QueryRow(`SELECT version, refund_state FROM orders WHERE order_no = ?`, orderNo).
		Scan(&version, &refundState); err != nil {
		t.Fatal(err)
	}
	if version != versionBefore {
		t.Fatalf("orders.version changed %d -> %d", versionBefore, version)
	}
	if refundState != refundStateNone {
		t.Fatalf("refund_state = %q, want NONE", refundState)
	}
	for _, query := range []string{
		`SELECT COUNT(*) FROM refund_applications WHERE order_no = ?`,
		`SELECT COUNT(*) FROM order_refund_admissions WHERE order_no = ?`,
		`SELECT COUNT(*) FROM order_refund_commitments WHERE order_no = ?`,
		`SELECT COUNT(*) FROM order_refund_commitment_lines l JOIN order_refund_commitments c ON c.refund_id = l.refund_id WHERE c.order_no = ?`,
		`SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = ?`,
		`SELECT COUNT(*) FROM order_status_logs WHERE order_no = ?`,
	} {
		if count := countRows(t, db, query, orderNo); count != 0 {
			t.Fatalf("fail-closed path wrote rows (%s -> %d)", query, count)
		}
	}
}

// R-1 (ADR Amd1 §4.1.2 step 2): a Σ mismatch writes exactly one
// PRECHECK_FAILED audit batch with the computed numbers and no order/permit/
// commitment/application/outbox/status-log write.
func TestTccCancelSigmaMismatchWritesAuditBatchOnly(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 10000})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-MISMATCH", 10000, 10000, 0, 0, "PAY-MISMATCH")
	seedTccGroup(t, db, "ORD-TCC-MISMATCH", "G1", 1, 0)
	seedTccLine(t, db, "ORD-TCC-MISMATCH", 1, "G1", 9000) // Σ = 9000 vs captured 10000
	seedTccPermit(t, db, "ORD-TCC-MISMATCH", "G1", permitConsumed, "SHIP-MISMATCH")
	version := tccVersion(t, db, "ORD-TCC-MISMATCH")

	_, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-MISMATCH", PaymentNo: "PAY-MISMATCH", PaidCent: 10000,
	})
	if err == nil {
		t.Fatal("Σ mismatch TCC cancel succeeded")
	}
	if !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("error code=%v, want FAILED_PRECONDITION: %v", apperr.From(err).Code, err)
	}
	details := apperr.From(err).Details
	if details["sum_group_cap_cent"] != int64(9000) || details["remaining_captured_cent"] != int64(10000) ||
		details["diff_cent"] != int64(-1000) {
		t.Fatalf("mismatch error lost its numbers: %v", details)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-MISMATCH'`); got != 1 {
		t.Fatalf("audit batches = %d, want exactly 1", got)
	}
	var batchID, state string
	var attemptSeq int
	var captured, refunded, sumCap, remaining int64
	if err := db.QueryRow(`SELECT batch_id, state, attempt_seq, captured_snapshot_cent,
		refunded_snapshot_cent, sum_group_cap_cent, remaining_captured_cent
		FROM refund_batches WHERE order_no = 'ORD-TCC-MISMATCH'`).
		Scan(&batchID, &state, &attemptSeq, &captured, &refunded, &sumCap, &remaining); err != nil {
		t.Fatal(err)
	}
	if state != refundBatchFailedPre || attemptSeq != 1 || batchID != "tcc:PAY-MISMATCH:1" {
		t.Fatalf("audit batch = %s/%s/attempt %d", batchID, state, attemptSeq)
	}
	if captured != 10000 || refunded != 0 || sumCap != 9000 || remaining != 10000 {
		t.Fatalf("audit numbers = %d/%d/%d/%d", captured, refunded, sumCap, remaining)
	}
	// The audit batch is the ONLY write.
	assertNoTccStateWrites(t, db, "ORD-TCC-MISMATCH", version)
	var permitState string
	var permitVersion int64
	if err := db.QueryRow(`SELECT state, version FROM order_shipping_permits
		WHERE order_no = 'ORD-TCC-MISMATCH' AND fulfillment_group_no = 'G1'`).
		Scan(&permitState, &permitVersion); err != nil {
		t.Fatal(err)
	}
	if permitState != permitConsumed || permitVersion != 1 {
		t.Fatalf("mismatch touched the permit: %s/version %d", permitState, permitVersion)
	}
}

// R-1 §2.2(3)/(5): repeated Σ failures accumulate distinct PRECHECK_FAILED
// attempts, the audit row is never resumed, and it does not block a later
// successful FROZEN attempt.
func TestTccCancelPrecheckFailureAuditDoesNotBlockNextAttempt(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 10000})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-AUDIT", 10000, 10000, 0, 0, "PAY-AUDIT")
	seedTccGroup(t, db, "ORD-TCC-AUDIT", "G1", 1, 0)
	seedTccLine(t, db, "ORD-TCC-AUDIT", 1, "G1", 9000)

	for i := 0; i < 2; i++ {
		if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
			OrderNo: "ORD-TCC-AUDIT", PaymentNo: "PAY-AUDIT", PaidCent: 10000,
		}); !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
			t.Fatalf("attempt %d error = %v", i, err)
		}
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-AUDIT' AND state = ?`, refundBatchFailedPre); got != 2 {
		t.Fatalf("audit attempts = %d, want 2", got)
	}
	var refundState string
	if err := db.QueryRow(`SELECT refund_state FROM orders WHERE order_no = 'ORD-TCC-AUDIT'`).Scan(&refundState); err != nil {
		t.Fatal(err)
	}
	if refundState != refundStateNone {
		t.Fatalf("failed prechecks changed refund_state to %s", refundState)
	}
	// Fix the line so Σ matches, then a new attempt must be created.
	if _, err := db.Exec(`UPDATE order_items SET line_paid_cent = 10000 WHERE order_no = 'ORD-TCC-AUDIT'`); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-AUDIT", PaymentNo: "PAY-AUDIT", PaidCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	var frozenID string
	var frozenSeq int
	if err := db.QueryRow(`SELECT batch_id, attempt_seq FROM refund_batches
		WHERE order_no = 'ORD-TCC-AUDIT' AND state = ?`, refundBatchFrozen).Scan(&frozenID, &frozenSeq); err != nil {
		t.Fatal(err)
	}
	if frozenID != "tcc:PAY-AUDIT:3" || frozenSeq != 3 {
		t.Fatalf("successful attempt = %s/seq %d, want tcc:PAY-AUDIT:3/3", frozenID, frozenSeq)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-AUDIT' AND state = ?`, refundBatchFailedPre); got != 2 {
		t.Fatalf("audit attempts after success = %d, want 2", got)
	}
	// A repeat on the frozen batch resumes it; no new batch/attempt.
	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-AUDIT", PaymentNo: "PAY-AUDIT", PaidCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-AUDIT'`); got != 3 {
		t.Fatalf("batches after resume = %d, want 3 (2 audit + 1 frozen)", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-AUDIT' AND state = ?`, refundBatchFrozen); got != 1 {
		t.Fatalf("frozen batches = %d, want 1", got)
	}
}

// T1.2: matching Σ creates one FROZEN batch and one application per active
// group, revokes NONE permits and writes no event (the legacy
// order.refund_requested trigger is not used by TCC anymore).
func TestTccCancelCreatesBatchApplicationsAndRevokesPermits(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 10000})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-OK", 10000, 10000, 500, 0, "PAY-OK")
	seedTccGroup(t, db, "ORD-TCC-OK", "G1", 1, 300)
	seedTccGroup(t, db, "ORD-TCC-OK", "G2", 2, 200)
	seedTccLine(t, db, "ORD-TCC-OK", 1, "G1", 5500)
	seedTccLine(t, db, "ORD-TCC-OK", 2, "G2", 4000)

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-OK", PaymentNo: "PAY-OK", PaidCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}

	var status, refundState string
	if err := db.QueryRow(`SELECT status, refund_state FROM orders WHERE order_no = 'ORD-TCC-OK'`).
		Scan(&status, &refundState); err != nil {
		t.Fatal(err)
	}
	if status != statusPaid || refundState != refundStateRefunding {
		t.Fatalf("order = %s/%s, want PAID/REFUNDING", status, refundState)
	}
	batch := loadTccBatchRow(t, db, "ORD-TCC-OK", "PAY-OK")
	if batch.BatchID != "tcc:PAY-OK:1" || batch.State != refundBatchFrozen || batch.AttemptSeq != 1 {
		t.Fatalf("batch = %+v", batch)
	}
	if batch.CapturedCent != 10000 || batch.RefundedCent != 0 || batch.SumCapCent != 10000 || batch.RemainingCent != 10000 {
		t.Fatalf("batch audit numbers = %+v", batch)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_applications WHERE batch_id = ?`, batch.BatchID); got != 2 {
		t.Fatalf("applications = %d, want 2", got)
	}
	for _, want := range []struct {
		group   string
		amount  int64
		goods   int64
		freight int64
	}{
		{"G1", 5800, 5500, 300},
		{"G2", 4200, 4000, 200},
	} {
		var amount, goods, freight int64
		var result string
		if err := db.QueryRow(`SELECT amount_cent, goods_cent, freight_cent, admission_result
			FROM refund_applications WHERE refund_id = ?`, "tcc:PAY-OK:"+want.group).
			Scan(&amount, &goods, &freight, &result); err != nil {
			t.Fatal(err)
		}
		if amount != want.amount || goods != want.goods || freight != want.freight || result != refundResultPre {
			t.Fatalf("%s application = %d/%d/%d/%s, want %d/%d/%d/%s",
				want.group, amount, goods, freight, result, want.amount, want.goods, want.freight, refundResultPre)
		}
	}
	for _, groupNo := range []string{"G1", "G2"} {
		var state string
		var version int64
		if err := db.QueryRow(`SELECT state, version FROM order_shipping_permits
			WHERE order_no = 'ORD-TCC-OK' AND fulfillment_group_no = ?`, groupNo).Scan(&state, &version); err != nil {
			t.Fatal(err)
		}
		if state != permitRevoked {
			t.Fatalf("%s permit = %s, want REVOKED", groupNo, state)
		}
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_admissions WHERE order_no = 'ORD-TCC-OK'`); got != 0 {
		t.Fatalf("NONE permits wrote remote admissions: %d", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE topic = ?`, topicOrderRefundRequested); got != 0 {
		t.Fatalf("order.refund_requested rows = %d, want 0", got)
	}
}

// T1.2: a CONSUMED permit is never moved to REVOKED. Tx1 persists the
// PENDING_REMOTE admission and the RESERVED commitment with its composition so
// the remote admission (T1.4) can resume after a crash.
func TestTccCancelConsumedPermitWritesRemoteAdmissionWithoutRevoking(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 10000})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-CONSUMED", 10000, 10000, 0, 0, "PAY-CONSUMED")
	seedTccGroup(t, db, "ORD-TCC-CONSUMED", "G1", 1, 0)
	seedTccLine(t, db, "ORD-TCC-CONSUMED", 1, "G1", 6000)
	seedTccLine(t, db, "ORD-TCC-CONSUMED", 2, "G1", 4000)
	seedTccPermit(t, db, "ORD-TCC-CONSUMED", "G1", permitConsumed, "SHIP-CONSUMED")

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-CONSUMED", PaymentNo: "PAY-CONSUMED", PaidCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	var state string
	if err := db.QueryRow(`SELECT state FROM order_shipping_permits
		WHERE order_no = 'ORD-TCC-CONSUMED' AND fulfillment_group_no = 'G1'`).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != permitConsumed {
		t.Fatalf("CONSUMED permit became %s (must never be REVOKED)", state)
	}
	var appState, appResult, commitmentState string
	var appAmount, goods, freight int64
	if err := db.QueryRow(`SELECT state, admission_result, amount_cent, goods_cent, freight_cent
		FROM refund_applications WHERE refund_id = 'tcc:PAY-CONSUMED:G1'`).
		Scan(&appState, &appResult, &appAmount, &goods, &freight); err != nil {
		t.Fatal(err)
	}
	if appState != refundApplicationPending || appResult != refundPending || appAmount != 10000 || goods != 10000 || freight != 0 {
		t.Fatalf("application = %s/%s/%d/%d/%d", appState, appResult, appAmount, goods, freight)
	}
	var admissionState, admissionResult string
	var admissionAmount int64
	if err := db.QueryRow(`SELECT state, result, amount_cent FROM order_refund_admissions
		WHERE refund_id = 'tcc:PAY-CONSUMED:G1'`).Scan(&admissionState, &admissionResult, &admissionAmount); err != nil {
		t.Fatal(err)
	}
	if admissionState != refundPending || admissionResult != refundPending || admissionAmount != 10000 {
		t.Fatalf("admission = %s/%s/%d", admissionState, admissionResult, admissionAmount)
	}
	if err := db.QueryRow(`SELECT state, amount_cent, goods_cent, freight_cent, composition_known
		FROM order_refund_commitments WHERE refund_id = 'tcc:PAY-CONSUMED:G1'`).
		Scan(&commitmentState, &appAmount, &goods, &freight, new(int)); err != nil {
		t.Fatal(err)
	}
	if commitmentState != commitmentReserved || appAmount != 10000 || goods != 10000 || freight != 0 {
		t.Fatalf("commitment = %s/%d/%d/%d", commitmentState, appAmount, goods, freight)
	}
	var lineCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM order_refund_commitment_lines
		WHERE refund_id = 'tcc:PAY-CONSUMED:G1'`).Scan(&lineCount); err != nil {
		t.Fatal(err)
	}
	if lineCount != 2 {
		t.Fatalf("commitment lines = %d, want 2 (two goods lines, no freight)", lineCount)
	}
}

// AC-65/AC-51: a historical order with no fulfillment group is calculated and
// frozen as G1, and the G1 permit is initialized NONE then revoked.
func TestTccCancelLegacyOrderInitializesG1Permit(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 8000})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-G1", 8000, 8000, 0, 0, "PAY-G1")
	seedTccLine(t, db, "ORD-TCC-G1", 1, "G1", 8000)

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-G1", PaymentNo: "PAY-G1", PaidCent: 8000,
	}); err != nil {
		t.Fatal(err)
	}
	var state string
	if err := db.QueryRow(`SELECT state FROM order_shipping_permits
		WHERE order_no = 'ORD-TCC-G1' AND fulfillment_group_no = 'G1'`).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != permitRevoked {
		t.Fatalf("G1 permit = %s, want REVOKED", state)
	}
	var amount int64
	if err := db.QueryRow(`SELECT amount_cent FROM refund_applications WHERE refund_id = 'tcc:PAY-G1:G1'`).
		Scan(&amount); err != nil {
		t.Fatal(err)
	}
	if amount != 8000 {
		t.Fatalf("G1 application amount = %d", amount)
	}
}

// AC-65: an already shipped group is POST_SHIP_ALLOWED: the refusal never
// cancels the shipment or revokes the consumed permit.
func TestTccCancelShippedGroupIsPostShipAllowed(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 10000})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-SHIPPED", 10000, 10000, 0, 0, "PAY-SHIPPED")
	seedTccGroup(t, db, "ORD-TCC-SHIPPED", "G1", 1, 0)
	seedTccLine(t, db, "ORD-TCC-SHIPPED", 1, "G1", 10000)
	seedTccPermit(t, db, "ORD-TCC-SHIPPED", "G1", permitConsumed, "SHIP-SHIPPED")
	setTccGroupStatus(t, db, "ORD-TCC-SHIPPED", "G1", "SHIPPED")

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-SHIPPED", PaymentNo: "PAY-SHIPPED", PaidCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	var state string
	var version int64
	if err := db.QueryRow(`SELECT state, version FROM order_shipping_permits
		WHERE order_no = 'ORD-TCC-SHIPPED' AND fulfillment_group_no = 'G1'`).Scan(&state, &version); err != nil {
		t.Fatal(err)
	}
	if state != permitConsumed || version != 1 {
		t.Fatalf("shipped permit = %s/version %d, want untouched CONSUMED/version 1", state, version)
	}
	var result string
	if err := db.QueryRow(`SELECT admission_result FROM refund_applications
		WHERE refund_id = 'tcc:PAY-SHIPPED:G1'`).Scan(&result); err != nil {
		t.Fatal(err)
	}
	if result != refundResultPost {
		t.Fatalf("shipped application admission_result = %s, want POST_SHIP_ALLOWED", result)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_admissions WHERE order_no = 'ORD-TCC-SHIPPED'`); got != 0 {
		t.Fatalf("shipped group wrote a remote admission in Tx1: %d", got)
	}
}

// AC-65: concurrent / repeated TCC cancels create exactly one batch, one
// application per group and revoke the permit once. Run with -race.
func TestTccCancelConcurrentAndRepeatedIsIdempotent(t *testing.T) {
	db := orderTestDB(t)
	payments := &countingPaymentPort{captured: 10000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-RACE", 10000, 10000, 200, 0, "PAY-RACE")
	seedTccGroup(t, db, "ORD-TCC-RACE", "G1", 1, 100)
	seedTccGroup(t, db, "ORD-TCC-RACE", "G2", 2, 100)
	seedTccLine(t, db, "ORD-TCC-RACE", 1, "G1", 4900)
	seedTccLine(t, db, "ORD-TCC-RACE", 2, "G2", 4900)

	const workers = 8
	errs := make([]error, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			_, errs[i] = svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
				OrderNo: "ORD-TCC-RACE", PaymentNo: "PAY-RACE", PaidCent: 10000,
			})
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("worker %d: %v", i, err)
		}
	}
	// Repeat after the race: the same batch is reused, nothing is duplicated.
	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-RACE", PaymentNo: "PAY-RACE", PaidCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-RACE'`); got != 1 {
		t.Fatalf("batches = %d, want 1", got)
	}
	batch := loadTccBatchRow(t, db, "ORD-TCC-RACE", "PAY-RACE")
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_applications WHERE batch_id = ?`, batch.BatchID); got != 2 {
		t.Fatalf("applications = %d, want 2", got)
	}
	for _, groupNo := range []string{"G1", "G2"} {
		var state string
		var version int64
		if err := db.QueryRow(`SELECT state, version FROM order_shipping_permits
			WHERE order_no = 'ORD-TCC-RACE' AND fulfillment_group_no = ?`, groupNo).Scan(&state, &version); err != nil {
			t.Fatal(err)
		}
		// The permit was created at version 1 and revoked exactly once.
		if state != permitRevoked || version != 2 {
			t.Fatalf("%s permit = %s/version %d, want REVOKED/version 2", groupNo, state, version)
		}
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_admissions WHERE order_no = 'ORD-TCC-RACE'`); got != 0 {
		t.Fatalf("NONE path wrote remote admissions: %d", got)
	}
	if payments.getCalls < 1 {
		t.Fatalf("precheck never read Payment: calls=%d", payments.getCalls)
	}
}

// T1.3: a rerun on REFUNDING fills missing applications/permits but never
// creates a second batch, refund_id or permit revoke.
func TestTccCancelResumeFillsMissingApplicationsAndPermits(t *testing.T) {
	t.Run("none permit", func(t *testing.T) {
		db := orderTestDB(t)
		svc := tccTestService(t, db, &fakePaymentPort{captured: 10000})
		ctx := context.Background()
		seedTccOrder(t, db, "ORD-TCC-RESUME", 10000, 10000, 0, 0, "PAY-RESUME")
		seedTccGroup(t, db, "ORD-TCC-RESUME", "G1", 1, 0)
		seedTccLine(t, db, "ORD-TCC-RESUME", 1, "G1", 10000)
		if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
			OrderNo: "ORD-TCC-RESUME", PaymentNo: "PAY-RESUME", PaidCent: 10000,
		}); err != nil {
			t.Fatal(err)
		}
		batch := loadTccBatchRow(t, db, "ORD-TCC-RESUME", "PAY-RESUME")
		// Simulate a crash before the group's application and permit were durable.
		if _, err := db.Exec(`DELETE FROM refund_applications WHERE batch_id = ?`, batch.BatchID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`DELETE FROM order_shipping_permits WHERE order_no = 'ORD-TCC-RESUME'`); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
			OrderNo: "ORD-TCC-RESUME", PaymentNo: "PAY-RESUME", PaidCent: 10000,
		}); err != nil {
			t.Fatal(err)
		}
		if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-RESUME'`); got != 1 {
			t.Fatalf("resume created a second batch: %d", got)
		}
		resumed := loadTccBatchRow(t, db, "ORD-TCC-RESUME", "PAY-RESUME")
		if resumed.BatchID != batch.BatchID {
			t.Fatalf("resume changed batch %s -> %s", batch.BatchID, resumed.BatchID)
		}
		if got := countRows(t, db, `SELECT COUNT(*) FROM refund_applications WHERE batch_id = ?`, batch.BatchID); got != 1 {
			t.Fatalf("applications after resume = %d, want 1", got)
		}
		var state string
		if err := db.QueryRow(`SELECT state FROM order_shipping_permits
			WHERE order_no = 'ORD-TCC-RESUME' AND fulfillment_group_no = 'G1'`).Scan(&state); err != nil {
			t.Fatal(err)
		}
		if state != permitRevoked {
			t.Fatalf("resumed permit = %s, want REVOKED", state)
		}
	})

	t.Run("consumed permit", func(t *testing.T) {
		db := orderTestDB(t)
		svc := tccTestService(t, db, &fakePaymentPort{captured: 10000})
		ctx := context.Background()
		seedTccOrder(t, db, "ORD-TCC-RESUME2", 10000, 10000, 0, 0, "PAY-RESUME2")
		seedTccGroup(t, db, "ORD-TCC-RESUME2", "G1", 1, 0)
		seedTccLine(t, db, "ORD-TCC-RESUME2", 1, "G1", 10000)
		seedTccPermit(t, db, "ORD-TCC-RESUME2", "G1", permitConsumed, "SHIP-RESUME2")
		if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
			OrderNo: "ORD-TCC-RESUME2", PaymentNo: "PAY-RESUME2", PaidCent: 10000,
		}); err != nil {
			t.Fatal(err)
		}
		batch := loadTccBatchRow(t, db, "ORD-TCC-RESUME2", "PAY-RESUME2")
		if _, err := db.Exec(`DELETE FROM refund_applications WHERE batch_id = ?`, batch.BatchID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`DELETE FROM order_refund_admissions WHERE order_no = 'ORD-TCC-RESUME2'`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`DELETE FROM order_refund_commitment_lines WHERE refund_id = 'tcc:PAY-RESUME2:G1'`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`DELETE FROM order_refund_commitments WHERE refund_id = 'tcc:PAY-RESUME2:G1'`); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
			OrderNo: "ORD-TCC-RESUME2", PaymentNo: "PAY-RESUME2", PaidCent: 10000,
		}); err != nil {
			t.Fatal(err)
		}
		if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-RESUME2'`); got != 1 {
			t.Fatalf("resume created a second batch: %d", got)
		}
		for _, query := range []string{
			`SELECT COUNT(*) FROM refund_applications WHERE refund_id = 'tcc:PAY-RESUME2:G1'`,
			`SELECT COUNT(*) FROM order_refund_admissions WHERE refund_id = 'tcc:PAY-RESUME2:G1'`,
			`SELECT COUNT(*) FROM order_refund_commitments WHERE refund_id = 'tcc:PAY-RESUME2:G1'`,
			`SELECT COUNT(*) FROM order_refund_commitment_lines WHERE refund_id = 'tcc:PAY-RESUME2:G1'`,
		} {
			if got := countRows(t, db, query); got != 1 {
				t.Fatalf("resume did not restore the group ledger (%s -> %d)", query, got)
			}
		}
		var state string
		if err := db.QueryRow(`SELECT state FROM order_shipping_permits
			WHERE order_no = 'ORD-TCC-RESUME2' AND fulfillment_group_no = 'G1'`).Scan(&state); err != nil {
			t.Fatal(err)
		}
		if state != permitConsumed {
			t.Fatalf("resumed CONSUMED permit = %s", state)
		}
	})
}

// T1.2/T1.3: REFUNDING is not an early exit; if a batch is missing it is
// created (with the same Σ gate), never skipped.
func TestTccCancelRefundingOrderWithoutBatchCreatesBatch(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 10000})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-ORPHAN", 10000, 10000, 0, 0, "PAY-ORPHAN")
	seedTccGroup(t, db, "ORD-TCC-ORPHAN", "G1", 1, 0)
	seedTccLine(t, db, "ORD-TCC-ORPHAN", 1, "G1", 10000)
	if _, err := db.Exec(`UPDATE orders SET refund_state = ?, version = version + 1 WHERE order_no = 'ORD-TCC-ORPHAN'`,
		refundStateRefunding); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-ORPHAN", PaymentNo: "PAY-ORPHAN", PaidCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	batch := loadTccBatchRow(t, db, "ORD-TCC-ORPHAN", "PAY-ORPHAN")
	if batch.State != refundBatchFrozen {
		t.Fatalf("batch state = %s", batch.State)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_applications WHERE batch_id = ?`, batch.BatchID); got != 1 {
		t.Fatalf("applications = %d, want 1", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_status_logs WHERE order_no = 'ORD-TCC-ORPHAN'`); got != 0 {
		t.Fatalf("resume rewrote the lifecycle status log: %d rows", got)
	}
}

// AC-65: a resume whose authoritative Payment snapshot changed stops before
// writing; it never extends a stale batch into a second refund.
func TestTccCancelResumeBlocksWhenPaymentSnapshotChanges(t *testing.T) {
	db := orderTestDB(t)
	payments := &countingPaymentPort{captured: 10000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-CHANGED", 10000, 10000, 0, 0, "PAY-CHANGED")
	seedTccGroup(t, db, "ORD-TCC-CHANGED", "G1", 1, 0)
	seedTccLine(t, db, "ORD-TCC-CHANGED", 1, "G1", 10000)
	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-CHANGED", PaymentNo: "PAY-CHANGED", PaidCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	batchBefore := loadTccBatchRow(t, db, "ORD-TCC-CHANGED", "PAY-CHANGED")
	payments.setRefunded(100)

	_, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-CHANGED", PaymentNo: "PAY-CHANGED", PaidCent: 10000,
	})
	if err == nil {
		t.Fatal("resume accepted a changed Payment snapshot")
	}
	if !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("error code=%v, want FAILED_PRECONDITION: %v", apperr.From(err).Code, err)
	}
	batchAfter := loadTccBatchRow(t, db, "ORD-TCC-CHANGED", "PAY-CHANGED")
	if batchAfter != batchBefore {
		t.Fatalf("failed resume mutated the batch: %+v -> %+v", batchBefore, batchAfter)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_applications WHERE batch_id = ?`, batchBefore.BatchID); got != 1 {
		t.Fatalf("failed resume duplicated applications: %d", got)
	}
}

// AC-65/AC-40 boundary: pay_cent > 0 with no captured money is not a group
// refund. T1.6 owns the cancel/zero-converge branch, so T1.2 fails closed and
// writes nothing instead of silently reporting success.
func TestTccCancelNoCapturedFailsClosed(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 0})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-NOCAP", 10000, 10000, 0, 0, "PAY-NOCAP")
	seedTccGroup(t, db, "ORD-TCC-NOCAP", "G1", 1, 0)
	seedTccLine(t, db, "ORD-TCC-NOCAP", 1, "G1", 10000)
	version := tccVersion(t, db, "ORD-TCC-NOCAP")

	_, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-NOCAP", PaymentNo: "PAY-NOCAP", PaidCent: 10000,
	})
	if err == nil || !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("no-captured TCC cancel = %v, want FAILED_PRECONDITION", err)
	}
	assertNoTccStateWrites(t, db, "ORD-TCC-NOCAP", version)
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-NOCAP'`); got != 0 {
		t.Fatalf("no-captured path wrote refund batches: %d", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_shipping_permits WHERE order_no = 'ORD-TCC-NOCAP'`); got != 0 {
		t.Fatalf("no-captured path created permits: %d", got)
	}
}

// ADR §4.3: a commitment whose goods+freight composition is unknown cannot be
// partially attributed. Tx1 must roll back entirely instead of guessing.
func TestTccCancelIncompleteCommitmentCompositionFailsClosed(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 10000, refunded: 100})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-LEGACY", 10000, 10000, 0, 100, "PAY-LEGACY")
	seedTccGroup(t, db, "ORD-TCC-LEGACY", "G1", 1, 0)
	seedTccLine(t, db, "ORD-TCC-LEGACY", 1, "G1", 10000)
	seedTccPermit(t, db, "ORD-TCC-LEGACY", "G1", permitConsumed, "SHIP-LEGACY")
	// Old writer: amount only, composition columns left at 0.
	seedTccCommitment(t, db, "ORD-TCC-LEGACY", "G1", "R-LEGACY", 100, commitmentReserved)
	version := tccVersion(t, db, "ORD-TCC-LEGACY")

	_, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-LEGACY", PaymentNo: "PAY-LEGACY", PaidCent: 10000,
	})
	if err == nil || !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("incomplete composition = %v, want FAILED_PRECONDITION", err)
	}
	var refundState string
	var versionAfter int64
	if err := db.QueryRow(`SELECT refund_state, version FROM orders WHERE order_no = 'ORD-TCC-LEGACY'`).
		Scan(&refundState, &versionAfter); err != nil {
		t.Fatal(err)
	}
	if refundState != refundStateNone || versionAfter != version {
		t.Fatalf("order mutated after rollback: refund_state=%s version=%d (want NONE/%d)", refundState, versionAfter, version)
	}
	for _, query := range []string{
		`SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-LEGACY'`,
		`SELECT COUNT(*) FROM refund_applications WHERE order_no = 'ORD-TCC-LEGACY'`,
		`SELECT COUNT(*) FROM order_refund_admissions WHERE order_no = 'ORD-TCC-LEGACY'`,
	} {
		if got := countRows(t, db, query); got != 0 {
			t.Fatalf("rolled-back Tx1 left rows (%s -> %d)", query, got)
		}
	}
	var permitState string
	if err := db.QueryRow(`SELECT state FROM order_shipping_permits
		WHERE order_no = 'ORD-TCC-LEGACY' AND fulfillment_group_no = 'G1'`).Scan(&permitState); err != nil {
		t.Fatal(err)
	}
	if permitState != permitConsumed {
		t.Fatalf("rolled-back Tx1 changed the permit to %s", permitState)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitments WHERE refund_id = 'R-LEGACY'`); got != 1 {
		t.Fatalf("legacy commitment was mutated: %d", got)
	}
}

// gatedPaymentPort blocks the first Payment read until released so a test can
// deterministically interleave two TCC cancels.
type gatedPaymentPort struct {
	mu       sync.Mutex
	calls    int
	entered  chan struct{}
	release  chan struct{}
	captured int64
}

func (f *gatedPaymentPort) GetIntent(context.Context, *pb.GetIntentRequest, ...grpc.CallOption) (*pb.Intent, error) {
	f.mu.Lock()
	f.calls++
	call := f.calls
	f.mu.Unlock()
	if call == 1 {
		close(f.entered)
		<-f.release
	}
	return &pb.Intent{PaymentNo: "PAY-1", CapturedCent: f.captured}, nil
}

func (f *gatedPaymentPort) ListOrderIntents(context.Context, *pb.ListOrderIntentsRequest, ...grpc.CallOption) (*pb.IntentList, error) {
	return &pb.IntentList{}, nil
}

// T1.3 concurrency: a caller whose Σ precheck raced a winning TCC freeze must
// converge on the existing batch instead of reporting a spurious mismatch (the
// winner's own commitments make the strict Σ look short).
func TestTccCancelConcurrentPrecheckConvergesOnFrozenBatch(t *testing.T) {
	db := orderTestDB(t)
	payments := &gatedPaymentPort{captured: 10000, entered: make(chan struct{}), release: make(chan struct{})}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-GATE", 10000, 10000, 0, 0, "PAY-GATE")
	seedTccGroup(t, db, "ORD-TCC-GATE", "G1", 1, 0)
	seedTccLine(t, db, "ORD-TCC-GATE", 1, "G1", 10000)
	seedTccPermit(t, db, "ORD-TCC-GATE", "G1", permitConsumed, "SHIP-GATE")

	var firstErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, firstErr = svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
			OrderNo: "ORD-TCC-GATE", PaymentNo: "PAY-GATE", PaidCent: 10000,
		})
	}()
	<-payments.entered // the first caller already read the order as NONE
	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-GATE", PaymentNo: "PAY-GATE", PaidCent: 10000,
	}); err != nil {
		t.Fatalf("winner: %v", err)
	}
	close(payments.release)
	wg.Wait()
	if firstErr != nil {
		t.Fatalf("racing caller did not converge on the frozen batch: %v", firstErr)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-GATE'`); got != 1 {
		t.Fatalf("batches = %d, want 1", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitments WHERE order_no = 'ORD-TCC-GATE'`); got != 1 {
		t.Fatalf("commitments = %d, want 1", got)
	}
	var state string
	if err := db.QueryRow(`SELECT state FROM order_shipping_permits
		WHERE order_no = 'ORD-TCC-GATE' AND fulfillment_group_no = 'G1'`).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != permitConsumed {
		t.Fatalf("permit = %s, want CONSUMED", state)
	}
}

func seedTccPendingOrder(t *testing.T, db *sql.DB, orderNo string, payCent, paidCent, freightCent int64) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO orders
		(order_no,user_id,status,total_cent,discount_cent,promotion_discount_cent,applied_promotions_json,
		 freight_cent,pay_cent,paid_cent,settlement_payment_no,refunded_cent,refund_status,refund_state,
		 receiver_name,receiver_phone,receiver_address,quote_id,client_request_id,
		 allocation_frozen_at,allocation_hash,package_count,warehouse_count,expire_at,created_at)
		VALUES (?,1,'PENDING_PAYMENT',?,0,0,'[]',?,?,?,'',0,'','NONE',
		 'A','1','R','','',NULL,'',1,1,NOW(3),NOW(3))`,
		orderNo, payCent, freightCent, payCent, paidCent); err != nil {
		t.Fatal(err)
	}
}

// Addendum 2 R-3: PENDING_PAYMENT + captured money belongs to T1.6. Until it
// lands the compensation must fail closed with an identifiable error, write
// nothing and not even call Payment.
func TestTccCancelPendingCapturedGuardWritesNothing(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	seedTccPendingOrder(t, db, "ORD-TCC-PENDING-CAP", 10000, 0, 0)
	seedTccGroup(t, db, "ORD-TCC-PENDING-CAP", "G1", 1, 0)
	seedTccLine(t, db, "ORD-TCC-PENDING-CAP", 1, "G1", 10000)
	version := tccVersion(t, db, "ORD-TCC-PENDING-CAP")

	_, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-PENDING-CAP", PaymentNo: "PAY-PENDING-CAP", PaidCent: 10000,
	})
	if err == nil {
		t.Fatal("captured PENDING_PAYMENT compensation was accepted")
	}
	if !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("error code=%v, want FAILED_PRECONDITION: %v", apperr.From(err).Code, err)
	}
	if got := apperr.From(err).Details["tcc_error"]; got != tccErrPendingCaptured {
		t.Fatalf("tcc_error detail = %v, want %s", got, tccErrPendingCaptured)
	}
	if payments.getCalls != 0 {
		t.Fatalf("PENDING guard called Payment %d times", payments.getCalls)
	}
	assertNoTccStateWrites(t, db, "ORD-TCC-PENDING-CAP", version)
	for _, query := range []string{
		`SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-PENDING-CAP'`,
		`SELECT COUNT(*) FROM order_shipping_permits WHERE order_no = 'ORD-TCC-PENDING-CAP'`,
	} {
		if got := countRows(t, db, query); got != 0 {
			t.Fatalf("PENDING guard wrote rows (%s -> %d)", query, got)
		}
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM orders WHERE order_no = 'ORD-TCC-PENDING-CAP'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != statusPendingPayment {
		t.Fatalf("PENDING guard changed status to %s", status)
	}
}

// Regression: the genuinely uncaptured PENDING branch still cancels locally
// (AC-65 no-captured branch) and never creates a refund batch.
func TestTccCancelPendingWithoutCaptureStillCancelsLocally(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	seedTccPendingOrder(t, db, "ORD-TCC-PENDING-ZERO", 10000, 0, 0)

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-PENDING-ZERO", PaymentNo: "PAY-PENDING-ZERO", PaidCent: 0,
	}); err != nil {
		t.Fatal(err)
	}
	var status, refundState string
	if err := db.QueryRow(`SELECT status, refund_state FROM orders WHERE order_no = 'ORD-TCC-PENDING-ZERO'`).
		Scan(&status, &refundState); err != nil {
		t.Fatal(err)
	}
	if status != statusCancelled || refundState != refundStateNone {
		t.Fatalf("local cancel = %s/%s, want CANCELLED/NONE", status, refundState)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-PENDING-ZERO'`); got != 0 {
		t.Fatalf("local cancel wrote refund batches: %d", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-TCC-PENDING-ZERO' AND event_type = 'order.cancelled'`); got != 1 {
		t.Fatalf("order.cancelled events = %d, want 1", got)
	}
	if payments.getCalls != 0 {
		t.Fatalf("local cancel called Payment %d times", payments.getCalls)
	}
}

// Addendum 2 R-2: an EXCEPTION shipment with shipped_at is already out of the
// warehouse even when the Order group projection is not SHIPPED. It must take
// the refund-only POST path: never revoke a permit, never reserve a commitment.
func TestTccCancelExceptionShippedGroupIsPostShipAllowed(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	fulfillment := &fakeFulfillmentPort{status: "CANCELLED", shipments: []*pb.Shipment{{
		ShipmentNo: "SHIP-EX", OrderNo: "ORD-TCC-EX", FulfillmentGroupNo: "G1",
		Status: "CREATED", ExceptionState: "OPEN", ShippedAtUnix: 1700000000,
	}}}
	svc := tccTestServiceWithFulfillment(t, db, payments, fulfillment)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-EX", 10000, 10000, 0, 0, "PAY-EX")
	seedTccGroup(t, db, "ORD-TCC-EX", "G1", 1, 0)
	seedTccLine(t, db, "ORD-TCC-EX", 1, "G1", 10000)

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-EX", PaymentNo: "PAY-EX", PaidCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	if got := fulfillment.listCalls.Load(); got != 1 {
		t.Fatalf("ListShipments calls = %d, want 1 before Tx1", got)
	}
	var result string
	var amount int64
	if err := db.QueryRow(`SELECT admission_result, amount_cent FROM refund_applications
		WHERE refund_id = 'tcc:PAY-EX:G1'`).Scan(&result, &amount); err != nil {
		t.Fatal(err)
	}
	if result != refundResultPost || amount != 10000 {
		t.Fatalf("exception-shipped application = %s/%d, want POST/10000", result, amount)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_shipping_permits WHERE order_no = 'ORD-TCC-EX'`); got != 0 {
		t.Fatalf("exception-shipped group had a permit write: %d", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_admissions WHERE order_no = 'ORD-TCC-EX'`); got != 0 {
		t.Fatalf("exception-shipped group wrote a remote admission: %d", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitments WHERE order_no = 'ORD-TCC-EX'`); got != 0 {
		t.Fatalf("exception-shipped group reserved a commitment: %d", got)
	}
}

// Addendum 2 R-2: exception_state without shipped_at is ambiguous and must
// fail closed before any write.
func TestTccCancelExceptionWithoutShippedAtFailsClosed(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	fulfillment := &fakeFulfillmentPort{status: "CANCELLED", shipments: []*pb.Shipment{{
		ShipmentNo: "SHIP-EX2", OrderNo: "ORD-TCC-EX2", FulfillmentGroupNo: "G1",
		Status: "CREATED", ExceptionState: "OPEN", ShippedAtUnix: 0,
	}}}
	svc := tccTestServiceWithFulfillment(t, db, payments, fulfillment)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-EX2", 10000, 10000, 0, 0, "PAY-EX2")
	seedTccGroup(t, db, "ORD-TCC-EX2", "G1", 1, 0)
	seedTccLine(t, db, "ORD-TCC-EX2", 1, "G1", 10000)
	version := tccVersion(t, db, "ORD-TCC-EX2")

	_, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-EX2", PaymentNo: "PAY-EX2", PaidCent: 10000,
	})
	if err == nil || !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("ambiguous exception = %v, want FAILED_PRECONDITION", err)
	}
	assertNoTccStateWrites(t, db, "ORD-TCC-EX2", version)
	for _, query := range []string{
		`SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-EX2'`,
		`SELECT COUNT(*) FROM order_shipping_permits WHERE order_no = 'ORD-TCC-EX2'`,
	} {
		if got := countRows(t, db, query); got != 0 {
			t.Fatalf("ambiguous exception wrote rows (%s -> %d)", query, got)
		}
	}
}

// Addendum 2 R-2: Fulfillment unavailable fails closed before any write; the
// refund never guesses "not shipped".
func TestTccCancelShipmentFactsUnavailableFailsClosed(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	fulfillment := &fakeFulfillmentPort{status: "CANCELLED", listErr: errors.New("fulfillment timeout")}
	svc := tccTestServiceWithFulfillment(t, db, payments, fulfillment)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-EXDOWN", 10000, 10000, 0, 0, "PAY-EXDOWN")
	seedTccGroup(t, db, "ORD-TCC-EXDOWN", "G1", 1, 0)
	seedTccLine(t, db, "ORD-TCC-EXDOWN", 1, "G1", 10000)
	version := tccVersion(t, db, "ORD-TCC-EXDOWN")

	_, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-EXDOWN", PaymentNo: "PAY-EXDOWN", PaidCent: 10000,
	})
	if err == nil || !apperr.IsCode(err, apperr.CodeUnavailable) {
		t.Fatalf("fulfillment timeout = %v, want UNAVAILABLE", err)
	}
	assertNoTccStateWrites(t, db, "ORD-TCC-EXDOWN", version)
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-EXDOWN'`); got != 0 {
		t.Fatalf("unavailable fulfillment wrote refund batches: %d", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_shipping_permits WHERE order_no = 'ORD-TCC-EXDOWN'`); got != 0 {
		t.Fatalf("unavailable fulfillment wrote permits: %d", got)
	}
}

// R-1: the Tx1 recheck failure (facts changed between the outside precheck and
// the order lock) rolls back and records the PRECHECK_FAILED audit batch too.
func TestTccCancelTx1RecheckMismatchWritesAuditOnly(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 10000})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-TX1", 10000, 10000, 0, 0, "PAY-TX1")
	seedTccGroup(t, db, "ORD-TCC-TX1", "G1", 1, 0)
	seedTccLine(t, db, "ORD-TCC-TX1", 1, "G1", 9000) // Σ = 9000 vs captured 10000
	version := tccVersion(t, db, "ORD-TCC-TX1")

	err := svc.applyTccCompensation(ctx, "ORD-TCC-TX1",
		&tccPaymentSnapshot{PaymentNo: "PAY-TX1", CapturedCent: 10000},
		&tccShipmentFacts{byGroup: map[string]*pb.Shipment{}}, 10000)
	if err == nil || !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("Tx1 recheck mismatch = %v, want FAILED_PRECONDITION", err)
	}
	if got := apperr.From(err).Details["sum_group_cap_cent"]; got != int64(9000) {
		t.Fatalf("Tx1 recheck lost its numbers: %v", apperr.From(err).Details)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-TX1' AND state = ?`, refundBatchFailedPre); got != 1 {
		t.Fatalf("Tx1 audit batches = %d, want 1", got)
	}
	assertNoTccStateWrites(t, db, "ORD-TCC-TX1", version)
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_shipping_permits WHERE order_no = 'ORD-TCC-TX1'`); got != 0 {
		t.Fatalf("Tx1 mismatch wrote permits: %d", got)
	}
}
