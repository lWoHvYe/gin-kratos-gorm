package gateway

import (
	"context"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/bff"
	"gin-kratos-gorm/internal/platform/httpx"

	"github.com/gin-gonic/gin"
)

const aiStreamTimeout = 120 * time.Second

type aiChatRequest struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

// aiChat proxies the assistant's gRPC stream as SSE. user_id always comes from
// the access token; a body field cannot override it.
func (h *handlers) aiChat(c *gin.Context) {
	if !h.aiEnabled || h.clients.AI == nil {
		httpx.Error(c, unavailableAI())
		return
	}
	uid, err := userID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var in aiChatRequest
	if err := bff.BindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	// The normal BFF budget is 15s; assistant streams get their own 120s
	// context. WithoutCancel keeps trace values while dropping the short
	// deadline, and a failed Send stops the loop when the client disconnects.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), aiStreamTimeout)
	defer cancel()
	stream, err := h.clients.AI.Chat(ctx, &pb.AIChatRequest{
		SessionId: in.SessionID,
		Message:   in.Message,
		UserId:    uid,
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	defer func() { _ = stream.CloseSend() }()
	bff.ProxyAIStream(c, stream)
}

func unavailableAI() error {
	return apperr.Unavailable("ai assistant is disabled", nil)
}

// aiStatus is public so the storefront can hide the assistant entry when the
// service is disabled; it exposes no provider details.
func (h *handlers) aiStatus(c *gin.Context) {
	httpx.OK(c, gin.H{"enabled": h.aiEnabled && h.clients.AI != nil})
}
