package middleware

import (
	"time"

	"gin-kratos-gorm/internal/pkg/httpx"
	"gin-kratos-gorm/internal/pkg/idgen"
	"gin-kratos-gorm/internal/pkg/metrics"

	"github.com/gin-gonic/gin"
	"github.com/go-kratos/kratos/v2/log"
)

// RequestID propagates X-Request-ID or generates one. It is the correlation id
// used in access logs and error responses.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(httpx.RequestIDHeader)
		if id == "" {
			id = idgen.EventID()
		}
		httpx.SetRequestID(c, id)
		c.Header(httpx.RequestIDHeader, id)
		c.Next()
	}
}

// AccessLog records method/path/status/latency and feeds RED metrics.
func AccessLog(logger log.Logger) gin.HandlerFunc {
	helper := log.NewHelper(logger)
	requests := metrics.Default().Counter(
		"http_requests_total",
		"Total HTTP requests.",
		"method", "path", "status",
	)
	duration := metrics.Default().Histogram(
		"http_request_duration_seconds",
		"HTTP request latency in seconds.",
		[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		"method", "path",
	)
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		status := c.Writer.Status()
		requests.Inc(c.Request.Method, path, statusText(status))
		duration.Observe(time.Since(start).Seconds(), c.Request.Method, path)
		helper.Infof("http request method=%s path=%s status=%d latency=%s request_id=%s ip=%s",
			c.Request.Method, path, status, time.Since(start), httpx.RequestID(c), c.ClientIP())
	}
}

func statusText(status int) string {
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
