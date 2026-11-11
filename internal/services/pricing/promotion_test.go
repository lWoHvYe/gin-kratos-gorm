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
