package pricing

import (
	"fmt"
	"sort"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/money"
)

const (
	maxQuoteLinesPerSKU = 2
	maxQuoteGroups      = 3
)

// expandQuoteLines applies the read-only allocator result to the priced items.
// A split SKU becomes several quote lines with the same unit price; line_no is
// assigned once, after ordering by (original item order, split_seq), and group
// numbers follow the first appearance of a warehouse. No allocations keeps the
// legacy single-warehouse/G1 behavior.
func expandQuoteLines(items []quoteLine, allocations []*pb.QuoteAllocation) ([]quoteLine, error) {
	if len(allocations) == 0 {
		for i := range items {
			items[i].LineNo = int32(i + 1)
			items[i].WarehouseID = 1
			items[i].GroupNo = "G1"
			items[i].SplitSeq = 0
		}
		return items, nil
	}
	bySKU := make(map[int64][]*pb.QuoteAllocation, len(items))
	for _, allocation := range allocations {
		if allocation == nil || allocation.GetSkuId() <= 0 || allocation.GetWarehouseId() <= 0 {
			return nil, apperr.Invalid("quote allocation needs positive sku_id and warehouse_id")
		}
		if allocation.GetQuantity() < 0 {
			return nil, apperr.Invalid("quote allocation quantity must not be negative")
		}
		bySKU[allocation.GetSkuId()] = append(bySKU[allocation.GetSkuId()], allocation)
	}
	itemSKUs := make(map[int64]struct{}, len(items))
	for _, item := range items {
		itemSKUs[item.SkuID] = struct{}{}
	}
	for skuID := range bySKU {
		if _, ok := itemSKUs[skuID]; !ok {
			return nil, apperr.Invalid("quote allocation references sku %d which is not in the quote", skuID)
		}
	}
	expanded := make([]quoteLine, 0, len(items))
	for _, item := range items {
		allocations := bySKU[item.SkuID]
		if len(allocations) == 0 {
			return nil, apperr.Invalid("quote allocations must cover sku %d", item.SkuID)
		}
		if len(allocations) > maxQuoteLinesPerSKU {
			return nil, apperr.Failed(fmt.Sprintf("sku %d needs more than %d warehouses", item.SkuID, maxQuoteLinesPerSKU)).
				WithDetails(map[string]any{"sku_id": item.SkuID, "min_packages_needed": len(allocations)})
		}
		sort.SliceStable(allocations, func(i, j int) bool {
			if allocations[i].GetSplitSeq() != allocations[j].GetSplitSeq() {
				return allocations[i].GetSplitSeq() < allocations[j].GetSplitSeq()
			}
			return allocations[i].GetWarehouseId() < allocations[j].GetWarehouseId()
		})
		var total int64
		for index, allocation := range allocations {
			quantity := int64(allocation.GetQuantity())
			if quantity == 0 {
				if len(allocations) != 1 {
					return nil, apperr.Invalid("split allocation for sku %d needs an explicit quantity", item.SkuID)
				}
				quantity = int64(item.Quantity)
			}
			total += quantity
			splitSeq := allocation.GetSplitSeq()
			if splitSeq == 0 {
				splitSeq = int32(index + 1)
			}
			line := item
			line.Quantity = int32(quantity)
			line.WarehouseID = allocation.GetWarehouseId()
			line.SplitSeq = splitSeq
			expanded = append(expanded, line)
		}
		if total != int64(item.Quantity) {
			return nil, apperr.Invalid("split quantity for sku %d must equal the requested quantity", item.SkuID)
		}
	}
	// Group numbers follow warehouse_id ascending (P-15), never priority or
	// first appearance. Line numbers remain ordered by (item, split_seq).
	warehouseIDs := make([]int64, 0, maxQuoteGroups)
	seenWarehouse := make(map[int64]struct{}, maxQuoteGroups)
	for i := range expanded {
		warehouseID := expanded[i].WarehouseID
		if _, ok := seenWarehouse[warehouseID]; !ok {
			seenWarehouse[warehouseID] = struct{}{}
			warehouseIDs = append(warehouseIDs, warehouseID)
		}
	}
	if len(warehouseIDs) > maxQuoteGroups {
		return nil, apperr.Failed("order needs more than three fulfillment groups").
			WithDetails(map[string]any{"min_packages_needed": len(warehouseIDs), "suggestion": "split_orders"})
	}
	sort.Slice(warehouseIDs, func(i, j int) bool { return warehouseIDs[i] < warehouseIDs[j] })
	groupByWarehouse := make(map[int64]string, len(warehouseIDs))
	for i, warehouseID := range warehouseIDs {
		groupByWarehouse[warehouseID] = fmt.Sprintf("G%d", i+1)
	}
	for i := range expanded {
		expanded[i].GroupNo = groupByWarehouse[expanded[i].WarehouseID]
		expanded[i].LineNo = int32(i + 1)
	}
	return expanded, nil
}

// allocateDiscount spreads the order discount over the expanded lines using the
// shared largest-remainder implementation, then derives the immutable line paid
// amount and unit paid price (floor, last refund adds the remainder).
func allocateDiscount(lines []quoteLine, totalCent, discountCent int64) {
	if len(lines) == 0 {
		return
	}
	weights := make([]int64, len(lines))
	for i := range lines {
		weights[i] = lines[i].PriceCent * int64(lines[i].Quantity)
	}
	var shares []int64
	if discountCent > 0 {
		if totalCent <= 0 {
			discountCent = 0
		}
		shares = money.AllocateProportional(discountCent, weights)
	}
	for i := range lines {
		discount := int64(0)
		if i < len(shares) {
			discount = shares[i]
		}
		goods := weights[i]
		if discount > goods {
			discount = goods
		}
		lines[i].DiscountCent = discount
		lines[i].LinePaidCent = goods - discount
		if lines[i].Quantity > 0 {
			lines[i].UnitPaidCent = lines[i].LinePaidCent / int64(lines[i].Quantity)
		}
	}
}

// buildQuoteGroups computes the internal per-group allocation of the single
// order-level freight. The user-visible freight is Quote.freight_cent only;
// group shares exist for cost attribution and must sum to it exactly (INV-20).
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
		group.GoodsCent += line.PriceCent * int64(line.Quantity)
		group.DiscountCent += line.DiscountCent
	}
	groups := make([]quoteGroup, 0, len(byGroup))
	var sumLinePaid int64
	for _, group := range byGroup {
		groups = append(groups, *group)
		sumLinePaid += group.GoodsCent - group.DiscountCent
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].GroupNo < groups[j].GroupNo })
	freight := freightFor(sumLinePaid)
	if shippingDiscountCent > 0 {
		if shippingDiscountCent >= freight {
			freight = 0
		} else {
			freight -= shippingDiscountCent
		}
	}
	weights := make([]int64, len(groups))
	for i := range groups {
		weights[i] = groups[i].GoodsCent
	}
	shares := money.AllocateProportional(freight, weights)
	for i := range groups {
		groups[i].FreightShareCent = shares[i]
		// Keep the legacy column aligned with the internal share; the quote-level
		// freight is still the only amount charged to the user.
		groups[i].FreightCent = shares[i]
	}
	return groups, freight
}
