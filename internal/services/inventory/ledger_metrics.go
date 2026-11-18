package inventory

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"gin-kratos-gorm/internal/platform/metrics"
)

// RunLedgerMetrics quantifies inventory_ledger growth before any retention or
// archive policy is chosen. It never deletes rows.
func RunLedgerMetrics(ctx context.Context, database *sql.DB, logger *slog.Logger, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			updateLedgerMetrics(ctx, database, logger)
		}
	}
}

func updateLedgerMetrics(ctx context.Context, database *sql.DB, logger *slog.Logger) {
	var rows, oldest int64
	err := database.QueryRowContext(ctx, `SELECT COUNT(*),
		COALESCE(TIMESTAMPDIFF(SECOND, MIN(created_at), NOW()), 0) FROM inventory_ledger`).Scan(&rows, &oldest)
	if err != nil {
		logger.Warn("inventory ledger metrics query failed", "error", err)
		return
	}
	metrics.InventoryLedgerRows.Set(float64(rows))
	metrics.InventoryLedgerOldestSeconds.Set(float64(oldest))
}
