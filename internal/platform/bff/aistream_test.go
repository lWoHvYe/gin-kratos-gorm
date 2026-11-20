package bff

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"

	"github.com/gin-gonic/gin"
)

type fakeAIEventStream struct {
	events []*pb.AIChatEvent
	index  int
}

func (f *fakeAIEventStream) Recv() (*pb.AIChatEvent, error) {
	if f.index >= len(f.events) {
		return nil, io.EOF
	}
	event := f.events[f.index]
	f.index++
	return event, nil
}

func TestProxyAIStreamForwardsAttribution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/ai/chat", nil)
	stream := &fakeAIEventStream{events: []*pb.AIChatEvent{
		{Type: "delta", Delta: "你好"},
		{Type: "attribution", AttributionJson: `{"brands":["云舟"],"source":"explicit_profile"}`},
		{Type: "done"},
	}}
	ProxyAIStream(ctx, stream)
	body := recorder.Body.String()
	if !strings.Contains(body, `"type":"attribution"`) || !strings.Contains(body, "explicit_profile") {
		t.Fatalf("attribution not proxied: %s", body)
	}
	if !strings.Contains(body, `"type":"delta"`) || !strings.Contains(body, `"type":"done"`) {
		t.Fatalf("stream order lost: %s", body)
	}
	if strings.Index(body, `"type":"attribution"`) > strings.Index(body, `"type":"done"`) {
		t.Fatalf("attribution after done: %s", body)
	}
}
