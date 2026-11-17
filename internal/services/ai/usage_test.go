package ai

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestRedisUsageStore(t *testing.T) {
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("set TEST_REDIS_ADDR to run the usage store test")
	}
	client := newTestRedisClient(addr)
	defer client.Close()
	store := newUsageStore(client)
	userID := time.Now().UnixNano()

	ctx := context.Background()
	if err := store.Add(ctx, userID, 100, 20, 0); err != nil {
		t.Fatal(err)
	}
	used, err := store.Load(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if used != 120 {
		t.Fatalf("used = %d, want 120", used)
	}
	// Same-day increments accumulate; cleanup keeps the shared Redis tidy.
	if err := store.Add(ctx, userID, 5, 5, 0); err != nil {
		t.Fatal(err)
	}
	if used, _ = store.Load(ctx, userID); used != 130 {
		t.Fatalf("used = %d, want 130", used)
	}
	rs := store.(*redisUsageStore)
	_ = client.Del(ctx, usageKey(userID, rs.day())).Err()
}
