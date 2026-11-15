package aftersales

import (
	"context"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

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

// GetAfterSaleStats aggregates after-sale volume and refunded amounts without
// exposing user-level rows.
func (s *Service) GetAfterSaleStats(ctx context.Context, req *pb.StatsRequest) (*pb.AfterSaleStats, error) {
	from, to, err := statsWindow(req.GetFromUnix(), req.GetToUnix())
	if err != nil {
		return nil, err
	}
	var stats pb.AfterSaleStats
	err = s.db.QueryRowContext(ctx, `SELECT
			COUNT(*),
			COALESCE(SUM(status = 'REQUESTED'), 0),
			COALESCE(SUM(status = 'APPROVED'), 0),
			COALESCE(SUM(status = 'REFUNDED'), 0),
			COALESCE(SUM(CASE WHEN status = 'REFUNDED' THEN refunded_cent ELSE 0 END), 0)
		FROM after_sales WHERE created_at >= ? AND created_at < ?`, from, to).Scan(
		&stats.Total, &stats.Requested, &stats.Approved, &stats.Refunded, &stats.RefundedCent)
	if err != nil {
		return nil, apperr.Internal("query after-sale stats", err)
	}
	stats.FromUnix = from.Unix()
	stats.ToUnix = to.Unix()
	return &stats, nil
}
