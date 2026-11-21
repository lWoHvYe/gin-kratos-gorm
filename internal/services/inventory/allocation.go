package inventory

import (
	"context"
	"fmt"
	"sort"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

// AllocateOrder is a read-only preview: it chooses one warehouse per requested
// line using available stock and warehouse priority (ascending warehouse_id in
// phase 2a). It never writes reservations or ledger rows. If a line cannot fit
// in one warehouse it fails instead of silently splitting the quantity; the
// checkout line must be split before payment (ADR 0002 v1).
func (s *Service) AllocateOrder(ctx context.Context, req *pb.AllocateOrderRequest) (*pb.AllocateOrderReply, error) {
	lines := req.GetLines()
	if len(lines) == 0 {
		return nil, apperr.Invalid("at least one allocation line is required")
	}
	// remaining[sku][warehouse] = available units still assignable for this
	// preview. Multiple lines for the same SKU consume from the same pool.
	remaining := make(map[int64]map[int64]int64)
	for _, line := range lines {
		if line.GetSkuId() <= 0 || line.GetQuantity() <= 0 {
			return nil, apperr.Invalid("allocation lines need positive sku_id and quantity")
		}
		sku := line.GetSkuId()
		if _, ok := remaining[sku]; ok {
			continue
		}
		rows, err := s.db.QueryContext(ctx, `SELECT warehouse_id, on_hand - reserved
			FROM inventory_stocks WHERE sku_id = ? AND warehouse_id > 0
			ORDER BY warehouse_id`, sku)
		if err != nil {
			return nil, apperr.Internal("load allocation stock", err)
		}
		byWarehouse := make(map[int64]int64)
		for rows.Next() {
			var warehouseID, available int64
			if err := rows.Scan(&warehouseID, &available); err != nil {
				rows.Close()
				return nil, apperr.Internal("scan allocation stock", err)
			}
			if available > 0 {
				byWarehouse[warehouseID] = available
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, apperr.Internal("iterate allocation stock", err)
		}
		rows.Close()
		remaining[sku] = byWarehouse
	}
	reply := &pb.AllocateOrderReply{Allocations: make([]*pb.AllocateOrderAllocation, 0, len(lines))}
	for i, line := range lines {
		warehouseID, ok := pickWarehouse(remaining[line.GetSkuId()], int64(line.GetQuantity()))
		if !ok {
			return nil, apperr.Failed(fmt.Sprintf("insufficient stock for sku %d in a single warehouse", line.GetSkuId()))
		}
		remaining[line.GetSkuId()][warehouseID] -= int64(line.GetQuantity())
		reply.Allocations = append(reply.Allocations, &pb.AllocateOrderAllocation{
			SkuId: line.GetSkuId(), Quantity: line.GetQuantity(),
			WarehouseId: warehouseID, LineNo: int32(i + 1),
		})
	}
	return reply, nil
}

// pickWarehouse returns the first priority warehouse that can serve the whole
// line. The map is small (warehouse count), so sorting ids is cheap and stable.
func pickWarehouse(byWarehouse map[int64]int64, quantity int64) (int64, bool) {
	warehouses := make([]int64, 0, len(byWarehouse))
	for warehouseID, available := range byWarehouse {
		if available >= quantity {
			warehouses = append(warehouses, warehouseID)
		}
	}
	if len(warehouses) == 0 {
		return 0, false
	}
	sort.Slice(warehouses, func(i, j int) bool { return warehouses[i] < warehouses[j] })
	return warehouses[0], true
}
