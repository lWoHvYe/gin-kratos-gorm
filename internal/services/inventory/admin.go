package inventory

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/id"
	"gin-kratos-gorm/internal/platform/outbox"
)

// AdminAdjustStock applies a signed inventory adjustment (inbound, damage,
// stocktake) and records a ledger row plus an outbox event. Positive deltas may
// create the stock row for a newly listed SKU; negative deltas can never take
// on_hand below reserved.
func (s *Service) AdminAdjustStock(ctx context.Context, req *pb.AdminAdjustStockRequest) (*pb.Stock, error) {
	skuID := req.GetSkuId()
	warehouseID := normalizeWarehouse(req.GetWarehouseId())
	delta := req.GetDelta()
	if skuID <= 0 {
		return nil, apperr.Invalid("sku_id is required")
	}
	if delta == 0 {
		return nil, apperr.Invalid("delta must not be zero")
	}
	if req.GetReason() == "" {
		return nil, apperr.Invalid("reason is required")
	}

	var result *stock
	err := s.write(ctx, func(tx *sql.Tx) error {
		if delta > 0 {
			if _, err := tx.ExecContext(ctx, `INSERT INTO inventory_stocks
				(sku_id, warehouse_id, on_hand, reserved, version, created_at, updated_at)
				VALUES (?, ?, ?, 0, 1, ?, ?)
				ON DUPLICATE KEY UPDATE on_hand = on_hand + VALUES(on_hand), version = version + 1, updated_at = VALUES(updated_at)`,
				skuID, warehouseID, delta, time.Now(), time.Now()); err != nil {
				return apperr.Internal("inbound stock", err)
			}
		} else {
			res, err := tx.ExecContext(ctx, `UPDATE inventory_stocks
				SET on_hand = on_hand + ?, version = version + 1, updated_at = ?
				WHERE sku_id = ? AND warehouse_id = ? AND on_hand + ? >= reserved`,
				delta, time.Now(), skuID, warehouseID, delta)
			if err != nil {
				return apperr.Internal("adjust stock", err)
			}
			affected, _ := res.RowsAffected()
			if affected == 0 {
				return apperr.Failed(fmt.Sprintf("adjustment %d would put on_hand below reserved or stock row is missing", delta))
			}
		}
		st, err := s.stock(ctx, tx, skuID, warehouseID)
		if err != nil {
			return err
		}
		result = st
		entry := &reservation{
			reservationNo: id.Reservation(),
			orderNo:       "ADMIN",
			skuID:         skuID,
			warehouseID:   warehouseID,
			quantity:      delta,
			status:        "ADJUSTED",
			expiresAt:     time.Now(),
			createdAt:     time.Now(),
		}
		if err := insertLedger(ctx, tx, entry, "ADJUST", st); err != nil {
			return err
		}
		if err := appendStockChanged(ctx, tx, st); err != nil {
			return err
		}
		return outbox.Append(ctx, tx, outbox.Event{
			Topic:         "shop.inventory.adjusted.v1",
			AggregateType: "inventory",
			AggregateID:   fmt.Sprintf("%d:%d", skuID, warehouseID),
			EventType:     "inventory.adjusted",
			Payload: map[string]any{
				"sku_id":       skuID,
				"warehouse_id": warehouseID,
				"delta":        delta,
				"reason":       req.GetReason(),
				"on_hand":      st.onHand,
				"reserved":     st.reserved,
				"adjusted_at":  time.Now().Unix(),
			},
		})
	})
	if err != nil {
		return nil, err
	}
	return &pb.Stock{
		SkuId:       result.skuID,
		WarehouseId: result.warehouseID,
		OnHand:      result.onHand,
		Reserved:    result.reserved,
		Available:   result.available(),
	}, nil
}
