package seckill

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	platformredis "gin-kratos-gorm/internal/platform/redis"
)

// TestChaosAdmissionSurvivesRedisRestart is driven by scripts/chaos-redis.sh.
// Admission must fail while Redis is down, then recover on the same keys once
// it returns, without corrupting stock accounting.
func TestChaosAdmissionSurvivesRedisRestart(t *testing.T) {
	if os.Getenv("CHAOS_REDIS") == "" {
		t.Skip("set CHAOS_REDIS=1 (run scripts/chaos-redis.sh)")
	}
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Fatal("TEST_REDIS_ADDR is required")
	}
	cache, err := platformredis.Open(platformredis.Config{Mode: "standalone", Addrs: []string{addr}})
	if err != nil {
		t.Fatalf("connect redis: %v", err)
	}
	defer cache.Close()
	ctx := context.Background()

	activity := time.Now().UnixNano()
	keys := []string{
		fmt.Sprintf("seckill:{%d}:meta", activity),
		fmt.Sprintf("seckill:{%d}:stock", activity),
		fmt.Sprintf("seckill:{%d}:users", activity),
		fmt.Sprintf("seckill:{%d}:tokens", activity),
	}
	t.Cleanup(func() { _ = cache.Del(context.Background(), keys...).Err() })
	if err := cache.HSet(ctx, keys[0], "status", "ACTIVE").Err(); err != nil {
		t.Fatal(err)
	}
	if err := cache.Set(ctx, keys[1], 5, 0).Err(); err != nil {
		t.Fatal(err)
	}

	// The script stops Redis while we sleep here.
	time.Sleep(8 * time.Second)

	failures := 0
	accepted := false
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		got, err := reserveScript.Run(ctx, cache, keys, int64(1), "chaos-token", 60).Int64()
		if err != nil {
			failures++
		} else if got == reserveAccepted {
			accepted = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if failures == 0 {
		t.Fatal("expected admission failures while Redis was down")
	}
	if !accepted {
		t.Fatal("admission did not recover after Redis restarted")
	}
	if stock, err := cache.Get(ctx, keys[1]).Int64(); err != nil || stock != 4 {
		t.Fatalf("stock after recovery = %d, %v (want 4)", stock, err)
	}
}
