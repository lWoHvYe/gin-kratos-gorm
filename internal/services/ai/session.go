package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/redis/go-redis/v9"
)

// sessionStore keeps a bounded conversation window per user and session.
type sessionStore interface {
	Load(ctx context.Context, userID int64, sessionID string) ([]*schema.Message, error)
	Append(ctx context.Context, userID int64, sessionID string, msgs ...*schema.Message) error
}

const (
	sessionTTL     = 30 * time.Minute
	sessionMaxMsgs = 20
)

type redisSessionStore struct {
	client redis.UniversalClient
}

func newSessionStore(client redis.UniversalClient) sessionStore {
	if client == nil {
		return memorySessionStore{}
	}
	return &redisSessionStore{client: client}
}

// sessionKey hashes the client-supplied id so a hostile value cannot grow keys
// or collide with other Redis users.
func sessionKey(userID int64, sessionID string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(sessionID)))
	return fmt.Sprintf("ai:sess:%d:%s", userID, hex.EncodeToString(sum[:12]))
}

func (s *redisSessionStore) Load(ctx context.Context, userID int64, sessionID string) ([]*schema.Message, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("user id is required")
	}
	raw, err := s.client.LRange(ctx, sessionKey(userID, sessionID), 0, sessionMaxMsgs-1).Result()
	if err != nil {
		return nil, err
	}
	msgs := make([]*schema.Message, 0, len(raw))
	for _, item := range raw {
		role, content, ok := strings.Cut(item, "\x00")
		if !ok {
			continue
		}
		msgs = append(msgs, &schema.Message{Role: schema.RoleType(role), Content: content})
	}
	return msgs, nil
}

func (s *redisSessionStore) Append(ctx context.Context, userID int64, sessionID string, msgs ...*schema.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	key := sessionKey(userID, sessionID)
	values := make([]interface{}, 0, len(msgs))
	for _, m := range msgs {
		if m == nil {
			continue
		}
		values = append(values, string(m.Role)+"\x00"+m.Content)
	}
	if len(values) == 0 {
		return nil
	}
	pipe := s.client.TxPipeline()
	pipe.RPush(ctx, key, values...)
	pipe.LTrim(ctx, key, -sessionMaxMsgs, -1)
	pipe.Expire(ctx, key, sessionTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// memorySessionStore is the nil-Redis fallback for tests. It keeps the same
// bounded-window semantics without persistence.
type memorySessionStore struct{}

func (memorySessionStore) Load(context.Context, int64, string) ([]*schema.Message, error) {
	return nil, nil
}
func (memorySessionStore) Append(context.Context, int64, string, ...*schema.Message) error {
	return nil
}
