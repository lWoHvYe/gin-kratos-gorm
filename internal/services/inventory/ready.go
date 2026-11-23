package inventory

import (
	"context"
	"fmt"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

// CheckOrderReady is the read-only readiness gate used by Order settlement.
// It never mutates stock; a missing reservation is reported as MISSING and
// makes ready=false so the caller fails closed.
func (s *Service) CheckOrderReady(ctx context.Context, req *pb.CheckOrderReadyRequest) (*pb.CheckOrderReadyReply, error) {
	orderNo := req.GetOrderNo()
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}

	type key struct {
		skuID       int64
		warehouseID int64
	}
	statuses := make(map[key]string)
	var reservationVersion int64
	rows, err := s.db.QueryContext(ctx, `SELECT id, sku_id, warehouse_id, quantity, status
		FROM stock_reservations WHERE order_no = ? ORDER BY id`, orderNo)
	if err != nil {
		return nil, apperr.Internal("query order reservations", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, skuID, warehouseID int64
		var quantity int32
		var status string
		if err := rows.Scan(&id, &skuID, &warehouseID, &quantity, &status); err != nil {
			return nil, apperr.Internal("scan order reservation", err)
		}
		if id > reservationVersion {
			reservationVersion = id
		}
		statuses[key{skuID: skuID, warehouseID: normalizeWarehouse(warehouseID)}] = status
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal("iterate order reservations", err)
	}

	refs := req.GetLines()
	if len(refs) == 0 {
		// No explicit lines: report every reservation and require at least one.
		// The caller normally passes lines; this branch is used by diagnostics.
		reply := &pb.CheckOrderReadyReply{Ready: len(statuses) > 0, ReservationVersion: reservationVersion}
		for k, status := range statuses {
			reply.Lines = append(reply.Lines, &pb.CheckOrderReadyLineStatus{
				SkuId: k.skuID, WarehouseId: k.warehouseID, Status: status,
			})
			if status != "CONFIRMED" {
				reply.Ready = false
			}
		}
		return reply, nil
	}

	reply := &pb.CheckOrderReadyReply{Ready: true, ReservationVersion: reservationVersion}
	for _, ref := range refs {
		warehouseID := normalizeWarehouse(ref.GetWarehouseId())
		status, ok := statuses[key{skuID: ref.GetSkuId(), warehouseID: warehouseID}]
		if !ok {
			status = "MISSING"
		}
		reply.Lines = append(reply.Lines, &pb.CheckOrderReadyLineStatus{
			LineNo: ref.GetLineNo(), SkuId: ref.GetSkuId(), WarehouseId: warehouseID,
			Quantity: ref.GetQuantity(), Status: status,
		})
		if status != "CONFIRMED" {
			reply.Ready = false
		}
	}
	if !reply.Ready {
		return reply, apperr.Failed(fmt.Sprintf("order %s inventory is not ready", orderNo)).
			WithDetails(map[string]any{"ready": false})
	}
	return reply, nil
}
