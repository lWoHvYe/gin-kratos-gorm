package order

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/db"
)

// Refund state is orthogonal to the lifecycle. It gates new shipments and
// drives the read model; it is never written into orders.status.
const (
	refundStateNone      = "NONE"
	refundStateRefunding = "REFUNDING"
	refundStatePartial   = "PARTIAL"
	refundStateFull      = "FULL"

	permitNone               = "NONE"
	permitGranted            = "GRANTED"
	permitConsumed           = "CONSUMED"
	permitRevoked            = "REVOKED"
	groupRefundStateRefunded = "REFUNDED"

	actionPay                 = "pay"
	actionCancel              = "cancel"
	actionShip                = "ship"
	actionReceive             = "receive"
	actionApplyAfterSale      = "apply_after_sale"
	actionShipGroup           = "ship_group"
	actionReceiveGroup        = "receive_group"
	actionApplyAfterSaleGroup = "apply_after_sale_group"
)

func refundStateFor(refundedCent, paidCent int64) string {
	switch {
	case refundedCent <= 0:
		return refundStateNone
	case paidCent > 0 && refundedCent >= paidCent:
		return refundStateFull
	default:
		return refundStatePartial
	}
}

// shipmentStatus maps the lifecycle to the five-value fulfillment projection.
// CREATED is a shipment state, never an order projection (ADR 0005 §3.2.1).
func shipmentStatus(status string) string {
	switch status {
	case statusPaid, statusAllocated:
		return "UNSHIPPED"
	case statusShipped:
		return "SHIPPED"
	case statusCompleted:
		return "RECEIVED"
	default:
		return ""
	}
}

// groupIsClosed means the group will never produce a parcel. It is the single
// closure authority for the S2 read model (SPEC-0001 v1.5 AC-27A) and the order
// is deliberate:
//
//  1. nil                        -> open
//  2. Status CANCELLED           -> closed (persisted CANCELLED only comes from
//     whole-order cancellation)
//  3. Status SHIPPED/RECEIVED    -> open (departure guard; before any refund fact)
//  4. permit REVOKED             -> closed (S5 terminal state, INV-45)
//  5. permit CONSUMED and PreShipAdmission -> closed (RefundAdmission
//     result=state=PRE_SHIP_REVOKED; AC-27A clause 5 "valid PRE")
//  6. otherwise                  -> open (NONE, PENDING_REMOTE, POST_SHIP_ALLOWED,
//     missing/unverifiable admission)
//
// Inputs are Order-owned facts only. refund_state (S9) and reservation_state are
// never inputs: refund progress must not decide fulfillment projection, and the
// reservation release fact belongs to Inventory.
func groupIsClosed(group *fulfillmentGroup) bool {
	if group == nil {
		return false
	}
	if group.Status == "CANCELLED" {
		return true
	}
	if group.Status == "SHIPPED" || group.Status == "RECEIVED" {
		return false
	}
	if group.PermitState == permitRevoked {
		return true
	}
	return group.PermitState == permitConsumed && group.PreShipAdmission
}

// effectiveGroupStatus is the read-model status shown to BFFs/UI. The persisted
// group status stays ALLOCATED for an unshipped refund; a revoked permit or a
// pre-ship RefundAdmission makes the group effectively CANCELLED (AC-27A).
func effectiveGroupStatus(group *fulfillmentGroup) string {
	if groupIsClosed(group) && group.Status != "CANCELLED" {
		return "CANCELLED"
	}
	return group.Status
}

func shipmentStatusForGroups(orderStatus string, groups []*fulfillmentGroup) string {
	if len(groups) == 0 {
		return shipmentStatus(orderStatus)
	}
	shipped, received, closed := 0, 0, 0
	for _, group := range groups {
		if groupIsClosed(group) {
			closed++
			continue
		}
		switch group.Status {
		case "SHIPPED":
			shipped++
		case "RECEIVED":
			shipped++
			received++
		}
	}
	remaining := len(groups) - closed
	switch {
	case remaining == 0:
		return "UNSHIPPED"
	case received == remaining:
		return "RECEIVED"
	case received > 0:
		return "PARTIALLY_RECEIVED"
	case shipped == remaining:
		return "SHIPPED"
	case shipped > 0:
		return "PARTIALLY_SHIPPED"
	case orderStatus == statusPaid || orderStatus == statusAllocated:
		return "UNSHIPPED"
	default:
		return ""
	}
}

// groupAction is the typed, group-scoped action contract. BFFs and UIs use
// this instead of parsing "ship_group:G1" strings.
type groupAction struct {
	GroupNo string
	Action  string
}

// orderActionPolicy is the single authority for lifecycle actions. It combines
// group status, permit state and the order-level refund projection; callers
// must not re-derive actions from status.
type orderActionPolicy struct {
	order  *orderRecord
	groups []*fulfillmentGroup
}

func newOrderActionPolicy(o *orderRecord, groups []*fulfillmentGroup) orderActionPolicy {
	return orderActionPolicy{order: o, groups: groups}
}

func (p orderActionPolicy) groupActions() []groupAction {
	if p.order == nil {
		return nil
	}
	actions := make([]groupAction, 0, len(p.groups)*2+2)
	if p.order.Status == statusPendingPayment {
		if p.order.RefundState != refundStateRefunding && p.order.RefundState != refundStateFull {
			actions = append(actions, groupAction{Action: actionPay}, groupAction{Action: actionCancel})
		}
		return actions
	}
	fullRefund := p.order.RefundState == refundStateFull
	for _, group := range p.groups {
		switch group.Status {
		case "PENDING", statusAllocated:
			permitOpen := group.PermitState == "" || group.PermitState == permitNone || group.PermitState == permitGranted
			if !fullRefund && permitOpen &&
				(p.order.Status == statusPaid || p.order.Status == statusAllocated) {
				actions = append(actions, groupAction{GroupNo: group.GroupNo, Action: actionShip})
			}
		case "SHIPPED":
			actions = append(actions, groupAction{GroupNo: group.GroupNo, Action: actionReceive})
			if !fullRefund {
				actions = append(actions, groupAction{GroupNo: group.GroupNo, Action: actionApplyAfterSale})
			}
		case "RECEIVED":
			if !fullRefund {
				actions = append(actions, groupAction{GroupNo: group.GroupNo, Action: actionApplyAfterSale})
			}
		}
	}
	return actions
}

// legacyActions is the compatibility projection. Single-group orders keep the
// historical unsuffixed actions; split orders expose suffixed group actions.
func (p orderActionPolicy) legacyActions() []string {
	if len(p.groups) <= 1 {
		return availableActions(p.order)
	}
	actions := p.groupActions()
	out := make([]string, 0, len(actions))
	for _, action := range actions {
		if action.GroupNo == "" {
			out = append(out, action.Action)
			continue
		}
		out = append(out, action.Action+"_group:"+action.GroupNo)
	}
	return out
}

// availableActions is the single server-owned action list consumed by both BFFs
// and the storefront/admin UIs. It must not be re-derived from status in a UI.
func availableActions(o *orderRecord) []string {
	if o == nil {
		return nil
	}
	actions := make([]string, 0, 3)
	switch o.Status {
	case statusPendingPayment:
		if o.RefundState != refundStateRefunding && o.RefundState != refundStateFull {
			actions = append(actions, actionPay, actionCancel)
		}
	case statusPaid, statusAllocated:
		if o.RefundState != refundStateRefunding && o.RefundState != refundStateFull {
			actions = append(actions, actionApplyAfterSale, actionShip)
		}
	case statusShipped:
		actions = append(actions, actionReceive)
		if o.RefundState != refundStateRefunding && o.RefundState != refundStateFull {
			actions = append(actions, actionApplyAfterSale)
		}
	case statusCompleted:
		if o.RefundState != refundStateRefunding && o.RefundState != refundStateFull {
			actions = append(actions, actionApplyAfterSale)
		}
	}
	return actions
}

type shippingPermit struct {
	groupNo     string
	state       string
	shipmentNo  string
	afterSaleNo string
	version     int64
}

func ensureGroupExistsTx(ctx context.Context, tx *sql.Tx, orderNo, groupNo string) error {
	if permitGroup(groupNo) == defaultGroupNo {
		return nil
	}
	var one int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM order_fulfillment_groups
		WHERE order_no = ? AND group_no = ?`, orderNo, permitGroup(groupNo)).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return apperr.Failed(fmt.Sprintf("fulfillment group %s is not part of order %s", permitGroup(groupNo), orderNo))
	}
	if err != nil {
		return apperr.Internal("load fulfillment group", err)
	}
	return nil
}

func permitGroup(groupNo string) string {
	groupNo = strings.TrimSpace(groupNo)
	if groupNo == "" {
		return defaultGroupNo
	}
	return groupNo
}

func findOrderForUpdate(ctx context.Context, tx *sql.Tx, orderNo string) (*orderRecord, error) {
	return scanOrder(tx.QueryRowContext(ctx, `SELECT `+orderColumns+` FROM orders WHERE order_no = ? FOR UPDATE`, orderNo))
}

func ensurePermitTx(ctx context.Context, tx *sql.Tx, orderNo, groupNo string) (*shippingPermit, error) {
	groupNo = permitGroup(groupNo)
	var permit shippingPermit
	permit.groupNo = groupNo
	err := tx.QueryRowContext(ctx, `SELECT state, shipment_no, after_sale_no, version
		FROM order_shipping_permits WHERE order_no = ? AND fulfillment_group_no = ? FOR UPDATE`, orderNo, groupNo).
		Scan(&permit.state, &permit.shipmentNo, &permit.afterSaleNo, &permit.version)
	if errors.Is(err, sql.ErrNoRows) {
		if _, err := tx.ExecContext(ctx, `INSERT INTO order_shipping_permits
			(order_no, fulfillment_group_no, state, shipment_no, after_sale_no, version, created_at, updated_at)
			VALUES (?, ?, ?, '', '', 1, NOW(3), NOW(3))`, orderNo, groupNo, permitNone); err != nil {
			return nil, apperr.Internal("create shipping permit", err)
		}
		permit.state = permitNone
		permit.version = 1
		return &permit, nil
	}
	if err != nil {
		return nil, apperr.Internal("load shipping permit", err)
	}
	return &permit, nil
}

// AuthorizeShipment serializes shipping against refund admission on the permit
// row. A refund requested first revokes the permit; a shipment authorized first
// may still be refunded later (refund-only after shipment).
func (s *Service) AuthorizeShipment(ctx context.Context, req *pb.ShipmentPermitRequest) (*pb.Empty, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	shipmentNo := strings.TrimSpace(req.GetShipmentNo())
	if orderNo == "" || shipmentNo == "" {
		return nil, apperr.Invalid("order_no and shipment_no are required")
	}
	items, err := loadOrderItems(ctx, s.db, orderNo)
	if err != nil {
		return nil, apperr.Internal("load order items for shipment", err)
	}
	// T2.2 boundary: a historical PAID order with no effective payment must be
	// resolved or queued before a shipping permit can be consumed. Production
	// always wires Payment; the nil check preserves the unit-style settlement
	// fixture that never exercises historical orders.
	if s.payments != nil {
		record, err := findOrder(ctx, s.db, orderNo)
		if err != nil {
			return nil, apperr.Internal("load order for legacy shipping gate", err)
		}
		if record != nil && (record.Status == statusPaid || record.Status == statusAllocated) &&
			strings.TrimSpace(record.SettlementPaymentNo) == "" {
			resolved, rerr := s.ensureLegacyResolution(ctx, orderNo)
			if rerr != nil {
				return nil, rerr
			}
			if resolved == nil || (resolved.State != legacyResolutionStateResolved &&
				resolved.State != legacyResolutionStateExpiredZero) {
				details := map[string]any{"order_no": orderNo, "reason": "LEGACY_PAYMENT_UNRESOLVED"}
				if resolved != nil {
					details["resolution_state"] = resolved.State
					details["candidate_count"] = resolved.CandidateCount
				}
				return nil, apperr.Failed("order has an unresolved historical payment; shipping is blocked").
					WithDetails(details)
			}
		}
	}
	if err := s.checkOrderReady(ctx, orderNo, items); err != nil {
		return nil, err
	}
	if err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		record, err := findOrderForUpdate(ctx, tx, orderNo)
		if err != nil {
			return err
		}
		if record == nil {
			return apperr.NotFound("order not found")
		}
		groupNo := permitGroup(req.GetFulfillmentGroupNo())
		if err := ensureGroupExistsTx(ctx, tx, orderNo, groupNo); err != nil {
			return err
		}
		permit, err := ensurePermitTx(ctx, tx, orderNo, groupNo)
		if err != nil {
			return err
		}
		// A retried Ship for the same already-authorized shipment is idempotent
		// even after the lifecycle advanced or a refund completed.
		if permit.state == permitConsumed && permit.shipmentNo == shipmentNo {
			return nil
		}
		if record.Status != statusPaid && record.Status != statusAllocated {
			return apperr.Failed(fmt.Sprintf("order cannot ship in status %s", record.Status))
		}
		if record.RefundState == refundStateFull {
			return apperr.Failed("order is fully refunded; shipping is blocked")
		}
		var groupRefundState string
		err = tx.QueryRowContext(ctx, `SELECT refund_state FROM order_fulfillment_groups
			WHERE order_no = ? AND group_no = ?`, orderNo, groupNo).Scan(&groupRefundState)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return apperr.Internal("load group refund state", err)
		}
		if groupRefundState == "REFUNDING" || groupRefundState == "REFUNDED" {
			return apperr.Failed("group refund is in progress; shipping is blocked")
		}
		var shortageStatus string
		err = tx.QueryRowContext(ctx, `SELECT status FROM order_shortage_cases
			WHERE order_no = ? AND group_no = ?`, orderNo, groupNo).Scan(&shortageStatus)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return apperr.Internal("load shortage case", err)
		}
		if shortageStatus == "NOTIFIED" || shortageStatus == "RESOLVED_WAIT" {
			return apperr.Failed("group has an open shortage case; shipping is blocked")
		}
		switch permit.state {
		case permitConsumed:
			return apperr.Conflict("another shipment already holds the shipping permit")
		case permitRevoked:
			return apperr.Failed("shipping permit was revoked by a refund request")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE order_shipping_permits
			SET state = ?, shipment_no = ?, version = version + 1, updated_at = NOW(3)
			WHERE order_no = ? AND fulfillment_group_no = ?`, permitConsumed, shipmentNo, orderNo, groupNo); err != nil {
			return apperr.Internal("consume shipping permit", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

// BlockShipmentForRefund records refund admission and revokes an unused permit.
// If shipping was already authorized, refund-only after shipment remains valid;
// the permit stays CONSUMED and the refund proceeds.
func (s *Service) BlockShipmentForRefund(ctx context.Context, req *pb.ShipmentPermitRequest) (*pb.Empty, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	afterSaleNo := strings.TrimSpace(req.GetAfterSaleNo())
	if orderNo == "" || afterSaleNo == "" {
		return nil, apperr.Invalid("order_no and after_sale_no are required")
	}
	if err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		record, err := findOrderForUpdate(ctx, tx, orderNo)
		if err != nil {
			return err
		}
		if record == nil {
			return apperr.NotFound("order not found")
		}
		switch record.Status {
		case statusPendingPayment, statusCancelled, statusClosed:
			return apperr.Failed(fmt.Sprintf("order cannot refund in status %s", record.Status))
		}
		groupNo := permitGroup(req.GetFulfillmentGroupNo())
		if err := ensureGroupExistsTx(ctx, tx, orderNo, groupNo); err != nil {
			return err
		}
		permit, err := ensurePermitTx(ctx, tx, orderNo, groupNo)
		if err != nil {
			return err
		}
		nextState := permitRevoked
		if permit.state == permitConsumed {
			nextState = permitConsumed
		}
		if _, err := tx.ExecContext(ctx, `UPDATE order_shipping_permits
			SET state = ?, after_sale_no = ?, version = version + 1, updated_at = NOW(3)
			WHERE order_no = ? AND fulfillment_group_no = ?`, nextState, afterSaleNo, orderNo, groupNo); err != nil {
			return apperr.Internal("block shipping permit", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE order_fulfillment_groups
			SET refund_state = ?, version = version + 1, updated_at = NOW(3)
			WHERE order_no = ? AND group_no = ? AND refund_state <> ?`,
			refundStateRefunding, orderNo, groupNo, groupRefundStateRefunded); err != nil {
			return apperr.Internal("mark group refunding", err)
		}
		if record.RefundState != refundStateFull {
			if _, err := tx.ExecContext(ctx, `UPDATE orders
				SET refund_state = ?, version = version + 1, updated_at = NOW(3)
				WHERE order_no = ?`, refundStateRefunding, orderNo); err != nil {
				return apperr.Internal("mark order refunding", err)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}
