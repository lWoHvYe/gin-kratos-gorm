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
	// Keep the initial TTL comfortably above the observation window. The old
	// 100ms TTL made this test fail under CI/load when renewal was scheduled a
	// few milliseconds late; the assertion below still proves the TTL grew.
	if err := client.Set(ctx, key, owner, 10*time.Second).Err(); err != nil {
		t.Fatal(err)
	}
	go renewRAGLock(ctx, client, key, owner, 20*time.Second, 10*time.Millisecond)
	time.Sleep(200 * time.Millisecond)
	ttl, err := client.PTTL(context.Background(), key).Result()
	cancel()
	if err != nil && err != redis.Nil {
		t.Fatal(err)
	}
	if ttl <= 10*time.Second {
		t.Fatalf("lease was not renewed, ttl=%v", ttl)
	}
	_ = client.Del(context.Background(), key).Err()
}
