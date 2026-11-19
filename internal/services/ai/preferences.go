package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"

	"github.com/redis/go-redis/v9"
)

const preferenceKeyPrefix = "ai:prefs:"

func preferenceKey(userID int64) string { return fmt.Sprintf("%s%d", preferenceKeyPrefix, userID) }

func (s *Service) GetPreferences(ctx context.Context, req *pb.GetAIPreferencesRequest) (*pb.AIPreferences, error) {
	if req.GetUserId() <= 0 {
		return nil, apperr.Invalid("user_id is required")
	}
	prefs, err := s.loadPreferences(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	return prefs, nil
}

func (s *Service) SetPreferences(ctx context.Context, req *pb.SetAIPreferencesRequest) (*pb.AIPreferences, error) {
	if req.GetUserId() <= 0 {
		return nil, apperr.Invalid("user_id is required")
	}
	prefs := req.GetPreferences()
	if prefs == nil {
		prefs = &pb.AIPreferences{}
	}
	if len(prefs.GetPreferredCategories()) > 10 || len(prefs.GetPreferredBrands()) > 10 || len(prefs.GetSizes()) > 10 {
		return nil, apperr.Invalid("at most 10 categories, brands and sizes")
	}
	if prefs.GetBudgetMinCent() < 0 || prefs.GetBudgetMaxCent() < 0 ||
		(prefs.GetBudgetMaxCent() > 0 && prefs.GetBudgetMinCent() > prefs.GetBudgetMaxCent()) {
		return nil, apperr.Invalid("invalid budget range")
	}
	for _, v := range append(append(prefs.GetPreferredCategories(), prefs.GetPreferredBrands()...), prefs.GetSizes()...) {
		if len(strings.TrimSpace(v)) > 40 {
			return nil, apperr.Invalid("preference value is too long")
		}
	}
	prefs.UpdatedAt = time.Now().Unix()
	raw, err := json.Marshal(prefs)
	if err != nil {
		return nil, apperr.Internal("encode preferences", err)
	}
	if err := s.cache.Set(ctx, preferenceKey(req.GetUserId()), raw, 0).Err(); err != nil {
		return nil, apperr.Internal("save preferences", err)
	}
	return prefs, nil
}

func (s *Service) ClearPreferences(ctx context.Context, req *pb.ClearAIPreferencesRequest) (*pb.Empty, error) {
	if req.GetUserId() <= 0 {
		return nil, apperr.Invalid("user_id is required")
	}
	if err := s.cache.Del(ctx, preferenceKey(req.GetUserId())).Err(); err != nil {
		return nil, apperr.Internal("clear preferences", err)
	}
	return &pb.Empty{}, nil
}

func (s *Service) loadPreferences(ctx context.Context, userID int64) (*pb.AIPreferences, error) {
	if s.cache == nil {
		return &pb.AIPreferences{}, nil
	}
	raw, err := s.cache.Get(ctx, preferenceKey(userID)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return &pb.AIPreferences{}, nil
		}
		return nil, apperr.Internal("load preferences", err)
	}
	var prefs pb.AIPreferences
	if err := json.Unmarshal(raw, &prefs); err != nil {
		return &pb.AIPreferences{}, nil
	}
	return &prefs, nil
}

func preferencePrompt(prefs *pb.AIPreferences) string {
	if prefs == nil {
		return ""
	}
	var parts []string
	if len(prefs.GetPreferredCategories()) > 0 {
		parts = append(parts, "偏好品类: "+strings.Join(prefs.GetPreferredCategories(), "、"))
	}
	if len(prefs.GetPreferredBrands()) > 0 {
		parts = append(parts, "偏好品牌: "+strings.Join(prefs.GetPreferredBrands(), "、"))
	}
	if prefs.GetBudgetMinCent() > 0 || prefs.GetBudgetMaxCent() > 0 {
		parts = append(parts, fmt.Sprintf("预算范围: %d-%d 分", prefs.GetBudgetMinCent(), prefs.GetBudgetMaxCent()))
	}
	if len(prefs.GetSizes()) > 0 {
		parts = append(parts, "常用尺码: "+strings.Join(prefs.GetSizes(), "、"))
	}
	if len(parts) == 0 {
		return ""
	}
	return "以下是用户显式保存的购物偏好，仅用于本回答，不要推断敏感属性，也不要跨用户使用：" + strings.Join(parts, "；")
}
