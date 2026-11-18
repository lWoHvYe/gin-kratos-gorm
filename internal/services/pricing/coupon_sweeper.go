package pricing

import (
	"context"
	"database/sql"
	"log/slog"
	"time"
)

// couponLockTTL is deliberately longer than the order payment window; a
// LOCKED coupon older than this is treated as abandoned (process crash,
// cancelled order without an event, etc.).
const couponLockTTL = 30 * time.Minute

// RunCouponLockSweeper returns abandoned LOCKED coupons to AVAILABLE. It is
// idempotent: the conditional update only wins for rows still LOCKED and still
// older than the cutoff.
func RunCouponLockSweeper(ctx context.Context, database *sql.DB, logger *slog.Logger, interval time.Duration) {
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
			sweepCouponLocks(ctx, database, logger)
		}
	}
}

func sweepCouponLocks(ctx context.Context, database *sql.DB, logger *slog.Logger) {
	cutoff := time.Now().Add(-couponLockTTL)
	rows, err := database.QueryContext(ctx,
		`SELECT id, code, order_no FROM coupons
		 WHERE status = ? AND locked_at IS NOT NULL AND locked_at < ? LIMIT 100`,
		couponLocked, cutoff)
	if err != nil {
		logger.Warn("coupon lock sweep query failed", "error", err)
		return
	}
	defer rows.Close()
	type stale struct {
		id      int64
		code    string
		orderNo string
	}
	var items []stale
	for rows.Next() {
		var s stale
		if err := rows.Scan(&s.id, &s.code, &s.orderNo); err != nil {
			logger.Warn("coupon lock sweep scan failed", "error", err)
			return
		}
		items = append(items, s)
	}
	if err := rows.Err(); err != nil {
		logger.Warn("coupon lock sweep rows failed", "error", err)
		return
	}
	for _, s := range items {
		res, err := database.ExecContext(ctx, `UPDATE coupons
			SET status = ?, order_no = '', locked_at = NULL, version = version + 1
			WHERE id = ? AND status = ? AND locked_at < ?`,
			couponAvailable, s.id, couponLocked, cutoff)
		if err != nil {
			logger.Warn("coupon unlock failed", "coupon_id", s.id, "error", err)
			continue
		}
		if n, _ := res.RowsAffected(); n > 0 {
			logger.Info("abandoned coupon lock released", "coupon_id", s.id, "code", s.code, "order_no", s.orderNo)
		}
	}
}
