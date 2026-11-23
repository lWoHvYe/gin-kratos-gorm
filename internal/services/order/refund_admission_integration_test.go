package order

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"

	"google.golang.org/grpc"
)

// fakePaymentPort is the order package's narrow Payment stand-in. It records
// calls so tests can prove a path never consulted Payment.
type fakePaymentPort struct {
	captured, refunded int64
	intents            []*pb.Intent
	getErr             error
	listErr            error
	getCalls           int
	listCalls          int
}

func (f *fakePaymentPort) GetIntent(context.Context, *pb.GetIntentRequest, ...grpc.CallOption) (*pb.Intent, error) {
	f.getCalls++
	if f.getErr != nil {
		return nil, f.getErr
	}
	return &pb.Intent{PaymentNo: "PAY-1", CapturedCent: f.captured, RefundedCent: f.refunded}, nil
}

func (f *fakePaymentPort) ListOrderIntents(context.Context, *pb.ListOrderIntentsRequest, ...grpc.CallOption) (*pb.IntentList, error) {
	f.listCalls++
	if f.listErr != nil {
		return nil, f.listErr
	}
	return &pb.IntentList{Intents: f.intents}, nil
}

type fakeFulfillmentPort struct {
	status    string
	shipments []*pb.Shipment
	listErr   error
	// listCalls is atomic because concurrent TCC cancel tests call the port
	// from several goroutines under -race.
	listCalls atomic.Int64
}

func (f *fakeFulfillmentPort) CancelIfCreated(context.Context, *pb.CancelIfCreatedRequest, ...grpc.CallOption) (*pb.CancelIfCreatedReply, error) {
	return &pb.CancelIfCreatedReply{Status: f.status}, nil
}

func (f *fakeFulfillmentPort) ListShipments(context.Context, *pb.ListShipmentsRequest, ...grpc.CallOption) (*pb.ShipmentList, error) {
	f.listCalls.Add(1)
	if f.listErr != nil {
		return nil, f.listErr
	}
	return &pb.ShipmentList{Total: int64(len(f.shipments)), Shipments: f.shipments}, nil
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
	if _, err := db.Exec(`INSERT INTO order_items
		(order_no,line_no,sku_id,name,quantity,warehouse_id,fulfillment_group_no,line_paid_cent,unit_paid_cent)
		VALUES ('ORD-ADM',1,1,'x',1,1,'G1',10000,10000)`); err != nil {
		t.Fatal(err)
	}
	reply, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-ADM-1", OrderNo: "ORD-ADM", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
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
		RefundId: "R-ADM-1", OrderNo: "ORD-ADM", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
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
	if _, err := db.Exec(`INSERT INTO order_items
		(order_no,line_no,sku_id,name,quantity,warehouse_id,fulfillment_group_no,line_paid_cent,unit_paid_cent)
		VALUES ('ORD-ADM2',1,1,'x',1,1,'G1',10000,10000)`); err != nil {
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
	_, _ = db.Exec(`INSERT INTO order_items
		(order_no,line_no,sku_id,name,quantity,warehouse_id,fulfillment_group_no,line_paid_cent,unit_paid_cent)
		VALUES ('ORD-ADM3',1,1,'x',1,1,'G1',10000,10000)`)
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

// TestAdmitRefundZeroAmountOrder pins the P-03 regression and AC-40: a
// `zero:<order_no>` order is admitted with amount 0, revokes the unshipped
// permit, and never consults Payment (no intent exists for a zero order).
func TestAdmitRefundZeroAmountOrder(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{}
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(payments, &fakeFulfillmentPort{status: "CANCELLED"})
	ctx := context.Background()
	if _, err := db.Exec(`INSERT INTO orders
		(order_no,user_id,status,total_cent,pay_cent,paid_cent,settlement_payment_no,expire_at,applied_promotions_json)
		VALUES ('ORD-ZADM',1,'PAID',100,0,0,'zero:ORD-ZADM',NOW(3),'[]')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO order_items
		(order_no,line_no,sku_id,name,quantity,warehouse_id,fulfillment_group_no,line_paid_cent,unit_paid_cent)
		VALUES ('ORD-ZADM',1,1,'x',1,1,'G1',0,0)`); err != nil {
		t.Fatal(err)
	}
	reply, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-ZERO", OrderNo: "ORD-ZADM", GroupNo: "G1", AmountCent: 0, Source: "aftersale",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetResult() != refundResultPre {
		t.Fatalf("zero refund result=%q, want PRE", reply.GetResult())
	}
	if payments.getCalls != 0 || payments.listCalls != 0 {
		t.Fatalf("zero refund consulted Payment (get=%d list=%d)", payments.getCalls, payments.listCalls)
	}
	var permitState string
	if err := db.QueryRow(`SELECT state FROM order_shipping_permits WHERE order_no='ORD-ZADM'`).Scan(&permitState); err != nil {
		t.Fatal(err)
	}
	if permitState != permitRevoked {
		t.Fatalf("permit=%q, want REVOKED", permitState)
	}
	var admitted, committed int64
	if err := db.QueryRow(`SELECT amount_cent FROM order_refund_admissions WHERE refund_id='R-ZERO'`).Scan(&admitted); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT amount_cent FROM order_refund_commitments WHERE refund_id='R-ZERO'`).Scan(&committed); err != nil {
		t.Fatal(err)
	}
	if admitted != 0 || committed != 0 {
		t.Fatalf("zero refund amounts: admission=%d commitment=%d, want 0/0", admitted, committed)
	}
	// A zero order has no captured quota: any positive admission is rejected
	// before a second admission row is written.
	if _, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-ZERO-OVER", OrderNo: "ORD-ZADM", GroupNo: "G1", AmountCent: 1,
	}); err == nil {
		t.Fatal("positive refund admitted on a zero-amount order")
	} else if !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("error code=%v, want FAILED_PRECONDITION: %v", apperr.From(err).Code, err)
	}
	var overRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM order_refund_admissions WHERE refund_id='R-ZERO-OVER'`).Scan(&overRows); err != nil {
		t.Fatal(err)
	}
	if overRows != 0 {
		t.Fatalf("over-quota admission rows=%d, want 0", overRows)
	}
	if payments.getCalls != 0 || payments.listCalls != 0 {
		t.Fatalf("zero refund consulted Payment (get=%d list=%d)", payments.getCalls, payments.listCalls)
	}
}
