package admin

import (
	"github.com/gin-gonic/gin"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/auth"
	"gin-kratos-gorm/internal/platform/bff"
	"gin-kratos-gorm/internal/platform/httpx"
)

func adminUserID(c *gin.Context) (int64, error) {
	claims, ok := auth.FromContext(c.Request.Context())
	if !ok || claims.UserID == 0 {
		return 0, apperr.Unauthenticated("authentication is required")
	}
	return int64(claims.UserID), nil
}

func (h *handlers) startCampaignDraft(c *gin.Context) {
	if h.clients.AI == nil {
		httpx.Error(c, apperr.Unavailable("ai assistant is not configured", nil))
		return
	}
	uid, err := adminUserID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var in pb.StartCampaignDraftRequest
	if err := bff.BindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	in.UserId = uid
	reply, err := h.clients.AI.StartCampaignDraft(c.Request.Context(), &in)
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, gin.H{"checkpoint_id": reply.GetCheckpointId(), "draft_json": reply.GetDraftJson()})
}

// confirmCampaignDraft closes the ADK checkpoint first, then saves through the
// existing DRAFT campaign/rule path. The model never writes to the database.
func (h *handlers) confirmCampaignDraft(c *gin.Context) {
	if h.clients.AI == nil {
		httpx.Error(c, apperr.Unavailable("ai assistant is not configured", nil))
		return
	}
	uid, err := adminUserID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	checkpoint := c.Param("checkpoint")
	var in struct {
		Campaign *pb.Campaign      `json:"campaign"`
		Rule     *pb.PromotionRule `json:"rule"`
		Confirm  bool              `json:"confirm"`
	}
	if err := bff.BindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	if !in.Confirm {
		httpx.Error(c, apperr.Invalid("confirm=true is required to save a draft"))
		return
	}
	if _, err := h.clients.AI.CheckCampaignDraft(c.Request.Context(), &pb.ResumeCampaignDraftRequest{
		UserId: uid, CheckpointId: checkpoint,
		CampaignCode: in.Campaign.GetCode(), RuleCode: in.Rule.GetCode(),
		Campaign: in.Campaign, Rule: in.Rule,
	}); err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	created, saved, err := h.saveDraft(c.Request.Context(), in.Campaign, in.Rule)
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	closed := true
	if _, err := h.clients.AI.ResumeCampaignDraft(c.Request.Context(), &pb.ResumeCampaignDraftRequest{
		UserId: uid, CheckpointId: checkpoint,
		CampaignCode: in.Campaign.GetCode(), RuleCode: in.Rule.GetCode(),
		Campaign: in.Campaign, Rule: in.Rule,
	}); err != nil {
		// The business write is committed; do not hide it. The checkpoint
		// remains open so an operator can inspect/close it separately.
		closed = false
	}
	httpx.OK(c, map[string]any{"campaign": created, "rule": saved, "checkpoint_closed": closed})
}
