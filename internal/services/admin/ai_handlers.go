package admin

import (
	"context"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/auth"
	"gin-kratos-gorm/internal/platform/bff"
	"gin-kratos-gorm/internal/platform/httpx"

	"github.com/gin-gonic/gin"
)

const aiStreamTimeout = 120 * time.Second

type aiChatRequest struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

// aiChat proxies the operations assistant. The route already requires the
// admin role; Admin=true selects the analytics agent, never the customer one.
func (h *handlers) aiChat(c *gin.Context) {
	if !h.aiEnabled || h.clients.AI == nil {
		httpx.Error(c, apperr.Unavailable("ai assistant is disabled", nil))
		return
	}
	uid, ok := auth.UserID(c.Request.Context())
	if !ok || uid == 0 {
		httpx.Error(c, apperr.Unauthenticated("authentication required"))
		return
	}
	var in aiChatRequest
	if err := bff.BindJSON(c, &in); err != nil {
		httpx.Error(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), aiStreamTimeout)
	defer cancel()
	stream, err := h.clients.AI.Chat(ctx, &pb.AIChatRequest{
		SessionId: in.SessionID,
		Message:   in.Message,
		UserId:    int64(uid),
		Admin:     true,
	})
	if err != nil {
		httpx.Error(c, bff.RemoteError(err))
		return
	}
	defer func() { _ = stream.CloseSend() }()
	bff.ProxyAIStream(c, stream)
}
