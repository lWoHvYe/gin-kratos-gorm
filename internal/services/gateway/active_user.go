package gateway

import (
	"github.com/gin-gonic/gin"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/auth"
	"gin-kratos-gorm/internal/platform/httpx"
)

// requireActive revalidates status/token_version at high-risk write entry
// points, so a disabled user's already-issued access token stops working.
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
	user, err := h.clients.Identity.GetUser(c.Request.Context(), &pb.GetUserRequest{UserId: int64(claims.UserID)})
	if err != nil {
		httpx.Error(c, apperr.Unauthenticated("account is not active"))
		c.Abort()
		return
	}
	if user.GetStatus() != "ACTIVE" || user.GetTokenVersion() != claims.TokenVersion {
		httpx.Error(c, apperr.Unauthenticated("account is not active"))
		c.Abort()
		return
	}
	c.Next()
}
