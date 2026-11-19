package ai

import (
	"context"
	"encoding/json"
	"sync"
)

// userIDKey carries the authenticated user from the gRPC handler into tools.
// Tools must never take a user id from model-generated arguments.
type userIDKey struct{}

func withUserID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, userIDKey{}, id)
}

func userIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userIDKey{}).(int64)
	return id, ok && id > 0
}

type draftSink struct {
	mu   sync.Mutex
	data []byte
}

type draftSinkKey struct{}

func withDraftSink(ctx context.Context) (context.Context, *draftSink) {
	s := &draftSink{}
	return context.WithValue(ctx, draftSinkKey{}, s), s
}

func publishDraft(ctx context.Context, v any) {
	s, ok := ctx.Value(draftSinkKey{}).(*draftSink)
	if !ok || s == nil {
		return
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return
	}
	s.mu.Lock()
	s.data = raw
	s.mu.Unlock()
}

func (s *draftSink) take() []byte {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.data
	s.data = nil
	return out
}
