package identity

import (
	"context"
	"testing"
	"time"
)

type fakeCounter struct {
	value int64
	ttl   time.Duration
}

func (f *fakeCounter) Incr(context.Context, string) (int64, error) {
	f.value++
	return f.value, nil
}
func (f *fakeCounter) Expire(_ context.Context, _ string, ttl time.Duration) error {
	f.ttl = ttl
	return nil
}
func (f *fakeCounter) Del(context.Context, string) error {
	f.value = 0
	return nil
}

func TestLoginLimiterBlocksAfterLimitAndResets(t *testing.T) {
	counter := &fakeCounter{}
	limiter := &redisLoginLimiter{counter: counter, limit: 3, window: time.Minute}
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		ok, err := limiter.Allow(ctx, "demo")
		if err != nil || !ok {
			t.Fatalf("attempt %d: ok=%v err=%v", i+1, ok, err)
		}
	}
	if ok, _ := limiter.Allow(ctx, "demo"); ok {
		t.Fatal("fourth attempt should be limited")
	}
	if counter.ttl != time.Minute {
		t.Fatalf("window ttl = %s", counter.ttl)
	}
	limiter.Reset(ctx, "demo")
	if ok, _ := limiter.Allow(ctx, "demo"); !ok {
		t.Fatal("attempt after reset should pass")
	}
}

func TestNilLimiterNeverBlocks(t *testing.T) {
	limiter := NewLoginLimiter(nil)
	if ok, err := limiter.Allow(context.Background(), "demo"); err != nil || !ok {
		t.Fatalf("noop limiter = %v, %v", ok, err)
	}
}
