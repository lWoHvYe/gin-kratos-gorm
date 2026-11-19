package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/id"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/redis/go-redis/v9"
)

const hitlTTL = 30 * time.Minute

// redisCheckpointStore persists ADK interrupt state. The agent never holds
// approval state in memory, so a pod restart between start and confirm is safe.
type redisCheckpointStore struct {
	cache redis.UniversalClient
}

func (s *redisCheckpointStore) Get(ctx context.Context, checkpointID string) ([]byte, bool, error) {
	data, err := s.cache.Get(ctx, "ai:hitl:cp:"+checkpointID).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

func (s *redisCheckpointStore) Set(ctx context.Context, checkpointID string, data []byte) error {
	return s.cache.Set(ctx, "ai:hitl:cp:"+checkpointID, data, hitlTTL).Err()
}

// draftReviewAgent is a deterministic HITL agent: it interrupts with the draft
// on the first run and only finishes when the admin resumes the checkpoint.
// Phase 4 replaces this bounded agent with a ChatModelAgent, not the other way.
type draftReviewAgent struct {
	payload []byte
}

func (a *draftReviewAgent) Name(context.Context) string { return "campaign_draft_review" }
func (a *draftReviewAgent) Description(context.Context) string {
	return "Admin review checkpoint for a campaign draft"
}

func (a *draftReviewAgent) Run(ctx context.Context, _ *adk.AgentInput, _ ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	gen.Send(adk.Interrupt(ctx, a.payload))
	gen.Close()
	return iter
}

func (a *draftReviewAgent) Resume(ctx context.Context, _ *adk.ResumeInfo, _ ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	gen.Send(&adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{Message: schema.AssistantMessage("draft approved", nil)},
		},
	})
	gen.Close()
	return iter
}

// StartCampaignDraft builds a draft, runs it through ADK until the interrupt,
// and returns a checkpoint the admin can confirm later. No database write.
func (s *Service) StartCampaignDraft(ctx context.Context, req *pb.StartCampaignDraftRequest) (*pb.CampaignDraftReply, error) {
	if !s.cfg.Enabled {
		return nil, apperr.Unavailable("ai assistant is disabled", nil)
	}
	if s.cache == nil {
		return nil, apperr.Unavailable("hitl checkpoint storage is not configured", nil)
	}
	uid := req.GetUserId()
	if uid <= 0 {
		return nil, apperr.Unauthenticated("user id is required")
	}
	out, err := buildCampaignDraft(campaignDraftInput{
		Name: req.GetName(), Type: req.GetType(),
		ThresholdCent: req.GetThresholdCent(), DiscountCent: req.GetDiscountCent(),
		Percent: req.GetPercent(), Days: req.GetDays(),
		BudgetCent: req.GetBudgetCent(), PerUserLimit: req.GetPerUserLimit(),
	})
	if err != nil {
		return nil, apperr.Invalid("draft: %v", err)
	}
	payload, err := json.Marshal(out)
	if err != nil {
		return nil, apperr.Internal("encode draft", err)
	}
	checkpointID := fmt.Sprintf("%d-%s", uid, id.Event())
	store := &redisCheckpointStore{cache: s.cache}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent: &draftReviewAgent{payload: payload}, CheckPointStore: store,
	})
	iter := runner.Run(ctx, []adk.Message{schema.UserMessage("review campaign draft")}, adk.WithCheckPointID(checkpointID))
	interrupted := false
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return nil, apperr.Internal("start campaign draft", event.Err)
		}
		if event.Action != nil && event.Action.Interrupted != nil {
			interrupted = true
			break
		}
	}
	if !interrupted {
		return nil, apperr.Internal("campaign draft did not interrupt", fmt.Errorf("no interrupt event"))
	}
	if err := s.cache.Set(ctx, "ai:hitl:owner:"+checkpointID, uid, hitlTTL).Err(); err != nil {
		return nil, apperr.Internal("store hitl owner", err)
	}
	meta, _ := json.Marshal(map[string]string{
		"campaign_code": out.Campaign.Code, "rule_code": out.Rule.Code,
	})
	if err := s.cache.Set(ctx, "ai:hitl:meta:"+checkpointID, meta, hitlTTL).Err(); err != nil {
		return nil, apperr.Internal("store hitl draft binding", err)
	}
	return &pb.CampaignDraftReply{CheckpointId: checkpointID, DraftJson: string(payload)}, nil
}

func (s *Service) validateDraftCheckpoint(ctx context.Context, uid int64, checkpointID, campaignCode, ruleCode string) error {
	if !strings.HasPrefix(checkpointID, strconv.FormatInt(uid, 10)+"-") {
		return apperr.Forbidden("checkpoint belongs to another admin")
	}
	owner, err := s.cache.Get(ctx, "ai:hitl:owner:"+checkpointID).Int64()
	if errors.Is(err, redis.Nil) {
		return apperr.NotFound("checkpoint not found or expired")
	}
	if err != nil {
		return apperr.Internal("read hitl owner", err)
	}
	if owner != uid {
		return apperr.Forbidden("checkpoint belongs to another admin")
	}
	if exists, err := s.cache.Exists(ctx, "ai:hitl:cp:"+checkpointID).Result(); err != nil {
		return apperr.Internal("read hitl checkpoint", err)
	} else if exists == 0 {
		return apperr.NotFound("checkpoint not found or expired")
	}
	var binding map[string]string
	if raw, err := s.cache.Get(ctx, "ai:hitl:meta:"+checkpointID).Bytes(); err == nil {
		_ = json.Unmarshal(raw, &binding)
	}
	if binding != nil && (campaignCode != binding["campaign_code"] || ruleCode != binding["rule_code"]) {
		return apperr.Conflict("confirm payload does not match the reviewed draft")
	}
	return nil
}

// CheckCampaignDraft validates binding without consuming the checkpoint; the
// BFF calls it before its business write and closes the checkpoint afterwards.
func (s *Service) CheckCampaignDraft(ctx context.Context, req *pb.ResumeCampaignDraftRequest) (*pb.Empty, error) {
	if s.cache == nil {
		return nil, apperr.Unavailable("hitl checkpoint storage is not configured", nil)
	}
	uid := req.GetUserId()
	checkpointID := strings.TrimSpace(req.GetCheckpointId())
	if uid <= 0 || checkpointID == "" {
		return nil, apperr.Invalid("user_id and checkpoint_id are required")
	}
	if err := s.validateDraftCheckpoint(ctx, uid, checkpointID, req.GetCampaignCode(), req.GetRuleCode()); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

// ResumeCampaignDraft closes a checkpoint after the BFF saved the draft. It
// binds the checkpoint to the admin who started it and is one-shot.
func (s *Service) ResumeCampaignDraft(ctx context.Context, req *pb.ResumeCampaignDraftRequest) (*pb.Empty, error) {
	if s.cache == nil {
		return nil, apperr.Unavailable("hitl checkpoint storage is not configured", nil)
	}
	uid := req.GetUserId()
	checkpointID := strings.TrimSpace(req.GetCheckpointId())
	if uid <= 0 || checkpointID == "" {
		return nil, apperr.Invalid("user_id and checkpoint_id are required")
	}
	if err := s.validateDraftCheckpoint(ctx, uid, checkpointID, req.GetCampaignCode(), req.GetRuleCode()); err != nil {
		return nil, err
	}
	store := &redisCheckpointStore{cache: s.cache}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent: &draftReviewAgent{}, CheckPointStore: store,
	})
	iter, err := runner.Resume(ctx, checkpointID)
	if err != nil {
		return nil, apperr.Internal("resume campaign draft", err)
	}
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return nil, apperr.Internal("resume campaign draft", event.Err)
		}
	}
	if err := s.cache.Del(ctx, "ai:hitl:cp:"+checkpointID, "ai:hitl:owner:"+checkpointID, "ai:hitl:meta:"+checkpointID).Err(); err != nil {
		s.log.Warn("delete hitl checkpoint", "checkpoint", checkpointID, "error", err)
	}
	return &pb.Empty{}, nil
}
