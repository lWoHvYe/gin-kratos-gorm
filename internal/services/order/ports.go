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
	AllocateOrder(ctx context.Context, in *pb.AllocateOrderRequest, opts ...grpc.CallOption) (*pb.AllocateOrderReply, error)
	Confirm(ctx context.Context, in *pb.ReserveRequest, opts ...grpc.CallOption) (*pb.Empty, error)
	Release(ctx context.Context, in *pb.ReserveRequest, opts ...grpc.CallOption) (*pb.Empty, error)
	CreateSeckillReservation(ctx context.Context, in *pb.CreateSeckillReservationRequest, opts ...grpc.CallOption) (*pb.ReserveReply, error)
	ReleaseSeckillReservation(ctx context.Context, in *pb.ReleaseSeckillReservationRequest, opts ...grpc.CallOption) (*pb.Empty, error)
	TccCancelConfirm(ctx context.Context, in *pb.TccPaymentRequest, opts ...grpc.CallOption) (*pb.Empty, error)
	CheckOrderReady(ctx context.Context, in *pb.CheckOrderReadyRequest, opts ...grpc.CallOption) (*pb.CheckOrderReadyReply, error)
}

type paymentPort interface {
	GetIntent(ctx context.Context, in *pb.GetIntentRequest, opts ...grpc.CallOption) (*pb.Intent, error)
	// ListOrderIntents answers "does any intent of this order have a captured
	// amount?" without a payment_no, which is required for the AC-40 zero-order
	// narrowing before a zero settlement.
	ListOrderIntents(ctx context.Context, in *pb.ListOrderIntentsRequest, opts ...grpc.CallOption) (*pb.IntentList, error)
}

type fulfillmentPort interface {
	CancelIfCreated(ctx context.Context, in *pb.CancelIfCreatedRequest, opts ...grpc.CallOption) (*pb.CancelIfCreatedReply, error)
}

type pricingPort interface {
	GetQuote(ctx context.Context, in *pb.GetQuoteRequest, opts ...grpc.CallOption) (*pb.Quote, error)
	LockCoupon(ctx context.Context, in *pb.LockCouponRequest, opts ...grpc.CallOption) (*pb.Empty, error)
	ConsumeCoupon(ctx context.Context, in *pb.LockCouponRequest, opts ...grpc.CallOption) (*pb.Empty, error)
	UnlockCoupon(ctx context.Context, in *pb.LockCouponRequest, opts ...grpc.CallOption) (*pb.Empty, error)
}
