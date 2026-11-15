package inventory

import (
	"context"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

// ListLowStock returns the rows closest to running out. Available is
// on_hand - reserved, so held stock is not reported as sellable.
func (s *Service) ListLowStock(ctx context.Context, req *pb.ListLowStockRequest) (*pb.StockList, error) {
	threshold := req.GetThreshold()
	if threshold < 0 {
		return nil, apperr.Invalid("threshold must be >= 0")
	}
	limit := req.GetLimit()
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	var total int64
	if err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM inventory_stocks WHERE on_hand - reserved <= ?", threshold).Scan(&total); err != nil {
		return nil, apperr.Internal("count low stock", err)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT sku_id, warehouse_id, on_hand, reserved
		FROM inventory_stocks WHERE on_hand - reserved <= ?
		ORDER BY (on_hand - reserved) ASC, sku_id LIMIT ?`, threshold, limit)
	if err != nil {
		return nil, apperr.Internal("list low stock", err)
	}
	defer rows.Close()
	reply := &pb.StockList{Total: total}
	for rows.Next() {
		var st stock
		if err := rows.Scan(&st.skuID, &st.warehouseID, &st.onHand, &st.reserved); err != nil {
			return nil, apperr.Internal("scan low stock", err)
		}
		reply.Stocks = append(reply.Stocks, &pb.Stock{
			SkuId: st.skuID, WarehouseId: st.warehouseID,
			OnHand: st.onHand, Reserved: st.reserved, Available: st.available(),
		})
	}
	return reply, rows.Err()
}
