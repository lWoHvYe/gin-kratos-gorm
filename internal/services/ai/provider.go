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
			APIKey:      cfg.APIKey,
			BaseURL:     cfg.BaseURL,
			Model:       cfg.Model,
			MaxTokens:   cfg.MaxTokens,
			Temperature: 0.2,
			Timeout:     time.Duration(cfg.TimeoutSec) * time.Second,
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
		return &schema.Message{Role: schema.Assistant, Content: mockAnswer(last.ToolName, last.Content)}, nil
	}
	query := ""
	for i := len(in) - 1; i >= 0; i-- {
		if in[i].Role == schema.User {
			query = in[i].Content
			break
		}
	}
	name, payload := m.mockToolCall(query)
	args, _ := json.Marshal(payload)
	return &schema.Message{
		Role: schema.Assistant,
		ToolCalls: []schema.ToolCall{{
			ID:   "mock-call-1",
			Type: "function",
			Function: schema.FunctionCall{
				Name:      name,
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

func (m *mockModel) mockToolCall(query string) (string, map[string]any) {
	q := strings.ToLower(query)
	has := func(name string) bool {
		for _, ti := range m.tools {
			if ti.Name == name {
				return true
			}
		}
		return false
	}
	if has("preview_campaign") && (strings.Contains(q, "预览") || strings.Contains(q, "试算")) {
		return "preview_campaign", map[string]any{"type": "THRESHOLD", "threshold_cent": 30000, "discount_cent": 3000, "sample_price_cent": 30000, "quantity": 1}
	}
	if has("sales_summary") { // admin agent
		switch {
		case strings.Contains(q, "退款") || strings.Contains(q, "售后"):
			return "refund_summary", map[string]any{"days": 30}
		case strings.Contains(q, "库存") || strings.Contains(q, "缺货") || strings.Contains(q, "补货"):
			return "low_stock", map[string]any{"threshold": 10, "limit": 10}
		case strings.Contains(q, "活动") || strings.Contains(q, "促销") || strings.Contains(q, "满减") || strings.Contains(q, "优惠"):
			return "draft_campaign", map[string]any{"name": "AI 满减活动", "type": "THRESHOLD", "threshold_cent": 30000, "discount_cent": 3000, "days": 7}
		default:
			return "sales_summary", map[string]any{"days": 30}
		}
	}
	if has("search_knowledge") && (strings.Contains(q, "适合") || strings.Contains(q, "怎么选") || strings.Contains(q, "对比") || strings.Contains(q, "有什么区别")) {
		return "search_knowledge", map[string]any{"query": query, "limit": 5}
	}
	switch {
	case has("search_policy") && (strings.Contains(q, "政策") || strings.Contains(q, "规则") || strings.Contains(q, "运费")):
		return "search_policy", map[string]any{"query": query}
	case has("list_my_orders") && (strings.Contains(q, "订单") || strings.Contains(q, "发货") || strings.Contains(q, "物流")):
		return "list_my_orders", map[string]any{"limit": 5}
	case has("list_my_after_sales") && (strings.Contains(q, "退货") || strings.Contains(q, "退款") || strings.Contains(q, "售后")):
		return "list_my_after_sales", map[string]any{"limit": 5}
	default:
		return "search_products", map[string]any{"query": query, "page_size": 5}
	}
}

func mockAnswer(toolName, toolResult string) string {
	var out struct {
		Products []struct {
			Name      string `json:"name"`
			PriceCent int64  `json:"min_price_cent"`
		} `json:"products"`
		Orders []struct {
			OrderNo string `json:"order_no"`
			Status  string `json:"status"`
			PayCent int64  `json:"pay_cent"`
		} `json:"orders"`
		AfterSales []struct {
			No     string `json:"after_sale_no"`
			Status string `json:"status"`
		} `json:"after_sales"`
		Stocks []struct {
			SkuID     int64 `json:"sku_id"`
			Available int64 `json:"available"`
		} `json:"stocks"`
		TotalOrders          int64    `json:"total_orders"`
		PaidOrders           int64    `json:"paid_orders"`
		CompletedOrders      int64    `json:"completed_orders"`
		CancelledOrders      int64    `json:"cancelled_orders"`
		PaidCent             int64    `json:"paid_cent"`
		Total                int64    `json:"total"`
		Requested            int64    `json:"requested"`
		Approved             int64    `json:"approved"`
		Refunded             int64    `json:"refunded"`
		RefundedCent         int64    `json:"refunded_cent"`
		DiscountCent         int64    `json:"discount_cent"`
		ShippingDiscountCent int64    `json:"shipping_discount_cent"`
		Applied              []string `json:"applied"`
		Results              []struct {
			Title   string `json:"title"`
			Content string `json:"content"`
			Kind    string `json:"kind"`
		} `json:"results"`
		Campaign struct {
			Code   string `json:"code"`
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"campaign"`
		Rule struct {
			Code          string `json:"code"`
			ConditionExpr string `json:"condition_expr"`
			EffectJSON    string `json:"effect_json"`
		} `json:"rule"`
		Answer string `json:"answer"`
	}
	if err := json.Unmarshal([]byte(toolResult), &out); err != nil {
		return "我暂时无法读取这项信息，请稍后再试或联系人工客服。"
	}
	switch toolName {
	case "search_knowledge":
		if len(out.Results) == 0 {
			return "知识库里没有找到相关内容，可以换个问法或联系人工客服。"
		}
		var b strings.Builder
		for i, r := range out.Results {
			if i >= 3 {
				break
			}
			content := []rune(r.Content)
			if len(content) > 160 {
				content = content[:160]
			}
			fmt.Fprintf(&b, "- %s：%s\n", r.Title, string(content))
		}
		return strings.TrimRight(b.String(), "\n")
	case "search_policy":
		if out.Answer == "" {
			return "没有找到对应的商城规则，建议转人工客服确认。"
		}
		return out.Answer
	case "sales_summary":
		return fmt.Sprintf("最近 30 天共 %d 笔订单，已支付 %d 笔，完成 %d 笔，取消 %d 笔，支付金额 ¥%.2f。",
			out.TotalOrders, out.PaidOrders, out.CompletedOrders, out.CancelledOrders, float64(out.PaidCent)/100)
	case "refund_summary":
		return fmt.Sprintf("最近 30 天售后 %d 条，待处理 %d 条，已通过 %d 条，已退款 %d 条，退款金额 ¥%.2f。",
			out.Total, out.Requested, out.Approved, out.Refunded, float64(out.RefundedCent)/100)
	case "preview_campaign":
		return fmt.Sprintf("试算结果：优惠 %.2f 元，运费优惠 %.2f 元；命中规则 %v。", float64(out.DiscountCent)/100, float64(out.ShippingDiscountCent)/100, out.Applied)
	case "draft_campaign":
		return fmt.Sprintf("已生成活动草案 %s（%s，状态 %s）：规则条件 %s，优惠 %s。草案未保存，需管理员确认。",
			out.Campaign.Code, out.Campaign.Name, out.Campaign.Status, out.Rule.ConditionExpr, out.Rule.EffectJSON)
	case "low_stock":
		var b strings.Builder
		fmt.Fprintf(&b, "库存偏低的 SKU 共 %d 个，例如：\n", out.Total)
		for i, st := range out.Stocks {
			if i >= 5 {
				break
			}
			fmt.Fprintf(&b, "- SKU %d：可用 %d\n", st.SkuID, st.Available)
		}
		return strings.TrimRight(b.String(), "\n")
	case "list_my_orders", "get_my_order":
		if len(out.Orders) == 0 {
			return "没有查到订单记录。"
		}
		var b strings.Builder
		fmt.Fprintf(&b, "最近 %d 笔订单：\n", len(out.Orders))
		for i, o := range out.Orders {
			if i >= 3 {
				break
			}
			fmt.Fprintf(&b, "- %s：%s，¥%.2f\n", o.OrderNo, o.Status, float64(o.PayCent)/100)
		}
		return strings.TrimRight(b.String(), "\n")
	case "list_my_after_sales", "get_my_after_sale":
		if len(out.AfterSales) == 0 {
			return "没有查到售后记录。"
		}
		var b strings.Builder
		fmt.Fprintf(&b, "最近 %d 条售后记录：\n", len(out.AfterSales))
		for i, a := range out.AfterSales {
			if i >= 3 {
				break
			}
			fmt.Fprintf(&b, "- %s：%s\n", a.No, a.Status)
		}
		return strings.TrimRight(b.String(), "\n")
	case "search_products", "get_product":
		if len(out.Products) == 0 {
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
	default:
		return "没有找到相关记录，可以换个关键词试试。"
	}
}
