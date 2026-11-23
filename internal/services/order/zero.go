package order

import (
	"context"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

const (
	// zeroPaymentPrefix marks the synthetic settlement identity of a zero-amount
	// order (AC-40 / INV-46: `zero:<order_no>`).
	zeroPaymentPrefix = "zero:"
	// paymentStatusSucceeded is Payment's terminal capture status. A SUCCEEDED
	// intent is a capture even if a legacy row carries captured_cent = 0.
	paymentStatusSucceeded = "SUCCEEDED"
)

// SettleZero settles a fully discounted order without a Payment intent or
// channel capture. It reuses the normal MarkPaid settlement path, so readiness,
// freeze, settlement identity and events are identical.
//
// AC-40 (SPEC-0002 v1.8, narrowed): a `pay_cent = 0` order may only be settled
// as a zero order when Payment reports no captured intent at all. `pay_cent = 0`
// with a capture is not a zero order and must go through the payment-level
// refund/legacy queue instead (the queue write itself is a later task).
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
		// Already settled: an idempotent read does not re-run the zero decision.
		return s.GetOrder(ctx, &pb.GetOrderRequest{OrderNo: orderNo, Admin: true})
	}
	if record.Status != statusPendingPayment {
		return nil, apperr.Conflict("order cannot be settled in status " + record.Status)
	}
	capturedCent, err := s.capturedCentForZeroOrder(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	if capturedCent > 0 {
		// AC-40 narrowing: never resolve `zero:<order_no>` or settle as zero
		// while any capture exists. Fail closed until the captures are refunded
		// through the payment-level/legacy path.
		return nil, apperr.Failed("zero-amount order has a captured payment; resolve the payment-level refund first").
			WithDetails(map[string]any{
				"order_no": orderNo, "pay_cent": record.PayCent, "captured_cent": capturedCent,
			})
	}
	return s.MarkPaid(ctx, &pb.MarkPaidRequest{
		OrderNo: orderNo, PaymentNo: zeroPaymentPrefix + orderNo, PaidCent: 0,
	})
}

// capturedCentForZeroOrder asks Payment whether the order has any captured
// intent. It is a read-only authority check: it must not be skipped, and any
// Payment error fails closed (never "assume zero").
func (s *Service) capturedCentForZeroOrder(ctx context.Context, orderNo string) (int64, error) {
	if s.payments == nil {
		return 0, apperr.Unavailable("payment client is required to verify a zero-amount order", nil)
	}
	intents, err := s.payments.ListOrderIntents(ctx, &pb.ListOrderIntentsRequest{OrderNo: orderNo})
	if err != nil {
		return 0, apperr.Unavailable("list payment intents for the zero-amount order", err)
	}
	var captured int64
	for _, intent := range intents.GetIntents() {
		amount := intent.GetCapturedCent()
		if amount <= 0 && intent.GetStatus() == paymentStatusSucceeded {
			amount = intent.GetAmountCent()
		}
		if amount > captured {
			captured = amount
		}
	}
	return captured, nil
}
