package ai

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/metrics"
)

type feedbackEntry struct {
	UserID        int64  `json:"user_id"`
	SessionID     string `json:"session_id"`
	Rating        int32  `json:"rating"`
	Comment       string `json:"comment,omitempty"`
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	PromptVersion string `json:"prompt_version"`
	CreatedAt     int64  `json:"created_at"`
}

// SubmitFeedback stores an explicit helpful/not-helpful rating. Identity comes
// from the request context; the body cannot choose another user.
func (s *Service) SubmitFeedback(ctx context.Context, req *pb.AIFeedbackRequest) (*pb.Empty, error) {
	uid := req.GetUserId()
	if uid <= 0 {
		return nil, apperr.Unauthenticated("user id is required")
	}
	if req.GetRating() != 1 && req.GetRating() != -1 {
		return nil, apperr.Invalid("rating must be 1 or -1")
	}
	if s.cache == nil {
		return nil, apperr.Unavailable("feedback storage is not configured", nil)
	}
	comment := strings.TrimSpace(req.GetComment())
	if len(comment) > 500 {
		return nil, apperr.Invalid("comment must be at most 500 characters")
	}
	entry := feedbackEntry{
		UserID: uid, SessionID: strings.TrimSpace(req.GetSessionId()),
		Rating: req.GetRating(), Comment: comment,
		Provider: s.cfg.Provider, Model: s.cfg.Model, PromptVersion: promptVersion,
		CreatedAt: time.Now().Unix(),
	}
	payload, err := json.Marshal(entry)
	if err != nil {
		return nil, apperr.Internal("encode feedback", err)
	}
	key := "ai:feedback:" + time.Now().UTC().Format("20060102")
	if err := s.cache.RPush(ctx, key, payload).Err(); err != nil {
		return nil, apperr.Unavailable("store feedback: "+err.Error(), err)
	}
	_ = s.cache.Expire(ctx, key, 90*24*time.Hour).Err()
	rating := "helpful"
	if req.GetRating() < 0 {
		rating = "not_helpful"
	}
	metrics.AIFeedback.WithLabelValues(rating).Inc()
	return &pb.Empty{}, nil
}
