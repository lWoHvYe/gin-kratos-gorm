package order

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

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
	mu          sync.Mutex
	captured    int64
	refunded    int64
	getCalls    int
	refundCalls int
	refundFacts map[string]*pb.Refund
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

// Refund is counted under the same mutex because T1.4b1's payment phase runs
// concurrently with the admission phase in the race test.
func (f *countingPaymentPort) Refund(context.Context, *pb.RefundRequest, ...grpc.CallOption) (*pb.Empty, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refundCalls++
	return &pb.Empty{}, nil
}

func (f *countingPaymentPort) GetRefundByKey(_ context.Context, in *pb.GetRefundByKeyRequest, _ ...grpc.CallOption) (*pb.Refund, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if fact := f.refundFacts[in.GetRefundId()]; fact != nil {
		return fact, nil
	}
	return nil, apperr.NotFound("refund not found")
}

func (f *countingPaymentPort) RefundBatch(_ context.Context, in *pb.RefundBatchRequest, _ ...grpc.CallOption) (*pb.RefundBatchReply, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	results := make([]*pb.RefundBatchItemResult, 0, len(in.GetItems()))
	var total, newSum int64
	for _, item := range in.GetItems() {
		total += item.GetAmountCent()
		if fact := f.refundFacts[item.GetRefundId()]; fact != nil {
			results = append(results, &pb.RefundBatchItemResult{
				RefundId: item.GetRefundId(), RefundNo: fact.GetRefundNo(),
				Status: refundBatchStatusAlreadyRefunded, AmountCent: item.GetAmountCent(),
			})
			continue
		}
		newSum += item.GetAmountCent()
	}
	if total != in.GetExpectedTotalCent() {
		return nil, fakeRefundBatchErr(apperr.CodeInvalidArgument, "BATCH_SUM_MISMATCH", "expected_total mismatch")
	}
	if newSum > f.captured-f.refunded {
		return nil, fakeRefundBatchErr(apperr.CodeFailedPrecondition, "INSUFFICIENT_REMAINING", "insufficient captured remaining")
	}
	status := refundBatchStatusAlreadyRefunded
	if newSum > 0 {
		status = refundBatchStatusRefunded
		f.refunded += newSum
		if f.refundFacts == nil {
			f.refundFacts = make(map[string]*pb.Refund)
		}
		for _, item := range in.GetItems() {
			if f.refundFacts[item.GetRefundId()] != nil {
				continue
			}
			fact := &pb.Refund{
				RefundNo: "RFD-" + item.GetRefundId(), RefundId: item.GetRefundId(),
				PaymentNo: in.GetPaymentNo(), AmountCent: item.GetAmountCent(),
				Status: tccRefundStatusSucceeded, CreatedAtUnix: time.Now().Unix(),
			}
			f.refundFacts[item.GetRefundId()] = fact
			results = append(results, &pb.RefundBatchItemResult{
				RefundId: item.GetRefundId(), RefundNo: fact.GetRefundNo(),
				Status: refundBatchStatusRefunded, AmountCent: item.GetAmountCent(),
			})
		}
	}
	return &pb.RefundBatchReply{
		BatchId: in.GetBatchId(), Status: status, RefundedCent: total, ItemResults: results,
	}, nil
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

// seedMigratedTccCommitment is the order/0018 historical-row shape:
// goods=amount, freight=0, composition_known=0, and no line rows.
func seedMigratedTccCommitment(t *testing.T, db *sql.DB, orderNo, groupNo, refundID string, amount int64) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO order_refund_commitments
		(order_no, group_no, refund_id, amount_cent, goods_cent, freight_cent,
		 composition_known, composition_source, state)
		VALUES (?, ?, ?, ?, ?, 0, 0, 'legacy', ?)`,
		orderNo, groupNo, refundID, amount, amount, commitmentReserved); err != nil {
		t.Fatal(err)
	}
}

// seedKnownTccCommitment is a composition_known=1 commitment whose line rows
// may or may not exist; used to prove the strict line validation stays on.
func seedKnownTccCommitment(t *testing.T, db *sql.DB, orderNo, groupNo, refundID string, amount, goods, freight int64) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO order_refund_commitments
		(order_no, group_no, refund_id, amount_cent, goods_cent, freight_cent,
		 composition_known, composition_source, state)
		VALUES (?, ?, ?, ?, ?, ?, 1, 'aftersale', ?)`,
		orderNo, groupNo, refundID, amount, goods, freight, commitmentReserved); err != nil {
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

// runTccTx1 runs ONLY Tx1 of a TCC compensation (ADR §4.1.2 step 3). T1.4a
// added the per-group admission phase after Tx1, so the Tx1-only tests call
// this helper to keep asserting the frozen ledger before admission resolves it.
func runTccTx1(t *testing.T, ctx context.Context, svc *Service, db *sql.DB, orderNo, paymentNo string, paid int64) error {
	t.Helper()
	record, err := findOrder(ctx, db, orderNo)
	if err != nil {
		t.Fatal(err)
	}
	if record == nil {
		t.Fatalf("order %s not found", orderNo)
	}
	snapshot, err := svc.readTccPaymentSnapshot(ctx, record, paymentNo)
	if err != nil {
		return err
	}
	shipments, err := svc.loadTccShipmentFacts(ctx, orderNo)
	if err != nil {
		return err
	}
	return svc.applyTccCompensation(ctx, orderNo, snapshot, shipments, paid)
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
	var admittedID string
	var admittedSeq int
	// T1.4a admission + T1.4b1 payment: the batch is PAYING when the
	// compensation call returns (refunds accepted, payment events pending).
	if err := db.QueryRow(`SELECT batch_id, attempt_seq FROM refund_batches
		WHERE order_no = 'ORD-TCC-AUDIT' AND state = ?`, refundBatchPaying).Scan(&admittedID, &admittedSeq); err != nil {
		t.Fatal(err)
	}
	if admittedID != "tcc:PAY-AUDIT:3" || admittedSeq != 3 {
		t.Fatalf("successful attempt = %s/seq %d, want tcc:PAY-AUDIT:3/3", admittedID, admittedSeq)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-AUDIT' AND state = ?`, refundBatchFailedPre); got != 2 {
		t.Fatalf("audit attempts after success = %d, want 2", got)
	}
	// A repeat on the admitted batch resumes it; no new batch/attempt. The
	// repeat also sees the fake's successful Payment refunds (no event consumer
	// runs here) and lazily reconciles the ledger, so the batch converges DONE.
	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-AUDIT", PaymentNo: "PAY-AUDIT", PaidCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-AUDIT'`); got != 3 {
		t.Fatalf("batches after resume = %d, want 3 (2 audit + 1 admitted)", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-AUDIT' AND state = ?`, refundBatchDone); got != 1 {
		t.Fatalf("done batches = %d, want 1 after lazy reconcile", got)
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

	if err := runTccTx1(t, ctx, svc, db, "ORD-TCC-OK", "PAY-OK", 10000); err != nil {
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

	if err := runTccTx1(t, ctx, svc, db, "ORD-TCC-CONSUMED", "PAY-CONSUMED", 10000); err != nil {
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

	if err := runTccTx1(t, ctx, svc, db, "ORD-TCC-SHIPPED", "PAY-SHIPPED", 10000); err != nil {
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
	// T1.4a resolves every active group to PRE and persists one admission row
	// per group; the permits stay REVOKED once.
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_admissions WHERE order_no = 'ORD-TCC-RACE'`); got != 2 {
		t.Fatalf("resolved admissions = %d, want 2", got)
	}
	for _, groupNo := range []string{"G1", "G2"} {
		var appState, appResult string
		if err := db.QueryRow(`SELECT state, admission_result FROM refund_applications
			WHERE refund_id = ?`, tccGroupRefundID("PAY-RACE", groupNo)).Scan(&appState, &appResult); err != nil {
			t.Fatal(err)
		}
		// The repeat after the race lazily reconciles the fake's successful
		// Payment facts: DONE is the expected converged state, and the resolved
		// admission result is still PRE.
		if appState != refundApplicationDone || appResult != refundResultPre {
			t.Fatalf("%s application = %s/%s, want DONE/PRE", groupNo, appState, appResult)
		}
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
	if batch.State != refundBatchPaying {
		t.Fatalf("batch state = %s, want PAYING after admission + T1.4b1 payment", batch.State)
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
	// Freeze Tx1 only (FROZEN batch): the strict Σ/snapshot resume gate applies
	// before admission. Once a batch is ADMITTED/PAYING the Payment authority
	// gates the remaining amounts instead (T1.4b1, ADR §4.1.3(5)).
	if err := runTccTx1(t, ctx, svc, db, "ORD-TCC-CHANGED", "PAY-CHANGED", 10000); err != nil {
		t.Fatal(err)
	}
	batchBefore := loadTccBatchRow(t, db, "ORD-TCC-CHANGED", "PAY-CHANGED")
	// The failed resume must not add a second application/admission/commitment
	// set to the frozen Tx1 ledger.
	commitmentsBefore := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitments WHERE order_no = 'ORD-TCC-CHANGED'`)
	admissionsBefore := countRows(t, db, `SELECT COUNT(*) FROM order_refund_admissions WHERE order_no = 'ORD-TCC-CHANGED'`)
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
	// Plan T1.2-R R-4 (m1): the resume Σ/snapshot change is audited as a new
	// PRECHECK_FAILED attempt, while the order/permit/commitment/applications
	// stay untouched.
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches
		WHERE order_no = 'ORD-TCC-CHANGED' AND state = ?`, refundBatchFailedPre); got != 1 {
		t.Fatalf("resume audit batches = %d, want 1", got)
	}
	var auditSum, auditRemaining int64
	var auditAttempt int
	if err := db.QueryRow(`SELECT attempt_seq, sum_group_cap_cent, remaining_captured_cent
		FROM refund_batches WHERE order_no = 'ORD-TCC-CHANGED' AND state = ?`, refundBatchFailedPre).
		Scan(&auditAttempt, &auditSum, &auditRemaining); err != nil {
		t.Fatal(err)
	}
	if auditAttempt != 2 || auditSum != 10000 || auditRemaining != 9900 {
		t.Fatalf("resume audit = attempt %d sum %d remaining %d, want 2/10000/9900", auditAttempt, auditSum, auditRemaining)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitments WHERE order_no = 'ORD-TCC-CHANGED'`); got != commitmentsBefore {
		t.Fatalf("failed resume wrote commitments: %d -> %d", commitmentsBefore, got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_admissions WHERE order_no = 'ORD-TCC-CHANGED'`); got != admissionsBefore {
		t.Fatalf("failed resume wrote admissions: %d -> %d", admissionsBefore, got)
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

// Plan T1.2-R M1 / migration order/0018 shape: a historical commitment is
// goods=amount, freight=0, composition_known=0 and has no line rows. The group
// may still be refunded as a whole: the cap is groupPaid + freight_share -
// RESERVED/SETTLED, and the new TCC commitment is written composition_known=0
// with no lines. No line attribution is guessed.
func TestTccCancelMigratedLegacyCompositionAllowsWholeGroupRefund(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 10000, refunded: 100})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-LEGACY", 10000, 10000, 0, 100, "PAY-LEGACY")
	seedTccGroup(t, db, "ORD-TCC-LEGACY", "G1", 1, 0)
	seedTccLine(t, db, "ORD-TCC-LEGACY", 1, "G1", 10000)
	seedTccPermit(t, db, "ORD-TCC-LEGACY", "G1", permitConsumed, "SHIP-LEGACY")
	// Exactly what order/0018 writes for a historical row.
	seedMigratedTccCommitment(t, db, "ORD-TCC-LEGACY", "G1", "R-LEGACY", 100)

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-LEGACY", PaymentNo: "PAY-LEGACY", PaidCent: 10000,
	}); err != nil {
		t.Fatalf("migrated legacy composition blocked the whole-group TCC refund: %v", err)
	}
	var refundState string
	if err := db.QueryRow(`SELECT refund_state FROM orders WHERE order_no = 'ORD-TCC-LEGACY'`).Scan(&refundState); err != nil {
		t.Fatal(err)
	}
	if refundState != refundStateRefunding {
		t.Fatalf("refund_state = %s, want REFUNDING", refundState)
	}
	// CapCent = 10000 + 0 - 100 = 9900, never the old commitment amount.
	var appAmount, appGoods, appFreight int64
	if err := db.QueryRow(`SELECT amount_cent, goods_cent, freight_cent FROM refund_applications
		WHERE refund_id = 'tcc:PAY-LEGACY:G1'`).Scan(&appAmount, &appGoods, &appFreight); err != nil {
		t.Fatal(err)
	}
	if appAmount != 9900 || appGoods != 9900 || appFreight != 0 {
		t.Fatalf("application = %d/%d/%d, want 9900/9900/0", appAmount, appGoods, appFreight)
	}
	var commitmentAmount int64
	var known int
	if err := db.QueryRow(`SELECT amount_cent, composition_known FROM order_refund_commitments
		WHERE refund_id = 'tcc:PAY-LEGACY:G1'`).Scan(&commitmentAmount, &known); err != nil {
		t.Fatal(err)
	}
	if commitmentAmount != 9900 || known != 0 {
		t.Fatalf("new commitment = amount %d/known %d, want 9900/0", commitmentAmount, known)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitment_lines
		WHERE refund_id = 'tcc:PAY-LEGACY:G1'`); got != 0 {
		t.Fatalf("known=0 TCC commitment wrote %d line rows, want 0", got)
	}
	// The historical row itself is untouched.
	var legacyAmount, legacyGoods int64
	var legacyKnown int
	if err := db.QueryRow(`SELECT amount_cent, goods_cent, composition_known FROM order_refund_commitments
		WHERE refund_id = 'R-LEGACY'`).Scan(&legacyAmount, &legacyGoods, &legacyKnown); err != nil {
		t.Fatal(err)
	}
	if legacyAmount != 100 || legacyGoods != 100 || legacyKnown != 0 {
		t.Fatalf("legacy commitment mutated: %d/%d/%d", legacyAmount, legacyGoods, legacyKnown)
	}
}

// Plan T1.2-R M1: composition_known=1 commitments must still have lines whose
// sums equal the header. A mismatch rolls back the whole Tx1.
func TestTccCancelKnownCompositionLineMismatchFailsClosed(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 10000, refunded: 100})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-KNOWN", 10000, 10000, 0, 100, "PAY-KNOWN")
	seedTccGroup(t, db, "ORD-TCC-KNOWN", "G1", 1, 0)
	seedTccLine(t, db, "ORD-TCC-KNOWN", 1, "G1", 10000)
	seedTccPermit(t, db, "ORD-TCC-KNOWN", "G1", permitConsumed, "SHIP-KNOWN")
	// Header says goods=100 but no line rows were written.
	seedKnownTccCommitment(t, db, "ORD-TCC-KNOWN", "G1", "R-KNOWN", 100, 100, 0)
	version := tccVersion(t, db, "ORD-TCC-KNOWN")

	_, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-KNOWN", PaymentNo: "PAY-KNOWN", PaidCent: 10000,
	})
	if err == nil || !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("known composition mismatch = %v, want FAILED_PRECONDITION", err)
	}
	var refundState string
	var versionAfter int64
	if err := db.QueryRow(`SELECT refund_state, version FROM orders WHERE order_no = 'ORD-TCC-KNOWN'`).
		Scan(&refundState, &versionAfter); err != nil {
		t.Fatal(err)
	}
	if refundState != refundStateNone || versionAfter != version {
		t.Fatalf("order mutated after rollback: refund_state=%s version=%d (want NONE/%d)", refundState, versionAfter, version)
	}
	for _, query := range []string{
		`SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-KNOWN'`,
		`SELECT COUNT(*) FROM refund_applications WHERE order_no = 'ORD-TCC-KNOWN'`,
		`SELECT COUNT(*) FROM order_refund_admissions WHERE order_no = 'ORD-TCC-KNOWN'`,
	} {
		if got := countRows(t, db, query); got != 0 {
			t.Fatalf("rolled-back Tx1 left rows (%s -> %d)", query, got)
		}
	}
	var permitState string
	if err := db.QueryRow(`SELECT state FROM order_shipping_permits
		WHERE order_no = 'ORD-TCC-KNOWN' AND fulfillment_group_no = 'G1'`).Scan(&permitState); err != nil {
		t.Fatal(err)
	}
	if permitState != permitConsumed {
		t.Fatalf("rolled-back Tx1 changed the permit to %s", permitState)
	}
}

// gatedPaymentPort blocks the first Payment read until released so a test can
// deterministically interleave two TCC cancels.
type gatedPaymentPort struct {
	mu          sync.Mutex
	calls       int
	entered     chan struct{}
	release     chan struct{}
	captured    int64
	refunded    int64
	refundFacts map[string]*pb.Refund
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

func (f *gatedPaymentPort) Refund(context.Context, *pb.RefundRequest, ...grpc.CallOption) (*pb.Empty, error) {
	return &pb.Empty{}, nil
}

func (f *gatedPaymentPort) GetRefundByKey(_ context.Context, in *pb.GetRefundByKeyRequest, _ ...grpc.CallOption) (*pb.Refund, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if fact := f.refundFacts[in.GetRefundId()]; fact != nil {
		return fact, nil
	}
	return nil, apperr.NotFound("refund not found")
}

func (f *gatedPaymentPort) RefundBatch(_ context.Context, in *pb.RefundBatchRequest, _ ...grpc.CallOption) (*pb.RefundBatchReply, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	results := make([]*pb.RefundBatchItemResult, 0, len(in.GetItems()))
	var total, newSum int64
	for _, item := range in.GetItems() {
		total += item.GetAmountCent()
		if fact := f.refundFacts[item.GetRefundId()]; fact != nil {
			results = append(results, &pb.RefundBatchItemResult{
				RefundId: item.GetRefundId(), RefundNo: fact.GetRefundNo(),
				Status: refundBatchStatusAlreadyRefunded, AmountCent: item.GetAmountCent(),
			})
			continue
		}
		newSum += item.GetAmountCent()
	}
	if total != in.GetExpectedTotalCent() {
		return nil, fakeRefundBatchErr(apperr.CodeInvalidArgument, "BATCH_SUM_MISMATCH", "expected_total mismatch")
	}
	if newSum > f.captured-f.refunded {
		return nil, fakeRefundBatchErr(apperr.CodeFailedPrecondition, "INSUFFICIENT_REMAINING", "insufficient captured remaining")
	}
	status := refundBatchStatusAlreadyRefunded
	if newSum > 0 {
		status = refundBatchStatusRefunded
		f.refunded += newSum
		if f.refundFacts == nil {
			f.refundFacts = make(map[string]*pb.Refund)
		}
		for _, item := range in.GetItems() {
			if f.refundFacts[item.GetRefundId()] != nil {
				continue
			}
			fact := &pb.Refund{
				RefundNo: "RFD-" + item.GetRefundId(), RefundId: item.GetRefundId(),
				PaymentNo: in.GetPaymentNo(), AmountCent: item.GetAmountCent(),
				Status: tccRefundStatusSucceeded, CreatedAtUnix: time.Now().Unix(),
			}
			f.refundFacts[item.GetRefundId()] = fact
			results = append(results, &pb.RefundBatchItemResult{
				RefundId: item.GetRefundId(), RefundNo: fact.GetRefundNo(),
				Status: refundBatchStatusRefunded, AmountCent: item.GetAmountCent(),
			})
		}
	}
	return &pb.RefundBatchReply{
		BatchId: in.GetBatchId(), Status: status, RefundedCent: total, ItemResults: results,
	}, nil
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

	if err := runTccTx1(t, ctx, svc, db, "ORD-TCC-EX", "PAY-EX", 10000); err != nil {
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

func setTccOrderStatus(t *testing.T, db *sql.DB, orderNo, status string) {
	t.Helper()
	if _, err := db.Exec(`UPDATE orders SET status = ? WHERE order_no = ?`, status, orderNo); err != nil {
		t.Fatal(err)
	}
}

func seedManualTccBatch(t *testing.T, db *sql.DB, orderNo, paymentNo, state string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO refund_batches
		(batch_id, source, order_no, payment_no, attempt_seq, state, captured_snapshot_cent,
		 refunded_snapshot_cent, sum_group_cap_cent, remaining_captured_cent, attempts, version)
		VALUES (?, 'tcc', ?, ?, 1, ?, 10000, 0, 10000, 10000, 0, 1)`,
		tccBatchID(paymentNo, 1), orderNo, paymentNo, state); err != nil {
		t.Fatal(err)
	}
}

// Plan T1.2-R R-1 (B1): a real ALLOCATED order whose G1 already shipped and
// whose G2 has not. The shipped group is refund-only POST; the unshipped group
// is PRE and its permit is revoked. The order lifecycle is never rewritten.
func TestTccCancelAllocatedOrderMixedGroups(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 10000})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-ALLOC", 10000, 10000, 0, 0, "PAY-ALLOC")
	seedTccGroup(t, db, "ORD-TCC-ALLOC", "G1", 1, 0)
	seedTccGroup(t, db, "ORD-TCC-ALLOC", "G2", 2, 0)
	seedTccLine(t, db, "ORD-TCC-ALLOC", 1, "G1", 6000)
	seedTccLine(t, db, "ORD-TCC-ALLOC", 2, "G2", 4000)
	setTccGroupStatus(t, db, "ORD-TCC-ALLOC", "G1", "SHIPPED")
	setTccGroupStatus(t, db, "ORD-TCC-ALLOC", "G2", statusAllocated)
	setTccOrderStatus(t, db, "ORD-TCC-ALLOC", statusAllocated)

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-ALLOC", PaymentNo: "PAY-ALLOC", PaidCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	var status, refundState string
	if err := db.QueryRow(`SELECT status, refund_state FROM orders WHERE order_no = 'ORD-TCC-ALLOC'`).
		Scan(&status, &refundState); err != nil {
		t.Fatal(err)
	}
	if status != statusAllocated || refundState != refundStateRefunding {
		t.Fatalf("order = %s/%s, want ALLOCATED/REFUNDING (lifecycle must not change)", status, refundState)
	}
	var g1Result, g2Result string
	if err := db.QueryRow(`SELECT admission_result FROM refund_applications WHERE refund_id = 'tcc:PAY-ALLOC:G1'`).Scan(&g1Result); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT admission_result FROM refund_applications WHERE refund_id = 'tcc:PAY-ALLOC:G2'`).Scan(&g2Result); err != nil {
		t.Fatal(err)
	}
	if g1Result != refundResultPost || g2Result != refundResultPre {
		t.Fatalf("applications G1=%s G2=%s, want POST/PRE", g1Result, g2Result)
	}
	var g2Permit string
	if err := db.QueryRow(`SELECT state FROM order_shipping_permits
		WHERE order_no = 'ORD-TCC-ALLOC' AND fulfillment_group_no = 'G2'`).Scan(&g2Permit); err != nil {
		t.Fatal(err)
	}
	if g2Permit != permitRevoked {
		t.Fatalf("G2 permit = %s, want REVOKED", g2Permit)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_shipping_permits
		WHERE order_no = 'ORD-TCC-ALLOC' AND fulfillment_group_no = 'G1'`); got != 0 {
		t.Fatalf("shipped G1 permit touched: %d", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events
		WHERE aggregate_id = 'ORD-TCC-ALLOC' AND event_type = 'order.cancelled'`); got != 0 {
		t.Fatalf("TCC compensation wrote order.cancelled: %d", got)
	}
}

// Plan T1.2-R R-1: real SHIPPED lifecycle (single group already out).
func TestTccCancelShippedLifecycle(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 10000})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-SHIP", 10000, 10000, 0, 0, "PAY-SHIP")
	seedTccGroup(t, db, "ORD-TCC-SHIP", "G1", 1, 0)
	seedTccLine(t, db, "ORD-TCC-SHIP", 1, "G1", 10000)
	seedTccPermit(t, db, "ORD-TCC-SHIP", "G1", permitConsumed, "SHIP-1")
	setTccGroupStatus(t, db, "ORD-TCC-SHIP", "G1", "SHIPPED")
	setTccOrderStatus(t, db, "ORD-TCC-SHIP", statusShipped)

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-SHIP", PaymentNo: "PAY-SHIP", PaidCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	var status, result string
	if err := db.QueryRow(`SELECT status, refund_state FROM orders WHERE order_no = 'ORD-TCC-SHIP'`).Scan(&status, new(string)); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT admission_result FROM refund_applications WHERE refund_id = 'tcc:PAY-SHIP:G1'`).Scan(&result); err != nil {
		t.Fatal(err)
	}
	if status != statusShipped || result != refundResultPost {
		t.Fatalf("shipped lifecycle = %s/%s, want SHIPPED/POST", status, result)
	}
	var permit string
	if err := db.QueryRow(`SELECT state FROM order_shipping_permits
		WHERE order_no = 'ORD-TCC-SHIP' AND fulfillment_group_no = 'G1'`).Scan(&permit); err != nil {
		t.Fatal(err)
	}
	if permit != permitConsumed {
		t.Fatalf("shipped permit = %s, want CONSUMED untouched", permit)
	}
}

// Plan T1.2-R R-1: real COMPLETED lifecycle (received group).
func TestTccCancelCompletedLifecycle(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 10000})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-DONE", 10000, 10000, 0, 0, "PAY-DONE")
	seedTccGroup(t, db, "ORD-TCC-DONE", "G1", 1, 0)
	seedTccLine(t, db, "ORD-TCC-DONE", 1, "G1", 10000)
	setTccGroupStatus(t, db, "ORD-TCC-DONE", "G1", "RECEIVED")
	setTccOrderStatus(t, db, "ORD-TCC-DONE", statusCompleted)

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-DONE", PaymentNo: "PAY-DONE", PaidCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	var status, result string
	if err := db.QueryRow(`SELECT status FROM orders WHERE order_no = 'ORD-TCC-DONE'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT admission_result FROM refund_applications WHERE refund_id = 'tcc:PAY-DONE:G1'`).Scan(&result); err != nil {
		t.Fatal(err)
	}
	if status != statusCompleted || result != refundResultPost {
		t.Fatalf("completed lifecycle = %s/%s, want COMPLETED/POST", status, result)
	}
}

// Plan T1.2-R R-1: CANCELLED + captured must not be reported as a completed
// compensation before T1.6; zero writes.
func TestTccCancelCancelledCapturedFailsClosed(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 1000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-CAN", 1000, 1000, 0, 0, "PAY-CAN")
	seedTccGroup(t, db, "ORD-TCC-CAN", "G1", 1, 0)
	seedTccLine(t, db, "ORD-TCC-CAN", 1, "G1", 1000)
	setTccOrderStatus(t, db, "ORD-TCC-CAN", statusCancelled)
	version := tccVersion(t, db, "ORD-TCC-CAN")

	_, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-CAN", PaymentNo: "PAY-CAN", PaidCent: 1000,
	})
	if err == nil || !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("cancelled captured = %v, want FAILED_PRECONDITION", err)
	}
	if got := apperr.From(err).Details["tcc_error"]; got != tccErrCancelledCaptured {
		t.Fatalf("tcc_error = %v, want %s", got, tccErrCancelledCaptured)
	}
	var status, refundState string
	var versionAfter int64
	if err := db.QueryRow(`SELECT status, refund_state, version FROM orders WHERE order_no = 'ORD-TCC-CAN'`).
		Scan(&status, &refundState, &versionAfter); err != nil {
		t.Fatal(err)
	}
	if status != statusCancelled || refundState != refundStateNone || versionAfter != version {
		t.Fatalf("order mutated: %s/%s/version %d", status, refundState, versionAfter)
	}
	for _, query := range []string{
		`SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-CAN'`,
		`SELECT COUNT(*) FROM refund_applications WHERE order_no = 'ORD-TCC-CAN'`,
		`SELECT COUNT(*) FROM order_shipping_permits WHERE order_no = 'ORD-TCC-CAN'`,
		`SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-TCC-CAN'`,
	} {
		if got := countRows(t, db, query); got != 0 {
			t.Fatalf("CANCELLED+captured wrote rows (%s -> %d)", query, got)
		}
	}
}

// Plan T1.2-R R-1: CANCELLED without captured money is idempotent success.
func TestTccCancelCancelledWithoutCapturedIsIdempotent(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-CAN0", 0, 0, 0, 0, "")
	setTccOrderStatus(t, db, "ORD-TCC-CAN0", statusCancelled)
	version := tccVersion(t, db, "ORD-TCC-CAN0")

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-CAN0", PaymentNo: "", PaidCent: 0,
	}); err != nil {
		t.Fatal(err)
	}
	if got := tccVersion(t, db, "ORD-TCC-CAN0"); got != version {
		t.Fatalf("idempotent cancelled compensation mutated version %d -> %d", version, got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-CAN0'`); got != 0 {
		t.Fatalf("cancelled zero compensation wrote batches: %d", got)
	}
}

// Plan T1.2-R R-1/m2: MANUAL_HOLD and FAILED_PERMANENT batches must not be
// resumed automatically; they stay exactly as parked.
func TestTccCancelManualHoldBatchIsNotResumed(t *testing.T) {
	for _, state := range []string{"MANUAL_HOLD", "FAILED_PERMANENT"} {
		t.Run(state, func(t *testing.T) {
			db := orderTestDB(t)
			svc := tccTestService(t, db, &fakePaymentPort{captured: 10000})
			ctx := context.Background()
			orderNo := "ORD-TCC-" + state
			seedTccOrder(t, db, orderNo, 10000, 10000, 0, 0, "PAY-"+state)
			seedTccGroup(t, db, orderNo, "G1", 1, 0)
			seedTccLine(t, db, orderNo, 1, "G1", 10000)
			if _, err := db.Exec(`UPDATE orders SET refund_state = ? WHERE order_no = ?`, refundStateRefunding, orderNo); err != nil {
				t.Fatal(err)
			}
			seedManualTccBatch(t, db, orderNo, "PAY-"+state, state)
			version := tccVersion(t, db, orderNo)

			_, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
				OrderNo: orderNo, PaymentNo: "PAY-" + state, PaidCent: 10000,
			})
			if err == nil || !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
				t.Fatalf("manual batch resume = %v, want FAILED_PRECONDITION", err)
			}
			if got := apperr.From(err).Details["tcc_error"]; got != tccErrBatchManual {
				t.Fatalf("tcc_error = %v, want %s", got, tccErrBatchManual)
			}
			if got := tccVersion(t, db, orderNo); got != version {
				t.Fatalf("manual batch resume mutated order version %d -> %d", version, got)
			}
			if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = ?`, orderNo); got != 1 {
				t.Fatalf("manual batch resume created batches: %d", got)
			}
			if got := countRows(t, db, `SELECT COUNT(*) FROM refund_applications WHERE order_no = ?`, orderNo); got != 0 {
				t.Fatalf("manual batch resume wrote applications: %d", got)
			}
		})
	}
}

// Plan T1.2-R R-1: an unsupported lifecycle state fails closed instead of
// silently succeeding.
func TestTccCancelUnsupportedStatusFailsClosed(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 1000})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-CLOSED", 1000, 1000, 0, 0, "PAY-CLOSED")
	seedTccGroup(t, db, "ORD-TCC-CLOSED", "G1", 1, 0)
	seedTccLine(t, db, "ORD-TCC-CLOSED", 1, "G1", 1000)
	setTccOrderStatus(t, db, "ORD-TCC-CLOSED", statusClosed)
	version := tccVersion(t, db, "ORD-TCC-CLOSED")

	_, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-CLOSED", PaymentNo: "PAY-CLOSED", PaidCent: 1000,
	})
	if err == nil || !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("unsupported status = %v, want FAILED_PRECONDITION", err)
	}
	if got := apperr.From(err).Details["tcc_error"]; got != tccErrStatusUnsupported {
		t.Fatalf("tcc_error = %v, want %s", got, tccErrStatusUnsupported)
	}
	if got := tccVersion(t, db, "ORD-TCC-CLOSED"); got != version {
		t.Fatalf("unsupported status wrote order state")
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-CLOSED'`); got != 0 {
		t.Fatalf("unsupported status wrote batches: %d", got)
	}
}

// Plan T1.2-R R-3 (M2): Σ == remaining == 0 means every group is already
// refunded; the compensation is idempotent success and writes no REFUNDING or
// empty batch.
func TestTccCancelNoActiveGroupsIsIdempotentSuccess(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 10000, refunded: 10000})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-ALLDONE", 10000, 10000, 0, 10000, "PAY-ALLDONE")
	seedTccGroup(t, db, "ORD-TCC-ALLDONE", "G1", 1, 0)
	seedTccLine(t, db, "ORD-TCC-ALLDONE", 1, "G1", 10000)
	if _, err := db.Exec(`INSERT INTO order_refund_commitments
		(order_no, group_no, refund_id, amount_cent, goods_cent, freight_cent, composition_known, composition_source, state)
		VALUES ('ORD-TCC-ALLDONE', 'G1', 'R-ALLDONE', 10000, 10000, 0, 1, 'aftersale', 'SETTLED')`); err != nil {
		t.Fatal(err)
	}
	version := tccVersion(t, db, "ORD-TCC-ALLDONE")

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-ALLDONE", PaymentNo: "PAY-ALLDONE", PaidCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	var refundState string
	var versionAfter int64
	if err := db.QueryRow(`SELECT refund_state, version FROM orders WHERE order_no = 'ORD-TCC-ALLDONE'`).
		Scan(&refundState, &versionAfter); err != nil {
		t.Fatal(err)
	}
	if refundState != refundStateNone || versionAfter != version {
		t.Fatalf("no-active compensation wrote order state: %s/version %d", refundState, versionAfter)
	}
	for _, query := range []string{
		`SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-ALLDONE'`,
		`SELECT COUNT(*) FROM refund_applications WHERE order_no = 'ORD-TCC-ALLDONE'`,
		`SELECT COUNT(*) FROM order_shipping_permits WHERE order_no = 'ORD-TCC-ALLDONE'`,
		`SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-TCC-ALLDONE'`,
	} {
		if got := countRows(t, db, query); got != 0 {
			t.Fatalf("no-active compensation wrote rows (%s -> %d)", query, got)
		}
	}
}

// Plan T1.2-R R-3 (M2): Σ must be evaluated before the no-active shortcut, so
// active=0 with remaining captured > 0 is a Σ mismatch and gets the audit batch.
func TestTccCancelNoActiveButRemainingCapturedWritesAudit(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 10000})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-GAP", 10000, 10000, 0, 0, "PAY-GAP")
	seedTccGroup(t, db, "ORD-TCC-GAP", "G1", 1, 0)
	seedTccLine(t, db, "ORD-TCC-GAP", 1, "G1", 0) // Σ cap 0, remaining 10000
	version := tccVersion(t, db, "ORD-TCC-GAP")

	_, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-GAP", PaymentNo: "PAY-GAP", PaidCent: 10000,
	})
	if err == nil || !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("active=0/remaining>0 = %v, want FAILED_PRECONDITION", err)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-GAP' AND state = ?`, refundBatchFailedPre); got != 1 {
		t.Fatalf("audit batches = %d, want 1", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-GAP' AND state = ?`, refundBatchFrozen); got != 0 {
		t.Fatalf("empty FROZEN batch was created: %d", got)
	}
	assertNoTccStateWrites(t, db, "ORD-TCC-GAP", version)
}

// Plan T1.2-R §4 (Review m6): the order test schema must be the real C-05
// migrations (order/0017 + order/0018), not a hand-written projection.
func TestOrderTestSchemaUsesC05RefundMigrations(t *testing.T) {
	db := orderTestDB(t)
	for _, table := range []string{"refund_batches", "refund_applications", "order_refund_commitment_lines"} {
		if got := countRows(t, db, `SELECT COUNT(*) FROM information_schema.TABLES
			WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?`, table); got != 1 {
			t.Fatalf("migration table %s missing", table)
		}
	}
	for _, column := range []string{"goods_cent", "freight_cent", "composition_known", "composition_source"} {
		if got := countRows(t, db, `SELECT COUNT(*) FROM information_schema.COLUMNS
			WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'order_refund_commitments' AND COLUMN_NAME = ?`, column); got != 1 {
			t.Fatalf("migration column order_refund_commitments.%s missing", column)
		}
	}
	for _, constraint := range []string{"chk_refund_batch_state", "chk_refund_app_amount", "chk_commitment_line_kind"} {
		if got := countRows(t, db, `SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
			WHERE CONSTRAINT_SCHEMA = DATABASE() AND CONSTRAINT_NAME = ? AND CONSTRAINT_TYPE = 'CHECK'`, constraint); got != 1 {
			t.Fatalf("migration CHECK %s missing", constraint)
		}
	}
}

// Plan T1.2-R R-1: REFUNDING is a refund_state, not an order status. A
// REFUNDING + ALLOCATED order resumes its batch by refund_state and fills only
// the missing group rows.
func TestTccCancelRefundingAllocatedOrderResumesByRefundState(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 10000})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-RESUME-ALLOC", 10000, 10000, 0, 0, "PAY-RESUME-ALLOC")
	seedTccGroup(t, db, "ORD-TCC-RESUME-ALLOC", "G1", 1, 0)
	seedTccGroup(t, db, "ORD-TCC-RESUME-ALLOC", "G2", 2, 0)
	seedTccLine(t, db, "ORD-TCC-RESUME-ALLOC", 1, "G1", 6000)
	seedTccLine(t, db, "ORD-TCC-RESUME-ALLOC", 2, "G2", 4000)
	setTccOrderStatus(t, db, "ORD-TCC-RESUME-ALLOC", statusAllocated)
	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-RESUME-ALLOC", PaymentNo: "PAY-RESUME-ALLOC", PaidCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	batch := loadTccBatchRow(t, db, "ORD-TCC-RESUME-ALLOC", "PAY-RESUME-ALLOC")
	if _, err := db.Exec(`DELETE FROM refund_applications WHERE batch_id = ? AND group_no = 'G2'`, batch.BatchID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM order_shipping_permits WHERE order_no = 'ORD-TCC-RESUME-ALLOC' AND fulfillment_group_no = 'G2'`); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-RESUME-ALLOC", PaymentNo: "PAY-RESUME-ALLOC", PaidCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	var status, refundState string
	if err := db.QueryRow(`SELECT status, refund_state FROM orders WHERE order_no = 'ORD-TCC-RESUME-ALLOC'`).
		Scan(&status, &refundState); err != nil {
		t.Fatal(err)
	}
	// T1.4b1-R2: the resume sees both successful Payment facts, reconciles the
	// ledger and projects the order amount, so the fully refunded compensation
	// ends FULL while the lifecycle status stays ALLOCATED.
	if status != statusAllocated || refundState != refundStateFull {
		t.Fatalf("resume changed lifecycle: %s/%s, want ALLOCATED/FULL", status, refundState)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-RESUME-ALLOC'`); got != 1 {
		t.Fatalf("resume created a second batch: %d", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_applications WHERE batch_id = ?`, batch.BatchID); got != 2 {
		t.Fatalf("resume did not restore G2 application: %d", got)
	}
	var g2Permit string
	if err := db.QueryRow(`SELECT state FROM order_shipping_permits
		WHERE order_no = 'ORD-TCC-RESUME-ALLOC' AND fulfillment_group_no = 'G2'`).Scan(&g2Permit); err != nil {
		t.Fatal(err)
	}
	if g2Permit != permitRevoked {
		t.Fatalf("resumed G2 permit = %s, want REVOKED", g2Permit)
	}
}

// Plan T1.2-R R-3 (M2): a FULL refund projection is idempotent success with no
// writes.
func TestTccCancelFullRefundStateIsIdempotent(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 10000, refunded: 10000})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-FULLSTATE", 10000, 10000, 0, 10000, "PAY-FULLSTATE")
	seedTccGroup(t, db, "ORD-TCC-FULLSTATE", "G1", 1, 0)
	seedTccLine(t, db, "ORD-TCC-FULLSTATE", 1, "G1", 10000)
	if _, err := db.Exec(`UPDATE orders SET refund_state = ? WHERE order_no = 'ORD-TCC-FULLSTATE'`, refundStateFull); err != nil {
		t.Fatal(err)
	}
	version := tccVersion(t, db, "ORD-TCC-FULLSTATE")

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-FULLSTATE", PaymentNo: "PAY-FULLSTATE", PaidCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	if got := tccVersion(t, db, "ORD-TCC-FULLSTATE"); got != version {
		t.Fatalf("FULL compensation mutated order version")
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-FULLSTATE'`); got != 0 {
		t.Fatalf("FULL compensation wrote batches: %d", got)
	}
}

// Review G1: a historical order with no fulfillment groups whose lifecycle is
// already SHIPPED/COMPLETED must derive the synthetic G1 status from
// orders.status. Without shipment facts it is POST, and the fallback must not
// initialize/revoke a G1 permit or write a commitment.
func TestTccCancelLegacyG1ShippedUsesOrderStatus(t *testing.T) {
	for _, tc := range []struct {
		orderStatus string
	}{
		{statusShipped},
		{statusCompleted},
	} {
		t.Run(tc.orderStatus, func(t *testing.T) {
			db := orderTestDB(t)
			svc := tccTestService(t, db, &fakePaymentPort{captured: 10000})
			ctx := context.Background()
			orderNo := "ORD-TCC-LEGACY-" + tc.orderStatus
			paymentNo := "PAY-LEGACY-" + tc.orderStatus
			seedTccOrder(t, db, orderNo, 10000, 10000, 0, 0, paymentNo)
			seedTccLine(t, db, orderNo, 1, "G1", 10000)
			setTccOrderStatus(t, db, orderNo, tc.orderStatus)

			if err := runTccTx1(t, ctx, svc, db, orderNo, paymentNo, 10000); err != nil {
				t.Fatal(err)
			}
			var refundState, result string
			if err := db.QueryRow(`SELECT refund_state FROM orders WHERE order_no = ?`, orderNo).Scan(&refundState); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(`SELECT admission_result FROM refund_applications
				WHERE refund_id = ?`, tccGroupRefundID(paymentNo, "G1")).Scan(&result); err != nil {
				t.Fatal(err)
			}
			if refundState != refundStateRefunding || result != refundResultPost {
				t.Fatalf("%s = refund_state %s/app %s, want REFUNDING/POST", tc.orderStatus, refundState, result)
			}
			for _, query := range []string{
				`SELECT COUNT(*) FROM order_shipping_permits WHERE order_no = ?`,
				`SELECT COUNT(*) FROM order_refund_admissions WHERE order_no = ?`,
				`SELECT COUNT(*) FROM order_refund_commitments WHERE order_no = ?`,
				`SELECT COUNT(*) FROM order_refund_commitment_lines l
					JOIN order_refund_commitments c ON c.refund_id = l.refund_id WHERE c.order_no = ?`,
			} {
				if got := countRows(t, db, query, orderNo); got != 0 {
					t.Fatalf("%s wrote rows (%s -> %d)", tc.orderStatus, query, got)
				}
			}
		})
	}
}

// Review G1: an unknown group status with no shipment facts is ambiguous and
// must fail closed, not silently take the PRE path.
func TestTccCancelUnknownGroupStatusNoFactsFailsClosed(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 10000})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-TCC-UNKNOWN-GROUP", 10000, 10000, 0, 0, "PAY-UNKNOWN-GROUP")
	seedTccGroup(t, db, "ORD-TCC-UNKNOWN-GROUP", "G1", 1, 0)
	seedTccLine(t, db, "ORD-TCC-UNKNOWN-GROUP", 1, "G1", 10000)
	if _, err := db.Exec(`UPDATE order_fulfillment_groups SET status = '' WHERE order_no = 'ORD-TCC-UNKNOWN-GROUP'`); err != nil {
		t.Fatal(err)
	}
	version := tccVersion(t, db, "ORD-TCC-UNKNOWN-GROUP")

	_, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-TCC-UNKNOWN-GROUP", PaymentNo: "PAY-UNKNOWN-GROUP", PaidCent: 10000,
	})
	if err == nil || !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("unknown group status = %v, want FAILED_PRECONDITION", err)
	}
	assertNoTccStateWrites(t, db, "ORD-TCC-UNKNOWN-GROUP", version)
	for _, query := range []string{
		`SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-UNKNOWN-GROUP'`,
		`SELECT COUNT(*) FROM order_shipping_permits WHERE order_no = 'ORD-TCC-UNKNOWN-GROUP'`,
	} {
		if got := countRows(t, db, query); got != 0 {
			t.Fatalf("unknown group status wrote rows (%s -> %d)", query, got)
		}
	}
}

// Review g1: a mixed group (known=0 legacy + known=1) must still validate the
// known=1 line sums. known=1 lines are not skipped just because the group also
// carries unattributed legacy quota.
func TestTccCancelMixedCompositionValidatesKnownLines(t *testing.T) {
	seedMixed := func(t *testing.T) (*sql.DB, *Service) {
		t.Helper()
		db := orderTestDB(t)
		svc := tccTestService(t, db, &fakePaymentPort{captured: 10000, refunded: 300})
		ctx := context.Background()
		seedTccOrder(t, db, "ORD-TCC-MIXED", 10000, 10000, 0, 300, "PAY-MIXED")
		seedTccGroup(t, db, "ORD-TCC-MIXED", "G1", 1, 0)
		seedTccLine(t, db, "ORD-TCC-MIXED", 1, "G1", 10000)
		seedTccPermit(t, db, "ORD-TCC-MIXED", "G1", permitConsumed, "SHIP-MIXED")
		seedMigratedTccCommitment(t, db, "ORD-TCC-MIXED", "G1", "R-LEGACY-MIXED", 100)
		seedKnownTccCommitment(t, db, "ORD-TCC-MIXED", "G1", "R-KNOWN-MIXED", 200, 200, 0)
		_ = ctx
		return db, svc
	}

	t.Run("known lines mismatch fails closed", func(t *testing.T) {
		db, svc := seedMixed(t)
		version := tccVersion(t, db, "ORD-TCC-MIXED")
		_, err := svc.TccCancelMarkPaid(context.Background(), &pb.TccPaymentRequest{
			OrderNo: "ORD-TCC-MIXED", PaymentNo: "PAY-MIXED", PaidCent: 10000,
		})
		if err == nil || !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
			t.Fatalf("mixed known line mismatch = %v, want FAILED_PRECONDITION", err)
		}
		var refundState string
		var versionAfter int64
		if err := db.QueryRow(`SELECT refund_state, version FROM orders WHERE order_no = 'ORD-TCC-MIXED'`).
			Scan(&refundState, &versionAfter); err != nil {
			t.Fatal(err)
		}
		if refundState != refundStateNone || versionAfter != version {
			t.Fatalf("mixed mismatch mutated order: refund_state=%s version=%d", refundState, versionAfter)
		}
		for _, query := range []string{
			`SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-MIXED'`,
			`SELECT COUNT(*) FROM refund_applications WHERE order_no = 'ORD-TCC-MIXED'`,
			`SELECT COUNT(*) FROM order_refund_admissions WHERE order_no = 'ORD-TCC-MIXED'`,
		} {
			if got := countRows(t, db, query); got != 0 {
				t.Fatalf("mixed mismatch wrote rows (%s -> %d)", query, got)
			}
		}
		var permitState string
		if err := db.QueryRow(`SELECT state FROM order_shipping_permits
			WHERE order_no = 'ORD-TCC-MIXED' AND fulfillment_group_no = 'G1'`).Scan(&permitState); err != nil {
			t.Fatal(err)
		}
		if permitState != permitConsumed {
			t.Fatalf("mixed mismatch changed permit to %s", permitState)
		}
		if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitments WHERE order_no = 'ORD-TCC-MIXED'`); got != 2 {
			t.Fatalf("mixed mismatch changed existing commitments: %d", got)
		}
	})

	t.Run("valid known lines allow whole group", func(t *testing.T) {
		db, svc := seedMixed(t)
		if _, err := db.Exec(`INSERT INTO order_refund_commitment_lines
			(refund_id, line_no, quantity, goods_cent, freight_cent)
			VALUES ('R-KNOWN-MIXED', 1, 1, 200, 0)`); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.TccCancelMarkPaid(context.Background(), &pb.TccPaymentRequest{
			OrderNo: "ORD-TCC-MIXED", PaymentNo: "PAY-MIXED", PaidCent: 10000,
		}); err != nil {
			t.Fatalf("valid mixed composition = %v", err)
		}
		// Cap: 10000 + 0 - (100 legacy + 200 known) = 9700; new TCC commitment
		// stays known=0 without lines.
		var amount int64
		var known int
		if err := db.QueryRow(`SELECT amount_cent, composition_known FROM order_refund_commitments
			WHERE refund_id = 'tcc:PAY-MIXED:G1'`).Scan(&amount, &known); err != nil {
			t.Fatal(err)
		}
		if amount != 9700 || known != 0 {
			t.Fatalf("mixed TCC commitment = %d/known %d, want 9700/0", amount, known)
		}
		if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitment_lines
			WHERE refund_id = 'tcc:PAY-MIXED:G1'`); got != 0 {
			t.Fatalf("mixed TCC commitment wrote lines: %d", got)
		}
		// The known=1 row's line is untouched.
		if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitment_lines
			WHERE refund_id = 'R-KNOWN-MIXED'`); got != 1 {
			t.Fatalf("known=1 line rows = %d, want 1", got)
		}
	})
}

// Review g2: Σ == remaining == 0 (already refunded) must return before the
// Fulfillment ListShipments dependency, but only after the Payment snapshot and
// Σ equation were validated.
func TestTccCancelNoActiveReturnsBeforeFulfillment(t *testing.T) {
	t.Run("fully refunded succeeds while Fulfillment is down", func(t *testing.T) {
		db := orderTestDB(t)
		payments := &countingPaymentPort{captured: 10000, refunded: 10000}
		fulfillment := &fakeFulfillmentPort{status: "CANCELLED", listErr: errors.New("fulfillment timeout")}
		svc := tccTestServiceWithFulfillment(t, db, payments, fulfillment)
		ctx := context.Background()
		seedTccOrder(t, db, "ORD-TCC-G2", 10000, 10000, 0, 10000, "PAY-G2")
		seedTccGroup(t, db, "ORD-TCC-G2", "G1", 1, 0)
		seedTccLine(t, db, "ORD-TCC-G2", 1, "G1", 10000)
		if _, err := db.Exec(`INSERT INTO order_refund_commitments
			(order_no, group_no, refund_id, amount_cent, goods_cent, freight_cent, composition_known, composition_source, state)
			VALUES ('ORD-TCC-G2', 'G1', 'R-G2', 10000, 10000, 0, 1, 'aftersale', 'SETTLED')`); err != nil {
			t.Fatal(err)
		}
		version := tccVersion(t, db, "ORD-TCC-G2")

		if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
			OrderNo: "ORD-TCC-G2", PaymentNo: "PAY-G2", PaidCent: 10000,
		}); err != nil {
			t.Fatalf("no-active compensation failed on Fulfillment outage: %v", err)
		}
		if fulfillment.listCalls.Load() != 0 {
			t.Fatalf("no-active compensation consulted ListShipments %d times", fulfillment.listCalls.Load())
		}
		if payments.getCalls < 1 {
			t.Fatal("no-active compensation skipped the Payment snapshot")
		}
		if got := tccVersion(t, db, "ORD-TCC-G2"); got != version {
			t.Fatalf("no-active compensation wrote order state")
		}
		if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-G2'`); got != 0 {
			t.Fatalf("no-active compensation wrote batches: %d", got)
		}
	})

	t.Run("Σ mismatch still audits before Fulfillment", func(t *testing.T) {
		db := orderTestDB(t)
		payments := &countingPaymentPort{captured: 10000}
		fulfillment := &fakeFulfillmentPort{status: "CANCELLED", listErr: errors.New("fulfillment timeout")}
		svc := tccTestServiceWithFulfillment(t, db, payments, fulfillment)
		ctx := context.Background()
		seedTccOrder(t, db, "ORD-TCC-G2BAD", 10000, 10000, 0, 0, "PAY-G2BAD")
		seedTccGroup(t, db, "ORD-TCC-G2BAD", "G1", 1, 0)
		seedTccLine(t, db, "ORD-TCC-G2BAD", 1, "G1", 9000)
		version := tccVersion(t, db, "ORD-TCC-G2BAD")

		_, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
			OrderNo: "ORD-TCC-G2BAD", PaymentNo: "PAY-G2BAD", PaidCent: 10000,
		})
		if err == nil || !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
			t.Fatalf("Σ mismatch = %v, want FAILED_PRECONDITION", err)
		}
		if fulfillment.listCalls.Load() != 0 {
			t.Fatalf("Σ mismatch consulted ListShipments %d times", fulfillment.listCalls.Load())
		}
		if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-TCC-G2BAD' AND state = ?`, refundBatchFailedPre); got != 1 {
			t.Fatalf("Σ mismatch audit batches = %d, want 1", got)
		}
		assertNoTccStateWrites(t, db, "ORD-TCC-G2BAD", version)
	})
}

// Review g3: the read-only TccQueryMarkPaid probe mirrors the B1 lifecycle gate.
func TestTccQueryMarkPaidMatchesLifecycleGate(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 1000})
	ctx := context.Background()
	for _, tc := range []struct {
		status string
		want   bool
	}{
		{statusPendingPayment, true},
		{statusPaid, true},
		{statusAllocated, true},
		{statusShipped, true},
		{statusCompleted, true},
		{statusCancelled, false},
		{statusClosed, false},
	} {
		orderNo := "ORD-TCC-QUERY-" + tc.status
		seedTccOrder(t, db, orderNo, 1000, 1000, 0, 0, "PAY-"+tc.status)
		setTccOrderStatus(t, db, orderNo, tc.status)
		reply, err := svc.TccQueryMarkPaid(ctx, &pb.TccPaymentRequest{OrderNo: orderNo, PaymentNo: "PAY-" + tc.status})
		if err != nil {
			t.Fatalf("%s query: %v", tc.status, err)
		}
		if reply.GetPrepared() != tc.want {
			t.Fatalf("%s Prepared = %v, want %v", tc.status, reply.GetPrepared(), tc.want)
		}
	}
}
