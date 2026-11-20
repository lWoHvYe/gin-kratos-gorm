package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
		"campaign_code": out.Campaign.Code, "rule_code": out.Rule.Code, "state": "INTERRUPTED",
		"draft_hash": draftOutputHash(&out),
	})
	if err := s.cache.Set(ctx, "ai:hitl:meta:"+checkpointID, meta, hitlTTL).Err(); err != nil {
		return nil, apperr.Internal("store hitl draft binding", err)
	}
	return &pb.CampaignDraftReply{CheckpointId: checkpointID, DraftJson: string(payload)}, nil
}

func draftOutputHash(out *campaignDraftOutput) string {
	return hashDraftBinding(out.Campaign, out.Rule)
}

func requestDraftHash(c *pb.Campaign, r *pb.PromotionRule) string {
	return hashDraftBinding(campaignDraft{
		Code: c.GetCode(), Name: c.GetName(), Type: c.GetType(), Status: c.GetStatus(),
		StartAtUnix: c.GetStartAtUnix(), EndAtUnix: c.GetEndAtUnix(), Priority: c.GetPriority(),
		BudgetCent: c.GetBudgetCent(), PerUserLimit: c.GetPerUserLimit(), StackMode: c.GetStackMode(),
	}, promotionRuleDraft{
		Code: r.GetCode(), Name: r.GetName(), Type: r.GetType(), Status: r.GetStatus(),
		Priority: r.GetPriority(), Exclusive: r.GetExclusive(), StackGroup: r.GetStackGroup(),
		ScopeJSON: r.GetScopeJson(), ConditionExpr: r.GetConditionExpr(), EffectJSON: r.GetEffectJson(),
		MaxDiscountCent: r.GetMaxDiscountCent(), BudgetCent: r.GetBudgetCent(),
		PerUserLimit: r.GetPerUserLimit(), AllowWithCoupon: r.GetAllowWithCoupon(),
		StartAtUnix: r.GetStartAtUnix(), EndAtUnix: r.GetEndAtUnix(),
	})
}

func hashDraftBinding(c campaignDraft, r promotionRuleDraft) string {
	raw, _ := json.Marshal(struct {
		Campaign campaignDraft      `json:"campaign"`
		Rule     promotionRuleDraft `json:"rule"`
	}{c, r})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (s *Service) validateDraftCheckpoint(ctx context.Context, uid int64, checkpointID, campaignCode, ruleCode string, draftHash string) error {
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
	raw, err := s.cache.Get(ctx, "ai:hitl:meta:"+checkpointID).Bytes()
	if errors.Is(err, redis.Nil) {
		return apperr.NotFound("reviewed draft binding not found or expired")
	}
	if err != nil {
		return apperr.Internal("read hitl draft binding", err)
	}
	var binding map[string]string
	if err := json.Unmarshal(raw, &binding); err != nil || binding["draft_hash"] == "" || binding["campaign_code"] == "" || binding["rule_code"] == "" {
		return apperr.Conflict("reviewed draft binding is invalid; restart the draft")
	}
	if campaignCode != binding["campaign_code"] || ruleCode != binding["rule_code"] || draftHash != binding["draft_hash"] {
		return apperr.Conflict("confirm payload does not match the reviewed draft")
	}
	if binding["state"] == "SAVED" {
		return nil
	}
	if exists, err := s.cache.Exists(ctx, "ai:hitl:cp:"+checkpointID).Result(); err != nil {
		return apperr.Internal("read hitl checkpoint", err)
	} else if exists == 0 {
		return apperr.NotFound("checkpoint not found or expired")
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
	if err := s.validateDraftCheckpoint(ctx, uid, checkpointID, req.GetCampaignCode(), req.GetRuleCode(), requestDraftHash(req.GetCampaign(), req.GetRule())); err != nil {
		return nil, err
	}
	if err := s.transitionDraftState(ctx, checkpointID, "INTERRUPTED,FAILED", "CONFIRMING", ""); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

// hitlStateTransitionCAS moves the audit state in one Redis script. A plain
// read/modify/write lets two concurrent confirms lose an update, resurrect a
// SAVED checkpoint, or overwrite a FAILED reason; the script compares the
// current state and writes atomically. Return codes:
//
//	1  transitioned (or already in the target state)
//	0  already SAVED; terminal no-op
//	-1 checkpoint meta is gone
//	-2 meta is corrupt
//	-3 current state is not an allowed source
var hitlStateTransitionCAS = redis.NewScript(`
local raw = redis.call('get', KEYS[1])
if not raw then return -1 end
local decoded, meta = pcall(cjson.decode, raw)
if not decoded or type(meta) ~= 'table' then return -2 end
local from = {}
for state in string.gmatch(ARGV[1], '[^,]+') do from[state] = true end
local to = ARGV[2]
local current = meta['state'] or ''
if current == 'SAVED' then return 0 end
if current == to then return 1 end
if not from[current] then return -3 end
meta['state'] = to
if to == 'FAILED' then
  meta['failure_reason'] = ARGV[3]
else
  meta['failure_reason'] = ''
end
redis.call('set', KEYS[1], cjson.encode(meta), 'keepttl')
return 1
`)

func (s *Service) transitionDraftState(ctx context.Context, checkpointID, from, to, reason string) error {
	result, err := hitlStateTransitionCAS.Run(ctx, s.cache,
		[]string{"ai:hitl:meta:" + checkpointID}, from, to, reason).Int64()
	if err != nil {
		return apperr.Internal("transition hitl state", err)
	}
	switch result {
	case 0, 1:
		return nil
	case -1:
		return apperr.NotFound("reviewed draft binding not found or expired")
	case -2:
		return apperr.Conflict("reviewed draft binding is invalid; restart the draft")
	default:
		return apperr.Conflict("checkpoint is not awaiting confirmation")
	}
}

func (s *Service) readDraftState(ctx context.Context, checkpointID string) (map[string]string, error) {
	raw, err := s.cache.Get(ctx, "ai:hitl:meta:"+checkpointID).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, apperr.NotFound("reviewed draft binding not found or expired")
	}
	if err != nil {
		return nil, apperr.Internal("read hitl state", err)
	}
	var meta map[string]string
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, apperr.Conflict("reviewed draft binding is invalid; restart the draft")
	}
	return meta, nil
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
	if err := s.validateDraftCheckpoint(ctx, uid, checkpointID, req.GetCampaignCode(), req.GetRuleCode(), requestDraftHash(req.GetCampaign(), req.GetRule())); err != nil {
		return nil, err
	}
	// A repeated confirm after SAVED is a no-op; the BFF returns the existing
	// campaign/rule via its code lookup.
	meta, err := s.readDraftState(ctx, checkpointID)
	if err != nil {
		return nil, err
	}
	if meta["state"] == "SAVED" {
		return &pb.Empty{}, s.closeDraftCheckpoint(ctx, checkpointID)
	}
	// CAS guard: FAILED must go through Check (FAILED -> CONFIRMING) before a
	// retry can resume. Duplicate concurrent confirms that both observe
	// CONFIRMING are allowed and converge on SAVED.
	if err := s.transitionDraftState(ctx, checkpointID, "CONFIRMING", "CONFIRMING", ""); err != nil {
		return nil, err
	}
	store := &redisCheckpointStore{cache: s.cache}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent: &draftReviewAgent{}, CheckPointStore: store,
	})
	iter, err := runner.Resume(ctx, checkpointID)
	if err == nil {
		for {
			event, ok := iter.Next()
			if !ok {
				break
			}
			if event.Err != nil {
				err = event.Err
				break
			}
		}
	}
	if err != nil {
		// A concurrent confirm may have closed the checkpoint first; if the
		// state is already SAVED the business write is committed, so report
		// success instead of a misleading resume failure.
		if current, readErr := s.readDraftState(ctx, checkpointID); readErr == nil && current["state"] == "SAVED" {
			return &pb.Empty{}, s.closeDraftCheckpoint(ctx, checkpointID)
		}
		return nil, apperr.Internal("resume campaign draft", err)
	}
	// CAS CONFIRMING -> SAVED. A concurrent request that already saved returns
	// the terminal no-op (0); either way the checkpoint is then closed.
	if err := s.transitionDraftState(ctx, checkpointID, "CONFIRMING", "SAVED", ""); err != nil {
		return nil, err
	}
	return &pb.Empty{}, s.closeDraftCheckpoint(ctx, checkpointID)
}

func (s *Service) closeDraftCheckpoint(ctx context.Context, checkpointID string) error {
	if err := s.cache.Del(ctx, "ai:hitl:cp:"+checkpointID).Err(); err != nil {
		return apperr.Internal("close hitl checkpoint", err)
	}
	return nil
}

// AbortCampaignDraft records a failed confirmation without deleting the
// review checkpoint. A later Check can move FAILED -> CONFIRMING and retry.
func (s *Service) AbortCampaignDraft(ctx context.Context, req *pb.AbortCampaignDraftRequest) (*pb.Empty, error) {
	if s.cache == nil {
		return nil, apperr.Unavailable("hitl checkpoint storage is not configured", nil)
	}
	uid, checkpointID := req.GetUserId(), strings.TrimSpace(req.GetCheckpointId())
	if uid <= 0 || checkpointID == "" {
		return nil, apperr.Invalid("user_id and checkpoint_id are required")
	}
	owner, err := s.cache.Get(ctx, "ai:hitl:owner:"+checkpointID).Int64()
	if errors.Is(err, redis.Nil) {
		return nil, apperr.NotFound("checkpoint not found or expired")
	}
	if err != nil {
		return nil, apperr.Internal("read hitl owner", err)
	}
	if owner != uid {
		return nil, apperr.Forbidden("checkpoint belongs to another admin")
	}
	// CAS to FAILED keeps the reason atomic with the state change and refuses
	// to regress a checkpoint that a concurrent confirm already saved.
	if err := s.transitionDraftState(ctx, checkpointID, "INTERRUPTED,CONFIRMING", "FAILED", strings.TrimSpace(req.GetReason())); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}
