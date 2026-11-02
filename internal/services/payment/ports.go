package payment

import (
	"context"

	pb "gin-kratos-gorm/api/gen/mall/v1"

	"google.golang.org/grpc"
)

// orderPort is the narrow order dependency of payment. It covers the direct
// fallback and the read-only TCC verification path.
type orderPort interface {
	GetOrder(ctx context.Context, in *pb.GetOrderRequest, opts ...grpc.CallOption) (*pb.Order, error)
	MarkPaid(ctx context.Context, in *pb.MarkPaidRequest, opts ...grpc.CallOption) (*pb.Order, error)
	TccQueryMarkPaid(ctx context.Context, in *pb.TccPaymentRequest, opts ...grpc.CallOption) (*pb.TccQueryReply, error)
}
