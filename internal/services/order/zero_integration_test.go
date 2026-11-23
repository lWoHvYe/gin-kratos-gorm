package order

import (
	"context"
	"io"
	"log/slog"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
)

func TestSettleZeroUsesSettlementIdentity(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	ctx := context.Background()
	if _, err := db.Exec(`INSERT INTO orders
		(order_no,user_id,status,total_cent,discount_cent,promotion_discount_cent,applied_promotions_json,
		 freight_cent,pay_cent,paid_cent,settlement_payment_no,refunded_cent,refund_status,refund_state,
		 receiver_name,receiver_phone,receiver_address,quote_id,client_request_id,
		 allocation_frozen_at,allocation_hash,package_count,warehouse_count,expire_at,created_at)
		VALUES ('ORD-ZERO',1,'PENDING_PAYMENT',100,100,0,'[]',0,0,0,'',0,'','NONE',
		 'Z','1','A','','',NULL,'',1,1,NOW(3),NOW(3))`); err != nil {
		t.Fatal(err)
	}
	out, err := svc.SettleZero(ctx, &pb.SettleZeroRequest{OrderNo: "ORD-ZERO"})
	if err != nil {
		t.Fatal(err)
	}
	if out.GetStatus() != statusPaid {
		t.Fatalf("status=%q, want PAID", out.GetStatus())
	}
	var paymentNo string
	if err := db.QueryRow(`SELECT settlement_payment_no FROM orders WHERE order_no='ORD-ZERO'`).Scan(&paymentNo); err != nil {
		t.Fatal(err)
	}
	if paymentNo != "zero:ORD-ZERO" {
		t.Fatalf("settlement_payment_no=%q", paymentNo)
	}
}
