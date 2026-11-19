package pricing

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"time"

	"gin-kratos-gorm/internal/platform/metrics"
)

// quoteRetention keeps expired quotes long enough for debugging/customer
// support; quote_lines are removed first so the parent delete is cheap.
const quoteRetention = 30 * 24 * time.Hour

func RunQuoteCleanup(ctx context.Context, database *sql.DB, logger *slog.Logger, interval time.Duration) {
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
			cleanupQuotes(ctx, database, logger)
		}
	}
}

func cleanupQuotes(ctx context.Context, database *sql.DB, logger *slog.Logger) {
	cutoff := time.Now().Add(-quoteRetention)
	var removed int64
	for i := 0; i < 20; i++ {
		// MySQL rejects LIMIT inside IN subqueries and multi-table DELETE; load
		// a bounded id batch, then delete children and parents by those ids.
		rows, err := database.QueryContext(ctx, `SELECT quote_id FROM quotes
			WHERE expires_at < ? LIMIT 500`, cutoff)
		if err != nil {
			logger.Warn("quote cleanup query failed", "error", err)
			return
		}
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err == nil {
				ids = append(ids, id)
			}
		}
		rows.Close()
		if len(ids) == 0 {
			break
		}
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
		args := make([]any, len(ids))
		for i, id := range ids {
			args[i] = id
		}
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			logger.Warn("quote cleanup transaction failed", "error", err)
			return
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM quote_lines WHERE quote_id IN (`+placeholders+`)`, args...); err != nil {
			_ = tx.Rollback()
			logger.Warn("quote line cleanup failed", "error", err)
			return
		}
		deleteArgs := append(append([]any{}, args...), cutoff)
		res, err := tx.ExecContext(ctx, `DELETE FROM quotes WHERE quote_id IN (`+placeholders+`) AND expires_at < ?`, deleteArgs...)
		if err != nil {
			_ = tx.Rollback()
			logger.Warn("quote cleanup failed", "error", err)
			return
		}
		quotes, _ := res.RowsAffected()
		if err := tx.Commit(); err != nil {
			logger.Warn("quote cleanup commit failed", "error", err)
			return
		}
		removed += quotes
		metrics.QuotesCleaned.Add(float64(quotes))
		if len(ids) < 500 {
			break
		}
	}
	if removed > 0 {
		logger.Info("expired quotes removed", "count", removed)
	}
}
