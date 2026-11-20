package ai

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRenewRAGLockExtendsOwnedLease(t *testing.T) {
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("set TEST_REDIS_ADDR")
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	defer client.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	key, owner := "rag-lease-test", "owner"
	_ = client.Del(ctx, key).Err()
	if err := client.Set(ctx, key, owner, 100*time.Millisecond).Err(); err != nil {
		t.Fatal(err)
	}
	go renewRAGLock(ctx, client, key, owner, 300*time.Millisecond, 50*time.Millisecond)
	time.Sleep(400 * time.Millisecond)
	ttl, err := client.PTTL(context.Background(), key).Result()
	cancel()
	if err != nil && err != redis.Nil {
		t.Fatal(err)
	}
	if ttl <= 0 {
		t.Fatalf("lease was not renewed, ttl=%v", ttl)
	}
	_ = client.Del(context.Background(), key).Err()
}
