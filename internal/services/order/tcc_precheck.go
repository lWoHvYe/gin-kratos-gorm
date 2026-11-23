package order

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

// This file implements the TCC Σ precheck only (SPEC-0002 v1.8 AC-65 / INV-48,
// ADR-0007 Amendment 1 §4.1.2). It is deliberately a read-only helper set:
//
//   (1) readTccPaymentSnapshot: the Payment-authoritative captured/refunded
//       snapshot, taken OUTSIDE any database transaction;
//   (2) computeTccPrecheck: the pure local calculation over the order, its
//       fulfillment groups, line_paid_cent and the RESERVED/SETTLED refund
//       commitments. It accepts any querier, so Tx1 can re-run it against the
//       locked order row without a second Payment round-trip;
//   (3) validateTccSum: the Σ equation itself.
//
// Nothing here writes a table, an outbox event or a state; the Tx1 writes
// (REFUNDING, batch, permits, admissions) are a separate task.

// tccPaymentSnapshot is the authoritative Payment fact for one effective
// payment. It is captured outside the transaction and carried into Tx1
// unchanged (ADR §4.1.2 step 1/3).
type tccPaymentSnapshot struct {
	PaymentNo    string
	CapturedCent int64
	RefundedCent int64
}

// tccGroupCap is one fulfillment group's contribution to the Σ equation.
// CapCent is the group's system-responsibility cap:
//
//	Σ line_paid_cent + 全部未退 freight_share - Σ (RESERVED|SETTLED) commitments
//
// Commitments are read at group level; composition is irrelevant for the group
// cap because amount_cent already equals goods + freight (ADR §4.3).
type tccGroupCap struct {
	GroupNo       string
	GroupStatus   string
	PermitState   string
	PaidCent      int64
	FreightCent   int64
	CommittedCent int64
	CapCent       int64
	Active        bool
}

// tccPrecheck is the read-only result of the Σ precheck.
type tccPrecheck struct {
	OrderNo               string
	PaymentNo             string
	PayCent               int64
	CapturedCent          int64
	RefundedCent          int64
	RemainingCapturedCent int64
	SumGroupCapCent       int64
	Groups                []tccGroupCap
	ActiveGroupCount      int
	// NoCaptured records AC-65's "订单无任何已捕获金额" branch: the caller must
	// cancel/zero-converge and must not run the per-group refund fan-out.
	NoCaptured bool
	// LegacyG1 records the AC-51/AC-65 fallback for historical orders without
	// fulfillment groups: the calculation used the single G1 group made of the
	// order lines and the order freight. Initializing the G1 permit is a Tx1
	// write and is not part of this helper.
	LegacyG1 bool
}

// DiffCent is SumGroupCapCent - RemainingCapturedCent (negative = short).
func (p *tccPrecheck) DiffCent() int64 {
	if p == nil {
		return 0
	}
	return p.SumGroupCapCent - p.RemainingCapturedCent
}

// readTccPaymentSnapshot loads the Payment-authoritative captured/refunded
// amounts for the compensation payment. paymentNo is the TCC payment identity
// (normally req.payment_no); when empty the order's settlement_payment_no is
// used. It must be called outside a DB transaction; a zero-amount order is
// answered from its `zero:` identity and never touches Payment (AC-40).
func (s *Service) readTccPaymentSnapshot(ctx context.Context, record *orderRecord, paymentNo string) (*tccPaymentSnapshot, error) {
	if record == nil {
		return nil, apperr.Internal("TCC precheck requires an order", nil)
	}
	paymentNo = strings.TrimSpace(paymentNo)
	if paymentNo == "" {
		paymentNo = strings.TrimSpace(record.SettlementPaymentNo)
	}
	if paymentNo == "" {
		// AC-63/INV-46: a PAID order without an effective payment must be
		// resolved by the legacy queue first; never guess a zero amount.
		return nil, apperr.Failed("order has no settlement payment; resolve historical payment first").
			WithDetails(map[string]any{"order_no": record.OrderNo, "pay_cent": record.PayCent})
	}
	if strings.HasPrefix(paymentNo, zeroPaymentPrefix) {
		if record.PayCent != 0 {
			return nil, apperr.Failed("zero settlement identity on a non-zero order").
				WithDetails(map[string]any{
					"order_no": record.OrderNo, "payment_no": paymentNo, "pay_cent": record.PayCent,
				})
		}
		if paymentNo != zeroPaymentPrefix+record.OrderNo {
			return nil, apperr.Failed("zero settlement identity does not match the order").
				WithDetails(map[string]any{"order_no": record.OrderNo, "payment_no": paymentNo})
		}
		return &tccPaymentSnapshot{PaymentNo: paymentNo}, nil
	}
	if s.payments == nil {
		return nil, apperr.Unavailable("payment client is required for the TCC refund precheck", nil)
	}
	intent, err := s.payments.GetIntent(ctx, &pb.GetIntentRequest{PaymentNo: paymentNo})
	if err != nil {
		return nil, apperr.Unavailable("load authoritative captured amount for TCC refund", err)
	}
	if intent == nil {
		return nil, apperr.Failed("payment intent is missing for the TCC refund precheck").
			WithDetails(map[string]any{"order_no": record.OrderNo, "payment_no": paymentNo})
	}
	if orderNo := strings.TrimSpace(intent.GetOrderNo()); orderNo != "" && orderNo != record.OrderNo {
		return nil, apperr.Failed("payment intent belongs to another order").
			WithDetails(map[string]any{
				"order_no": record.OrderNo, "payment_no": paymentNo, "intent_order_no": orderNo,
			})
	}
	if intent.GetCapturedCent() < 0 || intent.GetRefundedCent() < 0 {
		return nil, apperr.Failed("payment facts are negative").
			WithDetails(map[string]any{
				"payment_no": paymentNo, "captured_cent": intent.GetCapturedCent(),
				"refunded_cent": intent.GetRefundedCent(),
			})
	}
	if intent.GetRefundedCent() > intent.GetCapturedCent() {
		return nil, apperr.Failed("payment refunded amount exceeds the captured amount").
			WithDetails(map[string]any{
				"payment_no": paymentNo, "captured_cent": intent.GetCapturedCent(),
				"refunded_cent": intent.GetRefundedCent(),
			})
	}
	return &tccPaymentSnapshot{
		PaymentNo:    paymentNo,
		CapturedCent: intent.GetCapturedCent(),
		RefundedCent: intent.GetRefundedCent(),
	}, nil
}

// computeTccPrecheck calculates the Σ equation from local facts and a Payment
// snapshot. It performs no writes and no remote calls, so Tx1 can call it with
// *sql.Tx after locking the order row (ADR §4.1.2 step 3).
func computeTccPrecheck(ctx context.Context, q querier, record *orderRecord, snapshot *tccPaymentSnapshot) (*tccPrecheck, error) {
	if record == nil || snapshot == nil {
		return nil, apperr.Internal("TCC precheck requires an order and a payment snapshot", nil)
	}
	if settled := strings.TrimSpace(record.SettlementPaymentNo); settled != "" && snapshot.PaymentNo != settled {
		// The order was re-settled by another payment between the snapshot and
		// the locking transaction; the snapshot is no longer authoritative.
		// Before PAID, settlement_payment_no is still empty and the TCC payment
		// identity is the snapshot's own payment_no.
		return nil, apperr.Failed("TCC precheck payment snapshot is stale").
			WithDetails(map[string]any{
				"order_no": record.OrderNo, "snapshot_payment_no": snapshot.PaymentNo,
				"order_payment_no": record.SettlementPaymentNo,
			})
	}
	remaining := snapshot.CapturedCent - snapshot.RefundedCent
	if remaining < 0 {
		return nil, apperr.Failed("payment refunded amount exceeds the captured amount").
			WithDetails(map[string]any{
				"order_no": record.OrderNo, "payment_no": snapshot.PaymentNo,
				"captured_cent": snapshot.CapturedCent, "refunded_cent": snapshot.RefundedCent,
			})
	}
	result := &tccPrecheck{
		OrderNo:               record.OrderNo,
		PaymentNo:             snapshot.PaymentNo,
		PayCent:               record.PayCent,
		CapturedCent:          snapshot.CapturedCent,
		RefundedCent:          snapshot.RefundedCent,
		RemainingCapturedCent: remaining,
		NoCaptured:            snapshot.CapturedCent == 0,
	}
	// AC-65: no captured money means "cancel/zero converge", not a Σ refund.
	// For every compensated order the effective capture must equal pay_cent
	// (AC-39/INV-30); a mismatch (including pay_cent=0 with a capture, the AC-40
	// narrowing) is failed closed instead of being netted against the Σ.
	if !result.NoCaptured && snapshot.CapturedCent != record.PayCent {
		return nil, apperr.Failed(fmt.Sprintf(
			"effective captured amount %d does not equal pay_cent %d",
			snapshot.CapturedCent, record.PayCent)).
			WithDetails(map[string]any{
				"order_no": record.OrderNo, "payment_no": snapshot.PaymentNo,
				"captured_cent": snapshot.CapturedCent, "pay_cent": record.PayCent,
			})
	}
	paidByGroup, err := loadGroupPaidCent(ctx, q, record.OrderNo)
	if err != nil {
		return nil, apperr.Internal("sum group line paid amount", err)
	}
	committedByGroup, err := loadGroupCommittedCent(ctx, q, record.OrderNo)
	if err != nil {
		return nil, apperr.Internal("sum group refund commitments", err)
	}
	groups, err := loadFulfillmentGroups(ctx, q, record.OrderNo)
	if err != nil {
		return nil, apperr.Internal("load fulfillment groups for TCC precheck", err)
	}
	if len(groups) == 0 {
		// AC-51/AC-65: historical orders without fulfillment groups are treated
		// as one G1 group. The G1 permit itself is initialized by Tx1.
		var paid, committed int64
		for _, amount := range paidByGroup {
			paid += amount
		}
		for _, amount := range committedByGroup {
			committed += amount
		}
		capCent := paid + record.FreightCent - committed
		if capCent < 0 {
			return nil, apperr.Failed("group refund commitments exceed the group paid amount").
				WithDetails(map[string]any{
					"order_no": record.OrderNo, "group_no": defaultGroupNo,
					"paid_cent": paid, "freight_cent": record.FreightCent, "committed_cent": committed,
				})
		}
		result.LegacyG1 = true
		result.Groups = []tccGroupCap{{
			GroupNo: defaultGroupNo, PaidCent: paid, FreightCent: record.FreightCent,
			CommittedCent: committed, CapCent: capCent, Active: capCent > 0,
		}}
	} else {
		known := make(map[string]struct{}, len(groups))
		for _, group := range groups {
			known[group.GroupNo] = struct{}{}
			paid := paidByGroup[group.GroupNo]
			committed := committedByGroup[group.GroupNo]
			capCent := paid + group.FreightShareCent - committed
			if capCent < 0 {
				return nil, apperr.Failed("group refund commitments exceed the group paid amount").
					WithDetails(map[string]any{
						"order_no": record.OrderNo, "group_no": group.GroupNo,
						"paid_cent": paid, "freight_cent": group.FreightShareCent, "committed_cent": committed,
					})
			}
			result.Groups = append(result.Groups, tccGroupCap{
				GroupNo: group.GroupNo, GroupStatus: group.Status, PermitState: group.PermitState,
				PaidCent: paid, FreightCent: group.FreightShareCent, CommittedCent: committed,
				CapCent: capCent, Active: capCent > 0,
			})
		}
		for groupNo := range paidByGroup {
			if _, ok := known[groupNo]; !ok {
				return nil, apperr.Failed("order item belongs to an unknown fulfillment group").
					WithDetails(map[string]any{"order_no": record.OrderNo, "group_no": groupNo})
			}
		}
		for groupNo := range committedByGroup {
			if _, ok := known[groupNo]; !ok {
				return nil, apperr.Failed("refund commitment belongs to an unknown fulfillment group").
					WithDetails(map[string]any{"order_no": record.OrderNo, "group_no": groupNo})
			}
		}
	}
	for _, group := range result.Groups {
		result.SumGroupCapCent += group.CapCent
		if group.Active {
			result.ActiveGroupCount++
		}
	}
	return result, nil
}

// validateTccSum enforces the Σ equation on a computed precheck. It is shared
// by the out-of-transaction precheck and the Tx1 recheck so both use exactly the
// same rule.
func validateTccSum(result *tccPrecheck) error {
	if result == nil {
		return apperr.Internal("TCC precheck result is missing", nil)
	}
	if result.NoCaptured {
		// AC-65: no captured money → the caller cancels/zero-converges and must
		// not treat any group as active for the refund fan-out.
		return nil
	}
	if result.SumGroupCapCent != result.RemainingCapturedCent {
		return apperr.Failed(fmt.Sprintf(
			"TCC refund sum mismatch: sum_group_cap_cent=%d remaining_captured_cent=%d diff_cent=%d active_groups=%d",
			result.SumGroupCapCent, result.RemainingCapturedCent, result.DiffCent(), result.ActiveGroupCount)).
			WithDetails(map[string]any{
				"order_no": result.OrderNo, "payment_no": result.PaymentNo,
				"pay_cent": result.PayCent, "captured_cent": result.CapturedCent,
				"refunded_cent":           result.RefundedCent,
				"sum_group_cap_cent":      result.SumGroupCapCent,
				"remaining_captured_cent": result.RemainingCapturedCent,
				"diff_cent":               result.DiffCent(), "active_groups": result.ActiveGroupCount,
			})
	}
	return nil
}

// precheckTccSum is the AC-65 precheck entry point used before any state
// change: one Payment snapshot outside the transaction, the local Σ
// calculation, then the equation. A mismatch returns the computed result
// together with the error so the caller can log/audit the numbers; no table,
// event or state is modified.
func (s *Service) precheckTccSum(ctx context.Context, orderNo, paymentNo string) (*tccPrecheck, error) {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	record, err := findOrder(ctx, s.db, orderNo)
	if err != nil {
		return nil, apperr.Internal("load order for TCC precheck", err)
	}
	if record == nil {
		return nil, apperr.NotFound("order not found")
	}
	snapshot, err := s.readTccPaymentSnapshot(ctx, record, paymentNo)
	if err != nil {
		return nil, err
	}
	result, err := computeTccPrecheck(ctx, s.db, record, snapshot)
	if err != nil {
		return nil, err
	}
	if err := validateTccSum(result); err != nil {
		return result, err
	}
	return result, nil
}

// recheckTccSumTx re-runs the Σ precheck INSIDE Tx1 with the same Payment
// snapshot. It locks the order row, performs no remote call and no write, and
// fails closed when the order was re-settled between the snapshot and the lock.
// T1.2 calls this before writing REFUNDING/batch/permit state.
func (s *Service) recheckTccSumTx(ctx context.Context, tx *sql.Tx, orderNo string, snapshot *tccPaymentSnapshot) (*tccPrecheck, error) {
	if tx == nil {
		return nil, apperr.Internal("TCC Tx1 recheck requires a transaction", nil)
	}
	record, err := findOrderForUpdate(ctx, tx, orderNo)
	if err != nil {
		return nil, apperr.Internal("lock order for TCC recheck", err)
	}
	if record == nil {
		return nil, apperr.NotFound("order not found")
	}
	result, err := computeTccPrecheck(ctx, tx, record, snapshot)
	if err != nil {
		return nil, err
	}
	if err := validateTccSum(result); err != nil {
		return result, err
	}
	return result, nil
}

func loadGroupPaidCent(ctx context.Context, q querier, orderNo string) (map[string]int64, error) {
	rows, err := q.QueryContext(ctx, `SELECT fulfillment_group_no, COALESCE(SUM(line_paid_cent),0)
		FROM order_items WHERE order_no = ? GROUP BY fulfillment_group_no`, orderNo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int64, 2)
	for rows.Next() {
		var groupNo string
		var amount int64
		if err := rows.Scan(&groupNo, &amount); err != nil {
			return nil, err
		}
		groupNo = strings.TrimSpace(groupNo)
		if groupNo == "" {
			// order_items.fulfillment_group_no defaults to G1; an empty value
			// can only come from a legacy/manual insert.
			groupNo = defaultGroupNo
		}
		out[groupNo] += amount
	}
	return out, rows.Err()
}

func loadGroupCommittedCent(ctx context.Context, q querier, orderNo string) (map[string]int64, error) {
	rows, err := q.QueryContext(ctx, `SELECT group_no, COALESCE(SUM(amount_cent),0)
		FROM order_refund_commitments
		WHERE order_no = ? AND state IN ('RESERVED','SETTLED')
		GROUP BY group_no`, orderNo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int64, 2)
	for rows.Next() {
		var groupNo string
		var amount int64
		if err := rows.Scan(&groupNo, &amount); err != nil {
			return nil, err
		}
		out[strings.TrimSpace(groupNo)] += amount
	}
	return out, rows.Err()
}
