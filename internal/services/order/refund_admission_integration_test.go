package order

import (
	"context"
	"io"
	"log/slog"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"

	"google.golang.org/grpc"
)

type fakePaymentPort struct{ captured, refunded int64 }

func (f *fakePaymentPort) GetIntent(context.Context, *pb.GetIntentRequest, ...grpc.CallOption) (*pb.Intent, error) {
	return &pb.Intent{PaymentNo: "PAY-1", CapturedCent: f.captured, RefundedCent: f.refunded}, nil
}

type fakeFulfillmentPort struct{ status string }

func (f *fakeFulfillmentPort) CancelIfCreated(context.Context, *pb.CancelIfCreatedRequest, ...grpc.CallOption) (*pb.CancelIfCreatedReply, error) {
	return &pb.CancelIfCreatedReply{Status: f.status}, nil
}

func TestAdmitRefundPreShipAndIdempotent(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	ctx := context.Background()
	if _, err := db.Exec(`INSERT INTO orders
		(order_no,user_id,status,total_cent,discount_cent,promotion_discount_cent,applied_promotions_json,
		 freight_cent,pay_cent,paid_cent,settlement_payment_no,refunded_cent,refund_status,refund_state,
		 receiver_name,receiver_phone,receiver_address,quote_id,client_request_id,
		 allocation_frozen_at,allocation_hash,package_count,warehouse_count,expire_at,created_at)
		VALUES ('ORD-ADM',1,'PAID',10000,0,0,'[]',0,10000,10000,'PAY-1',0,'','NONE',
		 'A','1','R','','',NOW(3),'h',1,1,NOW(3),NOW(3))`); err != nil {
		t.Fatal(err)
	}
	reply, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-ADM-1", OrderNo: "ORD-ADM", GroupNo: "G1", AmountCent: 1000, Source: "aftersale",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetResult() != refundResultPre {
		t.Fatalf("result=%q, want PRE", reply.GetResult())
	}
	var permitState string
	if err := db.QueryRow(`SELECT state FROM order_shipping_permits WHERE order_no='ORD-ADM'`).Scan(&permitState); err != nil {
		t.Fatal(err)
	}
	if permitState != permitRevoked {
		t.Fatalf("permit=%q, want REVOKED", permitState)
	}
	again, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-ADM-1", OrderNo: "ORD-ADM", GroupNo: "G1", AmountCent: 1000, Source: "aftersale",
	})
	if err != nil || again.GetResult() != refundResultPre {
		t.Fatalf("idempotent retry = (%v, %v)", again, err)
	}
}

func TestAdmitRefundConsumedPermitTwoPhase(t *testing.T) {
	db := orderTestDB(t)
	fake := &fakeFulfillmentPort{status: "CANCELLED"}
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, fake)
	ctx := context.Background()
	if _, err := db.Exec(`INSERT INTO orders
		(order_no,user_id,status,total_cent,discount_cent,promotion_discount_cent,applied_promotions_json,
		 freight_cent,pay_cent,paid_cent,settlement_payment_no,refunded_cent,refund_status,refund_state,
		 receiver_name,receiver_phone,receiver_address,quote_id,client_request_id,
		 allocation_frozen_at,allocation_hash,package_count,warehouse_count,expire_at,created_at)
		VALUES ('ORD-ADM2',1,'PAID',10000,0,0,'[]',0,10000,10000,'PAY-1',0,'','NONE',
		 'A','1','R','','',NOW(3),'h',1,1,NOW(3),NOW(3))`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO order_shipping_permits (order_no, fulfillment_group_no, state)
		VALUES ('ORD-ADM2','G1','CONSUMED')`); err != nil {
		t.Fatal(err)
	}
	reply, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-ADM-2", OrderNo: "ORD-ADM2", GroupNo: "G1", AmountCent: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetResult() != refundResultPre {
		t.Fatalf("cancelled delivery result=%q, want PRE", reply.GetResult())
	}
	if _, err := db.Exec(`INSERT INTO order_shipping_permits (order_no, fulfillment_group_no, state)
		VALUES ('ORD-ADM3','G1','CONSUMED')`); err != nil {
		t.Fatal(err)
	}
	_, _ = db.Exec(`INSERT INTO orders (order_no,user_id,status,total_cent,pay_cent,paid_cent,settlement_payment_no,expire_at,applied_promotions_json)
		VALUES ('ORD-ADM3',1,'PAID',10000,10000,10000,'PAY-1',NOW(3),'[]')`)
	fake.status = "ALREADY_SHIPPED"
	reply, err = svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-ADM-3", OrderNo: "ORD-ADM3", GroupNo: "G1", AmountCent: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetResult() != refundResultPost {
		t.Fatalf("shipped result=%q, want POST", reply.GetResult())
	}
}
