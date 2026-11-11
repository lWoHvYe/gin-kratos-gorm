// Package pricing contains the quote/pricing service. Promotion lives in the
// same process and database for Phase 2: quote, promotion evaluation and budget
// reservation then share one transaction boundary and one deployment. The
// PromotionService gRPC boundary is kept separate so the engine can be split
// into its own service later without changing the order/admin clients.
package pricing

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

const (
	campaignStatusDraft  = "DRAFT"
	campaignStatusActive = "ACTIVE"
	campaignStatusPaused = "PAUSED"
	campaignStatusEnded  = "ENDED"

	ruleStatusDraft  = "DRAFT"
	ruleStatusActive = "ACTIVE"
	ruleStatusPaused = "PAUSED"

	maxPromotionStack = 10
)

// PromotionService owns promotion campaigns, rules, evaluation and budget
// reservations. It is intentionally stateless apart from the expression cache.
type PromotionService struct {
	pb.UnimplementedPromotionServiceServer
	db    *sql.DB
	log   *slog.Logger
	now   func() time.Time
	cache sync.Map // expression string -> compiled program
}

func NewPromotionService(database *sql.DB, logger *slog.Logger) *PromotionService {
	if logger == nil {
		logger = slog.Default()
	}
	return &PromotionService{db: database, log: logger, now: time.Now}
}

type promotionScope struct {
	All  bool    `json:"all"`
	Type string  `json:"type"`
	IDs  []int64 `json:"ids"`
}

type promotionTier struct {
	MinCent    int64 `json:"min_cent"`
	AmountCent int64 `json:"amount_cent"`
}

type promotionEffect struct {
	Type         string          `json:"type"`
	AmountCent   int64           `json:"amount_cent"`
	Percent      int32           `json:"percent"`
	MaxDiscount  int64           `json:"max_discount_cent"`
	ShippingCent int64           `json:"shipping_cent"`
	Tiers        []promotionTier `json:"tiers"`
}

type promotionRule struct {
	ID                int64
	CampaignID        int64
	Code              string
	Name              string
	Type              string
	Status            string
	Priority          int32
	Exclusive         bool
	StackGroup        string
	ScopeJSON         string
	ConditionExpr     string
	EffectJSON        string
	MaxDiscountCent   int64
	BudgetCent        int64
	ReservedCent      int64
	ConsumedCent      int64
	PerUserLimit      int32
	AllowWithCoupon   bool
	StartAt           time.Time
	EndAt             time.Time
	Version           int64
	CampaignName      string
	CampaignStatus    string
	CampaignStart     time.Time
	CampaignEnd       time.Time
	CampaignBudget    int64
	CampaignReserved  int64
	CampaignConsumed  int64
	CampaignStackMode string
}

type promoEnv struct {
	SubtotalCent  int64 `expr:"subtotal_cent"`
	CartTotalCent int64 `expr:"cart_total_cent"`
	Quantity      int64 `expr:"quantity"`
	ItemCount     int64 `expr:"item_count"`
	UserID        int64 `expr:"user_id"`
	CouponApplied bool  `expr:"coupon_applied"`
	NowUnix       int64 `expr:"now_unix"`
}

func exprCompile(condition string) (*vm.Program, error) {
	return expr.Compile(condition, expr.Env(promoEnv{}))
}

func (s *PromotionService) compileCondition(condition string) (*vm.Program, error) {
	condition = strings.TrimSpace(condition)
	if condition == "" {
		return nil, nil
	}
	if cached, ok := s.cache.Load(condition); ok {
		return cached.(*vm.Program), nil
	}
	program, err := expr.Compile(condition, expr.Env(promoEnv{}))
	if err != nil {
		return nil, err
	}
	s.cache.Store(condition, program)
	return program, nil
}

func (s *PromotionService) conditionMatches(ctx context.Context, rule *promotionRule, env promoEnv) (bool, error) {
	program, err := s.compileCondition(rule.ConditionExpr)
	if err != nil {
		return false, fmt.Errorf("compile condition: %w", err)
	}
	if program == nil {
		return true, nil
	}
	out, err := expr.Run(program, env)
	if err != nil {
		return false, fmt.Errorf("evaluate condition: %w", err)
	}
	ok, isBool := out.(bool)
	if !isBool {
		return false, fmt.Errorf("condition did not return bool")
	}
	return ok, nil
}

func parseScope(raw string) (promotionScope, error) {
	var scope promotionScope
	if strings.TrimSpace(raw) == "" {
		return promotionScope{All: true}, nil
	}
	if err := json.Unmarshal([]byte(raw), &scope); err != nil {
		return scope, err
	}
	if scope.Type == "" {
		scope.All = true
	}
	return scope, nil
}

func (a *promotionScope) matches(item *pb.PromotionEvaluateItem) bool {
	if a == nil || a.All || strings.EqualFold(a.Type, "ALL") {
		return true
	}
	if len(a.IDs) == 0 {
		return false
	}
	var value int64
	switch strings.ToUpper(a.Type) {
	case "SKU":
		value = item.GetSkuId()
	case "SPU":
		value = item.GetSpuId()
	case "CATEGORY":
		value = item.GetCategoryId()
	case "BRAND":
		value = item.GetBrandId()
	default:
		return false
	}
	for _, id := range a.IDs {
		if id == value {
			return true
		}
	}
	return false
}

type eligibleLine struct {
	item       *pb.PromotionEvaluateItem
	amountCent int64
}

func (s *PromotionService) eligibleLines(items []*pb.PromotionEvaluateItem, scope promotionScope) []eligibleLine {
	out := make([]eligibleLine, 0, len(items))
	for _, item := range items {
		if item == nil || item.GetQuantity() <= 0 || item.GetPriceCent() <= 0 {
			continue
		}
		if !scope.matches(item) {
			continue
		}
		out = append(out, eligibleLine{item: item, amountCent: item.GetPriceCent() * int64(item.GetQuantity())})
	}
	return out
}

func (s *PromotionService) effectAmount(effect promotionEffect, subtotal int64) (amount, shipping int64) {
	switch strings.ToUpper(effect.Type) {
	case "FIXED_AMOUNT":
		amount = effect.AmountCent
	case "PERCENT":
		amount = subtotal * int64(effect.Percent) / 100
		if effect.MaxDiscount > 0 && amount > effect.MaxDiscount {
			amount = effect.MaxDiscount
		}
	case "THRESHOLD":
		var selected *promotionTier
		for i := range effect.Tiers {
			tier := &effect.Tiers[i]
			if subtotal >= tier.MinCent && (selected == nil || tier.MinCent > selected.MinCent) {
				selected = tier
			}
		}
		if selected != nil {
			amount = selected.AmountCent
		}
	case "FREE_SHIPPING":
		shipping = effect.ShippingCent
		if shipping <= 0 {
			shipping = 1000
		}
	}
	if amount > subtotal {
		amount = subtotal
	}
	if amount < 0 {
		amount = 0
	}
	if shipping < 0 {
		shipping = 0
	}
	return amount, shipping
}

func allocatePromotion(amount int64, lines []eligibleLine) []*pb.PromotionAllocation {
	if amount <= 0 || len(lines) == 0 {
		return nil
	}
	var total int64
	for _, line := range lines {
		total += line.amountCent
	}
	if total <= 0 {
		return nil
	}
	// Proportional integer allocation. The final line absorbs the rounding
	// remainder so the sum is exactly amount.
	out := make([]*pb.PromotionAllocation, 0, len(lines))
	var allocated int64
	for i, line := range lines {
		part := amount * line.amountCent / total
		if i == len(lines)-1 {
			part = amount - allocated
		}
		allocated += part
		if part > 0 {
			out = append(out, &pb.PromotionAllocation{SkuId: line.item.GetSkuId(), DiscountCent: part})
		}
	}
	return out
}

type candidate struct {
	rule        *promotionRule
	amount      int64
	shipping    int64
	allocations []*pb.PromotionAllocation
}

func (s *PromotionService) evaluate(ctx context.Context, req *pb.PromotionEvaluateRequest) (*pb.PromotionEvaluation, error) {
	now := s.now()
	items := req.GetItems()
	var cartTotal, cartQty int64
	for _, item := range items {
		if item != nil && item.GetQuantity() > 0 {
			cartTotal += item.GetPriceCent() * int64(item.GetQuantity())
			cartQty += int64(item.GetQuantity())
		}
	}
	rules, err := s.loadActiveRules(ctx, now)
	if err != nil {
		return nil, err
	}
	applied := make([]candidate, 0)
	rejected := make([]*pb.RejectedPromotion, 0)
	for _, rule := range rules {
		if req.GetCouponApplied() && !rule.AllowWithCoupon {
			rejected = append(rejected, &pb.RejectedPromotion{RuleId: rule.ID, Code: rule.Code, Reason: "COUPON_NOT_ALLOWED"})
			continue
		}
		scope, err := parseScope(rule.ScopeJSON)
		if err != nil {
			rejected = append(rejected, &pb.RejectedPromotion{RuleId: rule.ID, Code: rule.Code, Reason: "INVALID_SCOPE"})
			continue
		}
		lines := s.eligibleLines(items, scope)
		if len(lines) == 0 {
			rejected = append(rejected, &pb.RejectedPromotion{RuleId: rule.ID, Code: rule.Code, Reason: "SCOPE_MISMATCH"})
			continue
		}
		var subtotal, qty int64
		for _, line := range lines {
			subtotal += line.amountCent
			qty += int64(line.item.GetQuantity())
		}
		env := promoEnv{SubtotalCent: subtotal, CartTotalCent: cartTotal, Quantity: qty,
			ItemCount: int64(len(lines)), UserID: req.GetUserId(), CouponApplied: req.GetCouponApplied(), NowUnix: now.Unix()}
		ok, err := s.conditionMatches(ctx, rule, env)
		if err != nil {
			s.log.Warn("promotion condition failed", "rule_id", rule.ID, "error", err)
			rejected = append(rejected, &pb.RejectedPromotion{RuleId: rule.ID, Code: rule.Code, Reason: "CONDITION_ERROR"})
			continue
		}
		if !ok {
			rejected = append(rejected, &pb.RejectedPromotion{RuleId: rule.ID, Code: rule.Code, Reason: "CONDITION_NOT_MET"})
			continue
		}
		var effect promotionEffect
		if err := json.Unmarshal([]byte(rule.EffectJSON), &effect); err != nil {
			rejected = append(rejected, &pb.RejectedPromotion{RuleId: rule.ID, Code: rule.Code, Reason: "INVALID_EFFECT"})
			continue
		}
		amount, shipping := s.effectAmount(effect, subtotal)
		if amount <= 0 && shipping <= 0 {
			rejected = append(rejected, &pb.RejectedPromotion{RuleId: rule.ID, Code: rule.Code, Reason: "NO_BENEFIT"})
			continue
		}
		if rule.MaxDiscountCent > 0 && amount > rule.MaxDiscountCent {
			amount = rule.MaxDiscountCent
		}
		applied = append(applied, candidate{rule: rule, amount: amount, shipping: shipping, allocations: allocatePromotion(amount, lines)})
	}

	sort.SliceStable(applied, func(i, j int) bool {
		if applied[i].rule.Priority != applied[j].rule.Priority {
			return applied[i].rule.Priority > applied[j].rule.Priority
		}
		return applied[i].rule.ID < applied[j].rule.ID
	})

	useBest := false
	for _, cand := range applied {
		if strings.EqualFold(cand.rule.CampaignStackMode, "BEST") {
			useBest = true
			break
		}
	}
	selected, rejected := selectPromotions(applied, rejected, useBest)

	var discount, shippingDiscount int64
	var version int64
	appliedPB := make([]*pb.AppliedPromotion, 0, len(selected))
	for _, cand := range selected {
		discount += cand.amount
		shippingDiscount += cand.shipping
		if cand.rule.Version > version {
			version = cand.rule.Version
		}
		appliedPB = append(appliedPB, &pb.AppliedPromotion{
			RuleId: cand.rule.ID, CampaignId: cand.rule.CampaignID, Code: cand.rule.Code,
			Name: cand.rule.Name, Type: cand.rule.Type, AmountCent: cand.amount,
			ShippingCent: cand.shipping, Allocations: cand.allocations,
			RuleVersion: cand.rule.Version, StackGroup: cand.rule.StackGroup, Priority: cand.rule.Priority,
		})
	}
	snapshot, _ := json.Marshal(appliedPB)
	return &pb.PromotionEvaluation{
		Applied: appliedPB, Rejected: rejected, DiscountCent: discount,
		ShippingDiscountCent: shippingDiscount, PromotionVersion: version, SnapshotJson: string(snapshot),
	}, nil
}

func selectPromotions(candidates []candidate, rejected []*pb.RejectedPromotion, best bool) ([]candidate, []*pb.RejectedPromotion) {
	if best && len(candidates) <= 12 {
		return selectBest(candidates, rejected)
	}
	return selectPriority(candidates, rejected)
}

func selectPriority(candidates []candidate, rejected []*pb.RejectedPromotion) ([]candidate, []*pb.RejectedPromotion) {
	selected := make([]candidate, 0, len(candidates))
	groups := make(map[string]bool)
	exclusiveSelected := false
	for _, cand := range candidates {
		rule := cand.rule
		if exclusiveSelected {
			rejected = append(rejected, &pb.RejectedPromotion{RuleId: rule.ID, Code: rule.Code, Reason: "EXCLUSIVE_SELECTED"})
			continue
		}
		if rule.Exclusive && len(selected) > 0 {
			rejected = append(rejected, &pb.RejectedPromotion{RuleId: rule.ID, Code: rule.Code, Reason: "EXCLUSIVE_CONFLICT"})
			continue
		}
		if rule.StackGroup != "" && groups[rule.StackGroup] {
			rejected = append(rejected, &pb.RejectedPromotion{RuleId: rule.ID, Code: rule.Code, Reason: "STACK_GROUP_CONFLICT"})
			continue
		}
		selected = append(selected, cand)
		if rule.StackGroup != "" {
			groups[rule.StackGroup] = true
		}
		if rule.Exclusive {
			exclusiveSelected = true
		}
		if len(selected) >= maxPromotionStack {
			break
		}
	}
	return selected, rejected
}

// selectBest enumerates all valid subsets for small candidate sets. It is exact
// and deterministic; for larger sets it falls back to the priority policy so a
// quote cannot become unbounded.
func selectBest(candidates []candidate, rejected []*pb.RejectedPromotion) ([]candidate, []*pb.RejectedPromotion) {
	if len(candidates) == 0 {
		return nil, rejected
	}
	var best []candidate
	var bestScore int64 = -1
	total := 1 << len(candidates)
	for mask := 0; mask < total; mask++ {
		chosen := make([]candidate, 0, len(candidates))
		groups := make(map[string]bool)
		campaigns := make(map[int64]int)
		exclusive := 0
		valid := true
		var score int64
		for i, cand := range candidates {
			if mask&(1<<i) == 0 {
				continue
			}
			rule := cand.rule
			if exclusive > 0 || (rule.Exclusive && len(chosen) > 0) {
				valid = false
				break
			}
			if rule.Exclusive {
				exclusive++
			}
			if rule.StackGroup != "" && groups[rule.StackGroup] {
				valid = false
				break
			}
			if strings.EqualFold(rule.CampaignStackMode, "EXCLUSIVE") && campaigns[rule.CampaignID] > 0 {
				valid = false
				break
			}
			groups[rule.StackGroup] = true
			campaigns[rule.CampaignID]++
			chosen = append(chosen, cand)
			score += cand.amount + cand.shipping
			if len(chosen) > maxPromotionStack {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		if score > bestScore {
			bestScore = score
			best = chosen
		}
	}
	if bestScore < 0 {
		best = nil
	}
	chosenIDs := make(map[int64]bool, len(best))
	for _, cand := range best {
		chosenIDs[cand.rule.ID] = true
	}
	for _, cand := range candidates {
		if !chosenIDs[cand.rule.ID] {
			rejected = append(rejected, &pb.RejectedPromotion{RuleId: cand.rule.ID, Code: cand.rule.Code, Reason: "BEST_COMBINATION"})
		}
	}
	return best, rejected
}

func (s *PromotionService) EvaluatePromotions(ctx context.Context, req *pb.PromotionEvaluateRequest) (*pb.PromotionEvaluation, error) {
	if req.GetUserId() <= 0 {
		return nil, apperr.Invalid("user_id is required")
	}
	if len(req.GetItems()) == 0 {
		return nil, apperr.Invalid("items are required")
	}
	return s.evaluate(ctx, req)
}
