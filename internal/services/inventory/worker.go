package inventory

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
)

// RunExpiry releases HELD reservations whose TTL elapsed. It is safe on every
// replica because Release uses a row lock and is idempotent.
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
			rows, err := db.QueryContext(ctx, `SELECT order_no, sku_id, quantity FROM stock_reservations
				WHERE status = 'HELD' AND expires_at <= NOW(3) ORDER BY expires_at LIMIT 100`)
			if err != nil {
				logger.Error("scan expired reservations", "error", err)
				continue
			}
			type expired struct {
				orderNo string
				skuID   int64
				qty     int32
			}
			var list []expired
			for rows.Next() {
				var e expired
				if err := rows.Scan(&e.orderNo, &e.skuID, &e.qty); err == nil {
					list = append(list, e)
				}
			}
			rows.Close()
			for _, e := range list {
				if _, err := svc.Release(ctx, &pb.ReserveRequest{OrderNo: e.orderNo, SkuId: e.skuID, Quantity: e.qty}); err != nil {
					logger.Warn("release expired reservation", "order_no", e.orderNo, "sku_id", e.skuID, "error", err)
				}
			}
			if len(list) > 0 {
				logger.Info("expired reservation sweep", "released", len(list))
			}
		}
	}
}
