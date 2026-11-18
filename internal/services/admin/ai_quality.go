package admin

import (
	"github.com/gin-gonic/gin"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/bff"
	"gin-kratos-gorm/internal/platform/httpx"
)

func (h *handlers) aiFeedbackSummary(c *gin.Context) {
	days, err := bff.QueryInt32(c, "days")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	if h.clients.AI == nil {
		httpx.Error(c, apperr.Unavailable("ai assistant is not configured", nil))
		return
	}
	reply, err := h.clients.AI.FeedbackSummary(c.Request.Context(), &pb.FeedbackSummaryRequest{Days: days})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}
