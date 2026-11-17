package ai

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// usageStore persists per-user and global token usage for daily budgets. A nil
// store disables budget checks.
type usageStore interface {
	Load(ctx context.Context, userID int64) (int64, error)
	Add(ctx context.Context, userID, prompt, completion, costCents int64) error
	LoadGlobal(ctx context.Context) (int64, error)
	AddGlobal(ctx context.Context, prompt, completion, costCents int64) error
}

type redisUsageStore struct {
	client redis.UniversalClient
	now    func() time.Time
}

func newUsageStore(client redis.UniversalClient) usageStore {
	if client == nil {
		return nil
	}
	return &redisUsageStore{client: client, now: time.Now}
}

func (s *redisUsageStore) day() string { return s.now().UTC().Format("20060102") }

func usageKey(userID int64, day string) string {
	return "ai:usage:" + day + ":" + itoa(userID)
}
func globalUsageKey(day string) string { return "ai:usage:" + day + ":global" }

func (s *redisUsageStore) Load(ctx context.Context, userID int64) (int64, error) {
	return s.load(ctx, usageKey(userID, s.day()))
}

func (s *redisUsageStore) LoadGlobal(ctx context.Context) (int64, error) {
	return s.load(ctx, globalUsageKey(s.day()))
}

func (s *redisUsageStore) load(ctx context.Context, key string) (int64, error) {
	n, err := s.client.HGet(ctx, key, "tokens").Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return n, err
}

func (s *redisUsageStore) Add(ctx context.Context, userID, prompt, completion, costCents int64) error {
	return s.add(ctx, usageKey(userID, s.day()), prompt, completion, costCents)
}

func (s *redisUsageStore) AddGlobal(ctx context.Context, prompt, completion, costCents int64) error {
	return s.add(ctx, globalUsageKey(s.day()), prompt, completion, costCents)
}

func (s *redisUsageStore) add(ctx context.Context, key string, prompt, completion, costCents int64) error {
	total := prompt + completion
	if total <= 0 && costCents <= 0 {
		return nil
	}
	pipe := s.client.TxPipeline()
	if total > 0 {
		pipe.HIncrBy(ctx, key, "tokens", total)
	}
	if costCents > 0 {
		pipe.HIncrBy(ctx, key, "cost_cents", costCents)
	}
	pipe.Expire(ctx, key, 48*time.Hour)
	_, err := pipe.Exec(ctx)
	return err
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
