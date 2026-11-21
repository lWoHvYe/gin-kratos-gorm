package inventory

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

// TccTryConfirm validates that every reservation of the order is still HELD or
// CONFIRMED. It is read-only; Confirm applies the state change.
func (s *Service) TccTryConfirm(ctx context.Context, req *pb.TccPaymentRequest) (*pb.Empty, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	items := req.GetItems()
	if len(items) == 0 {
		return nil, apperr.Invalid("items are required")
	}
	for _, item := range items {
		r, err := s.reservationByOrderSku(ctx, s.db, orderNo, item.GetSkuId(), item.GetWarehouseId(), false)
		if err != nil {
			return nil, apperr.Failed(fmt.Sprintf("reservation missing for sku %d", item.GetSkuId()))
		}
		if r.status != statusHeld && r.status != statusConfirmed {
			return nil, apperr.Failed(fmt.Sprintf("reservation %s is %s", r.reservationNo, r.status))
		}
	}
	return &pb.Empty{}, nil
}

func (s *Service) TccConfirm(ctx context.Context, req *pb.TccPaymentRequest) (*pb.Empty, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	if len(req.GetItems()) == 0 {
		return nil, apperr.Invalid("items are required")
	}
	summary, err := s.applyInventoryOperation(ctx, opTypeConfirm, orderNo, req.GetItems())
	if err != nil {
		return nil, err
	}
	if summary.Retryable > 0 {
		return nil, apperr.Unavailable(fmt.Sprintf("inventory confirm has %d pending item(s)", summary.Retryable), nil)
	}
	if summary.Failed > 0 {
		return nil, apperr.Failed(fmt.Sprintf("inventory confirm failed for %d item(s)", summary.Failed))
	}
	return &pb.Empty{}, nil
}

// TccCancelConfirm compensates either a HELD or an already CONFIRMED
// reservation. CONFIRMED requires restocking on_hand; HELD only releases the
// reserved quantity. A missing reservation is a terminal skip for cancel: there
// is nothing to release, and it must not drop the remaining items.
func (s *Service) TccCancelConfirm(ctx context.Context, req *pb.TccPaymentRequest) (*pb.Empty, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	if len(req.GetItems()) == 0 {
		return nil, apperr.Invalid("items are required")
	}
	summary, err := s.applyInventoryOperation(ctx, opTypeCancel, orderNo, req.GetItems())
	if err != nil {
		return nil, err
	}
	if summary.Retryable > 0 {
		return nil, apperr.Unavailable(fmt.Sprintf("inventory cancel has %d pending item(s)", summary.Retryable), nil)
	}
	if summary.Failed > 0 {
		return nil, apperr.Failed(fmt.Sprintf("inventory cancel failed for %d item(s)", summary.Failed))
	}
	return &pb.Empty{}, nil
}

// cancelReservationTx is the single-item cancel transaction shared by the
// operation runner and TccCancelConfirm. It locks the reservation, applies the
// HELD/CONFIRMED/seckill branch, and CASes the terminal status. The caller owns
// the transaction and any savepoint around a partial failure.
func (s *Service) cancelReservationTx(ctx context.Context, tx *sql.Tx, orderNo string, skuID, warehouseID, quantity int64) (*reservation, error) {
	r, err := s.reservationByOrderSku(ctx, tx, orderNo, skuID, warehouseID, true)
	if err != nil {
		if apperr.IsCode(err, apperr.CodeNotFound) {
			return nil, apperr.Wrap(apperr.CodeNotFound, "reservation missing", errReservationMissing)
		}
		return nil, err
	}
	if quantity > 0 && quantity != r.quantity {
		return nil, apperr.Invalid("quantity %d does not match reservation %s quantity %d", quantity, r.reservationNo, r.quantity)
	}
	if r.status == statusReleased {
		return r, nil
	}
	if r.status != statusHeld && r.status != statusConfirmed {
		return nil, apperr.Failed(fmt.Sprintf("reservation %s is %s", r.reservationNo, r.status))
	}
	originalStatus := r.status
	// Seckill reservations have a separate bucket + mapping. The generic path
	// below decrements inventory_stocks.reserved (which a seckill reservation
	// never incremented) and would corrupt accounting.
	mapping, merr := s.seckillOrderMapping(ctx, tx, r.orderNo, r.skuID, true)
	if merr != nil {
		return nil, merr
	}
	if mapping != nil {
		switch mapping.status {
		case statusReleased:
			// already compensated
		case statusHeld:
			if _, _, err := s.releaseSeckillMapping(ctx, tx, r); err != nil {
				return nil, err
			}
		case statusConfirmed:
			if err := s.cancelSeckillConfirmed(ctx, tx, r); err != nil {
				return nil, err
			}
		default:
			return nil, apperr.Failed("unexpected seckill order mapping status " + mapping.status)
		}
		res, err := tx.ExecContext(ctx, `UPDATE stock_reservations SET status = ?, updated_at = ?
			WHERE id = ? AND status = ?`, statusReleased, time.Now(), r.id, originalStatus)
		if err != nil {
			return nil, apperr.Internal("release seckill reservation", err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return nil, apperr.Wrap(apperr.CodeConflict,
				fmt.Sprintf("reservation %s changed concurrently", r.reservationNo), errReservationChanged)
		}
		r.status = statusReleased
		return r, nil
	}
	wasConfirmed := originalStatus == statusConfirmed
	if wasConfirmed {
		res, err := tx.ExecContext(ctx, `UPDATE inventory_stocks
			SET on_hand = on_hand + ?, version = version + 1 WHERE sku_id = ? AND warehouse_id = ?`,
			r.quantity, r.skuID, r.warehouseID)
		if err != nil {
			return nil, apperr.Internal("restock", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil, apperr.Failed("stock row missing while restocking")
		}
	} else {
		res, err := tx.ExecContext(ctx, `UPDATE inventory_stocks
			SET reserved = reserved - ?, version = version + 1
			WHERE sku_id = ? AND warehouse_id = ? AND reserved >= ?`,
			r.quantity, r.skuID, r.warehouseID, r.quantity)
		if err != nil {
			return nil, apperr.Internal("release", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil, apperr.Failed("stock row missing or reserved quantity inconsistent")
		}
	}
	res, err := tx.ExecContext(ctx, `UPDATE stock_reservations SET status = ?, updated_at = ?
		WHERE id = ? AND status = ?`, statusReleased, time.Now(), r.id, originalStatus)
	if err != nil {
		return nil, apperr.Internal("release reservation", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return nil, apperr.Wrap(apperr.CodeConflict,
			fmt.Sprintf("reservation %s changed concurrently", r.reservationNo), errReservationChanged)
	}
	st, err := s.stock(ctx, tx, r.skuID, r.warehouseID)
	if err != nil {
		return nil, err
	}
	entry := entryRelease
	if wasConfirmed {
		entry = entryRestock
	}
	r.status = statusReleased
	if err := insertLedger(ctx, tx, r, entry, st); err != nil {
		return nil, err
	}
	if err := appendStockChanged(ctx, tx, st); err != nil {
		return nil, err
	}
	if err := appendEvent(ctx, tx, r, topicReleased, eventReleased); err != nil {
		return nil, err
	}
	return r, nil
}

func (s *Service) TccQueryConfirm(ctx context.Context, req *pb.TccPaymentRequest) (*pb.TccQueryReply, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	for _, item := range req.GetItems() {
		r, err := s.reservationByOrderSku(ctx, s.db, orderNo, item.GetSkuId(), item.GetWarehouseId(), false)
		if err != nil || (r.status != statusHeld && r.status != statusConfirmed) {
			return &pb.TccQueryReply{Prepared: false}, nil
		}
	}
	return &pb.TccQueryReply{Prepared: true}, nil
}
