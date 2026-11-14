package payment

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
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
	if client == nil || client.JS == nil {
		return
	}
	handler := func(msg *nats.Msg) {
		var event orderRefundRequested
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			logger.Error("decode order.refund_requested", "error", err)
			_ = msg.Term()
			return
		}
		if event.PaidCent <= 0 {
			// No money was captured (e.g. a zero-amount or cancelled-before-pay
			// TCC branch). Acking is safe and avoids a poison loop; the order
			// side no longer creates REFUNDING rows for this case.
			metrics.PaymentRefundEvents.WithLabelValues("zero_amount").Inc()
			logger.Warn("order.refund_requested without captured amount", "order_no", event.OrderNo)
			_ = msg.Ack()
			return
		}
		if event.PaymentNo == "" {
			metrics.PaymentRefundEvents.WithLabelValues("invalid").Inc()
			logger.Error("order.refund_requested missing payment_no", "order_no", event.OrderNo, "paid_cent", event.PaidCent)
			_ = msg.Term()
			return
		}
		if _, err := svc.Refund(ctx, &pb.RefundRequest{
			PaymentNo:  event.PaymentNo,
			AmountCent: event.PaidCent,
			Reason:     "order refund requested after TCC cancel",
		}); err != nil {
			metrics.PaymentRefundEvents.WithLabelValues("retry").Inc()
			logger.Warn("refund order request", "order_no", event.OrderNo, "payment_no", event.PaymentNo, "error", err)
			_ = msg.NakWithDelay(5 * time.Second)
			return
		}
		metrics.PaymentRefundEvents.WithLabelValues("refunded").Inc()
		_ = msg.Ack()
	}
	if _, err := client.QueueSubscribe("shop.order.refund_requested.v1", "payment-refund", handler,
		nats.Durable("payment-order-refund"), nats.ManualAck(), nats.AckExplicit(), nats.MaxDeliver(10)); err != nil {
		logger.Error("subscribe order.refund_requested", "error", err)
	}
}
