package order

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"

	"google.golang.org/grpc"
)

func legacyTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db := orderTestDB(t)
	for _, table := range []string{"legacy_payment_resolution", "order_refund_sequences"} {
		if _, err := db.Exec("DROP TABLE IF EXISTS " + table); err != nil {
			t.Fatal(err)
		}
	}
	applyOrderTestMigration(t, db, "0019_legacy_payment_resolution.sql")
	return db
}

func legacyService(db *sql.DB) *Service {
	return &Service{db: db, log: slog.New(slog.NewTextHandler(io.Discard, nil)), now: time.Now}
}

func legacyIntent(paymentNo, orderNo, status string, captured, refunded int64) *pb.Intent {
	return &pb.Intent{PaymentNo: paymentNo, OrderNo: orderNo, Status: status, CapturedCent: captured, RefundedCent: refunded}
}

func TestLegacyResolutionUniqueCASRefundsOtherCaptures(t *testing.T) {
	db := legacyTestDB(t)
	payments := &fakePaymentPort{intents: []*pb.Intent{
		legacyIntent("PAY-LR-A", "ORD-LR-U", "SUCCEEDED", 10000, 0),
		legacyIntent("PAY-LR-B", "ORD-LR-U", "SUCCEEDED", 5000, 0),
		legacyIntent("PAY-LR-X", "ORD-OTHER", "SUCCEEDED", 10000, 0),
		legacyIntent("PAY-LR-C", "ORD-LR-U", "PENDING", 0, 0),
	}}
	svc := legacyService(db)
	svc.payments = payments
	seedTccOrder(t, db, "ORD-LR-U", 10000, 10000, 0, 0, "")

	res, err := svc.ensureLegacyResolution(context.Background(), "ORD-LR-U")
	if err != nil {
		t.Fatal(err)
	}
	if res.State != legacyResolutionStateResolved || res.PaymentNo != "PAY-LR-A" {
		t.Fatalf("resolution = %+v", res)
	}
	if got := orderSettlementNo(t, db, "ORD-LR-U"); got != "PAY-LR-A" {
		t.Fatalf("settlement_payment_no = %q", got)
	}
	if len(payments.refundCalls) != 1 || payments.refundCalls[0].GetPaymentNo() != "PAY-LR-B" ||
		payments.refundCalls[0].GetAmountCent() != 5000 || !payments.refundCalls[0].GetNonEffectiveCapture() ||
		payments.refundCalls[0].GetIdempotencyKey() != "pay:PAY-LR-B:dup" {
		t.Fatalf("duplicate refund calls = %+v", payments.refundCalls)
	}
	assertRefundApp(t, db, "pay:PAY-LR-B:dup", "DONE", 5000)
	if got := countRows(t, db, `SELECT COUNT(*) FROM legacy_payment_resolution WHERE order_no = 'ORD-LR-U'`); got != 0 {
		t.Fatalf("unique resolution queued %d rows", got)
	}
	// Repeated parse is a no-op: already resolved, no second remote refund.
	if _, err := svc.ensureLegacyResolution(context.Background(), "ORD-LR-U"); err != nil {
		t.Fatal(err)
	}
	if len(payments.refundCalls) != 1 {
		t.Fatalf("repeat resolution refunded again: %d calls", len(payments.refundCalls))
	}
}

func TestLegacyResolutionZeroNoMultiCandidatesQueue(t *testing.T) {
	db := legacyTestDB(t)
	svc := legacyService(db)
	ctx := context.Background()

	// Zero candidates (pay_cent > 0).
	payments := &fakePaymentPort{intents: []*pb.Intent{
		legacyIntent("PAY-LR-NONE", "ORD-LR-NONE", "SUCCEEDED", 0, 0),
		legacyIntent("PAY-LR-PENDING", "ORD-LR-NONE", "PENDING", 10000, 0),
	}}
	svc.payments = payments
	seedTccOrder(t, db, "ORD-LR-NONE", 10000, 10000, 0, 0, "")
	res, err := svc.ensureLegacyResolution(ctx, "ORD-LR-NONE")
	if err != nil {
		t.Fatal(err)
	}
	if res.State != legacyResolutionStateOpen || res.CandidateCount != 0 {
		t.Fatalf("zero-candidate resolution = %+v", res)
	}
	if got := orderSettlementNo(t, db, "ORD-LR-NONE"); got != "" {
		t.Fatalf("zero-candidate order wrote settlement %q", got)
	}
	assertQueueRow(t, db, "ORD-LR-NONE", 0, 10000)

	// Multiple candidates.
	payments2 := &fakePaymentPort{intents: []*pb.Intent{
		legacyIntent("PAY-LR-M1", "ORD-LR-MULTI", "SUCCEEDED", 10000, 0),
		legacyIntent("PAY-LR-M2", "ORD-LR-MULTI", "SUCCEEDED", 10000, 0),
	}}
	svc.payments = payments2
	seedTccOrder(t, db, "ORD-LR-MULTI", 10000, 10000, 0, 0, "")
	res, err = svc.ensureLegacyResolution(ctx, "ORD-LR-MULTI")
	if err != nil {
		t.Fatal(err)
	}
	if res.State != legacyResolutionStateOpen || res.CandidateCount != 2 {
		t.Fatalf("multi-candidate resolution = %+v", res)
	}
	if got := orderSettlementNo(t, db, "ORD-LR-MULTI"); got != "" {
		t.Fatalf("multi-candidate order wrote settlement %q", got)
	}
	assertQueueRow(t, db, "ORD-LR-MULTI", 2, 10000)
	if len(payments2.refundCalls) != 0 {
		t.Fatalf("ambiguous order refunded captures: %+v", payments2.refundCalls)
	}

	// Repeated ambiguous parse refreshes counts but does not reset expiry.
	var before time.Time
	if err := db.QueryRow(`SELECT expires_at FROM legacy_payment_resolution WHERE order_no = 'ORD-LR-MULTI'`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ensureLegacyResolution(ctx, "ORD-LR-MULTI"); err != nil {
		t.Fatal(err)
	}
	var after time.Time
	if err := db.QueryRow(`SELECT expires_at FROM legacy_payment_resolution WHERE order_no = 'ORD-LR-MULTI'`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if !after.Equal(before) {
		t.Fatalf("repeat parse reset expiry: %v -> %v", before, after)
	}
}

func TestLegacyResolutionZeroIdentityAndCapturedZeroConflict(t *testing.T) {
	db := legacyTestDB(t)
	svc := legacyService(db)
	ctx := context.Background()

	// pay_cent=0 with no captured intent -> zero:<order_no>, no queue.
	svc.payments = &fakePaymentPort{intents: []*pb.Intent{
		legacyIntent("PAY-LR-Z0", "ORD-LR-ZERO", "SUCCEEDED", 0, 0),
	}}
	seedTccOrder(t, db, "ORD-LR-ZERO", 0, 0, 0, 0, "")
	res, err := svc.ensureLegacyResolution(ctx, "ORD-LR-ZERO")
	if err != nil {
		t.Fatal(err)
	}
	if res.State != legacyResolutionStateExpiredZero || res.PaymentNo != "zero:ORD-LR-ZERO" {
		t.Fatalf("zero resolution = %+v", res)
	}
	if got := orderSettlementNo(t, db, "ORD-LR-ZERO"); got != "zero:ORD-LR-ZERO" {
		t.Fatalf("zero settlement = %q", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM legacy_payment_resolution WHERE order_no = 'ORD-LR-ZERO'`); got != 0 {
		t.Fatalf("zero resolution queued %d rows", got)
	}

	// pay_cent=0 but a captured intent exists -> never zero; queue instead.
	svc.payments = &fakePaymentPort{intents: []*pb.Intent{
		legacyIntent("PAY-LR-Z1", "ORD-LR-ZCAP", "SUCCEEDED", 500, 0),
	}}
	seedTccOrder(t, db, "ORD-LR-ZCAP", 0, 0, 0, 0, "")
	res, err = svc.ensureLegacyResolution(ctx, "ORD-LR-ZCAP")
	if err != nil {
		t.Fatal(err)
	}
	if res.State != legacyResolutionStateOpen {
		t.Fatalf("captured-zero resolution = %+v", res)
	}
	if got := orderSettlementNo(t, db, "ORD-LR-ZCAP"); got != "" {
		t.Fatalf("captured zero order resolved as %q", got)
	}
	assertQueueRow(t, db, "ORD-LR-ZCAP", 0, 0)
}

func TestLegacyResolutionConcurrentCASOneWriteOneRefund(t *testing.T) {
	db := legacyTestDB(t)
	payments := &safeLegacyPayment{intents: []*pb.Intent{
		legacyIntent("PAY-LR-C", "ORD-LR-CONC", "SUCCEEDED", 10000, 0),
		legacyIntent("PAY-LR-C2", "ORD-LR-CONC", "SUCCEEDED", 4000, 0),
	}}
	svc := legacyService(db)
	svc.payments = payments
	seedTccOrder(t, db, "ORD-LR-CONC", 10000, 10000, 0, 0, "")

	const callers = 8
	var wg sync.WaitGroup
	errs := make([]error, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, errs[index] = svc.ensureLegacyResolution(context.Background(), "ORD-LR-CONC")
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("caller %d: %v", i, err)
		}
	}
	if got := orderSettlementNo(t, db, "ORD-LR-CONC"); got != "PAY-LR-C" {
		t.Fatalf("settlement = %q", got)
	}
	if got := payments.refundCount(); got != 1 {
		t.Fatalf("duplicate refund calls = %d, want 1 CAS winner", got)
	}
	if got := payments.refundPayment(); got != "PAY-LR-C2" {
		t.Fatalf("refunded payment = %q", got)
	}
	assertRefundApp(t, db, "pay:PAY-LR-C2:dup", "DONE", 4000)
}

func TestLegacyProducerRejectsPAIDWithoutPaymentNo(t *testing.T) {
	db := legacyTestDB(t)
	svc := legacyService(db)
	ctx := context.Background()
	seedTccPendingOrder(t, db, "ORD-LR-PROD", 10000, 0, 0)
	if _, err := svc.MarkPaid(ctx, &pb.MarkPaidRequest{OrderNo: "ORD-LR-PROD", PaymentNo: "", PaidCent: 10000}); err == nil {
		t.Fatal("MarkPaid accepted an empty payment_no")
	}
	if got := orderStatus(t, db, "ORD-LR-PROD"); got != statusPendingPayment {
		t.Fatalf("MarkPaid empty payment_no changed status to %s", got)
	}
	if got := orderSettlementNo(t, db, "ORD-LR-PROD"); got != "" {
		t.Fatalf("MarkPaid empty payment_no wrote settlement %q", got)
	}
	if _, err := svc.TccConfirmMarkPaid(ctx, &pb.TccPaymentRequest{OrderNo: "ORD-LR-PROD", PaymentNo: "", PaidCent: 10000}); err == nil {
		t.Fatal("TccConfirmMarkPaid accepted an empty payment_no")
	}
	if got := orderStatus(t, db, "ORD-LR-PROD"); got != statusPendingPayment {
		t.Fatalf("TccConfirmMarkPaid empty payment_no changed status to %s", got)
	}
}

func TestLegacyResolutionListFailureFailsClosed(t *testing.T) {
	db := legacyTestDB(t)
	svc := legacyService(db)
	svc.payments = &fakePaymentPort{listErr: apperr.Unavailable("payment down", nil)}
	seedTccOrder(t, db, "ORD-LR-DOWN", 10000, 10000, 0, 0, "")
	if _, err := svc.ensureLegacyResolution(context.Background(), "ORD-LR-DOWN"); err == nil {
		t.Fatal("Payment list failure resolved the order")
	}
	if got := orderSettlementNo(t, db, "ORD-LR-DOWN"); got != "" {
		t.Fatalf("failure wrote settlement %q", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM legacy_payment_resolution WHERE order_no = 'ORD-LR-DOWN'`); got != 0 {
		t.Fatalf("failure queued %d rows", got)
	}
}

// safeLegacyPayment is a race-safe paymentPort for the concurrent CAS test.
type safeLegacyPayment struct {
	mu      sync.Mutex
	intents []*pb.Intent
	calls   []*pb.RefundRequest
}

func (f *safeLegacyPayment) GetIntent(context.Context, *pb.GetIntentRequest, ...grpc.CallOption) (*pb.Intent, error) {
	return &pb.Intent{PaymentNo: "PAY-LR-C", CapturedCent: 10000}, nil
}
func (f *safeLegacyPayment) ListOrderIntents(context.Context, *pb.ListOrderIntentsRequest, ...grpc.CallOption) (*pb.IntentList, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return &pb.IntentList{Intents: append([]*pb.Intent(nil), f.intents...)}, nil
}
func (f *safeLegacyPayment) GetRefundByKey(context.Context, *pb.GetRefundByKeyRequest, ...grpc.CallOption) (*pb.Refund, error) {
	return nil, apperr.NotFound("refund not found")
}
func (f *safeLegacyPayment) RefundBatch(context.Context, *pb.RefundBatchRequest, ...grpc.CallOption) (*pb.RefundBatchReply, error) {
	return nil, apperr.Failed("unexpected batch refund")
}
func (f *safeLegacyPayment) Refund(_ context.Context, in *pb.RefundRequest, _ ...grpc.CallOption) (*pb.Empty, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, in)
	return &pb.Empty{}, nil
}
func (f *safeLegacyPayment) refundCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}
func (f *safeLegacyPayment) refundPayment() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		return ""
	}
	return f.calls[0].GetPaymentNo()
}

func assertQueueRow(t *testing.T, db *sql.DB, orderNo string, candidates int, payCent int64) {
	t.Helper()
	var state string
	var gotCandidates int
	var gotPay int64
	var expiresIn float64
	if err := db.QueryRow(`SELECT state, candidate_count, pay_cent, TIMESTAMPDIFF(SECOND, entered_at, expires_at)
		FROM legacy_payment_resolution WHERE order_no = ?`, orderNo).Scan(&state, &gotCandidates, &gotPay, &expiresIn); err != nil {
		t.Fatal(err)
	}
	if state != legacyResolutionStateOpen || gotCandidates != candidates || gotPay != payCent {
		t.Fatalf("queue row = %s/%d/%d, want OPEN/%d/%d", state, gotCandidates, gotPay, candidates, payCent)
	}
	if expiresIn != 24*3600 {
		t.Fatalf("queue expiry = %v seconds, want 24h", expiresIn)
	}
}

func assertRefundApp(t *testing.T, db *sql.DB, refundID, state string, amount int64) {
	t.Helper()
	var gotState, source string
	var gotAmount int64
	var nonEffective bool
	if err := db.QueryRow(`SELECT state, source, amount_cent, non_effective_capture
		FROM refund_applications WHERE refund_id = ?`, refundID).
		Scan(&gotState, &source, &gotAmount, &nonEffective); err != nil {
		t.Fatal(err)
	}
	if gotState != state || source != "payment" || gotAmount != amount || !nonEffective {
		t.Fatalf("refund application = %s/%s/%d/%v", gotState, source, gotAmount, nonEffective)
	}
}

func orderSettlementNo(t *testing.T, db *sql.DB, orderNo string) string {
	t.Helper()
	var got string
	if err := db.QueryRow(`SELECT settlement_payment_no FROM orders WHERE order_no = ?`, orderNo).Scan(&got); err != nil {
		t.Fatal(err)
	}
	return got
}

func orderStatus(t *testing.T, db *sql.DB, orderNo string) string {
	t.Helper()
	var got string
	if err := db.QueryRow(`SELECT status FROM orders WHERE order_no = ?`, orderNo).Scan(&got); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestLegacyResolutionBlocksAdmissionAndShipping(t *testing.T) {
	db := legacyTestDB(t)
	payments := &fakePaymentPort{}
	svc := legacyService(db)
	svc.payments = payments
	seedTccOrder(t, db, "ORD-LR-GATE", 10000, 10000, 0, 0, "")
	ctx := context.Background()

	_, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-LR-GATE", OrderNo: "ORD-LR-GATE", GroupNo: "G1",
		AmountCent: 10000, Source: "aftersale",
	})
	if err == nil {
		t.Fatal("admission accepted an unresolved historical order")
	}
	if got := apperr.From(err).Details["reason"]; got != "LEGACY_PAYMENT_UNRESOLVED" {
		t.Fatalf("admission reason = %v", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_admissions WHERE refund_id = 'R-LR-GATE'`); got != 0 {
		t.Fatalf("blocked admission wrote %d rows", got)
	}
	assertQueueRow(t, db, "ORD-LR-GATE", 0, 10000)

	seedTccOrder(t, db, "ORD-LR-GATE2", 10000, 10000, 0, 0, "")
	if _, err := svc.AuthorizeShipment(ctx, &pb.ShipmentPermitRequest{
		OrderNo: "ORD-LR-GATE2", ShipmentNo: "SHIP-LR-GATE2",
	}); err == nil {
		t.Fatal("shipping accepted an unresolved historical order")
	} else if got := apperr.From(err).Details["reason"]; got != "LEGACY_PAYMENT_UNRESOLVED" {
		t.Fatalf("shipping reason = %v", got)
	}
	assertQueueRow(t, db, "ORD-LR-GATE2", 0, 10000)
}
