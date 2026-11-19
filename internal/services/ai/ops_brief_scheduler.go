package ai

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// RunOpsBriefScheduler generates the deterministic daily brief once per day.
// Delivery is outside business transactions; a failure only skips the brief and
// the date key is released for a retry.
func RunOpsBriefScheduler(ctx context.Context, cache redis.UniversalClient, svc *Service, logger *slog.Logger, interval time.Duration) {
	if interval <= 0 {
		interval = time.Hour
	}
	run := func() {
		day := time.Now().Format("2006-01-02")
		key := "ai:ops:brief:" + day
		ok, err := cache.SetNX(ctx, key, "1", 48*time.Hour).Result()
		if err != nil || !ok {
			return
		}
		brief, err := svc.OpsBrief(ctx, nil)
		if err != nil {
			_ = cache.Del(ctx, key).Err()
			logger.Warn("ops brief generation failed", "error", err)
			return
		}
		raw, _ := json.Marshal(brief)
		_ = cache.Set(ctx, "ai:ops:latest", raw, 48*time.Hour).Err()
		logger.Info("ops brief generated", "day", day, "orders", brief.GetTotalOrders(), "low_stock", brief.GetLowStock())
	}
	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
