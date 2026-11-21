package order

import pb "gin-kratos-gorm/api/gen/mall/v1"

// mergeReserveLines folds duplicate (sku, warehouse) quote lines into one
// reservation action because the inventory reservation identity is
// (order_no, sku_id, warehouse_id). Order lines remain separate for display,
// returns and per-line refund caps.
func mergeReserveLines(lines []*pb.QuoteLine) []*pb.QuoteLine {
	merged := make([]*pb.QuoteLine, 0, len(lines))
	index := make(map[[2]int64]int)
	for _, line := range lines {
		warehouseID := line.GetWarehouseId()
		if warehouseID <= 0 {
			warehouseID = defaultWarehouseID
		}
		key := [2]int64{line.GetSkuId(), warehouseID}
		if i, ok := index[key]; ok {
			merged[i].Quantity += line.GetQuantity()
			continue
		}
		index[key] = len(merged)
		merged = append(merged, &pb.QuoteLine{
			SkuId: line.GetSkuId(), Quantity: line.GetQuantity(), WarehouseId: warehouseID,
		})
	}
	return merged
}
