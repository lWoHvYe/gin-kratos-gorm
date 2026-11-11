package pricing

import (
	"testing"

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
