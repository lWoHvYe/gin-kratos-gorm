package ai

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

// TestGoldenAssistantTurns is a deterministic evaluation set for the agent
// graph. It exercises the real Eino ReAct loop with the mock model and fake
// domain clients, so routing/tool regressions fail without network access.
func TestGoldenAssistantTurns(t *testing.T) {
	d := Deps{
		Catalog:    &fakeCatalog{},
		Orders:     &fakeOrders{},
		AfterSales: &fakeAfterSales{},
		Inventory:  fakeInventory{},
	}
	customer, err := buildAgent(context.Background(), ProviderConfig{Provider: "mock"}, d)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := buildAdminAgent(context.Background(), ProviderConfig{Provider: "mock"}, d)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		run  agentRunner
		msg  string
		want []string
	}{
		{"product", customer, "Widget", []string{"静音键盘", "299.00"}},
		{"orders", customer, "我的订单", []string{"ORD-1", "PAID"}},
		{"policy", customer, "退货政策", []string{"退货退款流程"}},
		{"after-sales", customer, "售后", []string{"AS-1"}},
		{"admin-sales", admin, "最近销售", []string{"123.45", "4"}},
		{"admin-stock", admin, "库存", []string{"SKU 9", "可用 6"}},
		{"admin-refunds", admin, "退款", []string{"售后", "5.00"}},
		{"admin-campaign", admin, "做一个促销", []string{"草案", "subtotal_cent >= 30000"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runAgent(t, tc.run, tc.msg)
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Fatalf("answer %q does not contain %q", got, want)
				}
			}
		})
	}
}

func runAgent(t *testing.T, runner agentRunner, msg string) string {
	t.Helper()
	ctx := withUserID(context.Background(), 7)
	stream, err := runner.Stream(ctx, []*schema.Message{{Role: schema.User, Content: msg}})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	var b strings.Builder
	for {
		m, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			t.Fatal(recvErr)
		}
		b.WriteString(m.Content)
	}
	return b.String()
}
