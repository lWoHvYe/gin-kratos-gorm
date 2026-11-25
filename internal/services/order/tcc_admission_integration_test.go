package order

import (
	"context"
	"database/sql"
	"sync"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

// T1.4a evidence (SPEC-0002 v1.9 AC-51/AC-65, ADR-0007 Amendment 1 §4.1.2
// steps 4-5). Every test runs against real MySQL with the real C-05 migrations
// because the guarantees are transactional (batch CAS, application CAS, one
// admission per refund_id).
//
// Since T1.4b1 the same TccCancelMarkPaid call continues into the payment
// phase, so a successful path ends with the batch PAYING and one idempotent
// Payment.Refund per admitted group (see tcc_payment_integration_test.go for
// the payment-specific evidence). Admission-only failures still make zero
// payment calls.

func loadTccApplication(t *testing.T, ctx context.Context, db *sql.DB, refundID string) (state, result string, amount, goods, freight int64) {
	t.Helper()
	if err := db.QueryRowContext(ctx, `SELECT state, admission_result, amount_cent, goods_cent, freight_cent
		FROM refund_applications WHERE refund_id = ?`, refundID).
		Scan(&state, &result, &amount, &goods, &freight); err != nil {
		t.Fatal(err)
	}
	return state, result, amount, goods, freight
}

// T1.4a: every active NONE-permit group is admitted PRE, with one durable
// admission, one RESERVED commitment and the permit revoked once.
func TestTccAdmissionNonePermitResolvesPre(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 10000})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-ADM-PRE", 10000, 10000, 500, 0, "PAY-ADM-PRE")
	seedTccGroup(t, db, "ORD-ADM-PRE", "G1", 1, 300)
	seedTccGroup(t, db, "ORD-ADM-PRE", "G2", 2, 200)
	seedTccLine(t, db, "ORD-ADM-PRE", 1, "G1", 5500)
	seedTccLine(t, db, "ORD-ADM-PRE", 2, "G2", 4000)

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-ADM-PRE", PaymentNo: "PAY-ADM-PRE", PaidCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	batch := loadTccBatchRow(t, db, "ORD-ADM-PRE", "PAY-ADM-PRE")
	if batch.State != refundBatchPaying {
		t.Fatalf("batch state = %s, want PAYING after T1.4b1 payment was accepted", batch.State)
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
		refundID := tccGroupRefundID("PAY-ADM-PRE", want.group)
		state, result, amount, goods, freight := loadTccApplication(t, ctx, db, refundID)
		if state != refundApplicationProcessing || result != refundResultPre ||
			amount != want.amount || goods != want.goods || freight != want.freight {
			t.Fatalf("%s application = %s/%s/%d/%d/%d, want PROCESSING/PRE/%d/%d/%d",
				want.group, state, result, amount, goods, freight, want.amount, want.goods, want.freight)
		}
		var admissionResult, admissionState string
		var admissionAmount int64
		if err := db.QueryRow(`SELECT result, state, amount_cent FROM order_refund_admissions
			WHERE refund_id = ?`, refundID).Scan(&admissionResult, &admissionState, &admissionAmount); err != nil {
			t.Fatal(err)
		}
		if admissionResult != refundResultPre || admissionState != refundResultPre || admissionAmount != want.amount {
			t.Fatalf("%s admission = %s/%s/%d", want.group, admissionResult, admissionState, admissionAmount)
		}
		var commitmentState string
		var commitmentAmount int64
		if err := db.QueryRow(`SELECT state, amount_cent FROM order_refund_commitments
			WHERE refund_id = ?`, refundID).Scan(&commitmentState, &commitmentAmount); err != nil {
			t.Fatal(err)
		}
		if commitmentState != commitmentReserved || commitmentAmount != want.amount {
			t.Fatalf("%s commitment = %s/%d", want.group, commitmentState, commitmentAmount)
		}
		var permitState string
		if err := db.QueryRow(`SELECT state FROM order_shipping_permits
			WHERE order_no = 'ORD-ADM-PRE' AND fulfillment_group_no = ?`, want.group).Scan(&permitState); err != nil {
			t.Fatal(err)
		}
		if permitState != permitRevoked {
			t.Fatalf("%s permit = %s, want REVOKED", want.group, permitState)
		}
	}
	var status, refundState string
	if err := db.QueryRow(`SELECT status, refund_state FROM orders WHERE order_no = 'ORD-ADM-PRE'`).
		Scan(&status, &refundState); err != nil {
		t.Fatal(err)
	}
	if status != statusPaid || refundState != refundStateRefunding {
		t.Fatalf("order = %s/%s, want PAID/REFUNDING", status, refundState)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events
		WHERE aggregate_id = 'ORD-ADM-PRE' AND event_type = 'order.refund_requested'`); got != 0 {
		t.Fatalf("T1.4a sent order.refund_requested: %d", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events
		WHERE aggregate_id = 'ORD-ADM-PRE' AND event_type = 'order.refund_admitted'`); got != 2 {
		t.Fatalf("refund_admitted events = %d, want 2", got)
	}

	// Idempotent repeat: no new batch/application/admission/commitment.
	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-ADM-PRE", PaymentNo: "PAY-ADM-PRE", PaidCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-ADM-PRE'`,
		`SELECT COUNT(*) FROM order_refund_admissions WHERE order_no = 'ORD-ADM-PRE'`,
		`SELECT COUNT(*) FROM order_refund_commitments WHERE order_no = 'ORD-ADM-PRE'`,
		`SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-ADM-PRE' AND event_type = 'order.refund_admitted'`,
	} {
		want := 2
		if query == `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-ADM-PRE'` {
			want = 1
		}
		if got := countRows(t, db, query); got != want {
			t.Fatalf("repeat changed rows (%s -> %d, want %d)", query, got, want)
		}
	}
	// T1.4b1-R: the repeat sees both successful Payment refunds (the fake does
	// not run the event consumer) and lazily reconciles the ledger, so the
	// applications are DONE instead of staying PROCESSING forever.
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_applications
		WHERE order_no = 'ORD-ADM-PRE' AND state = ?`, refundApplicationDone); got != 2 {
		t.Fatalf("applications after repeat = %d, want 2 DONE after lazy reconcile", got)
	}
}

// T1.4a: a CONSUMED permit is resolved through CancelIfCreated; CANCELLED is
// PRE, the permit itself stays CONSUMED and the commitment reserved by Tx1 is
// not duplicated.
func TestTccAdmissionConsumedCreatedCancelIfCreatedPre(t *testing.T) {
	db := orderTestDB(t)
	fulfillment := &fakeFulfillmentPort{status: "CANCELLED"}
	svc := tccTestServiceWithFulfillment(t, db, &fakePaymentPort{captured: 10000}, fulfillment)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-ADM-CC", 10000, 10000, 0, 0, "PAY-ADM-CC")
	seedTccGroup(t, db, "ORD-ADM-CC", "G1", 1, 0)
	seedTccLine(t, db, "ORD-ADM-CC", 1, "G1", 10000)
	seedTccPermit(t, db, "ORD-ADM-CC", "G1", permitConsumed, "SHIP-ADM-CC")

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-ADM-CC", PaymentNo: "PAY-ADM-CC", PaidCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	if got := fulfillment.cancelCalls.Load(); got != 1 {
		t.Fatalf("CancelIfCreated calls = %d, want 1", got)
	}
	refundID := tccGroupRefundID("PAY-ADM-CC", "G1")
	state, result, _, _, _ := loadTccApplication(t, ctx, db, refundID)
	if state != refundApplicationProcessing || result != refundResultPre {
		t.Fatalf("application = %s/%s, want PROCESSING/PRE", state, result)
	}
	var admissionResult, admissionState string
	if err := db.QueryRow(`SELECT result, state FROM order_refund_admissions WHERE refund_id = ?`, refundID).
		Scan(&admissionResult, &admissionState); err != nil {
		t.Fatal(err)
	}
	if admissionResult != refundResultPre || admissionState != refundResultPre {
		t.Fatalf("admission = %s/%s, want PRE/PRE", admissionResult, admissionState)
	}
	var permitState string
	var permitVersion int64
	if err := db.QueryRow(`SELECT state, version FROM order_shipping_permits
		WHERE order_no = 'ORD-ADM-CC' AND fulfillment_group_no = 'G1'`).Scan(&permitState, &permitVersion); err != nil {
		t.Fatal(err)
	}
	if permitState != permitConsumed || permitVersion != 1 {
		t.Fatalf("CONSUMED permit = %s/version %d, want untouched", permitState, permitVersion)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitments WHERE refund_id = ?`, refundID); got != 1 {
		t.Fatalf("commitments = %d, want 1", got)
	}
	batch := loadTccBatchRow(t, db, "ORD-ADM-CC", "PAY-ADM-CC")
	if batch.State != refundBatchPaying {
		t.Fatalf("batch state = %s, want PAYING after T1.4b1 payment was accepted", batch.State)
	}
	// A repeat never calls CancelIfCreated again and never duplicates rows.
	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-ADM-CC", PaymentNo: "PAY-ADM-CC", PaidCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	if got := fulfillment.cancelCalls.Load(); got != 1 {
		t.Fatalf("repeat CancelIfCreated calls = %d, want 1", got)
	}
}

// T1.4a: an already shipped group is POST, never PRE, and its CONSUMED permit
// is not touched. Two ways the fact is observed:
//   - T1.2 already saw the shipment SHIPPED (application POST; no admission);
//   - the shipment left between Tx1 and admission (PENDING_REMOTE answered
//     ALREADY_SHIPPED by CancelIfCreated).
func TestTccAdmissionConsumedShippedPost(t *testing.T) {
	t.Run("shipped in Tx1 facts", func(t *testing.T) {
		db := orderTestDB(t)
		fulfillment := &fakeFulfillmentPort{status: "CANCELLED", shipments: []*pb.Shipment{{
			ShipmentNo: "SHIP-ADM-SHIP", OrderNo: "ORD-ADM-SHIP", FulfillmentGroupNo: "G1",
			Status: "SHIPPED",
		}}}
		svc := tccTestServiceWithFulfillment(t, db, &fakePaymentPort{captured: 10000}, fulfillment)
		ctx := context.Background()
		seedTccOrder(t, db, "ORD-ADM-SHIP", 10000, 10000, 0, 0, "PAY-ADM-SHIP")
		seedTccGroup(t, db, "ORD-ADM-SHIP", "G1", 1, 0)
		seedTccLine(t, db, "ORD-ADM-SHIP", 1, "G1", 10000)
		seedTccPermit(t, db, "ORD-ADM-SHIP", "G1", permitConsumed, "SHIP-ADM-SHIP")

		if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
			OrderNo: "ORD-ADM-SHIP", PaymentNo: "PAY-ADM-SHIP", PaidCent: 10000,
		}); err != nil {
			t.Fatal(err)
		}
		refundID := tccGroupRefundID("PAY-ADM-SHIP", "G1")
		state, result, amount, _, _ := loadTccApplication(t, ctx, db, refundID)
		if state != refundApplicationProcessing || result != refundResultPost || amount != 10000 {
			t.Fatalf("application = %s/%s/%d, want PROCESSING/POST/10000", state, result, amount)
		}
		var admissionResult string
		if err := db.QueryRow(`SELECT result FROM order_refund_admissions WHERE refund_id = ?`, refundID).
			Scan(&admissionResult); err != nil {
			t.Fatal(err)
		}
		if admissionResult != refundResultPost {
			t.Fatalf("admission = %s, want POST", admissionResult)
		}
		var permitState string
		var permitVersion int64
		if err := db.QueryRow(`SELECT state, version FROM order_shipping_permits
			WHERE order_no = 'ORD-ADM-SHIP' AND fulfillment_group_no = 'G1'`).Scan(&permitState, &permitVersion); err != nil {
			t.Fatal(err)
		}
		if permitState != permitConsumed || permitVersion != 1 {
			t.Fatalf("shipped permit = %s/version %d, want untouched CONSUMED", permitState, permitVersion)
		}
		if got := fulfillment.cancelCalls.Load(); got != 0 {
			t.Fatalf("shipped group called CancelIfCreated %d times", got)
		}
		if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitments WHERE refund_id = ?`, refundID); got != 1 {
			t.Fatalf("shipped group commitments = %d, want 1", got)
		}
	})

	t.Run("leaves between Tx1 and admission", func(t *testing.T) {
		db := orderTestDB(t)
		// T1.2 sees a CREATED shipment; admission then gets ALREADY_SHIPPED.
		fulfillment := &fakeFulfillmentPort{status: "ALREADY_SHIPPED", shipments: []*pb.Shipment{{
			ShipmentNo: "SHIP-ADM-RACE", OrderNo: "ORD-ADM-RACE", FulfillmentGroupNo: "G1",
			Status: "CREATED",
		}}}
		svc := tccTestServiceWithFulfillment(t, db, &fakePaymentPort{captured: 10000}, fulfillment)
		ctx := context.Background()
		seedTccOrder(t, db, "ORD-ADM-RACE", 10000, 10000, 0, 0, "PAY-ADM-RACE")
		seedTccGroup(t, db, "ORD-ADM-RACE", "G1", 1, 0)
		seedTccLine(t, db, "ORD-ADM-RACE", 1, "G1", 10000)
		seedTccPermit(t, db, "ORD-ADM-RACE", "G1", permitConsumed, "SHIP-ADM-RACE")

		if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
			OrderNo: "ORD-ADM-RACE", PaymentNo: "PAY-ADM-RACE", PaidCent: 10000,
		}); err != nil {
			t.Fatal(err)
		}
		if got := fulfillment.cancelCalls.Load(); got != 1 {
			t.Fatalf("CancelIfCreated calls = %d, want 1", got)
		}
		refundID := tccGroupRefundID("PAY-ADM-RACE", "G1")
		state, result, _, _, _ := loadTccApplication(t, ctx, db, refundID)
		if state != refundApplicationProcessing || result != refundResultPost {
			t.Fatalf("application = %s/%s, want PROCESSING/POST (never PRE after shipment left)", state, result)
		}
		var admissionResult string
		if err := db.QueryRow(`SELECT result FROM order_refund_admissions WHERE refund_id = ?`, refundID).
			Scan(&admissionResult); err != nil {
			t.Fatal(err)
		}
		if admissionResult != refundResultPost {
			t.Fatalf("admission = %s, want POST", admissionResult)
		}
		var permitState string
		if err := db.QueryRow(`SELECT state FROM order_shipping_permits
			WHERE order_no = 'ORD-ADM-RACE' AND fulfillment_group_no = 'G1'`).Scan(&permitState); err != nil {
			t.Fatal(err)
		}
		if permitState != permitConsumed {
			t.Fatalf("permit = %s, want untouched CONSUMED", permitState)
		}
	})
}

// T1.4a: a failed CancelIfCreated keeps the group PENDING_REMOTE and the batch
// ADMITTING (retryable); the rerun fills only the unfinished group and never
// duplicates the commitment.
func TestTccAdmissionCancelIfCreatedFailureKeepsPendingRemote(t *testing.T) {
	db := orderTestDB(t)
	fulfillment := &fakeFulfillmentPort{status: "CANCELLED"}
	fulfillment.setFailCancels(1)
	svc := tccTestServiceWithFulfillment(t, db, &fakePaymentPort{captured: 10000}, fulfillment)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-ADM-RETRY", 10000, 10000, 0, 0, "PAY-ADM-RETRY")
	seedTccGroup(t, db, "ORD-ADM-RETRY", "G1", 1, 0)
	seedTccLine(t, db, "ORD-ADM-RETRY", 1, "G1", 10000)
	seedTccPermit(t, db, "ORD-ADM-RETRY", "G1", permitConsumed, "SHIP-ADM-RETRY")

	_, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-ADM-RETRY", PaymentNo: "PAY-ADM-RETRY", PaidCent: 10000,
	})
	if err == nil || !apperr.IsCode(err, apperr.CodeUnavailable) {
		t.Fatalf("failed CancelIfCreated = %v, want UNAVAILABLE", err)
	}
	refundID := tccGroupRefundID("PAY-ADM-RETRY", "G1")
	state, result, _, _, _ := loadTccApplication(t, ctx, db, refundID)
	if state != refundApplicationPending || result != refundPending {
		t.Fatalf("after failure application = %s/%s, want PENDING/PENDING_REMOTE", state, result)
	}
	var admissionState string
	if err := db.QueryRow(`SELECT state FROM order_refund_admissions WHERE refund_id = ?`, refundID).
		Scan(&admissionState); err != nil {
		t.Fatal(err)
	}
	if admissionState != refundPending {
		t.Fatalf("after failure admission = %s, want PENDING_REMOTE", admissionState)
	}
	batch := loadTccBatchRow(t, db, "ORD-ADM-RETRY", "PAY-ADM-RETRY")
	if batch.State != refundBatchAdmitting {
		t.Fatalf("after failure batch = %s, want ADMITTING", batch.State)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitments WHERE refund_id = ?`, refundID); got != 1 {
		t.Fatalf("after failure commitments = %d, want 1", got)
	}

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-ADM-RETRY", PaymentNo: "PAY-ADM-RETRY", PaidCent: 10000,
	}); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if got := fulfillment.cancelCalls.Load(); got != 2 {
		t.Fatalf("CancelIfCreated calls = %d, want 2 (one failure + one retry)", got)
	}
	state, result, _, _, _ = loadTccApplication(t, ctx, db, refundID)
	if state != refundApplicationProcessing || result != refundResultPre {
		t.Fatalf("after retry application = %s/%s, want PROCESSING/PRE", state, result)
	}
	batch = loadTccBatchRow(t, db, "ORD-ADM-RETRY", "PAY-ADM-RETRY")
	if batch.State != refundBatchPaying {
		t.Fatalf("after retry batch = %s, want PAYING after T1.4b1 payment was accepted", batch.State)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_admissions WHERE refund_id = ?`, refundID); got != 1 {
		t.Fatalf("retry duplicated the admission: %d", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitments WHERE refund_id = ?`, refundID); got != 1 {
		t.Fatalf("retry duplicated the commitment: %d", got)
	}
}

// T1.4a: concurrent TCC retries admit each group once. Run with -race.
func TestTccAdmissionConcurrentIsIdempotent(t *testing.T) {
	db := orderTestDB(t)
	fulfillment := &fakeFulfillmentPort{status: "CANCELLED"}
	svc := tccTestServiceWithFulfillment(t, db, &countingPaymentPort{captured: 10000}, fulfillment)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-ADM-RACE", 10000, 10000, 200, 0, "PAY-ADM-RACE")
	seedTccGroup(t, db, "ORD-ADM-RACE", "G1", 1, 100)
	seedTccGroup(t, db, "ORD-ADM-RACE", "G2", 2, 100)
	seedTccLine(t, db, "ORD-ADM-RACE", 1, "G1", 4900)
	seedTccLine(t, db, "ORD-ADM-RACE", 2, "G2", 4900)

	const workers = 8
	errs := make([]error, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			_, errs[i] = svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
				OrderNo: "ORD-ADM-RACE", PaymentNo: "PAY-ADM-RACE", PaidCent: 10000,
			})
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("worker %d: %v", i, err)
		}
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = 'ORD-ADM-RACE'`); got != 1 {
		t.Fatalf("batches = %d, want 1", got)
	}
	batch := loadTccBatchRow(t, db, "ORD-ADM-RACE", "PAY-ADM-RACE")
	if batch.State != refundBatchPaying && batch.State != refundBatchDone {
		t.Fatalf("batch state = %s, want PAYING or DONE after T1.4b1 payment", batch.State)
	}
	// A final pass deterministically converges the fake's successful Payment
	// facts (no event consumer runs in this test) and closes the batch.
	if err := svc.payTccBatch(ctx, "ORD-ADM-RACE", "PAY-ADM-RACE"); err != nil {
		t.Fatalf("converge after concurrent admission: %v", err)
	}
	batch = loadTccBatchRow(t, db, "ORD-ADM-RACE", "PAY-ADM-RACE")
	if batch.State != refundBatchDone {
		t.Fatalf("batch state after convergence = %s, want DONE", batch.State)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_applications
		WHERE batch_id = ? AND state = ?`, batch.BatchID, refundApplicationDone); got != 2 {
		t.Fatalf("admitted applications = %d, want 2 DONE", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_admissions WHERE order_no = 'ORD-ADM-RACE'`); got != 2 {
		t.Fatalf("admissions = %d, want 2", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitments WHERE order_no = 'ORD-ADM-RACE'`); got != 2 {
		t.Fatalf("commitments = %d, want 2", got)
	}
	for _, groupNo := range []string{"G1", "G2"} {
		var state string
		var version int64
		if err := db.QueryRow(`SELECT state, version FROM order_shipping_permits
			WHERE order_no = 'ORD-ADM-RACE' AND fulfillment_group_no = ?`, groupNo).Scan(&state, &version); err != nil {
			t.Fatal(err)
		}
		if state != permitRevoked || version != 2 {
			t.Fatalf("%s permit = %s/version %d, want REVOKED/version 2", groupNo, state, version)
		}
	}
}

// T1.4a: a historical shipped order without a permit and without a fulfillment
// group must still be admitted POST without creating a synthetic permit or
// cancelling anything.
func TestTccAdmissionLegacyG1ShippedPost(t *testing.T) {
	db := orderTestDB(t)
	fulfillment := &fakeFulfillmentPort{status: "CANCELLED"}
	svc := tccTestServiceWithFulfillment(t, db, &fakePaymentPort{captured: 10000}, fulfillment)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-ADM-LEGACY", 10000, 10000, 0, 0, "PAY-ADM-LEGACY")
	seedTccLine(t, db, "ORD-ADM-LEGACY", 1, "G1", 10000)
	setTccOrderStatus(t, db, "ORD-ADM-LEGACY", statusShipped)

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: "ORD-ADM-LEGACY", PaymentNo: "PAY-ADM-LEGACY", PaidCent: 10000,
	}); err != nil {
		t.Fatal(err)
	}
	refundID := tccGroupRefundID("PAY-ADM-LEGACY", "G1")
	state, result, amount, _, _ := loadTccApplication(t, ctx, db, refundID)
	if state != refundApplicationProcessing || result != refundResultPost || amount != 10000 {
		t.Fatalf("legacy application = %s/%s/%d, want PROCESSING/POST/10000", state, result, amount)
	}
	var admissionResult string
	if err := db.QueryRow(`SELECT result FROM order_refund_admissions WHERE refund_id = ?`, refundID).
		Scan(&admissionResult); err != nil {
		t.Fatal(err)
	}
	if admissionResult != refundResultPost {
		t.Fatalf("legacy admission = %s, want POST", admissionResult)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_shipping_permits WHERE order_no = 'ORD-ADM-LEGACY'`); got != 0 {
		t.Fatalf("legacy shipped group created a synthetic permit: %d", got)
	}
	if got := fulfillment.cancelCalls.Load(); got != 0 {
		t.Fatalf("legacy shipped group called CancelIfCreated %d times", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitments WHERE refund_id = ?`, refundID); got != 1 {
		t.Fatalf("legacy shipped group commitments = %d, want 1", got)
	}
}
