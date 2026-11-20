package inventory

import (
	"context"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

// BatchGetStock returns the requested existing stock rows. Missing SKUs are
// omitted so callers can keep the per-line "insufficient stock" semantics.
func (s *Service) BatchGetStock(ctx context.Context, req *pb.BatchGetStockRequest) (*pb.StockList, error) {
	ids := req.GetSkuIds()
	if len(ids) == 0 || len(ids) > 200 {
		return nil, apperr.Invalid("1-200 sku_ids are required")
	}
	seen := make(map[int64]struct{}, len(ids))
	args := make([]any, 0, len(ids)+1)
	placeholders := make([]string, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, apperr.Invalid("sku_ids must be positive")
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	warehouseID := normalizeWarehouse(req.GetWarehouseId())
	args = append(args, warehouseID)
	rows, err := s.db.QueryContext(ctx, `SELECT sku_id, warehouse_id, on_hand, reserved, version
		FROM inventory_stocks WHERE sku_id IN (`+strings.Join(placeholders, ",")+`) AND warehouse_id = ?
		ORDER BY sku_id`, args...)
	if err != nil {
		return nil, apperr.Internal("batch get stock", err)
	}
	defer rows.Close()
	out := &pb.StockList{}
	for rows.Next() {
		var st stock
		if err := rows.Scan(&st.skuID, &st.warehouseID, &st.onHand, &st.reserved, &st.version); err != nil {
			return nil, apperr.Internal("scan stock", err)
		}
		out.Stocks = append(out.Stocks, &pb.Stock{
			SkuId: st.skuID, WarehouseId: st.warehouseID, OnHand: st.onHand,
			Reserved: st.reserved, Available: st.available(), Version: st.version,
		})
	}
	out.Total = int64(len(out.Stocks))
	return out, rows.Err()
}
