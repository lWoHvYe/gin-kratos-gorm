package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// ProviderConfig selects the model backend.
type ProviderConfig struct {
	Provider   string
	APIKey     string
	BaseURL    string
	Model      string
	MaxTokens  int
	TimeoutSec int
}

func newChatModel(ctx context.Context, cfg ProviderConfig) (model.ToolCallingChatModel, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "mock":
		return &mockModel{}, nil
	case "deepseek":
		if strings.TrimSpace(cfg.APIKey) == "" {
			return nil, fmt.Errorf("deepseek api key is required")
		}
		return deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
			APIKey:    cfg.APIKey,
			BaseURL:   cfg.BaseURL,
			Model:     cfg.Model,
			MaxTokens: cfg.MaxTokens,
			Timeout:   time.Duration(cfg.TimeoutSec) * time.Second,
		})
	default:
		return nil, fmt.Errorf("unsupported ai provider %q", cfg.Provider)
	}
}

// mockModel makes the assistant usable without network access. It calls
// search_products once and then answers from the tool result, which exercises
// the full Eino ReAct loop in tests and local compose.
type mockModel struct {
	tools []*schema.ToolInfo
}

func (m *mockModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	clone := *m
	clone.tools = tools
	return &clone, nil
}

func (m *mockModel) Generate(_ context.Context, in []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	last := in[len(in)-1]
	if last.Role == schema.Tool {
		return &schema.Message{Role: schema.Assistant, Content: mockAnswer(last.Content)}, nil
	}
	query := ""
	for i := len(in) - 1; i >= 0; i-- {
		if in[i].Role == schema.User {
			query = in[i].Content
			break
		}
	}
	args, _ := json.Marshal(map[string]any{"query": query, "page_size": 5})
	return &schema.Message{
		Role: schema.Assistant,
		ToolCalls: []schema.ToolCall{{
			ID:   "mock-call-1",
			Type: "function",
			Function: schema.FunctionCall{
				Name:      "search_products",
				Arguments: string(args),
			},
		}},
	}, nil
}

func (m *mockModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, in, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func mockAnswer(toolResult string) string {
	var out struct {
		Products []struct {
			Name      string `json:"name"`
			PriceCent int64  `json:"min_price_cent"`
		} `json:"products"`
	}
	if err := json.Unmarshal([]byte(toolResult), &out); err != nil || len(out.Products) == 0 {
		return "没有找到匹配的商品，可以换个关键词或预算试试。"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "找到 %d 件商品：\n", len(out.Products))
	for i, p := range out.Products {
		if i >= 3 {
			break
		}
		fmt.Fprintf(&b, "- %s（¥%.2f）\n", p.Name, float64(p.PriceCent)/100)
	}
	b.WriteString("需要我按价格或用途再筛选吗？")
	return b.String()
}
