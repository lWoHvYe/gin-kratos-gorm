package seckill

import (
	"context"
	"fmt"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/metrics"
)

// RunReconciler reports Redis/DB stock mismatches. It deliberately does not
// auto-repair: an operator needs a signal before stock is changed.
func (s *Service) RunReconciler(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.reconcileOnce(ctx)
		}
	}
}

func (s *Service) reconcileOnce(ctx context.Context) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM seckill_activities
		WHERE status IN ('READY','ACTIVE') ORDER BY id LIMIT 100`)
	if err != nil {
		s.log.Warn("query seckill reconciliation activities", "error", err)
		return
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		stock, err := s.inventory.GetSeckillStock(ctx, &pb.GetSeckillStockRequest{ActivityId: id})
		if err != nil {
			s.log.Warn("reconcile seckill inventory", "activity_id", id, "error", err)
			continue
		}
		if stock.GetTotal() != stock.GetAvailable()+stock.GetHeld()+stock.GetSold() {
			metrics.SeckillReconcileMismatch.WithLabelValues("invariant").Inc()
			s.log.Error("seckill bucket invariant violated", "activity_id", id,
				"total", stock.GetTotal(), "available", stock.GetAvailable(), "held", stock.GetHeld(), "sold", stock.GetSold())
		}
		if s.cache == nil {
			continue
		}
		raw, err := s.cache.Get(ctx, fmt.Sprintf("seckill:{%d}:stock", id)).Int64()
		if err != nil {
			continue
		}
		if raw != stock.GetAvailable() {
			metrics.SeckillReconcileMismatch.WithLabelValues("redis_db").Inc()
			s.log.Error("seckill redis/db stock mismatch", "activity_id", id, "redis", raw, "db", stock.GetAvailable())
		}
	}
}
