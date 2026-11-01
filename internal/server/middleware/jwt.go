package middleware

import (
	"strings"

	"gin-kratos-gorm/internal/pkg/apperr"
	"gin-kratos-gorm/internal/pkg/auth"
	"gin-kratos-gorm/internal/pkg/httpx"

	"github.com/gin-gonic/gin"
)

// JWTAuth authenticates a bearer token and injects the claims into both the Gin
// context and the standard request context (used by services/handlers).
func JWTAuth(manager *auth.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if strings.TrimSpace(header) == "" {
			httpx.Error(c, apperr.Unauthenticated("Authorization header is required"))
			return
		}
		claims, err := manager.ParseBearer(header)
		if err != nil {
			httpx.Error(c, err)
			return
		}
		ctx := auth.WithClaims(c.Request.Context(), claims)
		c.Request = c.Request.WithContext(ctx)
		c.Set("claims", claims)
		c.Set("userID", claims.UserID)
		c.Set("roleCodes", claims.RoleCodes)
		c.Next()
	}
}

// OptionalJWTAuth parses a token when present but never rejects the request.
// Useful for endpoints whose behavior changes for logged-in users.
func OptionalJWTAuth(manager *auth.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if strings.TrimSpace(header) == "" {
			c.Next()
			return
		}
		claims, err := manager.ParseBearer(header)
		if err == nil {
			ctx := auth.WithClaims(c.Request.Context(), claims)
			c.Request = c.Request.WithContext(ctx)
			c.Set("claims", claims)
		}
		c.Next()
	}
}
