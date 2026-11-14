package httpx

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/auth"
	"gin-kratos-gorm/internal/platform/id"
	"gin-kratos-gorm/internal/platform/metrics"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/proto"
)

const RequestIDHeader = "X-Request-ID"
const requestIDKey = "request_id"
const loggerKey = "logger"

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader(RequestIDHeader)
		if rid == "" {
			rid = id.Event()
		}
		c.Set(requestIDKey, rid)
		c.Header(RequestIDHeader, rid)
		c.Next()
	}
}
func RequestIDFrom(c *gin.Context) string { v, _ := c.Get(requestIDKey); s, _ := v.(string); return s }

func AccessLog(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}
	return func(c *gin.Context) {
		c.Set(loggerKey, logger)
		start := time.Now()
		c.Next()
		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		elapsed := time.Since(start)
		status := c.Writer.Status()
		metrics.HTTPRequests.WithLabelValues(c.Request.Method, path, statusClass(status)).Inc()
		metrics.HTTPDuration.WithLabelValues(c.Request.Method, path).Observe(elapsed.Seconds())
		logger.Info("http request", "method", c.Request.Method, "path", path, "status", status,
			"latency_ms", elapsed.Milliseconds(), "request_id", RequestIDFrom(c), "ip", c.ClientIP())
	}
}

func loggerFrom(c *gin.Context) *slog.Logger {
	if v, ok := c.Get(loggerKey); ok {
		if l, ok := v.(*slog.Logger); ok && l != nil {
			return l
		}
	}
	return slog.Default()
}

func Error(c *gin.Context, err error) {
	ae := apperr.From(err)
	if ae == nil {
		ae = apperr.Internal("internal error", err)
	}
	code := string(ae.Code)
	metrics.Errors.WithLabelValues("http", code).Inc()
	// 5xx root causes are logged with their chain; the client still receives a
	// redacted message. 4xx stays at debug to keep auth noise out of the log.
	if ae.Code == apperr.CodeInternal || ae.Code == apperr.CodeUnavailable {
		loggerFrom(c).Error("http error", "code", code, "path", c.FullPath(),
			"request_id", RequestIDFrom(c), "error", err)
	} else {
		loggerFrom(c).Debug("http error", "code", code, "path", c.FullPath(),
			"request_id", RequestIDFrom(c), "error", err)
	}
	msg := ae.Message
	if ae.Code == apperr.CodeInternal {
		msg = "internal server error"
	}
	c.AbortWithStatusJSON(apperr.HTTPStatus(err), gin.H{"code": code, "message": msg, "request_id": RequestIDFrom(c)})
}

// OK and Created serialize protobuf replies through ProtoToMap so the contract
// always contains zero-valued fields. Non-protobuf payloads (gin.H, DTOs) pass
// through unchanged.
func OK(c *gin.Context, data any)      { writeJSON(c, http.StatusOK, data) }
func Created(c *gin.Context, data any) { writeJSON(c, http.StatusCreated, data) }

func writeJSON(c *gin.Context, status int, data any) {
	if msg, ok := data.(proto.Message); ok {
		c.JSON(status, ProtoToMap(msg))
		return
	}
	c.JSON(status, data)
}

func Auth(manager *auth.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if strings.TrimSpace(h) == "" {
			Error(c, apperr.Unauthenticated("authorization required"))
			return
		}
		claims, err := manager.ParseBearer(h)
		if err != nil {
			Error(c, err)
			return
		}
		c.Request = c.Request.WithContext(auth.WithClaims(c.Request.Context(), claims))
		c.Set("claims", claims)
		c.Next()
	}
}
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !auth.HasRole(c.Request.Context(), role) {
			Error(c, apperr.Forbidden("insufficient role"))
			return
		}
		c.Next()
	}
}
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		if origin := c.GetHeader("Origin"); origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Request-ID")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func statusClass(status int) string {
	switch {
	case status >= 500:
		return "5xx"
	case status >= 400:
		return "4xx"
	case status >= 300:
		return "3xx"
	default:
		return "2xx"
	}
}
