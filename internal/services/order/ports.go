package order

import (
	"context"

	pb "gin-kratos-gorm/api/gen/mall/v1"

	"google.golang.org/grpc"
)

// Ports are the narrow outbound contracts of the order usecase. The generated
// Kratos clients satisfy them, while unit tests can provide small fakes instead
// of a full pb client.
type inventoryPort interface {
	Confirm(ctx context.Context, in *pb.ReserveRequest, opts ...grpc.CallOption) (*pb.Empty, error)
	Release(ctx context.Context, in *pb.ReserveRequest, opts ...grpc.CallOption) (*pb.Empty, error)
	CreateSeckillReservation(ctx context.Context, in *pb.CreateSeckillReservationRequest, opts ...grpc.CallOption) (*pb.ReserveReply, error)
	ReleaseSeckillReservation(ctx context.Context, in *pb.ReleaseSeckillReservationRequest, opts ...grpc.CallOption) (*pb.Empty, error)
}

type pricingPort interface {
	GetQuote(ctx context.Context, in *pb.GetQuoteRequest, opts ...grpc.CallOption) (*pb.Quote, error)
	LockCoupon(ctx context.Context, in *pb.LockCouponRequest, opts ...grpc.CallOption) (*pb.Empty, error)
	ConsumeCoupon(ctx context.Context, in *pb.LockCouponRequest, opts ...grpc.CallOption) (*pb.Empty, error)
	UnlockCoupon(ctx context.Context, in *pb.LockCouponRequest, opts ...grpc.CallOption) (*pb.Empty, error)
}
