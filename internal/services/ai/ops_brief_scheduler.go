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
		doneKey := "ai:ops:brief:" + day
		lockKey := "ai:ops:brief:lock:" + day
		if exists, _ := cache.Exists(ctx, doneKey).Result(); exists > 0 {
			return
		}
		// A crash while generating must not consume the day. The short-lived
		// lock is released by expiry; only a successful generation writes done.
		ok, err := cache.SetNX(ctx, lockKey, "1", 30*time.Minute).Result()
		if err != nil || !ok {
			return
		}
		defer cache.Del(ctx, lockKey)
		brief, err := svc.OpsBrief(ctx, nil)
		if err != nil {
			logger.Warn("ops brief generation failed", "error", err)
			return
		}
		raw, _ := json.Marshal(brief)
		pipe := cache.TxPipeline()
		pipe.Set(ctx, "ai:ops:latest", raw, 48*time.Hour)
		pipe.Set(ctx, doneKey, "1", 48*time.Hour)
		if _, err := pipe.Exec(ctx); err != nil {
			logger.Warn("publish ops brief failed", "day", day, "error", err)
			return
		}
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
