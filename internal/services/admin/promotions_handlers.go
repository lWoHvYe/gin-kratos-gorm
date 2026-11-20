package admin

import (
	"context"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/bff"
	"gin-kratos-gorm/internal/platform/httpx"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ---------------------------------------------------------------- promotion admin

func (h *handlers) listCampaigns(c *gin.Context) {
	reply, err := h.clients.Promotion.ListCampaigns(c.Request.Context(), &pb.Empty{})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}

func (h *handlers) createCampaign(c *gin.Context) {
	var campaign pb.Campaign
	if err := bff.BindJSON(c, &campaign); err != nil {
		httpx.Error(c, err)
		return
	}
	created, err := h.clients.Promotion.CreateCampaign(c.Request.Context(), &campaign)
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.Created(c, created)
}

func (h *handlers) updateCampaign(c *gin.Context) {
	id, err := bff.PathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var campaign pb.Campaign
	if err := bff.BindJSON(c, &campaign); err != nil {
		httpx.Error(c, err)
		return
	}
	campaign.Id = id
	updated, err := h.clients.Promotion.UpdateCampaign(c.Request.Context(), &campaign)
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, updated)
}

type promotionStatusRequest struct {
	Status string `json:"status"`
}

func (h *handlers) setCampaignStatus(c *gin.Context) {
	id, err := bff.PathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var in promotionStatusRequest
	if err := bff.BindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	if _, err := h.clients.Promotion.SetCampaignStatus(c.Request.Context(), &pb.SetPromotionStatusRequest{Id: id, Status: in.Status}); err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, gin.H{"status": in.Status})
}

func (h *handlers) listPromotions(c *gin.Context) {
	page, err := bff.QueryInt32(c, "page")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	pageSize, err := bff.QueryInt32(c, "page_size")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	campaignID, err := bff.QueryInt64(c, "campaign_id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	reply, err := h.clients.Promotion.ListPromotions(c.Request.Context(), &pb.PromotionListRequest{
		Page: page, PageSize: pageSize, CampaignId: campaignID, Status: strings.TrimSpace(c.Query("status")),
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}

func (h *handlers) createPromotion(c *gin.Context) {
	var rule pb.PromotionRule
	if err := bff.BindJSON(c, &rule); err != nil {
		httpx.Error(c, err)
		return
	}
	created, err := h.clients.Promotion.CreatePromotion(c.Request.Context(), &rule)
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.Created(c, created)
}

func (h *handlers) updatePromotion(c *gin.Context) {
	id, err := bff.PathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var rule pb.PromotionRule
	if err := bff.BindJSON(c, &rule); err != nil {
		httpx.Error(c, err)
		return
	}
	rule.Id = id
	updated, err := h.clients.Promotion.UpdatePromotion(c.Request.Context(), &rule)
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, updated)
}

func (h *handlers) setPromotionStatus(c *gin.Context) {
	id, err := bff.PathID(c, "id")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var in promotionStatusRequest
	if err := bff.BindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	if _, err := h.clients.Promotion.SetPromotionStatus(c.Request.Context(), &pb.SetPromotionStatusRequest{Id: id, Status: in.Status}); err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, gin.H{"status": in.Status})
}

func (h *handlers) listPromotionReservations(c *gin.Context) {
	page, err := bff.QueryInt32(c, "page")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	pageSize, err := bff.QueryInt32(c, "page_size")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	reply, err := h.clients.Promotion.ListPromotionReservations(c.Request.Context(), &pb.ListPromotionReservationsRequest{
		OrderNo: strings.TrimSpace(c.Query("order_no")), Page: page, PageSize: pageSize,
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}

// applyDraft is the human-confirmation path: the assistant may draft and
// preview, but only an admin request with confirm=true creates the campaign and
// rule. The campaign is created as DRAFT first; an invalid rule leaves a safe
// draft behind rather than an active promotion.
func (h *handlers) applyDraft(c *gin.Context) {
	var req struct {
		Campaign *pb.Campaign      `json:"campaign"`
		Rule     *pb.PromotionRule `json:"rule"`
		Confirm  bool              `json:"confirm"`
	}
	if err := bff.BindJSON(c, &req); err != nil {
		httpx.Error(c, err)
		return
	}
	if !req.Confirm {
		httpx.Error(c, apperr.Invalid("confirm=true is required to save a draft"))
		return
	}
	created, saved, err := h.saveDraft(c.Request.Context(), req.Campaign, req.Rule)
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, map[string]any{"campaign": created, "rule": saved})
}

// saveDraft is shared by the plain apply endpoint and the ADK-confirmed flow.
func (h *handlers) saveDraft(ctx context.Context, campaign *pb.Campaign, rule *pb.PromotionRule) (*pb.Campaign, *pb.PromotionRule, error) {
	if campaign == nil || rule == nil {
		return nil, nil, apperr.Invalid("campaign and rule are required")
	}
	if strings.TrimSpace(campaign.GetStatus()) == "" {
		campaign.Status = "DRAFT"
	}
	if strings.TrimSpace(rule.GetStatus()) == "" {
		rule.Status = "DRAFT"
	}
	reply, err := h.clients.Promotion.CreateCampaignWithRule(ctx, &pb.CreateCampaignWithRuleRequest{
		Campaign: campaign, Rule: rule,
	})
	if err != nil {
		// Duplicate confirm after a successful save is idempotent: the draft
		// codes are unique, so return the existing pair.
		if status.Code(err) == codes.AlreadyExists {
			existing, lookupErr := h.clients.Promotion.GetCampaignWithRuleByCode(ctx, &pb.GetCampaignWithRuleByCodeRequest{
				CampaignCode: campaign.GetCode(), RuleCode: rule.GetCode(),
			})
			if lookupErr == nil {
				return existing.GetCampaign(), existing.GetRule(), nil
			}
		}
		return nil, nil, err
	}
	return reply.GetCampaign(), reply.GetRule(), nil
}

func (h *handlers) evaluateDraft(c *gin.Context) {
	var req struct {
		Campaign   *pb.Campaign                 `json:"campaign"`
		Rule       *pb.PromotionRule            `json:"rule"`
		Evaluation *pb.PromotionEvaluateRequest `json:"evaluation"`
	}
	if err := bff.BindJSON(c, &req); err != nil {
		httpx.Error(c, err)
		return
	}
	reply, err := h.clients.Promotion.EvaluateDraft(c.Request.Context(), &pb.EvaluateDraftRequest{
		Evaluation: req.Evaluation, Campaign: req.Campaign, Rule: req.Rule,
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}

func (h *handlers) evaluatePromotions(c *gin.Context) {
	var req pb.PromotionEvaluateRequest
	if err := bff.BindJSON(c, &req); err != nil {
		httpx.Error(c, err)
		return
	}
	reply, err := h.clients.Promotion.EvaluatePromotions(c.Request.Context(), &req)
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}
