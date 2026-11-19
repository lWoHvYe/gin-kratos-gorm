package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"

	"github.com/redis/go-redis/v9"
)

const cartDraftTTL = 10 * time.Minute

type CartDraftWriter interface {
	Save(ctx context.Context, draftID string, userID int64, draft any) error
	PopRecent(ctx context.Context, userID int64) ([]byte, bool)
}

type redisCartDraftWriter struct{ cache redis.UniversalClient }

func NewCartDraftWriter(cache redis.UniversalClient) CartDraftWriter {
	return &redisCartDraftWriter{cache: cache}
}

type storedCartDraft struct {
	UserID int64           `json:"user_id"`
	Draft  json.RawMessage `json:"draft"`
}

func (w *redisCartDraftWriter) Save(ctx context.Context, draftID string, userID int64, draft any) error {
	raw, err := json.Marshal(draft)
	if err != nil {
		return err
	}
	body, err := json.Marshal(storedCartDraft{UserID: userID, Draft: raw})
	if err != nil {
		return err
	}
	if err := w.cache.Set(ctx, "ai:cart:draft:"+draftID, body, cartDraftTTL).Err(); err != nil {
		return err
	}
	recent := "ai:cart:recent:" + fmt.Sprintf("%d", userID)
	_ = w.cache.LPush(ctx, recent, raw).Err()
	_ = w.cache.LTrim(ctx, recent, 0, 4).Err()
	_ = w.cache.Expire(ctx, recent, cartDraftTTL).Err()
	return nil
}

func (w *redisCartDraftWriter) PopRecent(ctx context.Context, userID int64) ([]byte, bool) {
	raw, err := w.cache.LPop(ctx, "ai:cart:recent:"+fmt.Sprintf("%d", userID)).Bytes()
	if err != nil {
		return nil, false
	}
	return raw, true
}

func (s *Service) GetCartDraft(ctx context.Context, req *pb.GetCartDraftRequest) (*pb.CartDraftReply, error) {
	if req.GetUserId() <= 0 || req.GetDraftId() == "" {
		return nil, apperr.Invalid("draft_id and user_id are required")
	}
	key := "ai:cart:draft:" + req.GetDraftId()
	if done, _ := s.cache.Exists(ctx, "ai:cart:done:"+req.GetDraftId()).Result(); done > 0 {
		return &pb.CartDraftReply{Confirmed: true}, nil
	}
	raw, err := s.cache.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, apperr.NotFound("cart draft not found or expired")
		}
		return nil, apperr.Internal("load cart draft", err)
	}
	var stored storedCartDraft
	if err := json.Unmarshal(raw, &stored); err != nil || stored.UserID != req.GetUserId() {
		return nil, apperr.NotFound("cart draft not found")
	}
	confirmed, _ := s.cache.Exists(ctx, "ai:cart:done:"+req.GetDraftId()).Result()
	return &pb.CartDraftReply{DraftJson: string(stored.Draft), Confirmed: confirmed > 0}, nil
}

func (s *Service) CompleteCartDraft(ctx context.Context, req *pb.CompleteCartDraftRequest) (*pb.Empty, error) {
	if req.GetUserId() <= 0 || req.GetDraftId() == "" {
		return nil, apperr.Invalid("draft_id and user_id are required")
	}
	key := "ai:cart:draft:" + req.GetDraftId()
	if done, _ := s.cache.Exists(ctx, "ai:cart:done:"+req.GetDraftId()).Result(); done > 0 {
		return &pb.Empty{}, nil
	}
	raw, err := s.cache.Get(ctx, key).Bytes()
	if err != nil {
		return nil, apperr.NotFound("cart draft not found or expired")
	}
	var stored storedCartDraft
	if err := json.Unmarshal(raw, &stored); err != nil || stored.UserID != req.GetUserId() {
		return nil, apperr.NotFound("cart draft not found")
	}
	if err := s.cache.Set(ctx, "ai:cart:done:"+req.GetDraftId(), "1", cartDraftTTL).Err(); err != nil {
		return nil, apperr.Internal("complete cart draft", err)
	}
	_ = s.cache.Del(ctx, key).Err()
	return &pb.Empty{}, nil
}
