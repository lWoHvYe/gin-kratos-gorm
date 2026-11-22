package inventory

import (
	"context"
	"fmt"
	"sort"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

const (
	maxWarehousesPerSKU = 2
	maxGroupsPerOrder   = 3
	maxCandidatesPerSKU = 8
	maxPlannerNodes     = 20000
)

// AllocateOrder is a read-only preview: it chooses up to two warehouses per
// requested SKU and returns one allocation row per (warehouse, quantity) split.
// It never writes reservations or ledger rows. The result is deterministic for
// the same stock snapshot (ADR 0005 §5.2).
func (s *Service) AllocateOrder(ctx context.Context, req *pb.AllocateOrderRequest) (*pb.AllocateOrderReply, error) {
	items, err := mergeAllocationItems(req.GetLines())
	if err != nil {
		return nil, err
	}
	if !s.quantitySplitEnabled {
		return s.allocateWholeSKUs(ctx, items)
	}
	candidates := make([][]warehouseStock, len(items))
	availability := make([]*pb.AllocateOrderAvailability, 0, len(items)*2)
	for i, item := range items {
		snapshot, err := s.loadStockSnapshot(ctx, item.SkuID)
		if err != nil {
			return nil, err
		}
		for _, stock := range snapshot.All {
			availability = append(availability, &pb.AllocateOrderAvailability{
				SkuId: item.SkuID, WarehouseId: stock.WarehouseID, Available: stock.Available,
				Priority: int32(stock.Priority), Rank: int32(stock.Rank),
			})
		}
		candidates[i] = snapshot.Candidates
		if len(candidates[i]) == 0 {
			return nil, apperr.Failed(fmt.Sprintf("insufficient stock for sku %d", item.SkuID)).
				WithDetails(map[string]any{"sku_id": item.SkuID, "max_orderable_quantity": 0, "single_warehouse_max": 0})
		}
	}
	planner := newAllocationPlanner(items, candidates)
	plan, ok := planner.solve()
	if !ok {
		return nil, planner.failure()
	}
	reply := &pb.AllocateOrderReply{
		Allocations:  make([]*pb.AllocateOrderAllocation, 0, len(plan.rows)),
		Availability: availability,
	}
	for i, row := range plan.rows {
		reply.Allocations = append(reply.Allocations, &pb.AllocateOrderAllocation{
			SkuId: row.SkuID, Quantity: int32(row.Quantity), WarehouseId: row.WarehouseID,
			LineNo: int32(i + 1), SplitSeq: int32(row.SplitSeq),
		})
	}
	return reply, nil
}

// allocateWholeSKUs is the legacy C allocator: each SKU must fit entirely in
// one warehouse, choosing the first by (-available, warehouse_id).
func (s *Service) allocateWholeSKUs(ctx context.Context, items []allocationItem) (*pb.AllocateOrderReply, error) {
	reply := &pb.AllocateOrderReply{Allocations: make([]*pb.AllocateOrderAllocation, 0, len(items))}
	remaining := make(map[int64]map[int64]int64, len(items))
	for i, item := range items {
		if _, ok := remaining[item.SkuID]; !ok {
			snapshot, err := s.loadStockSnapshot(ctx, item.SkuID)
			if err != nil {
				return nil, err
			}
			pool := make(map[int64]int64, len(snapshot.Candidates))
			var total, single int64
			for _, candidate := range snapshot.Candidates {
				pool[candidate.WarehouseID] = candidate.Available
				total += candidate.Available
				if candidate.Available > single {
					single = candidate.Available
				}
			}
			if total < item.Quantity {
				return nil, apperr.Failed(fmt.Sprintf("insufficient stock for sku %d", item.SkuID)).
					WithDetails(map[string]any{"sku_id": item.SkuID, "max_orderable_quantity": total, "single_warehouse_max": single})
			}
			remaining[item.SkuID] = pool
		}
		warehouseID, ok := pickWholeWarehouse(remaining[item.SkuID], item.Quantity)
		if !ok {
			var single int64
			for _, available := range remaining[item.SkuID] {
				if available > single {
					single = available
				}
			}
			return nil, apperr.Failed(fmt.Sprintf("insufficient stock for sku %d in a single warehouse", item.SkuID)).
				WithDetails(map[string]any{"sku_id": item.SkuID, "single_warehouse_max": single, "suggestion": "reduce_quantity"})
		}
		remaining[item.SkuID][warehouseID] -= item.Quantity
		reply.Allocations = append(reply.Allocations, &pb.AllocateOrderAllocation{
			SkuId: item.SkuID, Quantity: int32(item.Quantity), WarehouseId: warehouseID,
			LineNo: int32(i + 1),
		})
	}
	return reply, nil
}

func pickWholeWarehouse(pool map[int64]int64, quantity int64) (int64, bool) {
	var best int64
	var bestAvailable int64
	for warehouseID, available := range pool {
		if available < quantity {
			continue
		}
		if best == 0 || available > bestAvailable || (available == bestAvailable && warehouseID < best) {
			best, bestAvailable = warehouseID, available
		}
	}
	return best, best != 0
}

type allocationItem struct {
	SkuID    int64
	Quantity int64
	Index    int
}

type warehouseStock struct {
	WarehouseID int64
	Available   int64
	Priority    int
	Rank        int
}

type lineAllocation struct {
	SkuID       int64
	WarehouseID int64
	Quantity    int64
	SplitSeq    int
}

func mergeAllocationItems(lines []*pb.AllocateOrderLine) ([]allocationItem, error) {
	if len(lines) == 0 {
		return nil, apperr.Invalid("at least one allocation line is required")
	}
	index := make(map[int64]int, len(lines))
	items := make([]allocationItem, 0, len(lines))
	for _, line := range lines {
		if line == nil || line.GetSkuId() <= 0 || line.GetQuantity() <= 0 {
			return nil, apperr.Invalid("allocation lines need positive sku_id and quantity")
		}
		if pos, ok := index[line.GetSkuId()]; ok {
			items[pos].Quantity += int64(line.GetQuantity())
			continue
		}
		index[line.GetSkuId()] = len(items)
		items = append(items, allocationItem{SkuID: line.GetSkuId(), Quantity: int64(line.GetQuantity()), Index: len(items)})
	}
	return items, nil
}

type stockSnapshot struct {
	All        []warehouseStock
	Candidates []warehouseStock
}

// loadStockSnapshot returns every warehouse row for the SKU, including
// available=0, so Order can audit why a warehouse was not chosen (INV-28).
func (s *Service) loadStockSnapshot(ctx context.Context, skuID int64) (stockSnapshot, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT warehouse_id, on_hand - reserved
		FROM inventory_stocks WHERE sku_id = ? AND warehouse_id > 0
		ORDER BY warehouse_id`, skuID)
	if err != nil {
		return stockSnapshot{}, apperr.Internal("load allocation stock", err)
	}
	defer rows.Close()
	all := make([]warehouseStock, 0, 4)
	for rows.Next() {
		var warehouseID, available int64
		if err := rows.Scan(&warehouseID, &available); err != nil {
			return stockSnapshot{}, apperr.Internal("scan allocation stock", err)
		}
		all = append(all, warehouseStock{WarehouseID: warehouseID, Available: available})
	}
	if err := rows.Err(); err != nil {
		return stockSnapshot{}, apperr.Internal("iterate allocation stock", err)
	}
	for i := range all {
		all[i].Priority = s.warehousePriorityRank(all[i].WarehouseID)
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].Priority != all[j].Priority {
			return all[i].Priority < all[j].Priority
		}
		if all[i].Available != all[j].Available {
			return all[i].Available > all[j].Available
		}
		return all[i].WarehouseID < all[j].WarehouseID
	})
	candidates := make([]warehouseStock, 0, len(all))
	for i := range all {
		all[i].Rank = i
		if all[i].Available > 0 {
			candidates = append(candidates, all[i])
		}
	}
	if len(candidates) > maxCandidatesPerSKU {
		candidates = candidates[:maxCandidatesPerSKU]
	}
	return stockSnapshot{All: all, Candidates: candidates}, nil
}

func (s *Service) warehousePriorityRank(warehouseID int64) int {
	for index, configured := range s.warehousePriority {
		if configured == warehouseID {
			return index
		}
	}
	return len(s.warehousePriority)
}

type skuScheme struct {
	Rows []lineAllocation
}

type planner struct {
	items          []allocationItem
	candidates     [][]warehouseStock
	nodes          int
	best           *plan
	deadline       time.Time
	budgetExceeded bool
	// minGroups is the smallest warehouse union observed for a feasible plan
	// ignoring the three-group cap, used only for failure details.
	minGroups int
}

type plan struct {
	rows []lineAllocation
}

func newAllocationPlanner(items []allocationItem, candidates [][]warehouseStock) *planner {
	return &planner{
		items: items, candidates: candidates, minGroups: maxGroupsPerOrder + 1,
		deadline: time.Now().Add(1500 * time.Millisecond),
	}
}

func (p *planner) solve() (*plan, bool) {
	schemes := make([][]skuScheme, len(p.items))
	for i, item := range p.items {
		schemes[i] = buildSchemes(item, p.candidates[i])
		if len(schemes[i]) == 0 {
			return nil, false
		}
	}
	// Search the most constrained SKU first; the final output is re-sorted by
	// original item order, so this only affects pruning, not determinism.
	order := make([]int, len(p.items))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		return len(schemes[order[a]]) < len(schemes[order[b]])
	})
	selected := make([]skuScheme, len(p.items))
	p.dfs(order, 0, schemes, selected, make(map[int64]struct{}, maxGroupsPerOrder))
	if p.budgetExceeded || p.best == nil {
		return nil, false
	}
	return p.best, true
}

func (p *planner) dfs(order []int, depth int, schemes [][]skuScheme, selected []skuScheme, used map[int64]struct{}) {
	p.nodes++
	if p.nodes > maxPlannerNodes || (p.nodes&255 == 0 && time.Now().After(p.deadline)) {
		p.budgetExceeded = true
		return
	}
	if depth == len(order) {
		if len(used) > maxGroupsPerOrder {
			if len(used) < p.minGroups {
				p.minGroups = len(used)
			}
			return
		}
		if len(used) < p.minGroups {
			p.minGroups = len(used)
		}
		candidate := make([]lineAllocation, 0, len(order))
		for _, itemIndex := range order {
			candidate = append(candidate, selected[itemIndex].Rows...)
		}
		p.consider(candidate, used)
		return
	}
	itemIndex := order[depth]
	for _, scheme := range schemes[itemIndex] {
		newWarehouses := make([]int64, 0, 2)
		skip := false
		for _, row := range scheme.Rows {
			if _, ok := used[row.WarehouseID]; ok {
				continue
			}
			if containsInt64(newWarehouses, row.WarehouseID) {
				continue
			}
			if len(used)+len(newWarehouses)+1 > maxGroupsPerOrder {
				skip = true
				break
			}
			newWarehouses = append(newWarehouses, row.WarehouseID)
		}
		if skip {
			continue
		}
		for _, warehouseID := range newWarehouses {
			used[warehouseID] = struct{}{}
		}
		selected[itemIndex] = scheme
		p.dfs(order, depth+1, schemes, selected, used)
		for _, warehouseID := range newWarehouses {
			delete(used, warehouseID)
		}
		if p.nodes > maxPlannerNodes {
			return
		}
	}
}

func (p *planner) consider(rows []lineAllocation, used map[int64]struct{}) {
	// Re-order output by the original request order and split_seq.
	bySku := make(map[int64][]lineAllocation, len(rows))
	for _, row := range rows {
		bySku[row.SkuID] = append(bySku[row.SkuID], row)
	}
	ordered := make([]lineAllocation, 0, len(rows))
	for _, item := range p.items {
		lines := bySku[item.SkuID]
		sort.Slice(lines, func(i, j int) bool { return lines[i].SplitSeq < lines[j].SplitSeq })
		ordered = append(ordered, lines...)
	}
	if p.best != nil && comparePlans(ordered, used, p.best.rows, p.usedSet(p.best.rows), p.candidates) >= 0 {
		return
	}
	p.best = &plan{rows: ordered}
}

func (p *planner) usedSet(rows []lineAllocation) map[int64]struct{} {
	used := make(map[int64]struct{}, maxGroupsPerOrder)
	for _, row := range rows {
		used[row.WarehouseID] = struct{}{}
	}
	return used
}

func buildSchemes(item allocationItem, candidates []warehouseStock) []skuScheme {
	schemes := make([]skuScheme, 0, len(candidates)+len(candidates)*len(candidates)/2)
	total := int64(0)
	for _, candidate := range candidates {
		total += candidate.Available
	}
	if total < item.Quantity {
		return nil
	}
	for _, candidate := range candidates {
		if candidate.Available >= item.Quantity {
			schemes = append(schemes, skuScheme{Rows: []lineAllocation{{
				SkuID: item.SkuID, WarehouseID: candidate.WarehouseID, Quantity: item.Quantity, SplitSeq: 1,
			}}})
		}
	}
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[i].Available+candidates[j].Available < item.Quantity {
				continue
			}
			first := candidates[i].Available
			if first > item.Quantity {
				first = item.Quantity
			}
			second := item.Quantity - first
			if second <= 0 || second > candidates[j].Available {
				continue
			}
			schemes = append(schemes, skuScheme{Rows: []lineAllocation{
				{SkuID: item.SkuID, WarehouseID: candidates[i].WarehouseID, Quantity: first, SplitSeq: 1},
				{SkuID: item.SkuID, WarehouseID: candidates[j].WarehouseID, Quantity: second, SplitSeq: 2},
			}})
		}
	}
	return schemes
}

func (p *planner) failure() error {
	if p.budgetExceeded {
		return apperr.Unavailable("allocation planning exceeded its budget; retry", nil)
	}
	var maxOrderable int64
	for i, item := range p.items {
		var available int64
		for _, candidate := range p.candidates[i] {
			available += candidate.Available
		}
		if available < item.Quantity {
			return apperr.Failed(fmt.Sprintf("insufficient stock for sku %d", item.SkuID)).
				WithDetails(map[string]any{
					"sku_id": item.SkuID, "max_orderable_quantity": available,
					"suggestion": "reduce_quantity",
				})
		}
		if maxOrderable == 0 || available < maxOrderable {
			maxOrderable = available
		}
	}
	minPackages := p.minimumPackages()
	details := map[string]any{
		"max_orderable_quantity": maxOrderable,
		"min_packages_needed":    minPackages,
		"suggestion":             "split_orders",
	}
	if minPackages <= maxGroupsPerOrder {
		details["suggestion"] = "reduce_quantity"
	}
	details["single_warehouse_max"] = p.singleWarehouseMax()
	return apperr.Failed("stock is spread across too many warehouses").WithDetails(details)
}

// comparePlans implements the lexicographic objective from ADR 0005 §5.2 step 3.
// It returns -1 when a is better than b, 0 when equal, 1 when worse.
func comparePlans(a []lineAllocation, aUsed map[int64]struct{}, b []lineAllocation, bUsed map[int64]struct{}, candidates [][]warehouseStock) int {
	if cmp := compareInt(len(aUsed), len(bUsed)); cmp != 0 {
		return cmp
	}
	if cmp := compareInt(splitSKUCount(a), splitSKUCount(b)); cmp != 0 {
		return cmp
	}
	// filled warehouses: a warehouse is filled when the plan assigns all of its
	// candidate availability. The allocator does not persist stock here, so use
	// the assigned quantity against the candidate available map.
	available := make(map[int64]int64)
	for _, list := range candidates {
		for _, candidate := range list {
			available[candidate.WarehouseID] = candidate.Available
		}
	}
	if cmp := compareInt(-filledWarehouseCount(a, available), -filledWarehouseCount(b, available)); cmp != 0 {
		return cmp
	}
	if cmp := compareInt64(-topPriorityQuantity(a, candidates), -topPriorityQuantity(b, candidates)); cmp != 0 {
		return cmp
	}
	if cmp := comparePriorityVector(aUsed, bUsed, candidates); cmp != 0 {
		return cmp
	}
	if cmp := compareAvailableVector(aUsed, bUsed, candidates); cmp != 0 {
		return cmp
	}
	return compareWarehouseIDs(aUsed, bUsed)
}

func splitSKUCount(rows []lineAllocation) int {
	counts := make(map[int64]int, len(rows))
	for _, row := range rows {
		counts[row.SkuID]++
	}
	split := 0
	for _, count := range counts {
		if count > 1 {
			split++
		}
	}
	return split
}

func filledWarehouseCount(rows []lineAllocation, available map[int64]int64) int {
	assigned := make(map[int64]int64)
	for _, row := range rows {
		assigned[row.WarehouseID] += row.Quantity
	}
	filled := 0
	for warehouseID, quantity := range assigned {
		if available[warehouseID] > 0 && quantity >= available[warehouseID] {
			filled++
		}
	}
	return filled
}

func topPriorityQuantity(rows []lineAllocation, candidates [][]warehouseStock) int64 {
	priority := make(map[int64]int)
	for _, list := range candidates {
		for _, candidate := range list {
			if existing, ok := priority[candidate.WarehouseID]; !ok || candidate.Priority < existing {
				priority[candidate.WarehouseID] = candidate.Priority
			}
		}
	}
	var topWarehouse int64
	topPriority := int(^uint(0) >> 1)
	for warehouseID := range warehouseTotals(rows) {
		if p, ok := priority[warehouseID]; ok && p < topPriority {
			topPriority = p
			topWarehouse = warehouseID
		}
	}
	return warehouseTotals(rows)[topWarehouse]
}

func warehouseTotals(rows []lineAllocation) map[int64]int64 {
	totals := make(map[int64]int64)
	for _, row := range rows {
		totals[row.WarehouseID] += row.Quantity
	}
	return totals
}

func comparePriorityVector(a, b map[int64]struct{}, candidates [][]warehouseStock) int {
	priority := make(map[int64]int)
	for _, list := range candidates {
		for _, candidate := range list {
			if existing, ok := priority[candidate.WarehouseID]; !ok || candidate.Priority < existing {
				priority[candidate.WarehouseID] = candidate.Priority
			}
		}
	}
	return compareIntVectors(sortedValues(a, priority), sortedValues(b, priority))
}

func sortedValues(set map[int64]struct{}, lookup map[int64]int) []int {
	values := make([]int, 0, len(set))
	for warehouseID := range set {
		if value, ok := lookup[warehouseID]; ok {
			values = append(values, value)
		} else {
			values = append(values, maxCandidatesPerSKU)
		}
	}
	sort.Ints(values)
	return values
}

func compareIntVectors(a, b []int) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if cmp := compareInt(a[i], b[i]); cmp != 0 {
			return cmp
		}
	}
	return compareInt(len(a), len(b))
}

func compareAvailableVector(a, b map[int64]struct{}, candidates [][]warehouseStock) int {
	available := make(map[int64]int64)
	for _, list := range candidates {
		for _, candidate := range list {
			available[candidate.WarehouseID] = candidate.Available
		}
	}
	aValues, bValues := make([]int64, 0, len(a)), make([]int64, 0, len(b))
	for warehouseID := range a {
		aValues = append(aValues, available[warehouseID])
	}
	for warehouseID := range b {
		bValues = append(bValues, available[warehouseID])
	}
	sort.Slice(aValues, func(i, j int) bool { return aValues[i] > aValues[j] })
	sort.Slice(bValues, func(i, j int) bool { return bValues[i] > bValues[j] })
	for i := 0; i < len(aValues) && i < len(bValues); i++ {
		if cmp := compareInt64(aValues[i], bValues[i]); cmp != 0 {
			return -cmp
		}
	}
	return compareInt(len(aValues), len(bValues))
}

func compareWarehouseIDs(a, b map[int64]struct{}) int {
	aIDs, bIDs := make([]int64, 0, len(a)), make([]int64, 0, len(b))
	for warehouseID := range a {
		aIDs = append(aIDs, warehouseID)
	}
	for warehouseID := range b {
		bIDs = append(bIDs, warehouseID)
	}
	sort.Slice(aIDs, func(i, j int) bool { return aIDs[i] < aIDs[j] })
	sort.Slice(bIDs, func(i, j int) bool { return bIDs[i] < bIDs[j] })
	for i := 0; i < len(aIDs) && i < len(bIDs); i++ {
		if cmp := compareInt64(aIDs[i], bIDs[i]); cmp != 0 {
			return cmp
		}
	}
	return compareInt(len(aIDs), len(bIDs))
}

func compareInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func compareInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func containsInt64(values []int64, target int64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// minimumPackages is a deterministic lower bound for the error contract: the
// smallest number of warehouses needed to cover every SKU's quantity when each
// warehouse is used at most once per SKU. It is used only for failure details,
// never to produce an allocation beyond the hard limits.
func (p *planner) minimumPackages() int {
	used := make(map[int64]struct{}, maxGroupsPerOrder)
	for i, item := range p.items {
		remaining := item.Quantity
		for _, candidate := range p.candidates[i] {
			if remaining <= 0 {
				break
			}
			used[candidate.WarehouseID] = struct{}{}
			remaining -= candidate.Available
		}
		if remaining > 0 {
			continue
		}
	}
	if len(used) == 0 {
		return 1
	}
	return len(used)
}

func (p *planner) singleWarehouseMax() int64 {
	maxOrderable := int64(0)
	for _, candidates := range p.candidates {
		var best int64
		for _, candidate := range candidates {
			if candidate.Available > best {
				best = candidate.Available
			}
		}
		if maxOrderable == 0 || best < maxOrderable {
			maxOrderable = best
		}
	}
	return maxOrderable
}
