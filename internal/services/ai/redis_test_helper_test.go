package ai

import (
	goredis "github.com/redis/go-redis/v9"
)

func newTestRedisClient(addr string) *goredis.Client {
	return goredis.NewClient(&goredis.Options{Addr: addr})
}
