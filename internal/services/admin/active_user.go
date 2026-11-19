package admin

import (
	"github.com/gin-gonic/gin"

	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/auth"
	"gin-kratos-gorm/internal/platform/httpx"

	pb "gin-kratos-gorm/api/gen/mall/v1"
)

func (h *handlers) requireActive(c *gin.Context) {
	if h.clients.Identity == nil {
		c.Next()
		return
	}
	claims, ok := auth.FromContext(c.Request.Context())
	if !ok || claims.UserID == 0 {
		httpx.Error(c, apperr.Unauthenticated("authentication is required"))
		c.Abort()
		return
	}
	if status, cached := h.active.Lookup(int64(claims.UserID), claims.TokenVersion); cached {
		if status == "ACTIVE" {
			c.Next()
			return
		}
		httpx.Error(c, apperr.Unauthenticated("admin account is not active"))
		c.Abort()
		return
	}
	user, err := h.clients.Identity.GetUser(c.Request.Context(), &pb.GetUserRequest{UserId: int64(claims.UserID)})
	if err != nil || user.GetStatus() != "ACTIVE" || user.GetTokenVersion() != claims.TokenVersion {
		httpx.Error(c, apperr.Unauthenticated("admin account is not active"))
		c.Abort()
		return
	}
	h.active.Set(int64(claims.UserID), claims.TokenVersion, user.GetStatus())
	c.Next()
}
