package payment

import (
	"context"
	"database/sql"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

func refundFactCounts(t *testing.T, db *sql.DB) (refunds, events, batches int) {
	t.Helper()
	if err := db.QueryRow(`SELECT COUNT(*) FROM refunds`).Scan(&refunds); err != nil {
		t.Fatalf("count refunds: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM outbox_events WHERE event_type = 'payment.refunded'`).Scan(&events); err != nil {
		t.Fatalf("count refund events: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM payment_refund_batches`).Scan(&batches); err != nil {
		t.Fatalf("count refund batches: %v", err)
	}
	return refunds, events, batches
}

// TestGetRefundByKeyReturnsPersistedFacts covers both refund writers: a row
// created by the atomic RefundBatch and a row created by the single Refund RPC.
// The lookup must return the stored refund_id (idempotency_key), not refund_no,
// and must not write anything.
func TestGetRefundByKeyReturnsPersistedFacts(t *testing.T) {
	db := paymentTestDB(t)
	seedPaymentIntent(t, db, "PAY-LOOKUP", "ORD-LOOKUP", statusSucceeded, 600, 600, 0)
	seedPaymentIntent(t, db, "PAY-LOOKUP-PLAIN", "ORD-LOOKUP-PLAIN", statusSucceeded, 200, 200, 0)
	svc := New(db, nil, "", nil)
	ctx := context.Background()

	const batchReason = "tcc refund shared reason"
	batchReply, err := svc.RefundBatch(ctx, &pb.RefundBatchRequest{
		PaymentNo: "PAY-LOOKUP", BatchId: "BATCH-LOOKUP", ExpectedTotalCent: 600,
		Items: []*pb.RefundBatchItem{
			{RefundId: "LOOKUP-1", AmountCent: 300},
			{RefundId: "LOOKUP-2", AmountCent: 300},
		},
		Reason: batchReason,
	})
	if err != nil {
		t.Fatalf("RefundBatch: %v", err)
	}
	if _, err := svc.Refund(ctx, &pb.RefundRequest{
		PaymentNo: "PAY-LOOKUP-PLAIN", AmountCent: 200, Reason: "plain lookup",
		IdempotencyKey: "LOOKUP-PLAIN",
	}); err != nil {
		t.Fatalf("plain Refund: %v", err)
	}

	refundsBefore, eventsBefore, batchesBefore := refundFactCounts(t, db)

	for _, refundID := range []string{"LOOKUP-1", "LOOKUP-2"} {
		item := batchItemResult(t, batchReply, refundID)
		got, err := svc.GetRefundByKey(ctx, &pb.GetRefundByKeyRequest{
			PaymentNo: "PAY-LOOKUP", RefundId: refundID,
		})
		if err != nil {
			t.Fatalf("GetRefundByKey(%s): %v", refundID, err)
		}
		if got.GetRefundId() != refundID {
			t.Fatalf("refund_id = %q, want %q (idempotency key, not refund_no)", got.GetRefundId(), refundID)
		}
		if got.GetRefundNo() != item.GetRefundNo() {
			t.Fatalf("refund_no = %q, want %q", got.GetRefundNo(), item.GetRefundNo())
		}
		if got.GetPaymentNo() != "PAY-LOOKUP" || got.GetOrderNo() != "ORD-LOOKUP" {
			t.Fatalf("payment/order = (%q,%q)", got.GetPaymentNo(), got.GetOrderNo())
		}
		if got.GetAmountCent() != 300 || got.GetReason() != batchReason || got.GetStatus() != statusSucceeded {
			t.Fatalf("refund = %+v", got)
		}
		if got.GetCreatedAtUnix() <= 0 {
			t.Fatalf("created_at_unix = %d, want > 0", got.GetCreatedAtUnix())
		}
	}

	plain, err := svc.GetRefundByKey(ctx, &pb.GetRefundByKeyRequest{
		PaymentNo: "PAY-LOOKUP-PLAIN", RefundId: "LOOKUP-PLAIN",
	})
	if err != nil {
		t.Fatalf("GetRefundByKey(plain): %v", err)
	}
	if plain.GetRefundId() != "LOOKUP-PLAIN" || plain.GetAmountCent() != 200 ||
		plain.GetOrderNo() != "ORD-LOOKUP-PLAIN" || plain.GetPaymentNo() != "PAY-LOOKUP-PLAIN" ||
		plain.GetReason() != "plain lookup" || plain.GetStatus() != statusSucceeded ||
		plain.GetCreatedAtUnix() <= 0 {
		t.Fatalf("plain refund = %+v", plain)
	}

	// The lookup is read-only: no refund, event or batch row was added.
	if refunds, events, batches := refundFactCounts(t, db); refunds != refundsBefore || events != eventsBefore || batches != batchesBefore {
		t.Fatalf("counts changed by lookup: before=(%d,%d,%d) after=(%d,%d,%d)",
			refundsBefore, eventsBefore, batchesBefore, refunds, events, batches)
	}
}

func TestGetRefundByKeyNotFound(t *testing.T) {
	db := paymentTestDB(t)
	seedPaymentIntent(t, db, "PAY-LOOKUP2", "ORD-LOOKUP2", statusSucceeded, 300, 300, 0)
	svc := New(db, nil, "", nil)
	ctx := context.Background()
	reply, err := svc.RefundBatch(ctx, &pb.RefundBatchRequest{
		PaymentNo: "PAY-LOOKUP2", BatchId: "BATCH-LOOKUP2", ExpectedTotalCent: 300,
		Items:  []*pb.RefundBatchItem{{RefundId: "LOOKUP2-1", AmountCent: 300}},
		Reason: "lookup not found",
	})
	if err != nil {
		t.Fatalf("RefundBatch: %v", err)
	}
	refundNo := batchItemResult(t, reply, "LOOKUP2-1").GetRefundNo()
	refundsBefore, eventsBefore, batchesBefore := refundFactCounts(t, db)

	cases := []struct {
		name string
		req  *pb.GetRefundByKeyRequest
	}{
		{name: "missing refund_id", req: &pb.GetRefundByKeyRequest{PaymentNo: "PAY-LOOKUP2", RefundId: "MISSING"}},
		{name: "wrong payment", req: &pb.GetRefundByKeyRequest{PaymentNo: "PAY-OTHER", RefundId: "LOOKUP2-1"}},
		{name: "refund_no is not the key", req: &pb.GetRefundByKeyRequest{PaymentNo: "PAY-LOOKUP2", RefundId: refundNo}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.GetRefundByKey(ctx, tc.req); !apperr.IsCode(err, apperr.CodeNotFound) {
				t.Fatalf("GetRefundByKey(%+v) error = %v, want NOT_FOUND", tc.req, err)
			}
		})
	}
	if refunds, events, batches := refundFactCounts(t, db); refunds != refundsBefore || events != eventsBefore || batches != batchesBefore {
		t.Fatalf("counts changed by not-found lookups: before=(%d,%d,%d) after=(%d,%d,%d)",
			refundsBefore, eventsBefore, batchesBefore, refunds, events, batches)
	}
}
