package order

import (
	"context"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

// statsWindow normalizes an optional from/to range. The default is the last
// 30 days; an explicit range is capped at one year to keep the aggregate cheap.
func statsWindow(fromUnix, toUnix int64) (time.Time, time.Time, error) {
	to := time.Now()
	if toUnix > 0 {
		to = time.Unix(toUnix, 0)
	}
	from := to.AddDate(0, 0, -30)
	if fromUnix > 0 {
		from = time.Unix(fromUnix, 0)
	}
	if !from.Before(to) {
		return time.Time{}, time.Time{}, apperr.Invalid("from_unix must be before to_unix")
	}
	if to.Sub(from) > 366*24*time.Hour {
		return time.Time{}, time.Time{}, apperr.Invalid("stats window must be at most 366 days")
	}
	return from, to, nil
}

// GetOrderStats returns read-only aggregates for the admin copilot. It never
// exposes buyer identity or individual orders.
func (s *Service) GetOrderStats(ctx context.Context, req *pb.StatsRequest) (*pb.OrderStats, error) {
	from, to, err := statsWindow(req.GetFromUnix(), req.GetToUnix())
	if err != nil {
		return nil, err
	}
	var stats pb.OrderStats
	err = s.db.QueryRowContext(ctx, `SELECT
			COUNT(*),
			COALESCE(SUM(status IN ('PAID','SHIPPED','COMPLETED')), 0),
			COALESCE(SUM(status = 'COMPLETED'), 0),
			COALESCE(SUM(status = 'CANCELLED'), 0),
			COALESCE(SUM(CASE WHEN status IN ('PAID','SHIPPED','COMPLETED') THEN paid_cent ELSE 0 END), 0)
		FROM orders WHERE created_at >= ? AND created_at < ?`, from, to).Scan(
		&stats.TotalOrders, &stats.PaidOrders, &stats.CompletedOrders,
		&stats.CancelledOrders, &stats.PaidCent)
	if err != nil {
		return nil, apperr.Internal("query order stats", err)
	}
	stats.FromUnix = from.Unix()
	stats.ToUnix = to.Unix()
	return &stats, nil
}
