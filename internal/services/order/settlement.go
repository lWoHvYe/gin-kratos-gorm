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

	permitNone     = "NONE"
	permitGranted  = "GRANTED"
	permitConsumed = "CONSUMED"
	permitRevoked  = "REVOKED"

	actionPay            = "pay"
	actionCancel         = "cancel"
	actionShip           = "ship"
	actionReceive        = "receive"
	actionApplyAfterSale = "apply_after_sale"
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

// shipmentStatus maps the lifecycle to the logistics projection. The shipment
// service remains the source of carrier/tracking details.
func shipmentStatus(status string) string {
	switch status {
	case statusAllocated:
		return "CREATED"
	case statusShipped:
		return "SHIPPED"
	case statusCompleted:
		return "RECEIVED"
	default:
		return ""
	}
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
	state       string
	shipmentNo  string
	afterSaleNo string
	version     int64
}

func findOrderForUpdate(ctx context.Context, tx *sql.Tx, orderNo string) (*orderRecord, error) {
	return scanOrder(tx.QueryRowContext(ctx, `SELECT `+orderColumns+` FROM orders WHERE order_no = ? FOR UPDATE`, orderNo))
}

func ensurePermitTx(ctx context.Context, tx *sql.Tx, orderNo string) (*shippingPermit, error) {
	var permit shippingPermit
	err := tx.QueryRowContext(ctx, `SELECT state, shipment_no, after_sale_no, version
		FROM order_shipping_permits WHERE order_no = ? FOR UPDATE`, orderNo).
		Scan(&permit.state, &permit.shipmentNo, &permit.afterSaleNo, &permit.version)
	if errors.Is(err, sql.ErrNoRows) {
		if _, err := tx.ExecContext(ctx, `INSERT INTO order_shipping_permits
			(order_no, state, shipment_no, after_sale_no, version, created_at, updated_at)
			VALUES (?, ?, '', '', 1, NOW(3), NOW(3))`, orderNo, permitNone); err != nil {
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
	if err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		record, err := findOrderForUpdate(ctx, tx, orderNo)
		if err != nil {
			return err
		}
		if record == nil {
			return apperr.NotFound("order not found")
		}
		permit, err := ensurePermitTx(ctx, tx, orderNo)
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
		if record.RefundState == refundStateRefunding || record.RefundState == refundStateFull {
			return apperr.Failed("order refund is in progress; shipping is blocked")
		}
		switch permit.state {
		case permitConsumed:
			return apperr.Conflict("another shipment already holds the shipping permit")
		case permitRevoked:
			return apperr.Failed("shipping permit was revoked by a refund request")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE order_shipping_permits
			SET state = ?, shipment_no = ?, version = version + 1, updated_at = NOW(3)
			WHERE order_no = ?`, permitConsumed, shipmentNo, orderNo); err != nil {
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
		permit, err := ensurePermitTx(ctx, tx, orderNo)
		if err != nil {
			return err
		}
		nextState := permitRevoked
		if permit.state == permitConsumed {
			nextState = permitConsumed
		}
		if _, err := tx.ExecContext(ctx, `UPDATE order_shipping_permits
			SET state = ?, after_sale_no = ?, version = version + 1, updated_at = NOW(3)
			WHERE order_no = ?`, nextState, afterSaleNo, orderNo); err != nil {
			return apperr.Internal("block shipping permit", err)
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
