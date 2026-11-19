package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/httpx"
)

func (h *handlers) ragSearch(c *gin.Context) {
	if h.clients.AI == nil {
		httpx.Error(c, apperr.Unavailable("ai assistant is not configured", nil))
		return
	}
	limit := int32(5)
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = int32(n)
		}
	}
	reply, err := h.clients.AI.SearchKnowledge(c.Request.Context(), &pb.KnowledgeSearchRequest{
		Query: c.Query("q"), Limit: limit,
	})
	if err != nil {
		httpx.Error(c, apperr.Unavailable("search knowledge: "+err.Error(), err))
		return
	}
	httpx.OK(c, gin.H{"docs": reply.GetDocs()})
}
