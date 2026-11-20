package bff

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/httpx"

	"github.com/gin-gonic/gin"
)

// AIEventStream is the client side of AIService.Chat. Both BFFs proxy it to SSE.
type AIEventStream interface {
	Recv() (*pb.AIChatEvent, error)
}

type aiEvent struct {
	Type            string `json:"type"`
	Delta           string `json:"delta,omitempty"`
	Tool            string `json:"tool,omitempty"`
	Error           string `json:"error,omitempty"`
	DraftJSON       string `json:"draft_json,omitempty"`
	AttributionJSON string `json:"attribution_json,omitempty"`
}

// ProxyAIStream forwards assistant events as SSE. It reads the first event
// before writing headers so quota/provider failures still produce a real HTTP
// status (429/503) instead of a 200 with an error frame.
func ProxyAIStream(c *gin.Context, stream AIEventStream) {
	first, err := stream.Recv()
	if err != nil {
		if errors.Is(err, io.EOF) {
			httpx.Error(c, RemoteError(err))
			return
		}
		httpx.Error(c, RemoteError(err))
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
		case "draft":
			return write(aiEvent{Type: "draft", DraftJSON: ev.GetDraftJson()})
		case "attribution":
			return write(aiEvent{Type: "attribution", AttributionJSON: ev.GetAttributionJson()})
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
