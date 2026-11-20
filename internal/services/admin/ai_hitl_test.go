package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeHitlAI records the state machine calls the BFF makes. It deliberately
// does not validate the draft hash; the AI service integration test covers CAS
// and binding.
type fakeHitlAI struct {
	pb.AIServiceClient
	state   string
	aborts  int
	checks  int
	resumes int
	reasons []string
}

func (f *fakeHitlAI) CheckCampaignDraft(context.Context, *pb.ResumeCampaignDraftRequest, ...grpc.CallOption) (*pb.Empty, error) {
	f.checks++
	if f.state == "INTERRUPTED" || f.state == "FAILED" || f.state == "CONFIRMING" {
		f.state = "CONFIRMING"
	}
	return &pb.Empty{}, nil
}

func (f *fakeHitlAI) ResumeCampaignDraft(context.Context, *pb.ResumeCampaignDraftRequest, ...grpc.CallOption) (*pb.Empty, error) {
	f.resumes++
	f.state = "SAVED"
	return &pb.Empty{}, nil
}

func (f *fakeHitlAI) AbortCampaignDraft(_ context.Context, in *pb.AbortCampaignDraftRequest, _ ...grpc.CallOption) (*pb.Empty, error) {
	f.aborts++
	f.state = "FAILED"
	f.reasons = append(f.reasons, in.GetReason())
	return &pb.Empty{}, nil
}

// fakePromotionClient fails the first save and then succeeds, so the BFF has to
// record FAILED and allow a retry.
type fakePromotionClient struct {
	pb.PromotionServiceClient
	calls int
}

func (f *fakePromotionClient) CreateCampaignWithRule(_ context.Context, in *pb.CreateCampaignWithRuleRequest, _ ...grpc.CallOption) (*pb.CampaignWithRule, error) {
	f.calls++
	if f.calls == 1 {
		return nil, status.Error(codes.Unavailable, "promotion write failed")
	}
	return &pb.CampaignWithRule{Campaign: in.GetCampaign(), Rule: in.GetRule()}, nil
}

func TestConfirmCampaignDraftAbortsWhenSaveFailsAndRetries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	manager := testManager(t)
	ai := &fakeHitlAI{state: "CONFIRMING"}
	promo := &fakePromotionClient{}
	engine := NewEngine(nil, manager, &Clients{AI: ai, Promotion: promo}, true)
	token, _, err := manager.Generate(1, []string{"admin"})
	if err != nil {
		t.Fatal(err)
	}
	body := `{"confirm":true,"campaign":{"code":"C-HITL","name":"活动","type":"THRESHOLD","status":"DRAFT"},"rule":{"code":"R-HITL","name":"规则","type":"THRESHOLD","status":"DRAFT"}}`
	post := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/admin/v1/ai/campaign-draft/cp-1/confirm", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		return rec
	}

	first := post()
	if first.Code != http.StatusServiceUnavailable {
		t.Fatalf("first confirm status = %d, body = %s", first.Code, first.Body.String())
	}
	if ai.state != "FAILED" || ai.aborts != 1 {
		t.Fatalf("save failure did not abort: state=%q aborts=%d", ai.state, ai.aborts)
	}
	if len(ai.reasons) != 1 || ai.reasons[0] != "draft save failed" {
		t.Fatalf("abort reason = %v", ai.reasons)
	}

	second := post()
	if second.Code != http.StatusOK {
		t.Fatalf("retry confirm status = %d, body = %s", second.Code, second.Body.String())
	}
	if ai.state != "SAVED" || ai.checks != 2 || ai.resumes != 1 || ai.aborts != 1 {
		t.Fatalf("retry call flow = state:%q checks:%d resumes:%d aborts:%d", ai.state, ai.checks, ai.resumes, ai.aborts)
	}
	if !strings.Contains(second.Body.String(), `"checkpoint_state":"SAVED"`) ||
		!strings.Contains(second.Body.String(), `"checkpoint_closed":true`) {
		t.Fatalf("retry body = %s", second.Body.String())
	}
}
