package activecache

import (
	"sync"
	"time"
)

type entry struct {
	status  string
	version int64
	expires time.Time
}

type Cache struct {
	mu      sync.Mutex
	entries map[int64]entry
	ttl     time.Duration
}

func New(ttl time.Duration) *Cache {
	if ttl <= 0 {
		ttl = 5 * time.Second
	}
	return &Cache{entries: map[int64]entry{}, ttl: ttl}
}

// Lookup returns a cached status when it matches the token version. A cached
// entry with a different version means the token was issued before a status
// change and must be rejected.
func (c *Cache) Lookup(userID, tokenVersion int64) (string, bool) {
	if c == nil {
		return "", false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[userID]
	if !ok || time.Now().After(e.expires) {
		return "", false
	}
	if e.version != tokenVersion {
		return "", true
	}
	return e.status, true
}

func (c *Cache) Set(userID, tokenVersion int64, status string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[userID] = entry{status: status, version: tokenVersion, expires: time.Now().Add(c.ttl)}
}
