package payment

import (
	"context"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

func TestRefundBatchValidationRejectsBeforeDatabase(t *testing.T) {
	svc := New(nil, nil, "", nil)
	ctx := context.Background()
	items := func(items ...*pb.RefundBatchItem) []*pb.RefundBatchItem { return items }

	tests := []struct {
		name       string
		req        *pb.RefundBatchRequest
		wantCode   apperr.Code
		wantReason string
	}{
		{
			name: "missing payment_no",
			req: &pb.RefundBatchRequest{BatchId: "B", ExpectedTotalCent: 10,
				Items: items(&pb.RefundBatchItem{RefundId: "R1", AmountCent: 10})},
			wantCode: apperr.CodeInvalidArgument, wantReason: refundBatchReasonInvalid,
		},
		{
			name: "missing batch_id",
			req: &pb.RefundBatchRequest{PaymentNo: "PAY", ExpectedTotalCent: 10,
				Items: items(&pb.RefundBatchItem{RefundId: "R1", AmountCent: 10})},
			wantCode: apperr.CodeInvalidArgument, wantReason: refundBatchReasonInvalid,
		},
		{
			name:     "empty items",
			req:      &pb.RefundBatchRequest{PaymentNo: "PAY", BatchId: "B", ExpectedTotalCent: 0},
			wantCode: apperr.CodeInvalidArgument, wantReason: refundBatchReasonInvalid,
		},
		{
			name: "nil item",
			req: &pb.RefundBatchRequest{PaymentNo: "PAY", BatchId: "B", ExpectedTotalCent: 10,
				Items: items(nil)},
			wantCode: apperr.CodeInvalidArgument, wantReason: refundBatchReasonInvalid,
		},
		{
			name: "zero amount",
			req: &pb.RefundBatchRequest{PaymentNo: "PAY", BatchId: "B", ExpectedTotalCent: 0,
				Items: items(&pb.RefundBatchItem{RefundId: "R1"})},
			wantCode: apperr.CodeInvalidArgument, wantReason: refundBatchReasonInvalid,
		},
		{
			name: "negative amount",
			req: &pb.RefundBatchRequest{PaymentNo: "PAY", BatchId: "B", ExpectedTotalCent: -10,
				Items: items(&pb.RefundBatchItem{RefundId: "R1", AmountCent: -10})},
			wantCode: apperr.CodeInvalidArgument, wantReason: refundBatchReasonInvalid,
		},
		{
			name: "duplicate refund_id",
			req: &pb.RefundBatchRequest{PaymentNo: "PAY", BatchId: "B", ExpectedTotalCent: 10,
				Items: items(
					&pb.RefundBatchItem{RefundId: "R1", AmountCent: 5},
					&pb.RefundBatchItem{RefundId: "R1", AmountCent: 5})},
			wantCode: apperr.CodeInvalidArgument, wantReason: refundBatchReasonInvalid,
		},
		{
			name: "refund_id with whitespace is not normalized",
			req: &pb.RefundBatchRequest{PaymentNo: "PAY", BatchId: "B", ExpectedTotalCent: 10,
				Items: items(&pb.RefundBatchItem{RefundId: " R1", AmountCent: 10})},
			wantCode: apperr.CodeInvalidArgument, wantReason: refundBatchReasonInvalid,
		},
		{
			name: "sum mismatch",
			req: &pb.RefundBatchRequest{PaymentNo: "PAY", BatchId: "B", ExpectedTotalCent: 99,
				Items: items(
					&pb.RefundBatchItem{RefundId: "R1", AmountCent: 5},
					&pb.RefundBatchItem{RefundId: "R2", AmountCent: 5})},
			wantCode: apperr.CodeInvalidArgument, wantReason: refundBatchReasonSumMismatch,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.RefundBatch(ctx, tc.req)
			if err == nil {
				t.Fatal("RefundBatch accepted an invalid request")
			}
			ae := apperr.From(err)
			if ae.Code != tc.wantCode {
				t.Fatalf("error code = %s, want %s (%v)", ae.Code, tc.wantCode, err)
			}
			if got := ae.Details["reason"]; got != tc.wantReason {
				t.Fatalf("error reason = %v, want %s (%v)", got, tc.wantReason, err)
			}
		})
	}
}

func TestRefundBatchFingerprintIsOrderInsensitive(t *testing.T) {
	a := []refundBatchItem{
		{refundID: "tcc:PAY:G1", amountCent: 6000},
		{refundID: "tcc:PAY:G2", amountCent: 4000},
	}
	b := []refundBatchItem{
		{refundID: "tcc:PAY:G2", amountCent: 4000},
		{refundID: "tcc:PAY:G1", amountCent: 6000},
	}
	fpA, totalA := refundBatchFingerprint(a)
	fpB, totalB := refundBatchFingerprint(b)
	if fpA != fpB || totalA != totalB || totalA != 10000 {
		t.Fatalf("fingerprint(a) = (%s,%d), fingerprint(b) = (%s,%d)", fpA, totalA, fpB, totalB)
	}
	if fpC, _ := refundBatchFingerprint([]refundBatchItem{
		{refundID: "tcc:PAY:G1", amountCent: 5999},
		{refundID: "tcc:PAY:G2", amountCent: 4000},
	}); fpC == fpA {
		t.Fatal("a different amount must change the fingerprint")
	}
}
