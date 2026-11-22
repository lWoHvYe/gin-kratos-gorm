package aftersales

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

const (
	partyMerchant     = "MERCHANT_WAREHOUSE"
	partyCarrier      = "CARRIER"
	partyCustomer     = "CUSTOMER"
	partyPlatform     = "PLATFORM"
	partyUndetermined = "UNDETERMINED"

	sourceAgent           = "AGENT"
	sourceCarrierEvidence = "CARRIER_EVIDENCE"
	sourceUserDeclaration = "USER_DECLARATION"
	sourceSystemValue     = "SYSTEM"

	freightStateHeld     = "HELD"
	freightStateRefunded = "REFUNDED"
	freightStateWaived   = "WAIVED"

	freightHoldWindow = 168 * time.Hour
)

var reasonCodes = map[string]map[string]struct{}{
	partyMerchant: {
		"OUT_OF_STOCK": {}, "WRONG_ITEM": {}, "MISSING_ITEM": {}, "DAMAGED_PACKAGING": {}, "LATE_DISPATCH": {},
	},
	partyCarrier: {
		"LOST_IN_TRANSIT": {}, "DAMAGED_IN_TRANSIT": {}, "WRONG_DELIVERY": {},
	},
	partyCustomer: {
		"NO_REASON_RETURN": {}, "REFUSED": {}, "ADDRESS_ERROR": {},
	},
	partyPlatform: {
		"SYSTEM_DEFECT": {}, "PRICING_ERROR": {},
	},
	partyUndetermined: {"PENDING_REVIEW": {}},
}

func validParty(party string) bool {
	switch party {
	case partyMerchant, partyCarrier, partyCustomer, partyPlatform, partyUndetermined:
		return true
	default:
		return false
	}
}

func validReason(party, code string) bool {
	codes, ok := reasonCodes[party]
	if !ok {
		return false
	}
	_, ok = codes[code]
	return ok
}

// DetermineResponsibility is the only manual responsibility write path. It
// requires a closed reason-code enum and evidence; free-text reason is never an
// input (INV-29/AC-33).
func (s *Service) DetermineResponsibility(ctx context.Context, req *pb.DetermineResponsibilityRequest) (*pb.AfterSale, error) {
	afterSaleNo := strings.TrimSpace(req.GetAfterSaleNo())
	party := strings.ToUpper(strings.TrimSpace(req.GetParty()))
	source := strings.ToUpper(strings.TrimSpace(req.GetSource()))
	code := strings.ToUpper(strings.TrimSpace(req.GetReasonCode()))
	operator := strings.TrimSpace(req.GetOperator())
	evidence := strings.TrimSpace(req.GetEvidenceRef())
	if afterSaleNo == "" || !validParty(party) || !validReason(party, code) {
		return nil, apperr.Invalid("invalid responsibility party or reason_code")
	}
	if party == partyUndetermined {
		return nil, apperr.Invalid("agent cannot set responsibility back to UNDETERMINED")
	}
	if source != sourceAgent && source != sourceCarrierEvidence && source != sourceUserDeclaration {
		return nil, apperr.Invalid("source must be AGENT, CARRIER_EVIDENCE or USER_DECLARATION")
	}
	if operator == "" || evidence == "" {
		return nil, apperr.Invalid("evidence_ref and operator are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin responsibility determination", err)
	}
	defer func() { _ = tx.Rollback() }()
	row, err := lockAfterSale(ctx, tx, afterSaleNo)
	if err != nil {
		return nil, err
	}
	if row.status == statusRefunded {
		return nil, apperr.Failed("cannot change responsibility after refund")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE after_sales
		SET responsibility_party = ?, responsibility_source = ?, responsibility_evidence_ref = ?,
		    responsibility_reason_code = ?, responsibility_determined_by = ?, responsibility_determined_at = NOW(3),
		    version = version + 1 WHERE id = ?`,
		party, source, evidence, code, operator, row.id); err != nil {
		return nil, apperr.Internal("determine responsibility", err)
	}
	if err := appendLog(ctx, tx, row.id, "RESPONSIBILITY", row.status, row.status, party+" "+code+" "+evidence); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, apperr.Internal("commit responsibility", err)
	}
	if err := s.applyFreightHold(ctx, afterSaleNo, party, code); err != nil {
		return nil, err
	}
	return s.loadAfterSale(ctx, afterSaleNo)
}

// applyFreightHold settles a pending freight hold after responsibility is
// determined. Product refund is never blocked by this call.
func (s *Service) applyFreightHold(ctx context.Context, afterSaleNo, party, code string) error {
	var freight int64
	var status string
	err := s.db.QueryRowContext(ctx, `SELECT freight_cent, status FROM after_sale_freight_holds
		WHERE after_sale_no = ?`, afterSaleNo).Scan(&freight, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return apperr.Internal("load freight hold", err)
	}
	if status != freightStateHeld {
		return nil
	}
	switch party {
	case partyCustomer:
		_, err = s.db.ExecContext(ctx, `UPDATE after_sale_freight_holds
			SET status = ?, determined_party = ?, determined_at = NOW(3), version = version + 1
			WHERE after_sale_no = ? AND status = ?`, freightStateWaived, party, afterSaleNo, freightStateHeld)
		return err
	case partyMerchant, partyCarrier, partyPlatform:
		return s.refundFreightHold(ctx, afterSaleNo, party, code)
	default:
		return nil
	}
}

// refundFreightHold pays the held freight once. The idempotency key is
// after_sale_no:freight so it never collides with the product refund.
func (s *Service) refundFreightHold(ctx context.Context, afterSaleNo, party, code string) error {
	row, err := s.getAfterSaleRow(ctx, afterSaleNo)
	if err != nil {
		return err
	}
	var freight int64
	var status string
	err = s.db.QueryRowContext(ctx, `SELECT freight_cent, status FROM after_sale_freight_holds
		WHERE after_sale_no = ?`, afterSaleNo).Scan(&freight, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return apperr.Internal("load freight hold", err)
	}
	if status == freightStateRefunded || status == freightStateWaived || freight <= 0 {
		return nil
	}
	if s.payments == nil {
		return apperr.Unavailable("payment client is not configured for freight refund", nil)
	}
	groupNo, err := s.groupForAfterSale(ctx, afterSaleNo)
	if err != nil {
		return err
	}
	rule := "MERCHANT_FULL_FREIGHT"
	if code == "PENDING_REVIEW" || code == "TIMEOUT" {
		rule = "FREIGHT_HOLD_TIMEOUT_MERCHANT"
	}
	reply, err := s.payments.RefundOrder(ctx, &pb.RefundOrderRequest{
		OrderNo: row.orderNo, AmountCent: freight, Reason: "after-sale freight " + afterSaleNo,
		IdempotencyKey: afterSaleNo + ":freight", GroupNo: groupNo, AfterSaleNo: afterSaleNo,
		GoodsCent: 0, FreightCent: freight, RuleId: rule,
	})
	if err != nil {
		return apperr.Unavailable("freight refund failed; retry is safe", err)
	}
	refundNo := ""
	if reply != nil {
		refundNo = reply.GetRefundNo()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return apperr.Internal("begin freight refund completion", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE after_sale_freight_holds
		SET status = ?, determined_party = ?, determined_at = NOW(3), version = version + 1
		WHERE after_sale_no = ? AND status = ?`, freightStateRefunded, party, afterSaleNo, freightStateHeld); err != nil {
		return apperr.Internal("mark freight hold refunded", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE after_sale_refund_breakdowns
		SET freight_cent = ?, total_cent = goods_cent + ?, rule_id = ?, responsibility_party = ?,
		    refund_idempotency_key = ?
		WHERE after_sale_no = ?`,
		freight, freight, rule, party, afterSaleNo+":freight", afterSaleNo); err != nil {
		return apperr.Internal("update freight breakdown", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE after_sales SET refund_no = COALESCE(NULLIF(refund_no, ''), ?), version = version + 1 WHERE id = ?`,
		refundNo, row.id); err != nil {
		return apperr.Internal("record freight refund number", err)
	}
	return tx.Commit()
}

// freightRefundFor is the single 6.5.1 calculator. It returns the freight to
// refund now, or hold=true when responsibility is still UNDETERMINED.
func (s *Service) freightRefundFor(ctx context.Context, q lineQuery, order *pb.Order, groupNo, party string) (amount int64, hold bool, rule string, err error) {
	var groupShare int64
	for _, group := range order.GetFulfillmentGroups() {
		if group.GetGroupNo() == groupNo {
			groupShare = group.GetFreightShareCent()
			break
		}
	}
	var already int64
	if err := q.QueryRowContext(ctx, `SELECT COALESCE(SUM(b.freight_cent), 0)
		FROM after_sale_refund_breakdowns b JOIN after_sales a ON a.after_sale_no = b.after_sale_no
		WHERE a.order_no = ? AND a.status = ?`, order.GetOrderNo(), statusRefunded).Scan(&already); err != nil {
		return 0, false, "", apperr.Internal("load refunded freight", err)
	}
	remaining := order.GetFreightCent() - already
	if remaining < 0 {
		remaining = 0
	}
	switch party {
	case partyCustomer:
		return 0, false, "CUSTOMER_NO_FREIGHT", nil
	case partyMerchant, partyCarrier, partyPlatform:
		if allOtherGroupsClosed(order, groupNo) {
			amount = remaining
			rule = "MERCHANT_FULL_FREIGHT"
		} else {
			amount = groupShare
			if amount > remaining {
				amount = remaining
			}
			rule = "SHORTAGE_MERCHANT_FREIGHT_SHARE"
		}
		return amount, false, rule, nil
	default:
		if groupShare > remaining {
			groupShare = remaining
		}
		return groupShare, groupShare > 0, "FREIGHT_HOLD", nil
	}
}

func allOtherGroupsClosed(order *pb.Order, groupNo string) bool {
	for _, group := range order.GetFulfillmentGroups() {
		if group.GetGroupNo() == groupNo {
			continue
		}
		if group.GetStatus() != "CANCELLED" && group.GetRefundState() != "REFUNDED" {
			return false
		}
	}
	return true
}

// RunFreightHoldWorker is the 168h fail-safe. It is a deadline timer over
// freight holds, not a business-status scanner.
func RunFreightHoldWorker(ctx context.Context, database *sql.DB, svc *Service, logger *slog.Logger, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			svc.processExpiredFreightHolds(ctx, logger)
		}
	}
}

func (s *Service) processExpiredFreightHolds(ctx context.Context, logger *slog.Logger) {
	rows, err := s.db.QueryContext(ctx, `SELECT after_sale_no FROM after_sale_freight_holds
		WHERE status = ? AND deadline_at <= NOW(3) LIMIT 50`, freightStateHeld)
	if err != nil {
		if logger != nil {
			logger.Error("scan expired freight holds", "error", err)
		}
		return
	}
	var numbers []string
	for rows.Next() {
		var no string
		if err := rows.Scan(&no); err == nil {
			numbers = append(numbers, no)
		}
	}
	_ = rows.Close()
	for _, afterSaleNo := range numbers {
		if err := s.refundFreightHold(ctx, afterSaleNo, partyMerchant, "TIMEOUT"); err != nil && logger != nil {
			logger.Warn("freight hold refund failed", "after_sale_no", afterSaleNo, "error", err)
		}
	}
}
