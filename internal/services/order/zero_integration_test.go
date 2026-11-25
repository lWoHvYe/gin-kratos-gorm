package order

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

const zeroOrderInsert = `INSERT INTO orders
	(order_no,user_id,status,total_cent,discount_cent,promotion_discount_cent,applied_promotions_json,
	 freight_cent,pay_cent,paid_cent,settlement_payment_no,refunded_cent,refund_status,refund_state,
	 receiver_name,receiver_phone,receiver_address,quote_id,client_request_id,
	 allocation_frozen_at,allocation_hash,package_count,warehouse_count,expire_at,created_at)
	VALUES (?,1,'PENDING_PAYMENT',100,100,0,'[]',0,0,0,'',0,'','NONE',
	 'Z','1','A','','',NULL,'',1,1,NOW(3),NOW(3))`

func TestSettleZeroUsesSettlementIdentity(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{}
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(payments, nil)
	ctx := context.Background()
	if _, err := db.Exec(zeroOrderInsert, "ORD-ZERO"); err != nil {
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
	// AC-40: the zero decision is taken from Payment's capture facts, and no
	// payment intent is needed to settle a genuine zero order.
	if payments.listCalls != 1 {
		t.Fatalf("ListOrderIntents calls=%d, want 1", payments.listCalls)
	}
	if payments.getCalls != 0 {
		t.Fatalf("GetIntent calls=%d, want 0 (no payment_no exists)", payments.getCalls)
	}
}

// AC-40 narrowing (SPEC-0002 v1.8): pay_cent=0 with a captured intent is NOT a
// zero order. SettleZero must fail closed and leave the order untouched.
func TestSettleZeroRejectsCapturedPayment(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{intents: []*pb.Intent{{
		PaymentNo: "PAY-ODD", OrderNo: "ORD-ZCAP", AmountCent: 500,
		CapturedCent: 500, Status: paymentStatusSucceeded,
	}}}, nil)
	ctx := context.Background()
	if _, err := db.Exec(zeroOrderInsert, "ORD-ZCAP"); err != nil {
		t.Fatal(err)
	}
	var versionBefore int64
	if err := db.QueryRow(`SELECT version FROM orders WHERE order_no='ORD-ZCAP'`).Scan(&versionBefore); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SettleZero(ctx, &pb.SettleZeroRequest{OrderNo: "ORD-ZCAP"}); err == nil {
		t.Fatal("SettleZero succeeded on a zero-amount order with a capture")
	} else {
		if !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
			t.Fatalf("error code=%v, want FAILED_PRECONDITION: %v", apperr.From(err).Code, err)
		}
		if got := apperr.From(err).Details["captured_cent"]; got != int64(500) {
			t.Fatalf("details=%v, want captured_cent=500", apperr.From(err).Details)
		}
	}
	var status, paymentNo, refundState string
	var paid, version int64
	if err := db.QueryRow(`SELECT status, paid_cent, settlement_payment_no, refund_state, version
		FROM orders WHERE order_no='ORD-ZCAP'`).Scan(&status, &paid, &paymentNo, &refundState, &version); err != nil {
		t.Fatal(err)
	}
	if status != statusPendingPayment || paid != 0 || paymentNo != "" || refundState != refundStateNone {
		t.Fatalf("order mutated: status=%q paid=%d payment_no=%q refund_state=%q", status, paid, paymentNo, refundState)
	}
	if version != versionBefore {
		t.Fatalf("order version changed %d -> %d", versionBefore, version)
	}
	var events int
	if err := db.QueryRow(`SELECT COUNT(*) FROM outbox_events WHERE aggregate_id='ORD-ZCAP'`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 0 {
		t.Fatalf("outbox events=%d, want 0", events)
	}
}

// A SUCCEEDED intent is a capture even when a legacy row left captured_cent at 0.
func TestSettleZeroRejectsSucceededIntentWithoutCapturedCent(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{intents: []*pb.Intent{{
		PaymentNo: "PAY-LEGACY", OrderNo: "ORD-ZSUCC", AmountCent: 700,
		CapturedCent: 0, Status: paymentStatusSucceeded,
	}}}, nil)
	ctx := context.Background()
	if _, err := db.Exec(zeroOrderInsert, "ORD-ZSUCC"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SettleZero(ctx, &pb.SettleZeroRequest{OrderNo: "ORD-ZSUCC"}); err == nil {
		t.Fatal("SettleZero succeeded with a SUCCEEDED intent")
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM orders WHERE order_no='ORD-ZSUCC'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != statusPendingPayment {
		t.Fatalf("status=%q, want PENDING_PAYMENT", status)
	}
}

// Payment must be a required dependency for the zero decision: an unavailable
// Payment or a missing client fails closed instead of assuming "no capture".
func TestSettleZeroFailsClosedWhenPaymentUnavailable(t *testing.T) {
	db := orderTestDB(t)
	ctx := context.Background()
	cases := []struct {
		name     string
		payments paymentPort
	}{
		{"payment error", &fakePaymentPort{listErr: errors.New("payment timeout")}},
		{"payment not configured", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orderNo := "ORD-ZUNAVL-" + tc.name
			if _, err := db.Exec(zeroOrderInsert, orderNo); err != nil {
				t.Fatal(err)
			}
			svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
			svc.EnableSettlementClients(tc.payments, nil)
			_, err := svc.SettleZero(ctx, &pb.SettleZeroRequest{OrderNo: orderNo})
			if err == nil {
				t.Fatal("SettleZero succeeded without payment facts")
			}
			if !apperr.IsCode(err, apperr.CodeUnavailable) {
				t.Fatalf("error code=%v, want UNAVAILABLE: %v", apperr.From(err).Code, err)
			}
			var status string
			if err := db.QueryRow(`SELECT status FROM orders WHERE order_no=?`, orderNo).Scan(&status); err != nil {
				t.Fatal(err)
			}
			if status != statusPendingPayment {
				t.Fatalf("status=%q, want PENDING_PAYMENT", status)
			}
		})
	}
}
