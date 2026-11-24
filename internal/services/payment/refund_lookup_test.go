package payment

import (
	"context"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

func TestGetRefundByKeyValidationRejectsBeforeDatabase(t *testing.T) {
	svc := New(nil, nil, "", nil)
	ctx := context.Background()
	cases := []struct {
		name string
		req  *pb.GetRefundByKeyRequest
	}{
		{name: "nil request", req: nil},
		{name: "missing payment_no", req: &pb.GetRefundByKeyRequest{RefundId: "R1"}},
		{name: "missing refund_id", req: &pb.GetRefundByKeyRequest{PaymentNo: "PAY1"}},
		{name: "whitespace only", req: &pb.GetRefundByKeyRequest{PaymentNo: "  ", RefundId: "  "}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.GetRefundByKey(ctx, tc.req); !apperr.IsCode(err, apperr.CodeInvalidArgument) {
				t.Fatalf("GetRefundByKey(%+v) error = %v, want INVALID_ARGUMENT", tc.req, err)
			}
		})
	}
}
