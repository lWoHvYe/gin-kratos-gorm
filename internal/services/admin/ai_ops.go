package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/bff"
	"gin-kratos-gorm/internal/platform/httpx"
)

func (h *handlers) opsBrief(c *gin.Context) {
	if h.clients.AI == nil {
		httpx.Error(c, apperr.Unavailable("ai assistant is not configured", nil))
		return
	}
	days := int32(1)
	if raw := c.Query("days"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			days = int32(n)
		}
	}
	reply, err := h.clients.AI.OpsBrief(c.Request.Context(), &pb.OpsBriefRequest{Days: days})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}

func (h *handlers) latestOpsBrief(c *gin.Context) {
	reply, err := h.clients.AI.LatestOpsBrief(c.Request.Context(), &pb.Empty{})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, reply)
}

type opsFeedbackBody struct {
	Day     string `json:"day"`
	Rating  int32  `json:"rating"`
	Comment string `json:"comment"`
}

func (h *handlers) opsFeedback(c *gin.Context) {
	uid, err := adminUserID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var body opsFeedbackBody
	if err := bff.BindJSON(c, &body); err != nil {
		httpx.Error(c, err)
		return
	}
	if _, err := h.clients.AI.OpsFeedback(c.Request.Context(), &pb.OpsFeedbackRequest{
		Day: body.Day, Rating: body.Rating, Comment: body.Comment, AdminUserId: uid,
	}); err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	httpx.OK(c, gin.H{"saved": true})
}
