package pricing

import (
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

// AC-02: a split SKU becomes two lines, two groups, and the quantities conserve.
func TestExpandQuoteLinesQuantitySplit(t *testing.T) {
	items := []quoteLine{{SkuID: 1, Name: "A", PriceCent: 1000, Quantity: 3}}
	lines, err := expandQuoteLines(items, []*pb.QuoteAllocation{
		{LineNo: 1, SkuId: 1, WarehouseId: 1, Quantity: 2, SplitSeq: 1},
		{LineNo: 2, SkuId: 1, WarehouseId: 2, Quantity: 1, SplitSeq: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 {
		t.Fatalf("lines = %+v", lines)
	}
	if lines[0].WarehouseID != 1 || lines[0].Quantity != 2 || lines[0].LineNo != 1 || lines[0].GroupNo != "G1" {
		t.Fatalf("first line = %+v", lines[0])
	}
	if lines[1].WarehouseID != 2 || lines[1].Quantity != 1 || lines[1].LineNo != 2 || lines[1].GroupNo != "G2" {
		t.Fatalf("second line = %+v", lines[1])
	}
	allocateDiscount(lines, 3000, 0)
	if lines[0].LinePaidCent != 2000 || lines[1].LinePaidCent != 1000 {
		t.Fatalf("line paid = %d/%d", lines[0].LinePaidCent, lines[1].LinePaidCent)
	}
}

// AC-04: freight is computed once on the order total and only shared internally.
func TestQuoteAllocationsChargeFreightOnce(t *testing.T) {
	items := []quoteLine{
		{SkuID: 1, PriceCent: 2000, Quantity: 2},
		{SkuID: 2, PriceCent: 2000, Quantity: 2},
	}
	lines, err := expandQuoteLines(items, []*pb.QuoteAllocation{
		{LineNo: 1, SkuId: 1, WarehouseId: 1, Quantity: 2, SplitSeq: 1},
		{LineNo: 2, SkuId: 2, WarehouseId: 2, Quantity: 2, SplitSeq: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	allocateDiscount(lines, 8000, 0)
	groups, freight := buildQuoteGroups(lines, 0)
	if len(groups) != 2 || groups[0].GroupNo != "G1" || groups[1].GroupNo != "G2" {
		t.Fatalf("groups = %+v", groups)
	}
	if freight != 1000 {
		t.Fatalf("order freight = %d, want 1000 (once)", freight)
	}
	var shares int64
	for _, group := range groups {
		shares += group.FreightShareCent
		if group.FreightCent != group.FreightShareCent {
			t.Fatalf("legacy group freight and share diverged: %+v", group)
		}
	}
	if shares != freight {
		t.Fatalf("group shares = %d, want %d", shares, freight)
	}
}

// AC-05: the free-shipping threshold is evaluated on the whole order, never per
// group, so two groups below the threshold still pay zero freight.
func TestFreeFreightThresholdIsOrderLevel(t *testing.T) {
	items := []quoteLine{
		{SkuID: 1, PriceCent: 4950, Quantity: 1},
		{SkuID: 2, PriceCent: 4950, Quantity: 1},
	}
	lines, err := expandQuoteLines(items, []*pb.QuoteAllocation{
		{LineNo: 1, SkuId: 1, WarehouseId: 1, Quantity: 1, SplitSeq: 1},
		{LineNo: 2, SkuId: 2, WarehouseId: 2, Quantity: 1, SplitSeq: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	allocateDiscount(lines, 9900, 0)
	groups, freight := buildQuoteGroups(lines, 0)
	if freight != 0 {
		t.Fatalf("freight = %d, want 0", freight)
	}
	for _, group := range groups {
		if group.FreightShareCent != 0 {
			t.Fatalf("group share = %+v", group)
		}
	}
}

func TestQuoteAllocationsDefaultAndValidation(t *testing.T) {
	lines, err := expandQuoteLines([]quoteLine{{SkuID: 1, PriceCent: 1000, Quantity: 1}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if lines[0].WarehouseID != 1 || lines[0].GroupNo != "G1" || lines[0].LineNo != 1 {
		t.Fatalf("default allocation = %+v", lines[0])
	}
	if _, err := expandQuoteLines([]quoteLine{{SkuID: 1, PriceCent: 1000, Quantity: 1}},
		[]*pb.QuoteAllocation{{SkuId: 2, WarehouseId: 1, Quantity: 1}}); !apperr.IsCode(err, apperr.CodeInvalidArgument) {
		t.Fatalf("uncovered sku err = %v", err)
	}
	if _, err := expandQuoteLines([]quoteLine{{SkuID: 1, PriceCent: 1000, Quantity: 3}},
		[]*pb.QuoteAllocation{
			{SkuId: 1, WarehouseId: 1, Quantity: 2, SplitSeq: 1},
			{SkuId: 1, WarehouseId: 2, Quantity: 2, SplitSeq: 2},
		}); !apperr.IsCode(err, apperr.CodeInvalidArgument) {
		t.Fatalf("quantity mismatch err = %v", err)
	}
}

// AC-38: group numbers are assigned by warehouse_id ascending, not by the
// order in which the allocator returned the rows, and are stable on replay.
func TestAC38GroupNumbersFollowWarehouseID(t *testing.T) {
	items := []quoteLine{{SkuID: 1, PriceCent: 1000, Quantity: 2}}
	allocations := []*pb.QuoteAllocation{
		{SkuId: 1, WarehouseId: 2, Quantity: 1, SplitSeq: 1},
		{SkuId: 1, WarehouseId: 1, Quantity: 1, SplitSeq: 2},
	}
	for i := 0; i < 2; i++ {
		lines, err := expandQuoteLines(append([]quoteLine(nil), items...), allocations)
		if err != nil {
			t.Fatal(err)
		}
		if lines[0].WarehouseID != 2 || lines[0].GroupNo != "G2" {
			t.Fatalf("warehouse 2 line = %+v", lines[0])
		}
		if lines[1].WarehouseID != 1 || lines[1].GroupNo != "G1" {
			t.Fatalf("warehouse 1 line = %+v", lines[1])
		}
	}
}

// AC-34: a freight coupon only reduces the order-level freight; it is never
// allocated into line discounts or line paid amounts.
func TestAC34FreightCouponDoesNotEnterLineDiscounts(t *testing.T) {
	items := []quoteLine{
		{SkuID: 1, PriceCent: 4000, Quantity: 1},
		{SkuID: 2, PriceCent: 4000, Quantity: 1},
	}
	lines, err := expandQuoteLines(items, []*pb.QuoteAllocation{
		{SkuId: 1, WarehouseId: 1, Quantity: 1, SplitSeq: 1},
		{SkuId: 2, WarehouseId: 2, Quantity: 1, SplitSeq: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	allocateDiscount(lines, 8000, 500)
	groups, freight := buildQuoteGroups(lines, 1000)
	if freight != 0 {
		t.Fatalf("freight = %d, want 0 after freight coupon", freight)
	}
	var lineDiscount int64
	for _, line := range lines {
		lineDiscount += line.DiscountCent
		if line.DiscountCent > 500 {
			t.Fatalf("freight coupon leaked into line discount: %+v", line)
		}
	}
	if lineDiscount != 500 {
		t.Fatalf("line discount sum = %d, want 500", lineDiscount)
	}
	for _, group := range groups {
		if group.FreightShareCent != 0 {
			t.Fatalf("group freight share = %+v", group)
		}
	}
}

// AC-03: splitting a SKU never changes the unit price and the line discounts
// still add up to the order discount.
func TestAC03SplitDoesNotChangePrice(t *testing.T) {
	items := []quoteLine{{SkuID: 1, PriceCent: 1000, Quantity: 3}}
	lines, err := expandQuoteLines(items, []*pb.QuoteAllocation{
		{SkuId: 1, WarehouseId: 1, Quantity: 2, SplitSeq: 1},
		{SkuId: 1, WarehouseId: 2, Quantity: 1, SplitSeq: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	allocateDiscount(lines, 3000, 500)
	var discounts, paid int64
	for _, line := range lines {
		if line.PriceCent != 1000 {
			t.Fatalf("unit price changed: %+v", line)
		}
		discounts += line.DiscountCent
		paid += line.LinePaidCent
	}
	groups, freight := buildQuoteGroups(lines, 0)
	if discounts != 500 || paid+freight != 2500+freight {
		t.Fatalf("discounts=%d paid=%d groups=%+v", discounts, paid, groups)
	}
}

// AC-28: the quote exposes the package/warehouse counts shown before payment.
func TestAC28QuoteExposesPackageCount(t *testing.T) {
	q := quote{
		TotalCent: 2000, FreightCent: 0, PayCent: 2000, PackageCount: 2, WarehouseCount: 2,
		Lines: []quoteLine{
			{LineNo: 1, SkuID: 1, PriceCent: 1000, Quantity: 1, WarehouseID: 1, GroupNo: "G1", LinePaidCent: 1000, UnitPaidCent: 1000},
			{LineNo: 2, SkuID: 2, PriceCent: 1000, Quantity: 1, WarehouseID: 2, GroupNo: "G2", LinePaidCent: 1000, UnitPaidCent: 1000},
		},
		Groups: []quoteGroup{
			{GroupNo: "G1", WarehouseID: 1, GoodsCent: 1000, FreightShareCent: 0},
			{GroupNo: "G2", WarehouseID: 2, GoodsCent: 1000, FreightShareCent: 0},
		},
	}
	out := q.toProto()
	if out.GetPackageCount() != 2 || out.GetWarehouseCount() != 2 {
		t.Fatalf("package/warehouse count = %d/%d", out.GetPackageCount(), out.GetWarehouseCount())
	}
}
