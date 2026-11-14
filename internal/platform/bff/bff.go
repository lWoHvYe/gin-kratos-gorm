// Package bff holds the HTTP plumbing shared by the gateway and admin BFFs:
// request binding, path/query parsing, timeouts, panic recovery and mapping of
// downstream gRPC errors. Keeping one copy prevents the two surfaces from
// drifting (they previously mapped ResourceExhausted differently).
package bff

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/httpx"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MaxBodyBytes caps request bodies at 1 MiB.
const MaxBodyBytes = 1 << 20

// RemoteError converts a downstream gRPC error into the BFF's domain error. It
// is the single mapping used by both BFFs.
func RemoteError(err error) error {
	if err == nil {
		return nil
	}
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae
	}
	st, ok := status.FromError(err)
	if !ok {
		return apperr.Unavailable("downstream call failed", err)
	}
	switch st.Code() {
	case codes.InvalidArgument:
		return apperr.Invalid("%s", st.Message())
	case codes.Unauthenticated:
		return apperr.Unauthenticated(st.Message())
	case codes.PermissionDenied:
		return apperr.Forbidden(st.Message())
	case codes.NotFound:
		return apperr.NotFound(st.Message())
	case codes.AlreadyExists, codes.Aborted:
		return apperr.Conflict(st.Message())
	case codes.FailedPrecondition, codes.OutOfRange:
		return apperr.Failed(st.Message())
	case codes.ResourceExhausted:
		return apperr.RateLimited(st.Message())
	case codes.Unavailable, codes.DeadlineExceeded, codes.Canceled:
		return apperr.Unavailable("downstream unavailable", err)
	default:
		return apperr.Internal("downstream error", err)
	}
}

// Recovery converts a panic into the regular JSON error envelope.
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		httpx.Error(c, apperr.Internal("panic recovered", fmt.Errorf("%v", recovered)))
	})
}

// Timeout caps the request context so a slow downstream cannot pin a request.
func Timeout(d time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// BindJSON decodes a JSON body with a size limit.
func BindJSON(c *gin.Context, dst any) error {
	if c.Request != nil && c.Request.Body != nil {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxBodyBytes)
	}
	if err := c.ShouldBindJSON(dst); err != nil {
		return apperr.Invalid("invalid request body: %v", err)
	}
	return nil
}

// BodyPresent reports whether the request carries a body.
func BodyPresent(c *gin.Context) bool {
	return c.Request != nil && c.Request.Body != nil && c.Request.ContentLength != 0
}

// PathID parses a positive int64 path parameter.
func PathID(c *gin.Context, name string) (int64, error) {
	raw := strings.TrimSpace(c.Param(name))
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.Invalid("%s must be a positive integer", name)
	}
	return id, nil
}

// QueryInt32 parses an optional int32 query parameter (0 when absent).
func QueryInt32(c *gin.Context, key string) (int32, error) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0, apperr.Invalid("%s must be an integer", key)
	}
	return int32(value), nil
}

// QueryInt64 parses an optional int64 query parameter (0 when absent).
func QueryInt64(c *gin.Context, key string) (int64, error) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, apperr.Invalid("%s must be an integer", key)
	}
	return value, nil
}
