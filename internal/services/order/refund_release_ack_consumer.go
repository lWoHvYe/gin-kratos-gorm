package order

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/consumer"
	"gin-kratos-gorm/internal/platform/db"
	"gin-kratos-gorm/internal/platform/events"
	"gin-kratos-gorm/internal/platform/mq"

	"github.com/nats-io/nats.go"
)

const (
	refundReleaseReceiptPending  = "PENDING"
	refundReleaseReceiptReleased = "RELEASED"
	refundReleaseReceiptFailed   = "FAILED"
)

// RunRefundReleaseAckConsumer consumes the Inventory ack and advances the
// Order-side receipt. It never calls Payment; the legacy compensation worker
// does that only after reading RELEASED.
func RunRefundReleaseAckConsumer(ctx context.Context, client *mq.Client, svc *Service, logger *slog.Logger) {
	err := consumer.Subscribe(ctx, client, logger, consumer.Config{
		Subject: events.RefundReleaseCompletedTopic,
		Durable: "order-refund-release-ack",
	}, func(hctx context.Context, msg *nats.Msg) error {
		if err := events.ValidateJSON(events.RefundReleaseCompletedEventType, msg.Data); err != nil {
			return consumer.Permanent(err)
		}
		var event events.RefundReleaseCompleted
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			return consumer.Permanent(err)
		}
		return applyRefundReleaseAck(hctx, svc, event)
	})
	if err != nil {
		logger.Error("subscribe inventory.refund_release_completed", "error", err)
	}
}

func applyRefundReleaseAck(ctx context.Context, svc *Service, event events.RefundReleaseCompleted) error {
	releaseID := strings.TrimSpace(event.ReleaseID)
	refundID := strings.TrimSpace(event.RefundID)
	if releaseID == "" || refundID == "" || strings.TrimSpace(event.OrderNo) == "" {
		return apperr.Invalid("release ack is missing identity")
	}
	if event.Status != events.RefundReleaseStatusReleased && event.Status != events.RefundReleaseStatusFailed {
		return apperr.Invalid("release ack status is not RELEASED/FAILED")
	}
	state := refundReleaseReceiptReleased
	lastError := ""
	if event.Status == events.RefundReleaseStatusFailed {
		state = refundReleaseReceiptFailed
		lastError = strings.TrimSpace(event.Error)
		if lastError == "" {
			lastError = "inventory release failed"
		}
		if len(lastError) > 512 {
			lastError = lastError[:512]
		}
	}
	return db.WithTx(ctx, svc.db, func(tx *sql.Tx) error {
		var current, orderNo string
		var version int64
		err := tx.QueryRowContext(ctx, `SELECT state, order_no, version FROM refund_release_receipts
			WHERE release_id = ? FOR UPDATE`, releaseID).Scan(&current, &orderNo, &version)
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound("refund release receipt not found")
		}
		if err != nil {
			return apperr.Internal("load refund release receipt", err)
		}
		if orderNo != strings.TrimSpace(event.OrderNo) {
			return apperr.Failed("release ack order does not match the receipt").
				WithDetails(map[string]any{"release_id": releaseID, "receipt_order_no": orderNo, "ack_order_no": event.OrderNo})
		}
		if current == state {
			return nil
		}
		if current != refundReleaseReceiptPending {
			return apperr.Failed("release receipt is already terminal").
				WithDetails(map[string]any{"release_id": releaseID, "state": current, "ack_state": state})
		}
		res, err := tx.ExecContext(ctx, `UPDATE refund_release_receipts
			SET state = ?, last_error = ?, version = version + 1, lease_owner = '', lease_until = NULL, updated_at = NOW(3)
			WHERE release_id = ? AND state = ?`, state, lastError, releaseID, refundReleaseReceiptPending)
		if err != nil {
			return apperr.Internal("update refund release receipt", err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return apperr.Conflict("release receipt changed concurrently")
		}
		return nil
	})
}

// insertRefundReleaseReceiptTx is used by the legacy compensation transaction
// that writes the release request. The ack consumer only advances PENDING rows.
func insertRefundReleaseReceiptTx(ctx context.Context, tx *sql.Tx, releaseID, refundID, orderNo string) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO refund_release_receipts
		(release_id, refund_id, order_no, state, version)
		VALUES (?, ?, ?, ?, 1)
		ON DUPLICATE KEY UPDATE release_id = release_id`,
		releaseID, refundID, orderNo, refundReleaseReceiptPending); err != nil {
		return apperr.Internal("insert refund release receipt", err)
	}
	return nil
}
