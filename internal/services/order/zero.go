package order

import (
	"context"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

// SettleZero settles a fully discounted order without a Payment intent or
// channel capture. It reuses the normal MarkPaid settlement path, so readiness,
// freeze, settlement identity and events are identical.
func (s *Service) SettleZero(ctx context.Context, req *pb.SettleZeroRequest) (*pb.Order, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	record, err := findOrder(ctx, s.db, orderNo)
	if err != nil {
		return nil, apperr.Internal("load order for zero settlement", err)
	}
	if record == nil {
		return nil, apperr.NotFound("order not found")
	}
	if record.PayCent != 0 {
		return nil, apperr.Invalid("order is not a zero-amount order")
	}
	if record.Status == statusPaid {
		return s.GetOrder(ctx, &pb.GetOrderRequest{OrderNo: orderNo, Admin: true})
	}
	if record.Status != statusPendingPayment {
		return nil, apperr.Conflict("order cannot be settled in status " + record.Status)
	}
	return s.MarkPaid(ctx, &pb.MarkPaidRequest{
		OrderNo: orderNo, PaymentNo: "zero:" + orderNo, PaidCent: 0,
	})
}
