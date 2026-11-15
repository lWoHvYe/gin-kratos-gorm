package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
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

type aiEvent struct {
	Type  string `json:"type"`
	Delta string `json:"delta,omitempty"`
	Tool  string `json:"tool,omitempty"`
	Error string `json:"error,omitempty"`
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

	// Read the first event before committing SSE headers: quota and provider
	// errors arrive here, so the HTTP status can still be 429/503.
	first, recvErr := stream.Recv()
	if recvErr != nil {
		if errors.Is(recvErr, io.EOF) {
			httpx.Error(c, apperr.Unavailable("assistant returned no events", nil))
			return
		}
		httpx.Error(c, bff.RemoteError(recvErr))
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	flusher, _ := c.Writer.(http.Flusher)

	write := func(ev aiEvent) bool {
		payload, err := json.Marshal(ev)
		if err != nil {
			return false
		}
		if _, err := c.Writer.Write([]byte("data: " + string(payload) + "\n\n")); err != nil {
			return false
		}
		if flusher != nil {
			flusher.Flush()
		}
		return true
	}

	handle := func(ev *pb.AIChatEvent) bool {
		switch ev.GetType() {
		case "delta":
			return write(aiEvent{Type: "delta", Delta: ev.GetDelta()})
		case "tool":
			return write(aiEvent{Type: "tool", Tool: ev.GetTool()})
		case "error":
			write(aiEvent{Type: "error", Error: ev.GetError()})
			return false
		case "done":
			write(aiEvent{Type: "done"})
			return false
		}
		return true
	}
	if !handle(first) {
		return
	}
	for {
		ev, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			return
		}
		if recvErr != nil {
			write(aiEvent{Type: "error", Error: "assistant stream interrupted"})
			return
		}
		if ev == nil {
			continue
		}
		if !handle(ev) {
			return
		}
	}
}

func unavailableAI() error {
	return apperr.Unavailable("ai assistant is disabled", nil)
}
