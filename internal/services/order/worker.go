package order

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/metrics"
)

// RunExpiry cancels unpaid orders whose payment window has elapsed. It is safe
// on multiple replicas: the status update is a compare-and-set and inventory
// release is idempotent.
func RunExpiry(ctx context.Context, db *sql.DB, svc *Service, logger *slog.Logger, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := refreshRefundReviewGauges(ctx, db); err != nil {
				logger.Warn("refresh refund review gauges", "error", err)
			}
			rows, err := db.QueryContext(ctx, `SELECT order_no FROM orders
				WHERE status = ? AND expire_at <= NOW(3) ORDER BY expire_at LIMIT 100`, statusPendingPayment)
			if err != nil {
				logger.Error("scan expired orders", "error", err)
				continue
			}
			var orderNos []string
			for rows.Next() {
				var no string
				if err := rows.Scan(&no); err == nil {
					orderNos = append(orderNos, no)
				}
			}
			rows.Close()
			for _, no := range orderNos {
				if _, err := svc.CancelOrder(ctx, &pb.CancelOrderRequest{OrderNo: no, Admin: true, Reason: "payment timeout"}); err != nil {
					logger.Warn("cancel expired order", "order_no", no, "error", err)
				}
			}
			if len(orderNos) > 0 {
				logger.Info("expired order scan", "cancelled", len(orderNos))
			}
		}
	}
}

// refreshRefundReviewGauges refreshes the AC-72 backend gauges from the
// authoritative admission rows. It is read-only and deliberately has no worker
// of its own: it rides the existing expiry tick. Zero-amount conflicts never
// write these rows, so they are excluded by construction.
func refreshRefundReviewGauges(ctx context.Context, db *sql.DB) error {
	var backlog, oldest int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*),
		COALESCE(TIMESTAMPDIFF(SECOND, MIN(updated_at), NOW(3)), 0)
		FROM order_refund_admissions
		WHERE result = ? AND state = ?`, "REVIEW_REQUIRED", "PENDING_REMOTE").
		Scan(&backlog, &oldest); err != nil {
		return err
	}
	metrics.RefundReviewBacklog.Set(float64(backlog))
	metrics.RefundReviewOldestSeconds.Set(float64(oldest))
	return nil
}
