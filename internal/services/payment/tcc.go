package payment

import (
	"context"
	"os"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/dtm"

	"github.com/dtm-labs/client/dtmgrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type tccConfig struct {
	client       *dtm.Client
	orderURL     string
	inventoryURL string
	pricingURL   string
	enabled      bool
}

// EnableTCC wires the payment-confirm TCC. It is additive: when disabled or
// unconfigured, Callback keeps using the direct MarkPaid path.
func (s *Service) EnableTCC(client *dtm.Client, orderURL, inventoryURL, pricingURL string, enabled bool) {
	s.tcc = &tccConfig{client: client, orderURL: orderURL, inventoryURL: inventoryURL, pricingURL: pricingURL, enabled: enabled}
}

func (s *Service) markPaidTCC(ctx context.Context, in *intent) error {
	if s.tcc == nil || !s.tcc.enabled || s.tcc.client == nil || s.orders == nil {
		return s.markPaidDirect(ctx, in)
	}
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	order, err := s.orders.GetOrder(callCtx, &pb.GetOrderRequest{OrderNo: in.OrderNo, Admin: true})
	if err != nil {
		return mapOrderError(err)
	}
	switch order.GetStatus() {
	case "CANCELLED", "CLOSED":
		return s.refundUnpayable(ctx, in)
	case "PAID", "ALLOCATED", "SHIPPED", "COMPLETED", "PARTIALLY_REFUNDED", "REFUNDED":
		return nil
	}
	req := &pb.TccPaymentRequest{
		OrderNo: in.OrderNo, PaymentNo: in.PaymentNo, UserId: in.UserID,
		PaidCent: in.AmountCent, QuoteId: order.GetQuoteId(), Items: order.GetItems(),
	}
	orderTry := strings.TrimRight(envOr("ORDER_TCC_TRY_URL", s.tcc.orderURL), "/")
	inventoryTry := strings.TrimRight(envOr("INVENTORY_TCC_TRY_URL", s.tcc.inventoryURL), "/")
	pricingTry := strings.TrimRight(envOr("PRICING_TCC_TRY_URL", s.tcc.pricingURL), "/")
	err = s.tcc.client.TCC(ctx, "payment-confirm:"+in.PaymentNo, func(tcc *dtmgrpc.TccGrpc) error {
		// Try runs in this process, while Confirm/Cancel run from the DTM
		// container. In Compose these are different addresses; in Kubernetes
		// both are the same Service DNS name.
		if err := tcc.CallBranch(req, orderTry+"/TccTryMarkPaid", s.tcc.orderURL+"/TccConfirmMarkPaid", s.tcc.orderURL+"/TccCancelMarkPaid", &pb.Empty{}); err != nil {
			return err
		}
		if err := tcc.CallBranch(req, inventoryTry+"/TccTryConfirm", s.tcc.inventoryURL+"/TccConfirm", s.tcc.inventoryURL+"/TccCancelConfirm", &pb.Empty{}); err != nil {
			return err
		}
		// Seckill orders have no pricing quote: price is fixed and coupons and
		// promotion stacking are intentionally bypassed. The order/inventory
		// branches remain mandatory.
		if strings.HasPrefix(order.GetQuoteId(), "seckill:") {
			return nil
		}
		return tcc.CallBranch(req, pricingTry+"/TccTryConsume", s.tcc.pricingURL+"/TccConfirmConsume", s.tcc.pricingURL+"/TccCancelConsume", &pb.Empty{})
	})
	if err != nil {
		s.log.Error("payment tcc failed", "payment_no", in.PaymentNo, "order_no", in.OrderNo, "error", err)
		// The order may have been cancelled between settlement and confirm. The
		// captured money must not disappear; refund it idempotently.
		if latest, lerr := s.orders.GetOrder(ctx, &pb.GetOrderRequest{OrderNo: in.OrderNo, Admin: true}); lerr == nil {
			if latest.GetStatus() == "CANCELLED" || latest.GetStatus() == "CLOSED" {
				if rerr := s.refundUnpayable(ctx, in); rerr == nil {
					return nil
				}
			}
		}
		return apperr.Unavailable("payment confirmation tcc failed", err)
	}
	return nil
}

func (s *Service) refundUnpayable(ctx context.Context, in *intent) error {
	amount := in.CapturedCent
	if amount <= 0 {
		amount = in.AmountCent
	}
	if amount <= 0 {
		return nil
	}
	_, err := s.Refund(ctx, &pb.RefundRequest{
		PaymentNo: in.PaymentNo, AmountCent: amount,
		Reason: "order not payable", IdempotencyKey: "auto-not-payable:" + in.PaymentNo,
	})
	return err
}

func (s *Service) markPaidDirect(ctx context.Context, in *intent) error {
	if s.orders == nil {
		s.log.Warn("order client is not configured; skipping MarkPaid", "payment_no", in.PaymentNo)
		return nil
	}
	callCtx, cancel := context.WithTimeout(ctx, orderCallTimeout)
	defer cancel()
	_, err := s.orders.MarkPaid(callCtx, &pb.MarkPaidRequest{OrderNo: in.OrderNo, PaymentNo: in.PaymentNo, PaidCent: in.AmountCent})
	if err == nil {
		return nil
	}
	if latest, lerr := s.orders.GetOrder(ctx, &pb.GetOrderRequest{OrderNo: in.OrderNo, Admin: true}); lerr == nil {
		if latest.GetStatus() == "CANCELLED" || latest.GetStatus() == "CLOSED" {
			if rerr := s.refundUnpayable(ctx, in); rerr == nil {
				return nil
			}
		}
	}
	return mapOrderError(err)
}

func mapOrderError(err error) error {
	switch status.Code(err) {
	case codes.NotFound:
		return apperr.NotFound("order not found for payment")
	case codes.InvalidArgument, codes.FailedPrecondition:
		return apperr.Failed("order rejected payment confirmation: " + status.Convert(err).Message())
	case codes.Unavailable, codes.DeadlineExceeded:
		return apperr.Unavailable("order service unavailable", err)
	default:
		return apperr.Internal("mark order paid", err)
	}
}

func tccURLs() (orderURL, inventoryURL, pricingURL string) {
	orderURL = envOr("ORDER_TCC_URL", "order:9006/mall.v1.OrderService")
	inventoryURL = envOr("INVENTORY_TCC_URL", "inventory:9003/mall.v1.InventoryService")
	pricingURL = envOr("PRICING_TCC_URL", "pricing:9005/mall.v1.PricingService")
	return
}

func tccEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("PAYMENT_TCC_ENABLED")))
	return v != "false" && v != "0"
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
