package middleware

import (
	"sync"
	"time"

	"gin-kratos-gorm/internal/pkg/apperr"
	"gin-kratos-gorm/internal/pkg/httpx"

	"github.com/gin-gonic/gin"
)

// LoginRateLimit is a small in-process limiter intended to slow down credential
// stuffing. For multi-replica deployments use a Redis-backed limiter.
func LoginRateLimit(limit int, window time.Duration) gin.HandlerFunc {
	if limit <= 0 {
		limit = 10
	}
	if window <= 0 {
		window = time.Minute
	}
	var mu sync.Mutex
	buckets := make(map[string]*bucket)
	return func(c *gin.Context) {
		key := c.ClientIP()
		now := time.Now()
		mu.Lock()
		b, ok := buckets[key]
		if !ok || now.Sub(b.windowStart) > window {
			b = &bucket{windowStart: now}
			buckets[key] = b
		}
		b.count++
		count := b.count
		if len(buckets) > 10000 {
			buckets = make(map[string]*bucket)
		}
		mu.Unlock()
		if count > limit {
			c.Header("Retry-After", "60")
			httpx.Error(c, apperr.New(apperr.CodeResourceExhausted, "too many login attempts, try again later"))
			return
		}
		c.Next()
	}
}

type bucket struct {
	windowStart time.Time
	count       int
}
