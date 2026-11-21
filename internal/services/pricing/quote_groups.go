package pricing

import (
	"fmt"
	"sort"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

// applyQuoteAllocations binds each quote line to the preview allocation. No
// allocation keeps the legacy warehouse-1/G1 behavior. A partial or mismatched
// allocation set is rejected instead of silently mixing warehouses.
func applyQuoteAllocations(lines []quoteLine, allocations []*pb.QuoteAllocation) error {
	if len(allocations) == 0 {
		for i := range lines {
			lines[i].LineNo = int32(i + 1)
			lines[i].WarehouseID = 1
			lines[i].GroupNo = "G1"
		}
		return nil
	}
	if len(allocations) != len(lines) {
		return apperr.Invalid("quote allocations must cover every line")
	}
	byLine := make(map[int32]*pb.QuoteAllocation, len(allocations))
	for _, allocation := range allocations {
		if allocation.GetLineNo() <= 0 || allocation.GetWarehouseId() <= 0 {
			return apperr.Invalid("quote allocation needs positive line_no and warehouse_id")
		}
		if _, exists := byLine[allocation.GetLineNo()]; exists {
			return apperr.Invalid("duplicate quote allocation line_no")
		}
		byLine[allocation.GetLineNo()] = allocation
	}
	for i := range lines {
		lineNo := int32(i + 1)
		allocation, ok := byLine[lineNo]
		if !ok {
			return apperr.Invalid("quote allocations must cover every line")
		}
		if allocation.GetSkuId() != lines[i].SkuID {
			return apperr.Invalid("quote allocation sku mismatch")
		}
		lines[i].LineNo = lineNo
		lines[i].WarehouseID = allocation.GetWarehouseId()
	}
	warehouses := make([]int64, 0, len(lines))
	seen := make(map[int64]struct{}, len(lines))
	for _, line := range lines {
		if _, ok := seen[line.WarehouseID]; !ok {
			seen[line.WarehouseID] = struct{}{}
			warehouses = append(warehouses, line.WarehouseID)
		}
	}
	sort.Slice(warehouses, func(i, j int) bool { return warehouses[i] < warehouses[j] })
	groupByWarehouse := make(map[int64]string, len(warehouses))
	for i, warehouseID := range warehouses {
		groupByWarehouse[warehouseID] = fmt.Sprintf("G%d", i+1)
	}
	for i := range lines {
		lines[i].GroupNo = groupByWarehouse[lines[i].WarehouseID]
	}
	return nil
}

// buildQuoteGroups computes per-warehouse goods/freight totals. Freight is
// charged per group (flat rule below the threshold, as in the single-group
// path) and the promotion shipping discount is consumed deterministically in
// group order, so the group freight sum stays equal to Quote.freight_cent.
func buildQuoteGroups(lines []quoteLine, shippingDiscountCent int64) ([]quoteGroup, int64) {
	byGroup := make(map[string]*quoteGroup)
	for _, line := range lines {
		groupNo := line.GroupNo
		if groupNo == "" {
			groupNo = "G1"
		}
		group := byGroup[groupNo]
		if group == nil {
			group = &quoteGroup{GroupNo: groupNo, WarehouseID: line.WarehouseID}
			if group.WarehouseID <= 0 {
				group.WarehouseID = 1
			}
			byGroup[groupNo] = group
		}
		amount, err := lineAmount(line.PriceCent, line.Quantity)
		if err != nil {
			// CreateQuote validated each amount already; keep the helper total
			// stable for callers that pass malformed lines.
			continue
		}
		group.GoodsCent += amount
		group.DiscountCent += line.DiscountCent
	}
	groups := make([]quoteGroup, 0, len(byGroup))
	for _, group := range byGroup {
		group.FreightCent = freightFor(group.GoodsCent)
		groups = append(groups, *group)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].GroupNo < groups[j].GroupNo })
	remaining := shippingDiscountCent
	var freight int64
	for i := range groups {
		if remaining > 0 {
			cut := groups[i].FreightCent
			if cut > remaining {
				cut = remaining
			}
			groups[i].FreightCent -= cut
			remaining -= cut
		}
		freight += groups[i].FreightCent
	}
	return groups, freight
}
