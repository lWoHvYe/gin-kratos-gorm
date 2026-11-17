package pricing

import (
	"context"
	"log/slog"
	"testing"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"

	"github.com/expr-lang/expr"
)

func TestPromotionConditionEnvironment(t *testing.T) {
	program, err := expr.Compile(`subtotal_cent >= 10000 && quantity >= 2 && coupon_applied == false`, expr.Env(promoEnv{}))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := expr.Run(program, promoEnv{SubtotalCent: 12000, Quantity: 2})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if out != true {
		t.Fatalf("condition = %v", out)
	}
}

func TestPromotionEffectAndAllocation(t *testing.T) {
	s := &PromotionService{}
	lines := []eligibleLine{
		{item: &pb.PromotionEvaluateItem{SkuId: 1}, amountCent: 6000},
		{item: &pb.PromotionEvaluateItem{SkuId: 2}, amountCent: 4000},
	}
	amount, _ := s.effectAmount(promotionEffect{Type: "PERCENT", Percent: 10}, 10000)
	alloc := allocatePromotion(amount, lines)
	if amount != 1000 || len(alloc) != 2 {
		t.Fatalf("amount=%d alloc=%+v", amount, alloc)
	}
	var sum int64
	for _, a := range alloc {
		sum += a.GetDiscountCent()
	}
	if sum != amount {
		t.Fatalf("allocation sum=%d want %d", sum, amount)
	}
}

func TestSelectBestChoosesHighestDiscount(t *testing.T) {
	rule := func(id int64, amount int64, exclusive bool) candidate {
		return candidate{rule: &promotionRule{ID: id, CampaignID: id, Exclusive: exclusive}, amount: amount}
	}
	cands := []candidate{
		rule(1, 500, true),
		rule(2, 600, false),
		rule(3, 400, false),
	}
	selected, _ := selectBest(cands, nil)
	if len(selected) != 2 || selected[0].rule.ID != 2 || selected[1].rule.ID != 3 {
		t.Fatalf("unexpected best selection: %+v", selected)
	}
}

func TestSelectPriorityDoesNotMixExclusive(t *testing.T) {
	cands := []candidate{
		{rule: &promotionRule{ID: 1}, amount: 500},
		{rule: &promotionRule{ID: 2, Exclusive: true}, amount: 600},
	}
	selected, rejected := selectPriority(cands, nil)
	if len(selected) != 1 || selected[0].rule.ID != 1 || len(rejected) != 1 || rejected[0].Reason != "EXCLUSIVE_CONFLICT" {
		t.Fatalf("selected=%+v rejected=%+v", selected, rejected)
	}
}

func TestEvaluateDraftDoesNotNeedDatabase(t *testing.T) {
	svc := &PromotionService{log: slog.Default(), now: func() time.Time { return time.Unix(1000, 0) }}
	evaluation := &pb.PromotionEvaluateRequest{
		UserId: 7,
		Items:  []*pb.PromotionEvaluateItem{{SkuId: 1, PriceCent: 10000, Quantity: 1}},
	}
	draft := &pb.PromotionRule{
		Code: "DRAFT10", Name: "10% off", Type: "PERCENT", Status: "DRAFT",
		ScopeJson: `{"all":true}`, EffectJson: `{"type":"PERCENT","percent":10}`,
		AllowWithCoupon: true,
	}
	got, err := svc.EvaluateDraft(context.Background(), &pb.EvaluateDraftRequest{
		Evaluation: evaluation, Rule: draft,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.GetDiscountCent() != 1000 {
		t.Fatalf("discount = %d, want 1000", got.GetDiscountCent())
	}
	if len(got.GetApplied()) != 1 || got.GetApplied()[0].GetCode() != "DRAFT10" {
		t.Fatalf("applied = %+v", got.GetApplied())
	}

	draft.Type = "THRESHOLD"
	draft.EffectJson = `{"type":"THRESHOLD","tiers":[{"min_cent":10000,"amount_cent":1500}]}`
	draft.ConditionExpr = "subtotal_cent >= 10000"
	got, err = svc.EvaluateDraft(context.Background(), &pb.EvaluateDraftRequest{Evaluation: evaluation, Rule: draft})
	if err != nil || got.GetDiscountCent() != 1500 {
		t.Fatalf("threshold discount = %d err = %v", got.GetDiscountCent(), err)
	}
}
