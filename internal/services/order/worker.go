package order

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
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
