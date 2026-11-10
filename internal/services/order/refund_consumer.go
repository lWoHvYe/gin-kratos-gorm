package order

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"

	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/db"
	"gin-kratos-gorm/internal/platform/mq"
	"gin-kratos-gorm/internal/platform/outbox"

	"github.com/nats-io/nats.go"
)

type paymentRefundedEvent struct {
	PaymentNo    string `json:"payment_no"`
	OrderNo      string `json:"order_no"`
	AmountCent   int64  `json:"amount_cent"`
	RefundedCent int64  `json:"refunded_cent"`
	CapturedCent int64  `json:"captured_cent"`
}

// RunRefundConsumer projects payment refunds onto the order aggregate. The
// payment event is the source of truth; the update is monotonic and idempotent.
func RunRefundConsumer(ctx context.Context, client *mq.Client, svc *Service, logger *slog.Logger) {
	if client == nil || client.JS == nil {
		return
	}
	handler := func(msg *nats.Msg) {
		var event paymentRefundedEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			logger.Error("decode payment.refunded", "error", err)
			_ = msg.Term()
			return
		}
		if event.OrderNo == "" || event.RefundedCent < 0 {
			_ = msg.Ack()
			return
		}
		err := db.WithTx(ctx, svc.db, func(tx *sql.Tx) error {
			var current int64
			var status string
			if err := tx.QueryRowContext(ctx, `SELECT refunded_cent, status FROM orders WHERE order_no = ? FOR UPDATE`, event.OrderNo).Scan(&current, &status); err != nil {
				return err
			}
			if event.RefundedCent <= current {
				return nil
			}
			next := statusPartiallyRefunded
			if event.CapturedCent > 0 && event.RefundedCent >= event.CapturedCent {
				next = statusRefunded
			}
			if _, err := tx.ExecContext(ctx, `UPDATE orders SET refunded_cent = ?, status = ?, version = version + 1, updated_at = ? WHERE order_no = ?`,
				event.RefundedCent, next, time.Now(), event.OrderNo); err != nil {
				return err
			}
			if err := insertStatusLog(ctx, tx, event.OrderNo, status, next, "payment refund", "payment"); err != nil {
				return err
			}
			return outbox.Append(ctx, tx, outbox.Event{
				Topic: "shop.order.refund_projected.v1", AggregateType: "order", AggregateID: event.OrderNo,
				EventType: "order.refund_projected",
				Payload: map[string]any{"order_no": event.OrderNo, "payment_no": event.PaymentNo,
					"refunded_cent": event.RefundedCent, "captured_cent": event.CapturedCent, "status": next},
			})
		})
		if err != nil {
			if apperr.IsCode(err, apperr.CodeNotFound) {
				_ = msg.Ack()
				return
			}
			logger.Warn("project payment refund", "order_no", event.OrderNo, "error", err)
			_ = msg.NakWithDelay(2 * time.Second)
			return
		}
		_ = msg.Ack()
	}
	if _, err := client.JS.QueueSubscribe("shop.payment.refunded.v1", "order-refund-projection", handler,
		nats.Durable("order-refund-projection"), nats.ManualAck(), nats.AckExplicit(), nats.MaxDeliver(20)); err != nil {
		logger.Error("subscribe payment.refunded", "error", err)
	}
}
