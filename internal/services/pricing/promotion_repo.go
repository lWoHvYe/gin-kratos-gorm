package pricing

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/id"

	"github.com/go-sql-driver/mysql"
)

func (s *PromotionService) loadActiveRules(ctx context.Context, now time.Time) ([]*promotionRule, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT r.id, r.campaign_id, r.code, r.name, r.type, r.status,
		r.priority, r.exclusive, r.stack_group, r.scope_json, r.condition_expr, r.effect_json,
		r.max_discount_cent, r.budget_cent, r.reserved_cent, r.consumed_cent, r.per_user_limit,
		r.allow_with_coupon, r.start_at, r.end_at, r.version,
		c.name, c.status, c.start_at, c.end_at, c.budget_cent, c.reserved_cent, c.consumed_cent, c.stack_mode
		FROM promotion_rules r JOIN promotion_campaigns c ON c.id = r.campaign_id
		WHERE r.status = ? AND c.status = ? AND r.start_at <= ? AND r.end_at >= ? AND c.start_at <= ? AND c.end_at >= ?
		ORDER BY r.priority DESC, r.id ASC`,
		ruleStatusActive, campaignStatusActive, now, now, now, now)
	if err != nil {
		return nil, apperr.Internal("load active promotions", err)
	}
	defer rows.Close()
	var rules []*promotionRule
	for rows.Next() {
		r := &promotionRule{}
		if err := rows.Scan(&r.ID, &r.CampaignID, &r.Code, &r.Name, &r.Type, &r.Status,
			&r.Priority, &r.Exclusive, &r.StackGroup, &r.ScopeJSON, &r.ConditionExpr, &r.EffectJSON,
			&r.MaxDiscountCent, &r.BudgetCent, &r.ReservedCent, &r.ConsumedCent, &r.PerUserLimit,
			&r.AllowWithCoupon, &r.StartAt, &r.EndAt, &r.Version,
			&r.CampaignName, &r.CampaignStatus, &r.CampaignStart, &r.CampaignEnd,
			&r.CampaignBudget, &r.CampaignReserved, &r.CampaignConsumed, &r.CampaignStackMode); err != nil {
			return nil, apperr.Internal("scan promotion", err)
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func validCampaignStatus(v string) bool {
	switch strings.ToUpper(strings.TrimSpace(v)) {
	case campaignStatusDraft, campaignStatusActive, campaignStatusPaused, campaignStatusEnded:
		return true
	default:
		return false
	}
}

func validStackMode(v string) bool {
	switch strings.ToUpper(strings.TrimSpace(v)) {
	case "PRIORITY", "BEST", "EXCLUSIVE":
		return true
	default:
		return false
	}
}

func validRuleStatus(v string) bool {
	switch strings.ToUpper(strings.TrimSpace(v)) {
	case ruleStatusDraft, ruleStatusActive, ruleStatusPaused:
		return true
	default:
		return false
	}
}

func validateRulePayload(rule *pb.PromotionRule) error {
	if rule == nil || strings.TrimSpace(rule.GetCode()) == "" || strings.TrimSpace(rule.GetName()) == "" {
		return apperr.Invalid("rule code and name are required")
	}
	if rule.GetCampaignId() <= 0 {
		return apperr.Invalid("campaign_id is required")
	}
	if rule.GetEndAtUnix() <= rule.GetStartAtUnix() {
		return apperr.Invalid("promotion end must be after start")
	}
	switch strings.ToUpper(strings.TrimSpace(rule.GetType())) {
	case "FIXED_AMOUNT", "PERCENT", "THRESHOLD", "FREE_SHIPPING":
	default:
		return apperr.Invalid("unsupported promotion type")
	}
	var effect promotionEffect
	if err := json.Unmarshal([]byte(rule.GetEffectJson()), &effect); err != nil {
		return apperr.Invalid("invalid effect_json")
	}
	if strings.EqualFold(rule.GetType(), "PERCENT") && (effect.Percent <= 0 || effect.Percent > 100) {
		return apperr.Invalid("percent must be between 1 and 100")
	}
	if strings.EqualFold(rule.GetType(), "FIXED_AMOUNT") && effect.AmountCent <= 0 {
		return apperr.Invalid("fixed amount must be positive")
	}
	if strings.EqualFold(rule.GetType(), "THRESHOLD") && len(effect.Tiers) == 0 {
		return apperr.Invalid("threshold promotion needs tiers")
	}
	if _, err := parseScope(rule.GetScopeJson()); err != nil {
		return apperr.Invalid("invalid scope_json")
	}
	if condition := strings.TrimSpace(rule.GetConditionExpr()); condition != "" {
		if _, err := exprCompile(condition); err != nil {
			return apperr.Invalid("invalid condition_expr: %v", err)
		}
	}
	return nil
}

func (s *PromotionService) CreateCampaign(ctx context.Context, c *pb.Campaign) (*pb.Campaign, error) {
	if c == nil || strings.TrimSpace(c.GetCode()) == "" || strings.TrimSpace(c.GetName()) == "" {
		return nil, apperr.Invalid("campaign code and name are required")
	}
	status := strings.ToUpper(strings.TrimSpace(c.GetStatus()))
	if status == "" {
		status = campaignStatusDraft
	}
	if !validCampaignStatus(status) {
		return nil, apperr.Invalid("invalid campaign status")
	}
	if c.GetEndAtUnix() <= c.GetStartAtUnix() {
		return nil, apperr.Invalid("campaign end must be after start")
	}
	if !validStackMode(defaultString(c.GetStackMode(), "PRIORITY")) {
		return nil, apperr.Invalid("invalid stack_mode")
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO promotion_campaigns
		(code,name,type,status,start_at,end_at,priority,budget_cent,per_user_limit,stack_mode)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		strings.TrimSpace(c.GetCode()), strings.TrimSpace(c.GetName()), defaultString(c.GetType(), "NORMAL"),
		status, time.Unix(c.GetStartAtUnix(), 0), time.Unix(c.GetEndAtUnix(), 0), c.GetPriority(),
		c.GetBudgetCent(), c.GetPerUserLimit(), defaultString(c.GetStackMode(), "PRIORITY"))
	if err != nil {
		return nil, apperr.Internal("create campaign", err)
	}
	c.Id, _ = res.LastInsertId()
	c.Status = status
	return c, nil
}

func (s *PromotionService) UpdateCampaign(ctx context.Context, c *pb.Campaign) (*pb.Campaign, error) {
	if c == nil || c.GetId() <= 0 || strings.TrimSpace(c.GetCode()) == "" || strings.TrimSpace(c.GetName()) == "" {
		return nil, apperr.Invalid("campaign id, code and name are required")
	}
	var currentStatus string
	if err := s.db.QueryRowContext(ctx, `SELECT status FROM promotion_campaigns WHERE id = ?`, c.GetId()).Scan(&currentStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("campaign not found")
		}
		return nil, apperr.Internal("load campaign", err)
	}
	status := strings.ToUpper(strings.TrimSpace(c.GetStatus()))
	if status == "" {
		status = currentStatus
	}
	if !validCampaignStatus(status) {
		return nil, apperr.Invalid("invalid campaign status")
	}
	if c.GetEndAtUnix() <= c.GetStartAtUnix() {
		return nil, apperr.Invalid("campaign end must be after start")
	}
	if !validStackMode(defaultString(c.GetStackMode(), "PRIORITY")) {
		return nil, apperr.Invalid("invalid stack_mode")
	}
	_, err := s.db.ExecContext(ctx, `UPDATE promotion_campaigns SET code=?,name=?,type=?,status=?,start_at=?,end_at=?,
		priority=?,budget_cent=?,per_user_limit=?,stack_mode=?,version=version+1 WHERE id=?`,
		strings.TrimSpace(c.GetCode()), strings.TrimSpace(c.GetName()), defaultString(c.GetType(), "NORMAL"),
		status, time.Unix(c.GetStartAtUnix(), 0), time.Unix(c.GetEndAtUnix(), 0), c.GetPriority(),
		c.GetBudgetCent(), c.GetPerUserLimit(), defaultString(c.GetStackMode(), "PRIORITY"), c.GetId())
	if err != nil {
		return nil, apperr.Internal("update campaign", err)
	}
	c.Status = status
	return c, nil
}

func (s *PromotionService) SetCampaignStatus(ctx context.Context, req *pb.SetPromotionStatusRequest) (*pb.Empty, error) {
	status := strings.ToUpper(strings.TrimSpace(req.GetStatus()))
	if req.GetId() <= 0 || !validCampaignStatus(status) {
		return nil, apperr.Invalid("valid campaign id and status are required")
	}
	res, err := s.db.ExecContext(ctx, `UPDATE promotion_campaigns SET status=?,version=version+1 WHERE id=?`, status, req.GetId())
	if err != nil {
		return nil, apperr.Internal("set campaign status", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, apperr.NotFound("campaign not found")
	}
	return &pb.Empty{}, nil
}

func (s *PromotionService) ListCampaigns(ctx context.Context, _ *pb.Empty) (*pb.CampaignList, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,code,name,type,status,start_at,end_at,priority,budget_cent,
		reserved_cent,consumed_cent,per_user_limit,stack_mode,version FROM promotion_campaigns ORDER BY id DESC`)
	if err != nil {
		return nil, apperr.Internal("list campaigns", err)
	}
	defer rows.Close()
	out := &pb.CampaignList{}
	for rows.Next() {
		c := &pb.Campaign{}
		var start, end time.Time
		if err := rows.Scan(&c.Id, &c.Code, &c.Name, &c.Type, &c.Status, &start, &end, &c.Priority,
			&c.BudgetCent, &c.ReservedCent, &c.ConsumedCent, &c.PerUserLimit, &c.StackMode, &c.Version); err != nil {
			return nil, apperr.Internal("scan campaign", err)
		}
		c.StartAtUnix, c.EndAtUnix = start.Unix(), end.Unix()
		out.Campaigns = append(out.Campaigns, c)
	}
	return out, rows.Err()
}

func (s *PromotionService) CreatePromotion(ctx context.Context, r *pb.PromotionRule) (*pb.PromotionRule, error) {
	if err := validateRulePayload(r); err != nil {
		return nil, err
	}
	if err := s.requireCampaign(ctx, r.GetCampaignId()); err != nil {
		return nil, err
	}
	status := strings.ToUpper(strings.TrimSpace(r.GetStatus()))
	if status == "" {
		status = ruleStatusDraft
	}
	if !validRuleStatus(status) {
		return nil, apperr.Invalid("invalid promotion status")
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO promotion_rules
		(campaign_id,code,name,type,status,priority,exclusive,stack_group,scope_json,condition_expr,effect_json,
		 max_discount_cent,budget_cent,per_user_limit,allow_with_coupon,start_at,end_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.GetCampaignId(), strings.TrimSpace(r.GetCode()), strings.TrimSpace(r.GetName()), strings.ToUpper(strings.TrimSpace(r.GetType())),
		status, r.GetPriority(), r.GetExclusive(), strings.TrimSpace(r.GetStackGroup()), defaultString(r.GetScopeJson(), `{"all":true}`),
		strings.TrimSpace(r.GetConditionExpr()), r.GetEffectJson(), r.GetMaxDiscountCent(), r.GetBudgetCent(),
		r.GetPerUserLimit(), r.GetAllowWithCoupon(), time.Unix(r.GetStartAtUnix(), 0), time.Unix(r.GetEndAtUnix(), 0))
	if err != nil {
		return nil, apperr.Internal("create promotion", err)
	}
	r.Id, _ = res.LastInsertId()
	r.Status = status
	return r, nil
}

func (s *PromotionService) UpdatePromotion(ctx context.Context, r *pb.PromotionRule) (*pb.PromotionRule, error) {
	if r == nil || r.GetId() <= 0 {
		return nil, apperr.Invalid("promotion id is required")
	}
	if err := validateRulePayload(r); err != nil {
		return nil, err
	}
	var currentStatus string
	if err := s.db.QueryRowContext(ctx, `SELECT status FROM promotion_rules WHERE id = ?`, r.GetId()).Scan(&currentStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("promotion not found")
		}
		return nil, apperr.Internal("load promotion", err)
	}
	status := strings.ToUpper(strings.TrimSpace(r.GetStatus()))
	if status == "" {
		status = currentStatus
	}
	if !validRuleStatus(status) {
		return nil, apperr.Invalid("invalid promotion status")
	}
	_, err := s.db.ExecContext(ctx, `UPDATE promotion_rules SET campaign_id=?,code=?,name=?,type=?,status=?,priority=?,
		exclusive=?,stack_group=?,scope_json=?,condition_expr=?,effect_json=?,max_discount_cent=?,budget_cent=?,
		per_user_limit=?,allow_with_coupon=?,start_at=?,end_at=?,version=version+1 WHERE id=?`,
		r.GetCampaignId(), strings.TrimSpace(r.GetCode()), strings.TrimSpace(r.GetName()), strings.ToUpper(strings.TrimSpace(r.GetType())),
		status, r.GetPriority(), r.GetExclusive(), strings.TrimSpace(r.GetStackGroup()), defaultString(r.GetScopeJson(), `{"all":true}`),
		strings.TrimSpace(r.GetConditionExpr()), r.GetEffectJson(), r.GetMaxDiscountCent(), r.GetBudgetCent(),
		r.GetPerUserLimit(), r.GetAllowWithCoupon(), time.Unix(r.GetStartAtUnix(), 0), time.Unix(r.GetEndAtUnix(), 0), r.GetId())
	if err != nil {
		return nil, apperr.Internal("update promotion", err)
	}
	r.Status = status
	return r, nil
}

func (s *PromotionService) SetPromotionStatus(ctx context.Context, req *pb.SetPromotionStatusRequest) (*pb.Empty, error) {
	status := strings.ToUpper(strings.TrimSpace(req.GetStatus()))
	if req.GetId() <= 0 || !validRuleStatus(status) {
		return nil, apperr.Invalid("valid promotion id and status are required")
	}
	res, err := s.db.ExecContext(ctx, `UPDATE promotion_rules SET status=?,version=version+1 WHERE id=?`, status, req.GetId())
	if err != nil {
		return nil, apperr.Internal("set promotion status", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, apperr.NotFound("promotion not found")
	}
	return &pb.Empty{}, nil
}

func (s *PromotionService) ListPromotions(ctx context.Context, req *pb.PromotionListRequest) (*pb.PromotionRuleList, error) {
	page, size := req.GetPage(), req.GetPageSize()
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	where := "WHERE 1=1"
	args := []any{}
	if req.GetCampaignId() > 0 {
		where += " AND campaign_id=?"
		args = append(args, req.GetCampaignId())
	}
	if status := strings.TrimSpace(req.GetStatus()); status != "" {
		where += " AND status=?"
		args = append(args, strings.ToUpper(status))
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM promotion_rules "+where, args...).Scan(&total); err != nil {
		return nil, apperr.Internal("count promotions", err)
	}
	args = append(args, size, (page-1)*size)
	rows, err := s.db.QueryContext(ctx, `SELECT id,campaign_id,code,name,type,status,priority,exclusive,stack_group,
		scope_json,condition_expr,effect_json,max_discount_cent,budget_cent,reserved_cent,consumed_cent,per_user_limit,
		allow_with_coupon,start_at,end_at,version FROM promotion_rules `+where+` ORDER BY id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, apperr.Internal("list promotions", err)
	}
	defer rows.Close()
	out := &pb.PromotionRuleList{Total: total}
	for rows.Next() {
		r := &pb.PromotionRule{}
		var start, end time.Time
		if err := rows.Scan(&r.Id, &r.CampaignId, &r.Code, &r.Name, &r.Type, &r.Status, &r.Priority, &r.Exclusive,
			&r.StackGroup, &r.ScopeJson, &r.ConditionExpr, &r.EffectJson, &r.MaxDiscountCent, &r.BudgetCent,
			&r.ReservedCent, &r.ConsumedCent, &r.PerUserLimit, &r.AllowWithCoupon, &start, &end, &r.Version); err != nil {
			return nil, apperr.Internal("scan promotion", err)
		}
		r.StartAtUnix, r.EndAtUnix = start.Unix(), end.Unix()
		out.Promotions = append(out.Promotions, r)
	}
	return out, rows.Err()
}

func (s *PromotionService) requireCampaign(ctx context.Context, id int64) error {
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM promotion_campaigns WHERE id = ?`, id).Scan(&exists); err != nil {
		return apperr.Internal("query campaign", err)
	}
	if exists == 0 {
		return apperr.NotFound("campaign not found")
	}
	return nil
}

func (s *PromotionService) ReservePromotions(ctx context.Context, req *pb.PromotionReserveRequest) (*pb.PromotionReserveReply, error) {
	quoteID, orderNo := strings.TrimSpace(req.GetQuoteId()), strings.TrimSpace(req.GetOrderNo())
	if quoteID == "" || orderNo == "" {
		return nil, apperr.Invalid("quote_id and order_no are required")
	}
	q, err := loadQuote(ctx, s.db, quoteID)
	if err != nil {
		return nil, err
	}
	if req.GetUserId() > 0 && q.UserID != req.GetUserId() {
		return nil, apperr.Forbidden("quote does not belong to user")
	}
	if !q.ExpiresAt.After(s.now()) {
		return nil, apperr.Failed("quote expired")
	}
	var applied []*pb.AppliedPromotion
	if strings.TrimSpace(q.AppliedPromotionsJSON) != "" {
		if err := json.Unmarshal([]byte(q.AppliedPromotionsJSON), &applied); err != nil {
			return nil, apperr.Internal("decode applied promotions", err)
		}
	}
	if len(applied) == 0 {
		return &pb.PromotionReserveReply{}, nil
	}
	var reservations []string
	var total int64
	err = withTx(ctx, s.db, func(tx *sql.Tx) error {
		for _, a := range applied {
			if a.GetAmountCent() <= 0 && a.GetShippingCent() <= 0 {
				continue
			}
			amount := a.GetAmountCent()
			key := orderNo + ":" + fmt.Sprint(a.GetRuleId())
			var existing string
			err := tx.QueryRowContext(ctx, `SELECT reservation_no FROM promotion_reservations WHERE idempotency_key=?`, key).Scan(&existing)
			if err == nil {
				reservations = append(reservations, existing)
				total += amount
				continue
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return apperr.Internal("query promotion reservation", err)
			}
			var rule promotionRule
			if err := tx.QueryRowContext(ctx, `SELECT id,campaign_id,code,status,budget_cent,reserved_cent,consumed_cent,per_user_limit
				FROM promotion_rules WHERE id=? FOR UPDATE`, a.GetRuleId()).Scan(&rule.ID, &rule.CampaignID, &rule.Code, &rule.Status, &rule.BudgetCent, &rule.ReservedCent, &rule.ConsumedCent, &rule.PerUserLimit); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return apperr.NotFound("promotion rule not found")
				}
				return apperr.Internal("lock promotion rule", err)
			}
			if rule.Status != ruleStatusActive {
				return apperr.Failed("promotion is no longer active")
			}
			var camp promotionRule
			if err := tx.QueryRowContext(ctx, `SELECT budget_cent,reserved_cent,consumed_cent,per_user_limit
				FROM promotion_campaigns WHERE id=? FOR UPDATE`, a.GetCampaignId()).Scan(&camp.CampaignBudget, &camp.CampaignReserved, &camp.CampaignConsumed, &camp.PerUserLimit); err != nil {
				return apperr.Internal("lock campaign budget", err)
			}
			if rule.BudgetCent > 0 && rule.ReservedCent+rule.ConsumedCent+amount > rule.BudgetCent {
				return apperr.Failed("promotion budget exhausted")
			}
			if camp.CampaignBudget > 0 && camp.CampaignReserved+camp.CampaignConsumed+amount > camp.CampaignBudget {
				return apperr.Failed("campaign budget exhausted")
			}
			if rule.PerUserLimit > 0 {
				var used int64
				if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM promotion_reservations
					WHERE order_no<>? AND user_id=? AND rule_id=? AND status IN ('HELD','CONFIRMED')`, orderNo, q.UserID, rule.ID).Scan(&used); err != nil {
					return apperr.Internal("count user promotions", err)
				}
				if used >= int64(rule.PerUserLimit) {
					return apperr.Failed("promotion per-user limit reached")
				}
			}
			if camp.PerUserLimit > 0 {
				var used int64
				if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM promotion_reservations
					WHERE order_no<>? AND user_id=? AND campaign_id=? AND status IN ('HELD','CONFIRMED')`, orderNo, q.UserID, a.GetCampaignId()).Scan(&used); err != nil {
					return apperr.Internal("count campaign user promotions", err)
				}
				if used >= int64(camp.PerUserLimit) {
					return apperr.Failed("campaign per-user limit reached")
				}
			}
			resNo := id.New("PR")
			if _, err := tx.ExecContext(ctx, `INSERT INTO promotion_reservations
				(reservation_no,quote_id,order_no,user_id,rule_id,campaign_id,amount_cent,status,idempotency_key,expires_at)
				VALUES (?,?,?,?,?,?,?,?,?,?)`, resNo, quoteID, orderNo, q.UserID, rule.ID, a.GetCampaignId(), amount, "HELD", key, q.ExpiresAt); err != nil {
				if isDuplicateKey(err) {
					var winner string
					if err := tx.QueryRowContext(ctx, `SELECT reservation_no FROM promotion_reservations WHERE idempotency_key=?`, key).Scan(&winner); err == nil {
						reservations = append(reservations, winner)
						total += amount
						continue
					}
				}
				return apperr.Internal("insert promotion reservation", err)
			}
			for _, alloc := range a.GetAllocations() {
				if _, err := tx.ExecContext(ctx, `INSERT INTO promotion_reservation_items (reservation_id,sku_id,discount_cent)
					SELECT id,?,? FROM promotion_reservations WHERE idempotency_key=?`, alloc.GetSkuId(), alloc.GetDiscountCent(), key); err != nil {
					return apperr.Internal("insert promotion reservation item", err)
				}
			}
			if _, err := tx.ExecContext(ctx, `UPDATE promotion_rules SET reserved_cent=reserved_cent+? WHERE id=?`, amount, rule.ID); err != nil {
				return apperr.Internal("reserve rule budget", err)
			}
			if _, err := tx.ExecContext(ctx, `UPDATE promotion_campaigns SET reserved_cent=reserved_cent+? WHERE id=?`, amount, a.GetCampaignId()); err != nil {
				return apperr.Internal("reserve campaign budget", err)
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO promotion_budget_ledger
				(reservation_no,rule_id,campaign_id,amount_cent,entry_type) VALUES (?,?,?,?,?)`,
				resNo, rule.ID, a.GetCampaignId(), amount, "RESERVE"); err != nil {
				return apperr.Internal("append promotion ledger", err)
			}
			reservations = append(reservations, resNo)
			total += amount
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &pb.PromotionReserveReply{ReservationNos: reservations, ReservedCent: total}, nil
}

func (s *PromotionService) ConfirmPromotions(ctx context.Context, req *pb.PromotionOrderRequest) (*pb.Empty, error) {
	return &pb.Empty{}, s.settlePromotions(ctx, strings.TrimSpace(req.GetOrderNo()), "CONFIRMED")
}

func (s *PromotionService) ReleasePromotions(ctx context.Context, req *pb.PromotionOrderRequest) (*pb.Empty, error) {
	return &pb.Empty{}, s.settlePromotions(ctx, strings.TrimSpace(req.GetOrderNo()), "RELEASED")
}

func (s *PromotionService) settlePromotions(ctx context.Context, orderNo, target string) error {
	if orderNo == "" {
		return apperr.Invalid("order_no is required")
	}
	return withTx(ctx, s.db, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT id,reservation_no,rule_id,campaign_id,amount_cent,status
			FROM promotion_reservations WHERE order_no=? FOR UPDATE`, orderNo)
		if err != nil {
			return apperr.Internal("lock promotions", err)
		}
		type row struct {
			id, ruleID, campaignID, amount int64
			no, status                     string
		}
		var list []row
		for rows.Next() {
			var r row
			if err := rows.Scan(&r.id, &r.no, &r.ruleID, &r.campaignID, &r.amount, &r.status); err != nil {
				rows.Close()
				return apperr.Internal("scan promotion reservation", err)
			}
			list = append(list, r)
		}
		rows.Close()
		for _, r := range list {
			if r.status != "HELD" {
				continue
			}
			if _, err := tx.ExecContext(ctx, `UPDATE promotion_reservations SET status=?,updated_at=? WHERE id=?`, target, time.Now(), r.id); err != nil {
				return apperr.Internal("update promotion reservation", err)
			}
			if target == "CONFIRMED" {
				if _, err := tx.ExecContext(ctx, `UPDATE promotion_rules SET reserved_cent=GREATEST(reserved_cent-?,0), consumed_cent=consumed_cent+? WHERE id=?`, r.amount, r.amount, r.ruleID); err != nil {
					return apperr.Internal("consume rule budget", err)
				}
				if _, err := tx.ExecContext(ctx, `UPDATE promotion_campaigns SET reserved_cent=GREATEST(reserved_cent-?,0), consumed_cent=consumed_cent+? WHERE id=?`, r.amount, r.amount, r.campaignID); err != nil {
					return apperr.Internal("consume campaign budget", err)
				}
			} else {
				if _, err := tx.ExecContext(ctx, `UPDATE promotion_rules SET reserved_cent=GREATEST(reserved_cent-?,0) WHERE id=?`, r.amount, r.ruleID); err != nil {
					return apperr.Internal("release rule budget", err)
				}
				if _, err := tx.ExecContext(ctx, `UPDATE promotion_campaigns SET reserved_cent=GREATEST(reserved_cent-?,0) WHERE id=?`, r.amount, r.campaignID); err != nil {
					return apperr.Internal("release campaign budget", err)
				}
			}
			entry := "CONSUME"
			if target == "RELEASED" {
				entry = "RELEASE"
			}
			if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO promotion_budget_ledger
				(reservation_no,rule_id,campaign_id,amount_cent,entry_type) VALUES (?,?,?,?,?)`,
				r.no, r.ruleID, r.campaignID, r.amount, entry); err != nil {
				return apperr.Internal("append promotion ledger", err)
			}
		}
		return nil
	})
}

func (s *PromotionService) ConfirmByOrder(ctx context.Context, orderNo string) error {
	if err := s.settlePromotions(ctx, orderNo, "CONFIRMED"); err != nil {
		return err
	}
	return nil
}

func (s *PromotionService) ReleaseByOrder(ctx context.Context, orderNo string) error {
	if err := s.settlePromotions(ctx, orderNo, "RELEASED"); err != nil {
		return err
	}
	return nil
}

func (s *PromotionService) ListPromotionReservations(ctx context.Context, req *pb.ListPromotionReservationsRequest) (*pb.PromotionReservationList, error) {
	page, size := req.GetPage(), req.GetPageSize()
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	where := "WHERE 1=1"
	args := []any{}
	if orderNo := strings.TrimSpace(req.GetOrderNo()); orderNo != "" {
		where += " AND order_no=?"
		args = append(args, orderNo)
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM promotion_reservations "+where, args...).Scan(&total); err != nil {
		return nil, apperr.Internal("count reservations", err)
	}
	args = append(args, size, (page-1)*size)
	rows, err := s.db.QueryContext(ctx, `SELECT reservation_no,quote_id,order_no,user_id,rule_id,campaign_id,amount_cent,status,created_at
		FROM promotion_reservations `+where+` ORDER BY id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, apperr.Internal("list reservations", err)
	}
	defer rows.Close()
	out := &pb.PromotionReservationList{Total: total}
	for rows.Next() {
		r := &pb.PromotionReservation{}
		var created time.Time
		if err := rows.Scan(&r.ReservationNo, &r.QuoteId, &r.OrderNo, &r.UserId, &r.RuleId, &r.CampaignId, &r.AmountCent, &r.Status, &created); err != nil {
			return nil, apperr.Internal("scan reservation", err)
		}
		r.CreatedAtUnix = created.Unix()
		out.Reservations = append(out.Reservations, r)
	}
	return out, rows.Err()
}

func withTx(ctx context.Context, database *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return apperr.Internal("begin promotion transaction", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return apperr.Internal("commit promotion transaction", err)
	}
	return nil
}

func isDuplicateKey(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

func defaultString(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

// ReleaseExpired releases HELD reservations whose quote/order window has
// elapsed. It is safe to run periodically on every pricing replica because the
// settle path locks the rows and only transitions HELD reservations.
func (s *PromotionService) ReleaseExpired(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT order_no FROM promotion_reservations
		WHERE status='HELD' AND expires_at IS NOT NULL AND expires_at <= NOW(3) ORDER BY id LIMIT ?`, limit)
	if err != nil {
		return 0, apperr.Internal("scan expired promotions", err)
	}
	var orders []string
	for rows.Next() {
		var orderNo string
		if err := rows.Scan(&orderNo); err == nil && orderNo != "" {
			orders = append(orders, orderNo)
		}
	}
	rows.Close()
	released := 0
	for _, orderNo := range orders {
		if err := s.settlePromotions(ctx, orderNo, "RELEASED"); err != nil {
			s.log.Warn("release expired promotion", "order_no", orderNo, "error", err)
			continue
		}
		released++
	}
	return released, rows.Err()
}
