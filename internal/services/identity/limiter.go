package identity

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// loginLimiter throttles login attempts. Implementations must fail open: a
// broken limiter must not lock users out of the mall.
type loginLimiter interface {
	Allow(ctx context.Context, username string) (bool, error)
	Reset(ctx context.Context, username string)
}

// counter is the tiny Redis surface the limiter needs; tests use a fake.
type counter interface {
	Incr(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
	Del(ctx context.Context, key string) error
}

type redisCounter struct{ client redis.UniversalClient }

func (r redisCounter) Incr(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, key).Result()
}
func (r redisCounter) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return r.client.Expire(ctx, key, ttl).Err()
}
func (r redisCounter) Del(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

type redisLoginLimiter struct {
	counter counter
	limit   int64
	window  time.Duration
}

// NewLoginLimiter allows loginLimit attempts per loginWindow per username.
// A nil/empty Redis client yields a limiter that never blocks (dev/tests).
func NewLoginLimiter(client redis.UniversalClient) loginLimiter {
	if client == nil {
		return noopLimiter{}
	}
	return &redisLoginLimiter{counter: redisCounter{client: client}, limit: 10, window: 5 * time.Minute}
}

func (l *redisLoginLimiter) key(username string) string {
	return "login:rl:" + strings.ToLower(strings.TrimSpace(username))
}

func (l *redisLoginLimiter) Allow(ctx context.Context, username string) (bool, error) {
	if username == "" {
		return true, nil
	}
	key := l.key(username)
	n, err := l.counter.Incr(ctx, key)
	if err != nil {
		return true, err
	}
	if n == 1 {
		if err := l.counter.Expire(ctx, key, l.window); err != nil {
			return true, err
		}
	}
	return n <= l.limit, nil
}

func (l *redisLoginLimiter) Reset(ctx context.Context, username string) {
	_ = l.counter.Del(ctx, l.key(username))
}

type noopLimiter struct{}

func (noopLimiter) Allow(context.Context, string) (bool, error) { return true, nil }
func (noopLimiter) Reset(context.Context, string)               {}
