package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

// TestDeepSeekAdapterContract verifies the real adapter against an
// OpenAI-compatible server: auth, path, model and message parsing.
func TestDeepSeekAdapterContract(t *testing.T) {
	var sawAuth, sawPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		sawPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": 1,
			"model":   "deepseek-chat",
			"choices": []any{map[string]any{
				"index":         0,
				"message":       map[string]any{"role": "assistant", "content": "你好，我是助手"},
				"finish_reason": "stop",
			}},
			"usage": map[string]any{"prompt_tokens": 3, "completion_tokens": 5, "total_tokens": 8},
		})
	}))
	defer srv.Close()

	cm, err := newChatModel(context.Background(), ProviderConfig{
		Provider: "deepseek", APIKey: "test-key", BaseURL: srv.URL + "/",
		Model: "deepseek-chat", MaxTokens: 64, TimeoutSec: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	msg, err := cm.Generate(context.Background(), []*schema.Message{
		{Role: schema.System, Content: "be brief"},
		{Role: schema.User, Content: "hi"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sawPath, "chat/completions") {
		t.Fatalf("path = %q", sawPath)
	}
	if sawAuth != "Bearer test-key" {
		t.Fatalf("auth = %q", sawAuth)
	}
	if msg.Content != "你好，我是助手" {
		t.Fatalf("content = %q", msg.Content)
	}
	if msg.ResponseMeta == nil || msg.ResponseMeta.Usage == nil || msg.ResponseMeta.Usage.CompletionTokens != 5 {
		t.Fatalf("usage = %+v", msg.ResponseMeta)
	}
}

func TestDeepSeekAdapterMissingKey(t *testing.T) {
	if _, err := newChatModel(context.Background(), ProviderConfig{Provider: "deepseek", Model: "deepseek-chat"}); err == nil {
		t.Fatal("expected missing key error")
	}
}
