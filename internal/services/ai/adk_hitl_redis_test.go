package ai

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"

	"github.com/redis/go-redis/v9"
)

func newHitlTestService(t *testing.T) (*Service, *redis.Client) {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("set TEST_REDIS_ADDR to run the HITL state machine test")
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = client.Close() })
	svc := &Service{cfg: Config{Enabled: true}, cache: client, log: testLogger()}
	return svc, client
}

func draftRequestFromOutput(t *testing.T, uid int64, checkpointID, raw string) *pb.ResumeCampaignDraftRequest {
	t.Helper()
	var out campaignDraftOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("decode draft: %v", err)
	}
	c, r := out.Campaign, out.Rule
	return &pb.ResumeCampaignDraftRequest{
		UserId:       uid,
		CheckpointId: checkpointID,
		CampaignCode: c.Code,
		RuleCode:     r.Code,
		Campaign: &pb.Campaign{
			Code: c.Code, Name: c.Name, Type: c.Type, Status: c.Status,
			StartAtUnix: c.StartAtUnix, EndAtUnix: c.EndAtUnix, Priority: c.Priority,
			BudgetCent: c.BudgetCent, PerUserLimit: c.PerUserLimit, StackMode: c.StackMode,
		},
		Rule: &pb.PromotionRule{
			Code: r.Code, Name: r.Name, Type: r.Type, Status: r.Status,
			Priority: r.Priority, Exclusive: r.Exclusive, StackGroup: r.StackGroup,
			ScopeJson: r.ScopeJSON, ConditionExpr: r.ConditionExpr, EffectJson: r.EffectJSON,
			MaxDiscountCent: r.MaxDiscountCent, BudgetCent: r.BudgetCent,
			PerUserLimit: r.PerUserLimit, AllowWithCoupon: r.AllowWithCoupon,
			StartAtUnix: r.StartAtUnix, EndAtUnix: r.EndAtUnix,
		},
	}
}

func hitlMeta(t *testing.T, client *redis.Client, checkpointID string) map[string]string {
	t.Helper()
	raw, err := client.Get(context.Background(), "ai:hitl:meta:"+checkpointID).Bytes()
	if err != nil {
		t.Fatalf("read hitl meta: %v", err)
	}
	var meta map[string]string
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("decode hitl meta: %v", err)
	}
	return meta
}

func cleanHitl(t *testing.T, client *redis.Client, checkpointID string) {
	t.Helper()
	ctx := context.Background()
	_ = client.Del(ctx, "ai:hitl:meta:"+checkpointID, "ai:hitl:owner:"+checkpointID, "ai:hitl:cp:"+checkpointID).Err()
}

// TestHitlSaveFailureThenFailedThenRetry mirrors the BFF flow: Check
// (CONFIRMING) -> business save fails -> Abort (FAILED) -> retry Check
// (FAILED -> CONFIRMING) -> Resume (SAVED).
func TestHitlSaveFailureThenFailedThenRetry(t *testing.T) {
	svc, client := newHitlTestService(t)
	ctx := context.Background()
	const uid = int64(4242)
	started, err := svc.StartCampaignDraft(ctx, &pb.StartCampaignDraftRequest{
		UserId: uid, Name: "失败重试活动", Type: "THRESHOLD",
		ThresholdCent: 10000, DiscountCent: 1000, Days: 7,
	})
	if err != nil {
		t.Fatalf("start draft: %v", err)
	}
	checkpointID := started.GetCheckpointId()
	t.Cleanup(func() { cleanHitl(t, client, checkpointID) })
	req := draftRequestFromOutput(t, uid, checkpointID, started.GetDraftJson())

	if _, err := svc.CheckCampaignDraft(ctx, req); err != nil {
		t.Fatalf("first check: %v", err)
	}
	if got := hitlMeta(t, client, checkpointID)["state"]; got != "CONFIRMING" {
		t.Fatalf("state after check = %q", got)
	}
	// The BFF's saveDraft failed, so it records FAILED without closing the
	// checkpoint.
	if _, err := svc.AbortCampaignDraft(ctx, &pb.AbortCampaignDraftRequest{
		UserId: uid, CheckpointId: checkpointID, Reason: "draft save failed",
	}); err != nil {
		t.Fatalf("abort: %v", err)
	}
	meta := hitlMeta(t, client, checkpointID)
	if meta["state"] != "FAILED" || meta["failure_reason"] != "draft save failed" {
		t.Fatalf("state after abort = %#v", meta)
	}
	// Direct Resume from FAILED must be refused; the admin has to re-run Check.
	if _, err := svc.ResumeCampaignDraft(ctx, req); !apperr.IsCode(err, apperr.CodeConflict) {
		t.Fatalf("resume from FAILED = %v, want conflict", err)
	}
	// Retry: Check moves FAILED -> CONFIRMING, then the save succeeds and
	// Resume closes the checkpoint.
	if _, err := svc.CheckCampaignDraft(ctx, req); err != nil {
		t.Fatalf("retry check: %v", err)
	}
	if got := hitlMeta(t, client, checkpointID)["state"]; got != "CONFIRMING" {
		t.Fatalf("state after retry check = %q", got)
	}
	if _, err := svc.ResumeCampaignDraft(ctx, req); err != nil {
		t.Fatalf("retry resume: %v", err)
	}
	if got := hitlMeta(t, client, checkpointID)["state"]; got != "SAVED" {
		t.Fatalf("state after resume = %q", got)
	}
	if exists, _ := client.Exists(ctx, "ai:hitl:cp:"+checkpointID).Result(); exists != 0 {
		t.Fatal("checkpoint was not closed after save")
	}
	// Duplicate confirm after SAVED stays a no-op.
	if _, err := svc.ResumeCampaignDraft(ctx, req); err != nil {
		t.Fatalf("duplicate resume: %v", err)
	}
}

func TestHitlCASKeepsSavedTerminal(t *testing.T) {
	svc, client := newHitlTestService(t)
	ctx := context.Background()
	checkpointID := "cas-terminal-" + time.Now().Format("150405.000000000")
	t.Cleanup(func() { cleanHitl(t, client, checkpointID) })
	raw, _ := json.Marshal(map[string]string{
		"campaign_code": "C1", "rule_code": "R1", "state": "SAVED", "draft_hash": "h",
	})
	if err := client.Set(ctx, "ai:hitl:meta:"+checkpointID, raw, hitlTTL).Err(); err != nil {
		t.Fatal(err)
	}
	if err := svc.transitionDraftState(ctx, checkpointID, "INTERRUPTED,FAILED", "CONFIRMING", ""); err != nil {
		t.Fatalf("check on SAVED: %v", err)
	}
	if err := svc.transitionDraftState(ctx, checkpointID, "INTERRUPTED,CONFIRMING", "FAILED", "late failure"); err != nil {
		t.Fatalf("abort on SAVED: %v", err)
	}
	meta := hitlMeta(t, client, checkpointID)
	if meta["state"] != "SAVED" || meta["failure_reason"] != "" {
		t.Fatalf("SAVED regressed: %#v", meta)
	}
}

func TestHitlCASRejectsIllegalSource(t *testing.T) {
	svc, client := newHitlTestService(t)
	ctx := context.Background()
	checkpointID := "cas-illegal-" + time.Now().Format("150405.000000000")
	t.Cleanup(func() { cleanHitl(t, client, checkpointID) })
	raw, _ := json.Marshal(map[string]string{
		"campaign_code": "C1", "rule_code": "R1", "state": "FAILED", "draft_hash": "h", "failure_reason": "boom",
	})
	if err := client.Set(ctx, "ai:hitl:meta:"+checkpointID, raw, hitlTTL).Err(); err != nil {
		t.Fatal(err)
	}
	// Resume's CAS guard only accepts CONFIRMING, so FAILED is refused until
	// Check moves it forward.
	err := svc.transitionDraftState(ctx, checkpointID, "CONFIRMING", "CONFIRMING", "")
	if !apperr.IsCode(err, apperr.CodeConflict) {
		t.Fatalf("guard from FAILED = %v, want conflict", err)
	}
	if _, err := svc.AbortCampaignDraft(ctx, &pb.AbortCampaignDraftRequest{
		UserId: 0, CheckpointId: checkpointID,
	}); !apperr.IsCode(err, apperr.CodeInvalidArgument) {
		t.Fatalf("abort without user = %v", err)
	}
	// The failed reason survives an illegal transition attempt.
	if got := hitlMeta(t, client, checkpointID)["failure_reason"]; got != "boom" {
		t.Fatalf("failure reason changed to %q", got)
	}
}

func TestHitlCASMissingAndCorruptMeta(t *testing.T) {
	svc, client := newHitlTestService(t)
	ctx := context.Background()
	missing := "cas-missing-" + time.Now().Format("150405.000000000")
	t.Cleanup(func() { cleanHitl(t, client, missing) })
	if err := svc.transitionDraftState(ctx, missing, "INTERRUPTED", "CONFIRMING", ""); !apperr.IsCode(err, apperr.CodeNotFound) {
		t.Fatalf("missing meta = %v, want not found", err)
	}
	corrupt := "cas-corrupt-" + time.Now().Format("150405.000000000")
	t.Cleanup(func() { cleanHitl(t, client, corrupt) })
	if err := client.Set(ctx, "ai:hitl:meta:"+corrupt, "{not-json", hitlTTL).Err(); err != nil {
		t.Fatal(err)
	}
	if err := svc.transitionDraftState(ctx, corrupt, "INTERRUPTED", "CONFIRMING", ""); !apperr.IsCode(err, apperr.CodeConflict) {
		t.Fatalf("corrupt meta = %v, want conflict", err)
	}
	if _, err := svc.readDraftState(ctx, corrupt); !apperr.IsCode(err, apperr.CodeConflict) {
		t.Fatalf("read corrupt meta = %v, want conflict", err)
	}
	var ae *apperr.Error
	if err := svc.transitionDraftState(ctx, missing, "INTERRUPTED", "CONFIRMING", ""); !errors.As(err, &ae) {
		t.Fatalf("error is not apperr: %v", err)
	}
}
