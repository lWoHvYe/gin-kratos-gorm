package order

import (
	"context"
	"database/sql"
	"sort"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/refundpolicy"
)

// refundComposition is the validated line composition of one AdmitRefund
// request. Goods + Freight is the request amount; Lines always includes the
// freight line (line_no=0) when freight is positive. A nil composition means
// the caller used the legacy amount-only request (no lines), which keeps the
// pre-T3.5 behavior and is only legal where the existing partial/whole-group
// rules allow it.
type refundComposition struct {
	Amount  int64
	Goods   int64
	Freight int64
	Source  string
	Lines   []tccCommitmentLine
}

// validateRefundComposition enforces the line half of INV-24/INV-47:
//
//	request line goods <= line_paid - RESERVED/SETTLED commitment goods
//	request line quantity <= line quantity - committed quantity
//	sum(goods) + freight == amount
//	freight <= group freight_share - committed freight
//
// A group that already has a composition_known=0 commitment cannot be
// attributed per line; it fails closed with composition lines rather than
// inventing an attribution. The responsibility/reason pair is enforced here;
// the 168h freight hold itself remains an AfterSales/T3.6 concern.
func validateRefundComposition(ctx context.Context, q querier, req *pb.AdmitRefundRequest,
	orderNo, groupNo string) (*refundComposition, error) {
	responsibility, err := validateRefundResponsibility(req)
	if err != nil {
		return nil, err
	}
	reqLines := req.GetLines()
	freight := req.GetFreightRefundableCent()
	if len(reqLines) == 0 {
		if freight != 0 {
			return nil, apperr.Invalid("freight_refundable_cent requires an explicit line composition").
				WithDetails(map[string]any{"order_no": orderNo, "group_no": groupNo})
		}
		return nil, nil
	}
	if freight < 0 {
		return nil, apperr.Invalid("freight_refundable_cent must not be negative")
	}
	if responsibility == refundpolicy.ResponsibilityUndetermined && freight > 0 {
		return nil, apperr.Failed("UNDETERMINED responsibility must hold freight; freight_refundable_cent must be 0").
			WithDetails(map[string]any{"order_no": orderNo, "group_no": groupNo, "freight_refundable_cent": freight})
	}

	// No prefix exclusion: an existing admission for this refund_id already
	// returned through the resume path; prefix matching would also exclude a
	// different refund such as R-10 when this request is R-1.
	comps, err := loadTccGroupComposition(ctx, q, orderNo, "")
	if err != nil {
		return nil, err
	}
	comp := comps[groupNo]
	if comp == nil {
		comp = &tccGroupComposition{LineGoods: map[int]int64{}, LineQuantity: map[int]int64{}}
	}
	if comp.UnknownComposition {
		return nil, apperr.Failed("group has a legacy commitment without line composition; only a whole-group amount-only refund is allowed").
			WithDetails(map[string]any{"order_no": orderNo, "group_no": groupNo})
	}
	linesByGroup, err := loadTccLinesByGroup(ctx, q, orderNo)
	if err != nil {
		return nil, err
	}
	orderLines := make(map[int]tccOrderLine, len(linesByGroup[groupNo]))
	for _, line := range linesByGroup[groupNo] {
		orderLines[int(line.LineNo)] = line
	}

	out := make([]tccCommitmentLine, 0, len(reqLines)+1)
	seen := make(map[int]struct{}, len(reqLines))
	var goodsTotal int64
	for _, line := range reqLines {
		lineNo := int(line.GetLineNo())
		quantity := int64(line.GetQuantity())
		goods := line.GetGoodsCent()
		if lineNo <= 0 {
			return nil, apperr.Invalid("refund line_no must be positive").
				WithDetails(map[string]any{"line_no": lineNo})
		}
		if quantity < 0 || goods < 0 {
			return nil, apperr.Invalid("refund line quantity must not be negative and goods_cent must be non-negative").
				WithDetails(map[string]any{"line_no": lineNo, "quantity": quantity, "goods_cent": goods})
		}
		if _, dup := seen[lineNo]; dup {
			return nil, apperr.Invalid("refund line_no is duplicated").
				WithDetails(map[string]any{"line_no": lineNo})
		}
		seen[lineNo] = struct{}{}
		orderLine, ok := orderLines[lineNo]
		if !ok {
			return nil, apperr.Invalid("refund line does not belong to the group").
				WithDetails(map[string]any{"order_no": orderNo, "group_no": groupNo, "line_no": lineNo})
		}
		remainingGoods := orderLine.LinePaidCent - comp.LineGoods[lineNo]
		remainingQuantity := int64(orderLine.Quantity) - comp.LineQuantity[lineNo]
		if remainingGoods < 0 || remainingQuantity < 0 {
			return nil, apperr.Failed("refund line is already over-committed").
				WithDetails(map[string]any{"line_no": lineNo, "line_paid_cent": orderLine.LinePaidCent,
					"committed_goods_cent": comp.LineGoods[lineNo], "quantity": orderLine.Quantity,
					"committed_quantity": comp.LineQuantity[lineNo]})
		}
		// quantity=0 models a partial-amount line that does not consume a unit
		// (for example a post-departure price adjustment); only the amount is
		// checked. quantity>0 also consumes the line quantity.
		if goods > remainingGoods || (quantity > 0 && quantity > remainingQuantity) {
			return nil, apperr.Failed("refund line exceeds the remaining line amount").
				WithDetails(map[string]any{"line_no": lineNo, "requested_goods_cent": goods,
					"remaining_goods_cent": remainingGoods, "requested_quantity": quantity,
					"remaining_quantity": remainingQuantity})
		}
		out = append(out, tccCommitmentLine{LineNo: lineNo, Quantity: quantity, GoodsCent: goods})
		goodsTotal += goods
	}

	var groupFreight int64
	if err := q.QueryRowContext(ctx, `SELECT COALESCE(SUM(freight_share_cent),0) FROM order_fulfillment_groups
		WHERE order_no = ? AND group_no = ?`, orderNo, groupNo).Scan(&groupFreight); err != nil {
		return nil, apperr.Internal("load group freight share for refund composition", err)
	}
	remainingFreight := groupFreight - comp.CommittedFreight
	if remainingFreight < 0 {
		remainingFreight = 0
	}
	if freight > remainingFreight {
		return nil, apperr.Failed("freight refund exceeds the remaining group freight").
			WithDetails(map[string]any{"group_freight_cent": groupFreight,
				"committed_freight_cent": comp.CommittedFreight, "requested_freight_cent": freight})
	}
	if goodsTotal+freight != req.GetAmountCent() {
		return nil, apperr.Invalid("refund lines plus freight do not equal amount_cent").
			WithDetails(map[string]any{"goods_cent": goodsTotal, "freight_cent": freight,
				"amount_cent": req.GetAmountCent()})
	}
	if freight > 0 {
		out = append(out, tccCommitmentLine{LineNo: 0, FreightCent: freight})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LineNo < out[j].LineNo })

	source := strings.TrimSpace(req.GetSource())
	if source != refundApplicationSourceTCC {
		source = "aftersale"
	}
	return &refundComposition{
		Amount: req.GetAmountCent(), Goods: goodsTotal, Freight: freight, Source: source, Lines: out,
	}, nil
}

// validateRefundResponsibility enforces the closed responsibility/reason pair
// shared with AfterSales. An empty responsibility is the legacy call path
// (T3.6 will populate it); a non-empty value must be a valid pair. Order never
// infers responsibility from free text.
func validateRefundResponsibility(req *pb.AdmitRefundRequest) (string, error) {
	party := refundpolicy.Normalize(req.GetResponsibility())
	reason := refundpolicy.Normalize(req.GetReasonCode())
	if party == "" {
		if reason != "" {
			return "", apperr.Invalid("reason_code requires responsibility")
		}
		return "", nil
	}
	if !refundpolicy.ValidResponsibility(party) {
		return "", apperr.Invalid("responsibility is not a closed enum value").
			WithDetails(map[string]any{"responsibility": party})
	}
	if !refundpolicy.ValidReason(party, reason) {
		return "", apperr.Invalid("reason_code does not belong to responsibility").
			WithDetails(map[string]any{"responsibility": party, "reason_code": reason})
	}
	return party, nil
}

// insertRefundCommitmentTx writes the commitment header and, for a composed
// request, its line rows. It is idempotent for the same refund_id/amount and
// fails closed if an existing header has a different composition.
func insertRefundCommitmentTx(ctx context.Context, tx *sql.Tx, orderNo, groupNo, refundID string,
	amount int64, comp *refundComposition) error {
	if comp == nil {
		if _, err := tx.ExecContext(ctx, `INSERT INTO order_refund_commitments
			(order_no, group_no, refund_id, amount_cent, state, version)
			VALUES (?, ?, ?, ?, ?, 1)
			ON DUPLICATE KEY UPDATE refund_id = refund_id`,
			orderNo, groupNo, refundID, amount, commitmentReserved); err != nil {
			return apperr.Internal("insert refund commitment", err)
		}
		return nil
	}
	if amount != comp.Amount || comp.Goods+comp.Freight != amount {
		return apperr.Internal("refund composition does not match the commitment amount", nil)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO order_refund_commitments
		(order_no, group_no, refund_id, amount_cent, goods_cent, freight_cent,
		 composition_known, composition_source, state, version)
		VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?, 1)
		ON DUPLICATE KEY UPDATE refund_id = refund_id`,
		orderNo, groupNo, refundID, amount, comp.Goods, comp.Freight,
		comp.Source, commitmentReserved); err != nil {
		return apperr.Internal("insert refund commitment with composition", err)
	}
	var (
		existingAmount, existingGoods, existingFreight      int64
		compositionKnown                                    int
		state, existingOrder, existingGroup, existingSource string
	)
	if err := tx.QueryRowContext(ctx, `SELECT amount_cent, goods_cent, freight_cent, composition_known, state,
		order_no, group_no, composition_source
		FROM order_refund_commitments WHERE refund_id = ? FOR UPDATE`, refundID).
		Scan(&existingAmount, &existingGoods, &existingFreight, &compositionKnown, &state,
			&existingOrder, &existingGroup, &existingSource); err != nil {
		return apperr.Internal("load refund commitment composition", err)
	}
	if existingAmount != amount || existingGoods != comp.Goods || existingFreight != comp.Freight ||
		compositionKnown != 1 || existingOrder != orderNo || existingGroup != groupNo || existingSource != comp.Source {
		return apperr.Failed("refund commitment conflicts with the requested composition").
			WithDetails(map[string]any{"refund_id": refundID, "amount_cent": existingAmount,
				"goods_cent": existingGoods, "freight_cent": existingFreight,
				"composition_known": compositionKnown, "want_goods_cent": comp.Goods,
				"want_freight_cent": comp.Freight, "order_no": existingOrder, "group_no": existingGroup,
				"composition_source": existingSource, "want_source": comp.Source})
	}
	if state != commitmentReserved && state != commitmentSettled {
		return apperr.Failed("refund commitment is not resumable in its current state").
			WithDetails(map[string]any{"refund_id": refundID, "state": state})
	}
	for _, line := range comp.Lines {
		if _, err := tx.ExecContext(ctx, `INSERT INTO order_refund_commitment_lines
			(refund_id, line_no, quantity, goods_cent, freight_cent)
			VALUES (?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE line_no = line_no`,
			refundID, line.LineNo, line.Quantity, line.GoodsCent, line.FreightCent); err != nil {
			return apperr.Internal("insert refund commitment line", err)
		}
	}
	stored, err := loadTccCommitmentLines(ctx, tx, refundID)
	if err != nil {
		return err
	}
	if !equalTccCommitmentLines(comp.Lines, stored) {
		return apperr.Failed("refund commitment line composition does not match the header").
			WithDetails(map[string]any{"refund_id": refundID,
				"expected_lines": comp.Lines, "stored_lines": stored})
	}
	return nil
}
