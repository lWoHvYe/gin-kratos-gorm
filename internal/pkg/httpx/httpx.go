// Package httpx centralizes JSON responses and error mapping for Gin.
package httpx

import (
	"net/http"

	"gin-kratos-gorm/internal/pkg/apperr"

	"github.com/gin-gonic/gin"
)

const RequestIDHeader = "X-Request-ID"
const requestIDKey = "request_id"

type ErrorBody struct {
	Code      apperr.Code `json:"code"`
	Message   string      `json:"message"`
	RequestID string      `json:"request_id,omitempty"`
}

func SetRequestID(c *gin.Context, id string) {
	c.Set(requestIDKey, id)
}

func RequestID(c *gin.Context) string {
	if v, ok := c.Get(requestIDKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// Error writes a consistent error envelope. Internal details are never leaked
// to clients; the full error is expected to be logged by the access log.
func Error(c *gin.Context, err error) {
	ae := apperr.From(err)
	status := apperr.HTTPStatus(err)
	message := ae.Message
	if ae.Code == apperr.CodeInternal {
		message = "internal server error"
	}
	c.AbortWithStatusJSON(status, ErrorBody{
		Code:      ae.Code,
		Message:   message,
		RequestID: RequestID(c),
	})
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, data)
}

func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, data)
}

func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}
