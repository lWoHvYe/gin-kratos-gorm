package order

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"gin-kratos-gorm/internal/platform/events"
)

func TestPaymentRefundedSettlesCommitment(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	ctx := context.Background()
	if _, err := db.Exec(`INSERT INTO orders
		(order_no,user_id,status,total_cent,pay_cent,paid_cent,settlement_payment_no,expire_at,applied_promotions_json)
		VALUES ('ORD-SETTLE',1,'PAID',10000,10000,10000,'PAY-1',NOW(3),'[]')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO order_refund_commitments
		(order_no,group_no,refund_id,amount_cent,state) VALUES ('ORD-SETTLE','G1','R-SETTLE',1000,'RESERVED')`); err != nil {
		t.Fatal(err)
	}
	err := applyPaymentRefunded(ctx, svc, events.PaymentRefunded{
		PaymentNo: "PAY-1", OrderNo: "ORD-SETTLE", RefundID: "R-SETTLE",
		AmountCent: 1000, RefundedCent: 1000, OrderRefundedCent: 1000, CapturedCent: 10000,
	})
	if err != nil {
		t.Fatal(err)
	}
	var state string
	if err := db.QueryRow(`SELECT state FROM order_refund_commitments WHERE refund_id='R-SETTLE'`).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "SETTLED" {
		t.Fatalf("state=%q, want SETTLED", state)
	}
}
