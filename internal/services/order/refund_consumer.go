package order

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/consumer"
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
	err := consumer.Subscribe(ctx, client, logger, consumer.Config{
		Subject: "shop.payment.refunded.v1",
		Durable: "order-refund-projection",
	}, func(hctx context.Context, msg *nats.Msg) error {
		var event paymentRefundedEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			return consumer.Permanent(err)
		}
		if event.OrderNo == "" || event.RefundedCent < 0 {
			return nil
		}
		err := db.WithTx(hctx, svc.db, func(tx *sql.Tx) error {
			var current int64
			var status, refundStatus string
			if err := tx.QueryRowContext(hctx, `SELECT refunded_cent, status, refund_status FROM orders WHERE order_no = ? FOR UPDATE`, event.OrderNo).Scan(&current, &status, &refundStatus); err != nil {
				return err
			}
			if event.RefundedCent <= current {
				return nil
			}
			next := statusPartiallyRefunded
			if event.CapturedCent > 0 && event.RefundedCent >= event.CapturedCent {
				next = statusRefunded
			}
			if _, err := tx.ExecContext(hctx, `UPDATE orders SET refunded_cent = ?, refund_status = ?, version = version + 1, updated_at = ? WHERE order_no = ?`,
				event.RefundedCent, next, time.Now(), event.OrderNo); err != nil {
				return err
			}
			if err := insertStatusLog(hctx, tx, event.OrderNo, status, status, "payment refund "+next, "payment"); err != nil {
				return err
			}
			return outbox.Append(hctx, tx, outbox.Event{
				Topic: "shop.order.refund_projected.v1", AggregateType: "order", AggregateID: event.OrderNo,
				EventType: "order.refund_projected",
				Payload: map[string]any{"order_no": event.OrderNo, "payment_no": event.PaymentNo,
					"refunded_cent": event.RefundedCent, "captured_cent": event.CapturedCent, "status": next},
			})
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || apperr.IsCode(err, apperr.CodeNotFound) {
				logger.Warn("refund projection order missing", "order_no", event.OrderNo)
				return nil
			}
			return err
		}
		return nil
	})
	if err != nil {
		logger.Error("subscribe payment.refunded", "error", err)
	}
}
