package aftersales

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/metrics"
)

// RunRefundRecovery retries refunds that were persisted as REFUNDING but never
// reached REFUNDED (a crash or a payment outage between the two phases). Refund
// is idempotent by after-sale number, so this is safe on every replica.
func RunRefundRecovery(ctx context.Context, database *sql.DB, svc *Service, logger *slog.Logger, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	if logger == nil {
		logger = slog.Default()
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			recoverRefunds(ctx, database, svc, logger)
		}
	}
}

func recoverRefunds(ctx context.Context, database *sql.DB, svc *Service, logger *slog.Logger) {
	rows, err := database.QueryContext(ctx, `SELECT after_sale_no FROM after_sales
		WHERE status = ? AND updated_at <= DATE_SUB(NOW(3), INTERVAL 2 MINUTE)
		ORDER BY id LIMIT 50`, statusRefunding)
	if err != nil {
		logger.Warn("scan refunding after-sales", "error", err)
		return
	}
	var numbers []string
	for rows.Next() {
		var no string
		if err := rows.Scan(&no); err == nil {
			numbers = append(numbers, no)
		}
	}
	rows.Close()
	for _, no := range numbers {
		if _, err := svc.Refund(ctx, &pb.RefundAfterSaleRequest{AfterSaleNo: no}); err != nil {
			metrics.AfterSaleRefundRecovery.WithLabelValues("retry").Inc()
			logger.Warn("retry stuck refund", "after_sale_no", no, "error", err)
			continue
		}
		metrics.AfterSaleRefundRecovery.WithLabelValues("recovered").Inc()
		logger.Info("recovered stuck refund", "after_sale_no", no)
	}
	if len(numbers) > 0 {
		logger.Info("refund recovery scan", "candidates", len(numbers))
	}
}
