package payment

import (
	"context"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"

	"google.golang.org/grpc"
)

type fakeOrderPort struct{ order *pb.Order }

func (f *fakeOrderPort) GetOrder(context.Context, *pb.GetOrderRequest, ...grpc.CallOption) (*pb.Order, error) {
	return f.order, nil
}
func (f *fakeOrderPort) MarkPaid(context.Context, *pb.MarkPaidRequest, ...grpc.CallOption) (*pb.Order, error) {
	return f.order, nil
}
func (f *fakeOrderPort) TccQueryMarkPaid(context.Context, *pb.TccPaymentRequest, ...grpc.CallOption) (*pb.TccQueryReply, error) {
	return &pb.TccQueryReply{}, nil
}

func TestCreateIntentValidatesOrderAmount(t *testing.T) {
	svc := New(nil, &fakeOrderPort{order: &pb.Order{
		OrderNo: "ORD-INT", UserId: 7, Status: "PENDING_PAYMENT", PayCent: 100,
	}}, "", nil)
	ctx := context.Background()
	_, err := svc.CreateIntent(ctx, &pb.CreateIntentRequest{
		OrderNo: "ORD-INT", UserId: 7, AmountCent: 90, Channel: "mock",
	})
	if !apperr.IsCode(err, apperr.CodeInvalidArgument) {
		t.Fatalf("underpay must be rejected, got %v", err)
	}
	_, err = svc.CreateIntent(ctx, &pb.CreateIntentRequest{
		OrderNo: "ORD-INT", UserId: 8, AmountCent: 100, Channel: "mock",
	})
	if !apperr.IsCode(err, apperr.CodePermissionDenied) {
		t.Fatalf("other user must be rejected, got %v", err)
	}
}
