package order

import (
	"context"
	"io"
	"log/slog"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
)

func TestAC45TccConfirmRejectsSecondPayment(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	ctx := context.Background()
	if _, err := db.Exec(`INSERT INTO orders
		(order_no,user_id,status,total_cent,discount_cent,promotion_discount_cent,applied_promotions_json,
		 freight_cent,pay_cent,paid_cent,settlement_payment_no,refunded_cent,refund_status,refund_state,
		 receiver_name,receiver_phone,receiver_address,quote_id,client_request_id,
		 allocation_frozen_at,allocation_hash,package_count,warehouse_count,expire_at,created_at)
		VALUES ('ORD-DUP',1,'PENDING_PAYMENT',10000,0,0,'[]',0,10000,0,'',0,'','NONE',
		 'A','1','R','','',NULL,'',1,1,NOW(3),NOW(3))`); err != nil {
		t.Fatal(err)
	}
	first := &pb.TccPaymentRequest{OrderNo: "ORD-DUP", PaymentNo: "PAY-A", PaidCent: 10000}
	if _, err := svc.TccConfirmMarkPaid(ctx, first); err != nil {
		t.Fatalf("first confirm: %v", err)
	}
	second := &pb.TccPaymentRequest{OrderNo: "ORD-DUP", PaymentNo: "PAY-B", PaidCent: 10000}
	if _, err := svc.TccConfirmMarkPaid(ctx, second); err == nil {
		t.Fatal("second payment must be rejected, not treated as idempotent")
	}
	// The same payment identity is still idempotent.
	if _, err := svc.TccConfirmMarkPaid(ctx, first); err != nil {
		t.Fatalf("same payment retry: %v", err)
	}
}
