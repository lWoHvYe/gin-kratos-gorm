package payment

import (
	"context"
	"encoding/json"
	"log/slog"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/consumer"
	"gin-kratos-gorm/internal/platform/metrics"
	"gin-kratos-gorm/internal/platform/mq"

	"github.com/nats-io/nats.go"
)

type orderRefundRequested struct {
	OrderNo   string `json:"order_no"`
	PaymentNo string `json:"payment_no"`
	PaidCent  int64  `json:"paid_cent"`
}

// RunRefundConsumer compensates a payment-confirm TCC failure. The order
// service publishes shop.order.refund_requested.v1 when its TCC Cancel runs;
// this consumer refunds the captured payment idempotently.
func RunRefundConsumer(ctx context.Context, client *mq.Client, svc *Service, logger *slog.Logger) {
	err := consumer.Subscribe(ctx, client, logger, consumer.Config{
		Subject:    "shop.order.refund_requested.v1",
		Durable:    "payment-order-refund",
		MaxDeliver: 10,
	}, func(hctx context.Context, msg *nats.Msg) error {
		var event orderRefundRequested
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			return consumer.Permanent(err)
		}
		if event.PaidCent <= 0 {
			// No money was captured (zero-amount TCC branch). Acking is safe and
			// avoids a poison loop; the order side no longer creates REFUNDING
			// rows for this case.
			metrics.PaymentRefundEvents.WithLabelValues("zero_amount").Inc()
			logger.Warn("order.refund_requested without captured amount", "order_no", event.OrderNo)
			return nil
		}
		if event.PaymentNo == "" {
			metrics.PaymentRefundEvents.WithLabelValues("invalid").Inc()
			return consumer.Permanent(errMissingPaymentNo)
		}
		if _, err := svc.Refund(hctx, &pb.RefundRequest{
			PaymentNo:  event.PaymentNo,
			AmountCent: event.PaidCent,
			Reason:     "order refund requested after TCC cancel",
		}); err != nil {
			metrics.PaymentRefundEvents.WithLabelValues("retry").Inc()
			return err
		}
		metrics.PaymentRefundEvents.WithLabelValues("refunded").Inc()
		return nil
	})
	if err != nil {
		logger.Error("subscribe order.refund_requested", "error", err)
	}
}

type missingPaymentNoError struct{}

func (missingPaymentNoError) Error() string { return "order.refund_requested missing payment_no" }

var errMissingPaymentNo = missingPaymentNoError{}
