package seckill

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	platformredis "gin-kratos-gorm/internal/platform/redis"
)

func TestReserveScriptIdempotencyAndCompensation(t *testing.T) {
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("set TEST_REDIS_ADDR to run the seckill Lua integration test")
	}
	cache, err := platformredis.Open(platformredis.Config{
		Mode: "standalone", Addrs: []string{addr}, Password: os.Getenv("TEST_REDIS_PASSWORD"),
	})
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
	t.Cleanup(func() { _ = cache.Del(ctx, keys...).Err() })

	if err := cache.HSet(ctx, keys[0], "status", "ACTIVE").Err(); err != nil {
		t.Fatal(err)
	}
	if err := cache.Set(ctx, keys[1], 2, 0).Err(); err != nil {
		t.Fatal(err)
	}

	reserve := func(user int64, token string) int64 {
		t.Helper()
		got, err := reserveScript.Run(ctx, cache, keys, user, token, 60).Int64()
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	if got := reserve(1, "token-1"); got != reserveAccepted {
		t.Fatalf("first reserve = %d, want accepted", got)
	}
	if got := reserve(1, "token-1"); got != reserveExisting {
		t.Fatalf("replay reserve = %d, want existing", got)
	}
	if stock, _ := cache.Get(ctx, keys[1]).Int64(); stock != 1 {
		t.Fatalf("stock = %d, want 1 after one admission", stock)
	}
	if got := reserve(2, "token-2"); got != reserveAccepted {
		t.Fatalf("second user reserve = %d", got)
	}
	if got := reserve(3, "token-3"); got != reserveSoldOut {
		t.Fatalf("sold-out reserve = %d", got)
	}

	// compensateScript takes users, tokens, stock, meta (KEYS[1..4]).
	compensateKeys := []string{keys[2], keys[3], keys[1], keys[0]}
	compensate := func(user int64, token string) int64 {
		t.Helper()
		got, err := compensateScript.Run(ctx, cache, compensateKeys, user, token).Int64()
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	if got := compensate(1, "token-1"); got != 1 {
		t.Fatalf("compensate = %d, want 1", got)
	}
	// Two admissions consumed both units; compensating one returns exactly one.
	if stock, _ := cache.Get(ctx, keys[1]).Int64(); stock != 1 {
		t.Fatalf("stock = %d, want 1 after compensation", stock)
	}
	if got := compensate(1, "token-1"); got != 0 {
		t.Fatalf("replayed compensate = %d, want 0", got)
	}
	if stock, _ := cache.Get(ctx, keys[1]).Int64(); stock != 1 {
		t.Fatalf("stock = %d after replayed compensation", stock)
	}
}
