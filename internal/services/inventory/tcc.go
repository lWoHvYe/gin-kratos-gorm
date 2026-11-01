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
		r, err := s.reservationByOrderSku(ctx, s.db, orderNo, item.GetSkuId(), false)
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
	for _, item := range req.GetItems() {
		if _, err := s.Confirm(ctx, &pb.ReserveRequest{
			OrderNo: orderNo, SkuId: item.GetSkuId(), WarehouseId: normalizeWarehouse(0), Quantity: item.GetQuantity(),
		}); err != nil {
			return nil, err
		}
	}
	return &pb.Empty{}, nil
}

// TccCancelConfirm compensates either a HELD or an already CONFIRMED
// reservation. CONFIRMED requires restocking on_hand; HELD only releases the
// reserved quantity. Repeated calls are no-ops.
func (s *Service) TccCancelConfirm(ctx context.Context, req *pb.TccPaymentRequest) (*pb.Empty, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	for _, item := range req.GetItems() {
		skuID := item.GetSkuId()
		if err := s.write(ctx, func(tx *sql.Tx) error {
			r, err := s.reservationByOrderSku(ctx, tx, orderNo, skuID, true)
			if err != nil {
				return err
			}
			if r.status == statusReleased {
				return nil
			}
			if r.status != statusHeld && r.status != statusConfirmed {
				return apperr.Failed(fmt.Sprintf("reservation %s is %s", r.reservationNo, r.status))
			}
			if r.status == statusConfirmed {
				if _, err := tx.ExecContext(ctx, `UPDATE inventory_stocks
					SET on_hand = on_hand + ?, version = version + 1 WHERE sku_id = ? AND warehouse_id = ?`,
					r.quantity, r.skuID, r.warehouseID); err != nil {
					return apperr.Internal("restock", err)
				}
			} else {
				if _, err := tx.ExecContext(ctx, `UPDATE inventory_stocks
					SET reserved = reserved - ?, version = version + 1
					WHERE sku_id = ? AND warehouse_id = ? AND reserved >= ?`,
					r.quantity, r.skuID, r.warehouseID, r.quantity); err != nil {
					return apperr.Internal("release", err)
				}
			}
			if _, err := tx.ExecContext(ctx, `UPDATE stock_reservations SET status = ?, updated_at = ? WHERE id = ?`,
				statusReleased, time.Now(), r.id); err != nil {
				return apperr.Internal("release reservation", err)
			}
			st, err := s.stock(ctx, tx, r.skuID, r.warehouseID)
			if err != nil {
				return err
			}
			r.status = statusReleased
			if err := insertLedger(ctx, tx, r, entryRelease, st); err != nil {
				return err
			}
			return appendEvent(ctx, tx, r, topicReleased, eventReleased)
		}); err != nil {
			return nil, err
		}
	}
	return &pb.Empty{}, nil
}

func (s *Service) TccQueryConfirm(ctx context.Context, req *pb.TccPaymentRequest) (*pb.TccQueryReply, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	for _, item := range req.GetItems() {
		if _, err := s.reservationByOrderSku(ctx, s.db, orderNo, item.GetSkuId(), false); err != nil {
			return &pb.TccQueryReply{Prepared: false}, nil
		}
	}
	return &pb.TccQueryReply{Prepared: true}, nil
}
