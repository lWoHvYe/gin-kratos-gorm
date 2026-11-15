package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// limiter throttles assistant turns per user. The AI surface fails closed: if
// the limiter cannot be reached the assistant is unavailable, while the rest of
// the mall keeps working.
type limiter interface {
	Allow(ctx context.Context, userID int64) (bool, error)
}

type redisLimiter struct {
	client redis.UniversalClient
	limit  int64
	window time.Duration
}

func newLimiter(client redis.UniversalClient, limit int, window time.Duration) limiter {
	if limit <= 0 {
		limit = 20
	}
	if window <= 0 {
		window = 10 * time.Minute
	}
	if client == nil {
		return noopLimiter{}
	}
	return &redisLimiter{client: client, limit: int64(limit), window: window}
}

func (l *redisLimiter) Allow(ctx context.Context, userID int64) (bool, error) {
	if userID <= 0 {
		return false, fmt.Errorf("user id is required")
	}
	key := fmt.Sprintf("ai:rl:%d", userID)
	n, err := l.client.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if n == 1 {
		if err := l.client.Expire(ctx, key, l.window).Err(); err != nil {
			return false, err
		}
	}
	return n <= l.limit, nil
}

type noopLimiter struct{}

func (noopLimiter) Allow(context.Context, int64) (bool, error) { return true, nil }
