package middleware

import (
	"gin-kratos-gorm/internal/pkg/apperr"
	"gin-kratos-gorm/internal/pkg/auth"
	"gin-kratos-gorm/internal/pkg/httpx"

	"github.com/casbin/casbin/v3"
	"github.com/gin-gonic/gin"
)

// Casbin enforces RBAC using the role codes carried in the JWT. Policies use
// subjects such as "r:admin" and "r:user"; per-user grouping is still supported
// by the model but is not required for the common case.
func Casbin(enforcer *casbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		if enforcer == nil {
			c.Next()
			return
		}
		claims, ok := auth.FromContext(c.Request.Context())
		if !ok {
			httpx.Error(c, apperr.Unauthenticated("login required"))
			return
		}
		obj := c.FullPath()
		if obj == "" {
			obj = c.Request.URL.Path
		}
		act := c.Request.Method
		for _, role := range claims.RoleCodes {
			allowed, err := enforcer.Enforce("r:"+role, obj, act)
			if err != nil {
				httpx.Error(c, apperr.Internal("authorization check failed", err))
				return
			}
			if allowed {
				c.Next()
				return
			}
		}
		httpx.Error(c, apperr.Forbidden("you do not have permission to access this resource"))
	}
}

// RequireRole is a lightweight fallback used when Casbin is disabled.
func RequireRole(codes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !auth.HasRole(c.Request.Context(), codes...) {
			httpx.Error(c, apperr.Forbidden("insufficient role"))
			return
		}
		c.Next()
	}
}
