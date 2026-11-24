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
	// GetRefundByKey reads the Payment-owned refund fact by
	// (payment_no, refund_id). The T1.4b1-R preflight uses it to attribute what
	// is already refunded/in-flight before any new money moves; a NotFound means
	// "no refund with this key" (outstanding), not a transport failure.
	GetRefundByKey(ctx context.Context, in *pb.GetRefundByKeyRequest, opts ...grpc.CallOption) (*pb.Refund, error)
	// RefundBatch atomically refunds every outstanding group of a TCC batch
	// (C-04). Payment commits all refund rows/events in one transaction and
	// replays the persisted reply for the same (payment_no, batch_id); Order
	// never falls back to Refund per group.
	RefundBatch(ctx context.Context, in *pb.RefundBatchRequest, opts ...grpc.CallOption) (*pb.RefundBatchReply, error)
	// Refund executes one captured-payment refund for non-TCC flows (for
	// example an after-sales refund admitted by C-04). The TCC group payment
	// phase must use RefundBatch only: per-group Refund cannot close the
	// external-refund race (C-04 §2.3).
	Refund(ctx context.Context, in *pb.RefundRequest, opts ...grpc.CallOption) (*pb.Empty, error)
}

type fulfillmentPort interface {
	CancelIfCreated(ctx context.Context, in *pb.CancelIfCreatedRequest, opts ...grpc.CallOption) (*pb.CancelIfCreatedReply, error)
	// ListShipments reads the Fulfillment-owned shipment facts (S6). Order has
	// no shipment/shipped_at storage, so the AC-51 "EXCEPTION and shipped_at"
	// already-left-warehouse rule can only be evaluated from these facts
	// (ADR-0007 Amd1 Addendum 2 R-2). The existing C-04 RPC is reused; no proto
	// change is needed.
	ListShipments(ctx context.Context, in *pb.ListShipmentsRequest, opts ...grpc.CallOption) (*pb.ShipmentList, error)
}

type pricingPort interface {
	GetQuote(ctx context.Context, in *pb.GetQuoteRequest, opts ...grpc.CallOption) (*pb.Quote, error)
	LockCoupon(ctx context.Context, in *pb.LockCouponRequest, opts ...grpc.CallOption) (*pb.Empty, error)
	ConsumeCoupon(ctx context.Context, in *pb.LockCouponRequest, opts ...grpc.CallOption) (*pb.Empty, error)
	UnlockCoupon(ctx context.Context, in *pb.LockCouponRequest, opts ...grpc.CallOption) (*pb.Empty, error)
}
