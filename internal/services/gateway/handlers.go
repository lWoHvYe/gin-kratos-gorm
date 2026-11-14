package gateway

import (
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/auth"
	"gin-kratos-gorm/internal/platform/httpx"
	"log/slog"
	"math"

	"github.com/gin-gonic/gin"
)

// handlers holds the downstream clients. It is stateless.
type handlers struct {
	clients *Clients
	log     *slog.Logger
}

// ---------------------------------------------------------------- middleware

// requireUser rejects unauthenticated requests before they reach a handler.
func requireUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, err := userID(c); err != nil {
			httpx.Error(c, err)
			return
		}
		c.Next()
	}
}

// userID returns the authenticated user. The auth middleware stores the claims
// in the request context; a missing or zero id is rejected.
func userID(c *gin.Context) (int64, error) {
	uid, ok := auth.UserID(c.Request.Context())
	if !ok || uid == 0 {
		return 0, apperr.Unauthenticated("authentication required")
	}
	if uid > math.MaxInt64 {
		return 0, apperr.Unauthenticated("invalid user id")
	}
	return int64(uid), nil
}

// bindJSON decodes the request body into dst and maps decoding failures onto an
// invalid-argument error.
// bodyPresent reports whether the request carries a body.
// pathID parses a positive int64 path parameter.
// queryInt64 parses an optional int64 query parameter; an absent value is 0.
// queryInt32 parses an optional int32 query parameter; an absent value is 0.
// remoteError translates a downstream gRPC status into the domain error model
// so httpx.Error emits the right HTTP status. Domain errors pass through.
// ---------------------------------------------------------------- after-sales / shipment
