package order

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/allocation"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/outbox"
)

const (
	maxLinesPerSKU = 2
	maxOrderGroups = 3
)

// validateQuoteForOrder is the Order-side second check of INV-17/18/19/20/21.
// Pricing is the only calculator; Order rejects anything that does not conserve
// money or that no longer matches the preview allocation hash.
func validateQuoteForOrder(quote *pb.Quote, allowSplit bool) error {
	if quote == nil || len(quote.GetLines()) == 0 {
		return apperr.Failed("quote has no lines")
	}
	groups := make(map[string]*pb.QuoteGroup, len(quote.GetGroups()))
	warehouses := make(map[int64]struct{}, len(quote.GetGroups()))
	for _, group := range quote.GetGroups() {
		if group.GetGroupNo() == "" || group.GetWarehouseId() <= 0 {
			return apperr.Internal("pricing returned an invalid fulfillment group", nil)
		}
		if _, exists := groups[group.GetGroupNo()]; exists {
			return apperr.Internal("pricing returned a duplicate fulfillment group", nil)
		}
		if _, exists := warehouses[group.GetWarehouseId()]; exists {
			return apperr.Internal("pricing returned two groups for one warehouse", nil)
		}
		groups[group.GetGroupNo()] = group
		warehouses[group.GetWarehouseId()] = struct{}{}
	}
	if len(groups) == 0 {
		return apperr.Internal("pricing returned a quote without fulfillment groups", nil)
	}
	if len(groups) > maxOrderGroups {
		return apperr.Failed("order needs more than three fulfillment groups").
			WithDetails(map[string]any{"min_packages_needed": len(groups), "suggestion": "split_orders"})
	}
	if !allowSplit {
		if len(groups) > 1 {
			return apperr.Failed("cross-warehouse split is not enabled")
		}
	}
	perSKU := make(map[int64]int, len(quote.GetLines()))
	var (
		sumGoods    int64
		sumDiscount int64
		sumPaid     int64
	)
	for _, line := range quote.GetLines() {
		if line.GetSkuId() <= 0 || line.GetQuantity() <= 0 {
			return apperr.Internal("pricing returned a line without sku/quantity", nil)
		}
		if line.GetWarehouseId() <= 0 || line.GetGroupNo() == "" {
			return apperr.Internal("pricing returned a split line without group/warehouse", nil)
		}
		group, ok := groups[line.GetGroupNo()]
		if !ok {
			return apperr.Internal("pricing returned a line for an unknown group", nil)
		}
		if group.GetWarehouseId() != line.GetWarehouseId() {
			return apperr.Internal("pricing returned a line/group warehouse mismatch", nil)
		}
		perSKU[line.GetSkuId()]++
		if perSKU[line.GetSkuId()] > maxLinesPerSKU {
			return apperr.Failed(fmt.Sprintf("sku %d was split into more than %d rows", line.GetSkuId(), maxLinesPerSKU)).
				WithDetails(map[string]any{"sku_id": line.GetSkuId(), "min_packages_needed": perSKU[line.GetSkuId()]})
		}
		goods := line.GetPriceCent() * int64(line.GetQuantity())
		linePaid := goods - line.GetDiscountCent()
		if linePaid < 0 || line.GetDiscountCent() < 0 {
			return apperr.Failed("quote line amounts are invalid")
		}
		if line.GetLinePaidCent() != linePaid {
			return apperr.Failed("quote line paid amount does not match price minus discount")
		}
		if line.GetQuantity() > 0 && line.GetUnitPaidCent() != linePaid/int64(line.GetQuantity()) {
			return apperr.Failed("quote line unit paid amount does not match the floor rule")
		}
		sumGoods += goods
		sumDiscount += line.GetDiscountCent()
		sumPaid += linePaid
	}
	if sumGoods != quote.GetTotalCent() {
		return apperr.Failed("quote total does not equal the sum of line goods")
	}
	if sumDiscount != quote.GetDiscountCent() {
		return apperr.Failed("quote discount does not equal the sum of line discounts")
	}
	if sumPaid+quote.GetFreightCent() != quote.GetPayCent() {
		return apperr.Failed("quote payable does not equal line paid plus freight")
	}
	var groupShares, groupGoods, groupDiscount int64
	for _, group := range quote.GetGroups() {
		groupShares += group.GetFreightShareCent()
		groupGoods += group.GetGoodsCent()
		groupDiscount += group.GetDiscountCent()
	}
	if groupShares != quote.GetFreightCent() {
		return apperr.Failed("group freight shares do not add up to the order freight")
	}
	if groupGoods != quote.GetTotalCent() || groupDiscount != quote.GetDiscountCent() {
		return apperr.Failed("group goods/discount do not add up to the order totals")
	}
	if quote.GetPackageCount() != 0 && int(quote.GetPackageCount()) != len(groups) {
		return apperr.Failed("quote package_count changed")
	}
	if quote.GetWarehouseCount() != 0 && int(quote.GetWarehouseCount()) != len(warehouses) {
		return apperr.Failed("quote warehouse_count changed")
	}
	if quote.GetAllocationHash() != "" {
		if got := quoteAllocationHash(quote); got != quote.GetAllocationHash() {
			return apperr.Failed("quote allocation no longer matches the preview").
				WithDetails(map[string]any{"retry_preview": true})
		}
	}
	return nil
}

func quoteAllocationHash(quote *pb.Quote) string {
	lines := make([]allocation.Line, 0, len(quote.GetLines()))
	for _, line := range quote.GetLines() {
		lines = append(lines, allocation.Line{
			LineNo: line.GetLineNo(), SkuID: line.GetSkuId(), WarehouseID: line.GetWarehouseId(),
			GroupNo: line.GetGroupNo(), Quantity: line.GetQuantity(), PriceCent: line.GetPriceCent(),
			DiscountCent: line.GetDiscountCent(), LinePaidCent: line.GetLinePaidCent(),
			UnitPaidCent: line.GetUnitPaidCent(),
		})
	}
	groups := make([]allocation.Group, 0, len(quote.GetGroups()))
	for _, group := range quote.GetGroups() {
		groups = append(groups, allocation.Group{
			GroupNo: group.GetGroupNo(), WarehouseID: group.GetWarehouseId(), GoodsCent: group.GetGoodsCent(),
			DiscountCent: group.GetDiscountCent(), FreightShareCent: group.GetFreightShareCent(),
		})
	}
	return allocation.Hash(lines, groups)
}

// revalidateAllocation asks Inventory for a fresh read-only allocation and
// compares it with the priced quote as a multiset. Any drift rejects the order
// before coupons/promotions are locked (E5).
func (s *Service) revalidateAllocation(ctx context.Context, quote *pb.Quote) (*pb.AllocateOrderReply, error) {
	if s.inventory == nil {
		return nil, apperr.Unavailable("inventory client is not configured for allocation revalidation", nil)
	}
	lines := make([]*pb.AllocateOrderLine, 0, len(quote.GetLines()))
	index := make(map[int64]int, len(quote.GetLines()))
	for _, line := range quote.GetLines() {
		if pos, ok := index[line.GetSkuId()]; ok {
			lines[pos].Quantity += line.GetQuantity()
			continue
		}
		index[line.GetSkuId()] = len(lines)
		lines = append(lines, &pb.AllocateOrderLine{SkuId: line.GetSkuId(), Quantity: line.GetQuantity()})
	}
	callCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	reply, err := s.inventory.AllocateOrder(callCtx, &pb.AllocateOrderRequest{Lines: lines})
	if err != nil {
		return nil, apperr.Failed("allocation changed before order creation").
			WithDetails(map[string]any{"stock_changed": true, "retry_preview": true, "cause": err.Error()})
	}
	if reply == nil || len(reply.GetAvailability()) == 0 {
		return nil, apperr.Failed("allocator returned no availability snapshot").
			WithDetails(map[string]any{"stock_changed": true, "retry_preview": true})
	}
	if !sameAllocationMultiset(quote, reply.GetAllocations()) {
		return nil, apperr.Failed("allocation changed before order creation").
			WithDetails(map[string]any{"stock_changed": true, "retry_preview": true})
	}
	return reply, nil
}

func sameAllocationMultiset(quote *pb.Quote, allocations []*pb.AllocateOrderAllocation) bool {
	expected := make([]string, 0, len(quote.GetLines()))
	for _, line := range quote.GetLines() {
		expected = append(expected, allocationKey(line.GetSkuId(), line.GetWarehouseId(), int64(line.GetQuantity())))
	}
	actual := make([]string, 0, len(allocations))
	for _, allocation := range allocations {
		actual = append(actual, allocationKey(allocation.GetSkuId(), allocation.GetWarehouseId(), int64(allocation.GetQuantity())))
	}
	if len(expected) != len(actual) {
		return false
	}
	sort.Strings(expected)
	sort.Strings(actual)
	for i := range expected {
		if expected[i] != actual[i] {
			return false
		}
	}
	return true
}

func allocationKey(skuID, warehouseID, quantity int64) string {
	return fmt.Sprintf("%d:%d:%d", skuID, warehouseID, quantity)
}

// recordAllocationDecision appends the audit row for "why this allocation",
// including the allocator's warehouse availability snapshot (INV-28).
func (s *Service) recordAllocationDecision(ctx context.Context, orderNo string, quote *pb.Quote, allocation *pb.AllocateOrderReply) error {
	request := make([]map[string]any, 0, len(quote.GetLines()))
	chosen := make([]map[string]any, 0, len(quote.GetLines()))
	for _, line := range quote.GetLines() {
		request = append(request, map[string]any{
			"sku_id": line.GetSkuId(), "quantity": line.GetQuantity(),
		})
		chosen = append(chosen, map[string]any{
			"line_no": line.GetLineNo(), "sku_id": line.GetSkuId(), "warehouse_id": line.GetWarehouseId(),
			"group_no": line.GetGroupNo(), "quantity": line.GetQuantity(),
		})
	}
	warehouses := make([]map[string]any, 0, len(allocation.GetAvailability()))
	for _, availability := range allocation.GetAvailability() {
		warehouses = append(warehouses, map[string]any{
			"sku_id": availability.GetSkuId(), "warehouse_id": availability.GetWarehouseId(),
			"available": availability.GetAvailable(), "priority": availability.GetPriority(),
			"rank": availability.GetRank(),
		})
	}
	availability := map[string]any{
		"captured_at_unix": s.now().Unix(),
		"source":           "ORDER_REVALIDATION",
		"request":          request,
		"warehouses":       warehouses,
	}
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return apperr.Internal("marshal allocation request", err)
	}
	chosenJSON, err := json.Marshal(chosen)
	if err != nil {
		return apperr.Internal("marshal allocation decision", err)
	}
	availabilityJSON, err := json.Marshal(availability)
	if err != nil {
		return apperr.Internal("marshal allocation availability", err)
	}
	rules := deriveRuleHits(quote, allocation)
	rulesJSON, err := json.Marshal(rules)
	if err != nil {
		return apperr.Internal("marshal allocation rule hits", err)
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO order_allocation_decisions
		(order_no, quote_id, request_json, availability_json, chosen_json, rule_hits_json)
		VALUES (?, ?, ?, ?, ?, ?)`,
		orderNo, quote.GetQuoteId(), requestJSON, availabilityJSON, chosenJSON, rulesJSON)
	if err != nil {
		return apperr.Internal("record allocation decision", err)
	}
	return nil
}

func deriveRuleHits(quote *pb.Quote, allocation *pb.AllocateOrderReply) []string {
	perSKU := make(map[int64]int, len(quote.GetLines()))
	for _, line := range quote.GetLines() {
		perSKU[line.GetSkuId()]++
	}
	split := false
	for _, count := range perSKU {
		if count > 1 {
			split = true
			break
		}
	}
	rules := make([]string, 0, 6)
	if !split {
		rules = append(rules, "6.2:single_warehouse")
	} else {
		rules = append(rules, "6.2:min_warehouses", "6.2:concentration")
	}
	if len(allocation.GetAvailability()) > 0 {
		rules = append(rules, "6.2:priority")
	}
	rules = append(rules, "6.2:available_desc", "6.2:stable")
	return rules
}

type snapshotLine struct {
	LineNo       int32  `json:"line_no"`
	SkuID        int64  `json:"sku_id"`
	Name         string `json:"name"`
	PriceCent    int64  `json:"price_cent"`
	Quantity     int32  `json:"quantity"`
	DiscountCent int64  `json:"discount_cent"`
	LinePaidCent int64  `json:"line_paid_cent"`
	UnitPaidCent int64  `json:"unit_paid_cent"`
	WarehouseID  int64  `json:"warehouse_id"`
	GroupNo      string `json:"group_no"`
}

type snapshotGroup struct {
	GroupNo          string `json:"group_no"`
	WarehouseID      int64  `json:"warehouse_id"`
	FreightCent      int64  `json:"freight_cent"`
	FreightShareCent int64  `json:"freight_share_cent"`
}

func snapshotJSON(items []*pb.OrderItem, groups []*pb.OrderFulfillmentGroup) ([]byte, []byte, error) {
	lines := make([]snapshotLine, 0, len(items))
	for _, item := range items {
		lines = append(lines, snapshotLine{
			LineNo: item.GetLineNo(), SkuID: item.GetSkuId(), Name: item.GetName(),
			PriceCent: item.GetPriceCent(), Quantity: item.GetQuantity(), DiscountCent: item.GetDiscountCent(),
			LinePaidCent: item.GetLinePaidCent(), UnitPaidCent: item.GetUnitPaidCent(),
			WarehouseID: item.GetWarehouseId(), GroupNo: item.GetGroupNo(),
		})
	}
	groupRows := make([]snapshotGroup, 0, len(groups))
	for _, group := range groups {
		groupRows = append(groupRows, snapshotGroup{
			GroupNo: group.GetGroupNo(), WarehouseID: group.GetWarehouseId(), FreightCent: group.GetFreightCent(),
			FreightShareCent: group.GetFreightShareCent(),
		})
	}
	linesJSON, err := json.Marshal(lines)
	if err != nil {
		return nil, nil, apperr.Internal("marshal allocation lines snapshot", err)
	}
	groupsJSON, err := json.Marshal(groupRows)
	if err != nil {
		return nil, nil, apperr.Internal("marshal allocation groups snapshot", err)
	}
	return linesJSON, groupsJSON, nil
}

// insertAllocationSnapshotTx writes the append-only snapshot at order creation.
// It is written before payment so MarkPaid only has to set frozen_at once.
func insertAllocationSnapshotTx(ctx context.Context, tx *sql.Tx, req *pb.CreateOrderInnerRequest) error {
	linesJSON, groupsJSON, err := snapshotJSON(req.GetItems(), req.GetFulfillmentGroups())
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO order_allocation_snapshots
		(order_no, allocation_hash, lines_json, groups_json)
		VALUES (?, ?, ?, ?)`,
		req.GetOrderNo(), req.GetAllocationHash(), linesJSON, groupsJSON); err != nil {
		if isDuplicateKey(err) {
			return nil
		}
		return apperr.Internal("insert allocation snapshot", err)
	}
	return nil
}

// freezeAllocationTx is the single freeze point: the same transaction that
// flips PENDING_PAYMENT -> PAID sets allocation_frozen_at and seals the
// snapshot. A second writer gets zero affected rows and the caller reloads.
func freezeAllocationTx(ctx context.Context, tx *sql.Tx, orderNo string, paidCent int64, paymentNo string) (int64, error) {
	res, err := tx.ExecContext(ctx, `UPDATE orders
		SET status = ?, paid_cent = ?, settlement_payment_no = ?, allocation_frozen_at = NOW(3), version = version + 1, updated_at = NOW(3)
		WHERE order_no = ? AND status = ? AND allocation_frozen_at IS NULL`,
		statusPaid, paidCent, paymentNo, orderNo, statusPendingPayment)
	if err != nil {
		return 0, apperr.Internal("freeze order allocation", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, apperr.Internal("freeze order allocation rows", err)
	}
	if affected == 0 {
		return 0, nil
	}
	res, err = tx.ExecContext(ctx, `UPDATE order_allocation_snapshots
		SET frozen_at = NOW(3) WHERE order_no = ? AND frozen_at IS NULL`, orderNo)
	if err != nil {
		return 0, apperr.Internal("freeze allocation snapshot", err)
	}
	if _, err := res.RowsAffected(); err != nil {
		return 0, apperr.Internal("freeze allocation snapshot rows", err)
	}
	return affected, nil
}

// ensureAllocationSnapshotTx creates a snapshot for legacy orders created
// before this ADR. It must only run before the freeze.
func ensureAllocationSnapshotTx(ctx context.Context, tx *sql.Tx, record *orderRecord) error {
	items, err := loadOrderItems(ctx, tx, record.OrderNo)
	if err != nil {
		return apperr.Internal("load order items for snapshot", err)
	}
	groups, err := loadFulfillmentGroupsTx(ctx, tx, record.OrderNo)
	if err != nil {
		return apperr.Internal("load groups for snapshot", err)
	}
	pbGroups := make([]*pb.OrderFulfillmentGroup, 0, len(groups))
	for _, group := range groups {
		pbGroups = append(pbGroups, &pb.OrderFulfillmentGroup{
			GroupNo: group.GroupNo, WarehouseId: group.WarehouseID, FreightCent: group.FreightCent,
			FreightShareCent: group.FreightShareCent,
		})
	}
	linesJSON, groupsJSON, err := snapshotJSON(items, pbGroups)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO order_allocation_snapshots
		(order_no, allocation_hash, lines_json, groups_json)
		VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE order_no = order_no`,
		record.OrderNo, record.AllocationHash, linesJSON, groupsJSON); err != nil {
		return apperr.Internal("backfill allocation snapshot", err)
	}
	return nil
}

const topicOrderAllocationFrozen = "shop.order.allocation_frozen.v1"

func appendAllocationFrozenEvent(ctx context.Context, tx *sql.Tx, orderNo, hash string, packageCount, warehouseCount int32) error {
	return outbox.Append(ctx, tx, outbox.Event{
		Topic: topicOrderAllocationFrozen, AggregateType: "order", AggregateID: orderNo,
		EventType: "order.allocation_frozen",
		Payload: map[string]any{
			"order_no": orderNo, "allocation_hash": hash, "package_count": packageCount,
			"warehouse_count": warehouseCount, "frozen_at": time.Now().Unix(),
		},
	})
}

const topicOrderSettled = "shop.order.settled.v1"

// appendOrderSettledEvent emits the new settlement fact alongside the legacy
// order.paid event. Fulfillment consumes this event, never order.paid, for
// admission.
func appendOrderSettledEvent(ctx context.Context, tx *sql.Tx, record *orderRecord, paymentNo string) error {
	groups, err := loadFulfillmentGroupsTx(ctx, tx, record.OrderNo)
	if err != nil {
		return err
	}
	groupNos := make([]string, 0, len(groups))
	for _, group := range groups {
		groupNos = append(groupNos, group.GroupNo)
	}
	return outbox.Append(ctx, tx, outbox.Event{
		Topic: topicOrderSettled, AggregateType: "order", AggregateID: record.OrderNo,
		EventType: "order.settled",
		Payload: map[string]any{
			"order_no": record.OrderNo, "payment_no": paymentNo,
			"settled_at": time.Now().Unix(), "allocation_hash": record.AllocationHash,
			"package_count": record.PackageCount, "warehouse_count": record.WarehouseCount,
			"groups": groupNos,
		},
	})
}
