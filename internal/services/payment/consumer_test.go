package payment

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

// T1.6 evidence (ADR-0007 Amd1 T1.6 Addendum §3.3/§3.4): order.refund_requested
// executes only the payment-level cancel command, uses the fixed refund_id as
// the Payment idempotency key, and never refunds group/TCC events.

type fakeRefundCommand struct {
	calls    []*pb.RefundRequest
	err      error
	intent   *pb.Intent
	getErr   error
	getCalls int
}

func (f *fakeRefundCommand) Refund(_ context.Context, in *pb.RefundRequest) (*pb.Empty, error) {
	f.calls = append(f.calls, in)
	if f.err != nil {
		return nil, f.err
	}
	return &pb.Empty{}, nil
}

func (f *fakeRefundCommand) GetIntent(_ context.Context, in *pb.GetIntentRequest) (*pb.Intent, error) {
	f.getCalls++
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.intent == nil {
		return nil, apperr.NotFound("payment intent not found")
	}
	return &pb.Intent{
		PaymentNo: in.GetPaymentNo(), AmountCent: f.intent.GetAmountCent(),
		CapturedCent: f.intent.GetCapturedCent(), RefundedCent: f.intent.GetRefundedCent(),
	}, nil
}

func t16Event(t *testing.T, event orderRefundRequested) []byte {
	t.Helper()
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func t16Logger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestHandleOrderRefundRequestedExecutesPaymentCancelCommand(t *testing.T) {
	fake := &fakeRefundCommand{}
	event := orderRefundRequested{
		OrderNo: "ORD-T16", PaymentNo: "PAY-T16", PaidCent: 1000,
		RefundID: "pay:PAY-T16:cancel", Source: paymentCancelSource,
		NonEffectiveCapture: true, AmountCent: 800,
		Reason: "pending captured cancel", Status: "CANCELLED",
	}
	if err := handleOrderRefundRequested(context.Background(), fake, t16Logger(), t16Event(t, event)); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("Refund calls = %d, want 1", len(fake.calls))
	}
	req := fake.calls[0]
	if req.GetIdempotencyKey() != "pay:PAY-T16:cancel" {
		t.Fatalf("idempotency_key = %q, want the payment-level refund_id", req.GetIdempotencyKey())
	}
	if !req.GetNonEffectiveCapture() {
		t.Fatal("payment-level cancel must set non_effective_capture")
	}
	if req.GetAmountCent() != 800 {
		t.Fatalf("amount = %d, want the event's authoritative captured amount 800", req.GetAmountCent())
	}
	if req.GetPaymentNo() != "PAY-T16" || req.GetReason() != "pending captured cancel" {
		t.Fatalf("refund request = %+v", req)
	}
}

func TestHandleOrderRefundRequestedLegacyCancelAfterConfirm(t *testing.T) {
	fake := &fakeRefundCommand{intent: &pb.Intent{CapturedCent: 500, AmountCent: 500, RefundedCent: 0}}
	event := orderRefundRequested{
		OrderNo: "ORD-T16-LEGACY", PaymentNo: "PAY-T16-LEGACY", PaidCent: 500,
		Reason: "order cancelled after inventory confirm", Status: "CANCELLED",
	}
	if err := handleOrderRefundRequested(context.Background(), fake, t16Logger(), t16Event(t, event)); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("legacy Refund calls = %d, want 1", len(fake.calls))
	}
	if got := fake.calls[0].GetIdempotencyKey(); got != "pay:PAY-T16-LEGACY:cancel" {
		t.Fatalf("legacy idempotency_key = %q, want the deterministic cancel identity", got)
	}
	if got := fake.calls[0].GetAmountCent(); got != 500 {
		t.Fatalf("legacy amount = %d, want authoritative net 500", got)
	}
	if fake.getCalls != 1 {
		t.Fatalf("legacy GetIntent calls = %d, want 1", fake.getCalls)
	}
}

func TestHandleOrderRefundRequestedLegacyUsesNetNotPaidCent(t *testing.T) {
	// paid_cent is the full capture; 200 was already refunded. The command must
	// refund the remaining 300, not the full 500.
	fake := &fakeRefundCommand{intent: &pb.Intent{CapturedCent: 500, AmountCent: 500, RefundedCent: 200}}
	event := orderRefundRequested{
		OrderNo: "ORD-T16-LEGACY-NET", PaymentNo: "PAY-T16-LEGACY-NET", PaidCent: 500,
		Reason: "order cancelled after inventory confirm", Status: "CANCELLED",
	}
	if err := handleOrderRefundRequested(context.Background(), fake, t16Logger(), t16Event(t, event)); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("legacy net Refund calls = %d, want 1", len(fake.calls))
	}
	if got := fake.calls[0].GetAmountCent(); got != 300 {
		t.Fatalf("legacy amount = %d, want captured-refunded net 300", got)
	}
}

func TestHandleOrderRefundRequestedLegacyAlreadyRefundedAcks(t *testing.T) {
	fake := &fakeRefundCommand{intent: &pb.Intent{CapturedCent: 500, AmountCent: 500, RefundedCent: 500}}
	event := orderRefundRequested{
		OrderNo: "ORD-T16-LEGACY-FULL", PaymentNo: "PAY-T16-LEGACY-FULL", PaidCent: 500,
		Reason: "order cancelled after inventory confirm", Status: "CANCELLED",
	}
	if err := handleOrderRefundRequested(context.Background(), fake, t16Logger(), t16Event(t, event)); err != nil {
		t.Fatalf("already-refunded legacy command must be acked: %v", err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("already-refunded legacy called Refund %d times", len(fake.calls))
	}
}

func TestHandleOrderRefundRequestedLegacyNetUnavailableRetries(t *testing.T) {
	fake := &fakeRefundCommand{getErr: apperr.Unavailable("payment intent timeout", nil)}
	event := orderRefundRequested{
		OrderNo: "ORD-T16-LEGACY-RETRY", PaymentNo: "PAY-T16-LEGACY-RETRY", PaidCent: 500,
		Reason: "order cancelled after inventory confirm", Status: "CANCELLED",
	}
	err := handleOrderRefundRequested(context.Background(), fake, t16Logger(), t16Event(t, event))
	if err == nil || !apperr.IsCode(err, apperr.CodeUnavailable) {
		t.Fatalf("legacy net unavailable = %v, want UNAVAILABLE retry", err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("legacy net unavailable called Refund %d times", len(fake.calls))
	}
}

func TestHandleOrderRefundRequestedLegacyNetNotFoundDoesNotFallbackPaidCent(t *testing.T) {
	fake := &fakeRefundCommand{getErr: apperr.NotFound("payment intent not found")}
	event := orderRefundRequested{
		OrderNo: "ORD-T16-LEGACY-MISS", PaymentNo: "PAY-T16-LEGACY-MISS", PaidCent: 500,
		Reason: "order cancelled after inventory confirm", Status: "CANCELLED",
	}
	if err := handleOrderRefundRequested(context.Background(), fake, t16Logger(), t16Event(t, event)); err != nil {
		t.Fatalf("unresolvable legacy net must be observed, not retried: %v", err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("unresolvable legacy net fell back to paid_cent: %d Refund calls", len(fake.calls))
	}
}

func TestHandleOrderRefundRequestedObservesForeignCommands(t *testing.T) {
	cases := []orderRefundRequested{
		{OrderNo: "O1", PaymentNo: "P1", PaidCent: 100, RefundID: "tcc:P1:G1", Source: "tcc"},
		{OrderNo: "O2", PaymentNo: "P2", PaidCent: 100, RefundID: "pay:P2:cancel", Source: "tcc"},
		{OrderNo: "O3", PaymentNo: "P3", PaidCent: 100, Reason: "tcc refund batch"},
		{OrderNo: "O4", PaymentNo: "P4", PaidCent: 100},
	}
	for _, event := range cases {
		fake := &fakeRefundCommand{}
		if err := handleOrderRefundRequested(context.Background(), fake, t16Logger(), t16Event(t, event)); err != nil {
			t.Fatalf("%s: observe-only returned error: %v", event.OrderNo, err)
		}
		if len(fake.calls) != 0 {
			t.Fatalf("%s: observe-only called Payment.Refund %d times", event.OrderNo, len(fake.calls))
		}
	}
}

func TestHandleOrderRefundRequestedRetriesTransientFailure(t *testing.T) {
	fake := &fakeRefundCommand{err: apperr.Unavailable("payment channel timeout", nil)}
	event := orderRefundRequested{
		OrderNo: "ORD-T16-TIMEOUT", PaymentNo: "PAY-T16-TIMEOUT", PaidCent: 100,
		RefundID: "pay:PAY-T16-TIMEOUT:cancel", Source: paymentCancelSource,
		NonEffectiveCapture: true, AmountCent: 100,
	}
	err := handleOrderRefundRequested(context.Background(), fake, t16Logger(), t16Event(t, event))
	if err == nil {
		t.Fatal("Payment timeout was acked instead of retried")
	}
	if !apperr.IsCode(err, apperr.CodeUnavailable) {
		t.Fatalf("returned error = %v, want UNAVAILABLE retry", err)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("Refund calls = %d, want 1", len(fake.calls))
	}
}

func TestHandleOrderRefundRequestedAcksRejectedRefund(t *testing.T) {
	fake := &fakeRefundCommand{err: apperr.Invalid("refund amount 100 exceeds remaining refundable amount 0")}
	event := orderRefundRequested{
		OrderNo: "ORD-T16-REJECT", PaymentNo: "PAY-T16-REJECT", PaidCent: 100,
		RefundID: "pay:PAY-T16-REJECT:cancel", Source: paymentCancelSource,
		NonEffectiveCapture: true, AmountCent: 100,
	}
	if err := handleOrderRefundRequested(context.Background(), fake, t16Logger(), t16Event(t, event)); err != nil {
		t.Fatalf("deterministic rejection must be acked, got %v", err)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("Refund calls = %d, want 1", len(fake.calls))
	}
}

func TestHandleOrderRefundRequestedNoCapturedDoesNotRefund(t *testing.T) {
	fake := &fakeRefundCommand{}
	event := orderRefundRequested{
		OrderNo: "ORD-T16-ZERO", PaymentNo: "PAY-T16-ZERO",
		RefundID: "pay:PAY-T16-ZERO:cancel", Source: paymentCancelSource,
	}
	if err := handleOrderRefundRequested(context.Background(), fake, t16Logger(), t16Event(t, event)); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("zero-capture command called Payment.Refund %d times", len(fake.calls))
	}
}

func TestOrderRefundRequestedCommandIsIdempotent(t *testing.T) {
	db := paymentTestDB(t)
	seedPaymentIntent(t, db, "PAY-T16-IDEM", "ORD-T16-IDEM", statusSucceeded, 1000, 1000, 0)
	svc := New(db, nil, "", t16Logger())
	event := orderRefundRequested{
		OrderNo: "ORD-T16-IDEM", PaymentNo: "PAY-T16-IDEM", PaidCent: 700,
		RefundID: "pay:PAY-T16-IDEM:cancel", Source: paymentCancelSource,
		NonEffectiveCapture: true, AmountCent: 700,
		Reason: "pending captured cancel", Status: "CANCELLED",
	}
	data := t16Event(t, event)
	for i := 0; i < 2; i++ {
		if err := handleOrderRefundRequested(context.Background(), svc, t16Logger(), data); err != nil {
			t.Fatalf("delivery %d: %v", i, err)
		}
	}
	refunds, events, _ := refundFactCounts(t, db)
	if refunds != 1 || events != 1 {
		t.Fatalf("refunds/events = %d/%d, want 1/1 (idempotent by refund_id)", refunds, events)
	}
	var key string
	var amount int64
	if err := db.QueryRow(`SELECT idempotency_key, amount_cent FROM refunds WHERE payment_no = 'PAY-T16-IDEM'`).
		Scan(&key, &amount); err != nil {
		t.Fatal(err)
	}
	if key != "pay:PAY-T16-IDEM:cancel" || amount != 700 {
		t.Fatalf("stored refund = %q/%d", key, amount)
	}
	var raw []byte
	if err := db.QueryRow(`SELECT payload FROM outbox_events WHERE event_type = 'payment.refunded' AND aggregate_id = 'PAY-T16-IDEM'`).
		Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["refund_id"] != "pay:PAY-T16-IDEM:cancel" {
		t.Fatalf("payment.refunded refund_id = %v", payload["refund_id"])
	}
	if payload["non_effective_capture"] != true {
		t.Fatalf("payment.refunded non_effective_capture = %v", payload["non_effective_capture"])
	}
	if !strings.Contains(payload["reason"].(string), "pending captured cancel") {
		t.Fatalf("payment.refunded reason = %v", payload["reason"])
	}
}

func TestOrderRefundRequestedAlreadyRefundedIsAcked(t *testing.T) {
	db := paymentTestDB(t)
	seedPaymentIntent(t, db, "PAY-T16-ALREADY", "ORD-T16-ALREADY", statusSucceeded, 1000, 1000, 1000)
	svc := New(db, nil, "", t16Logger())
	event := orderRefundRequested{
		OrderNo: "ORD-T16-ALREADY", PaymentNo: "PAY-T16-ALREADY", PaidCent: 1000,
		RefundID: "pay:PAY-T16-ALREADY:cancel", Source: paymentCancelSource,
		NonEffectiveCapture: true, AmountCent: 1000,
	}
	if err := handleOrderRefundRequested(context.Background(), svc, t16Logger(), t16Event(t, event)); err != nil {
		t.Fatalf("already-refunded command must be acked, got %v", err)
	}
	refunds, events, _ := refundFactCounts(t, db)
	if refunds != 0 || events != 0 {
		t.Fatalf("already-refunded wrote refund/event: %d/%d", refunds, events)
	}
}
