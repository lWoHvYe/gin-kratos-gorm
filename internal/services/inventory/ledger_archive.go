package inventory

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gin-kratos-gorm/internal/platform/metrics"
)

const ledgerRetention = 180 * 24 * time.Hour

// RunLedgerArchive moves ledger rows older than the retention window into
// inventory_ledger_archive. It preserves the audit trail but keeps the hot
// table bounded.
func RunLedgerArchive(ctx context.Context, database *sql.DB, logger *slog.Logger, interval time.Duration) {
	if interval <= 0 {
		interval = time.Hour
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := archiveLedgerOnce(ctx, database); err != nil {
				logger.Warn("inventory ledger archive failed", "error", err)
			} else if n > 0 {
				logger.Info("inventory ledger archived", "rows", n)
			}
		}
	}
}

func archiveLedgerOnce(ctx context.Context, db *sql.DB) (int64, error) {
	cutoff := time.Now().Add(-ledgerRetention)
	var total int64
	for i := 0; i < 5; i++ {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return total, err
		}
		rows, err := tx.QueryContext(ctx, `SELECT id FROM inventory_ledger
			WHERE created_at < ? ORDER BY id LIMIT 1000`, cutoff)
		if err != nil {
			_ = tx.Rollback()
			return total, err
		}
		var ids []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				_ = tx.Rollback()
				return total, err
			}
			ids = append(ids, id)
		}
		rows.Close()
		if len(ids) == 0 {
			_ = tx.Rollback()
			break
		}
		ph := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
		args := make([]any, len(ids))
		for i, id := range ids {
			args[i] = id
		}
		if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO inventory_ledger_archive
			(id,reservation_no,order_no,sku_id,warehouse_id,quantity,entry_type,on_hand_after,reserved_after,created_at,archived_at)
			SELECT id,reservation_no,order_no,sku_id,warehouse_id,quantity,entry_type,on_hand_after,reserved_after,created_at,NOW(3)
			FROM inventory_ledger WHERE id IN (`+ph+`)`, args...); err != nil {
			_ = tx.Rollback()
			return total, fmt.Errorf("archive ledger: %w", err)
		}
		res, err := tx.ExecContext(ctx, `DELETE FROM inventory_ledger WHERE id IN (`+ph+`)`, args...)
		if err != nil {
			_ = tx.Rollback()
			return total, err
		}
		n, _ := res.RowsAffected()
		if err := tx.Commit(); err != nil {
			return total, err
		}
		total += n
		metrics.InventoryLedgerArchived.Add(float64(n))
		if len(ids) < 1000 {
			break
		}
	}
	return total, nil
}
