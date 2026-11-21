package pricing

import (
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

func TestQuoteAllocationsDriveGroupFreight(t *testing.T) {
	lines := []quoteLine{
		{SkuID: 1, PriceCent: 5000, Quantity: 1},
		{SkuID: 2, PriceCent: 5000, Quantity: 1},
	}
	if err := applyQuoteAllocations(lines, []*pb.QuoteAllocation{
		{LineNo: 1, SkuId: 1, WarehouseId: 1},
		{LineNo: 2, SkuId: 2, WarehouseId: 2},
	}); err != nil {
		t.Fatal(err)
	}
	if lines[0].GroupNo != "G1" || lines[1].GroupNo != "G2" {
		t.Fatalf("groups = %s/%s", lines[0].GroupNo, lines[1].GroupNo)
	}
	groups, freight := buildQuoteGroups(lines, 0)
	if len(groups) != 2 || groups[0].GroupNo != "G1" || groups[1].GroupNo != "G2" {
		t.Fatalf("groups = %+v", groups)
	}
	if groups[0].FreightCent != 1000 || groups[1].FreightCent != 1000 || freight != 2000 {
		t.Fatalf("freight = %+v total=%d", groups, freight)
	}
	groups, freight = buildQuoteGroups(lines, 500)
	if freight != 1500 || groups[0].FreightCent != 500 || groups[1].FreightCent != 1000 {
		t.Fatalf("discounted freight = %+v total=%d", groups, freight)
	}
}

func TestQuoteAllocationsDefaultAndValidation(t *testing.T) {
	lines := []quoteLine{{SkuID: 1, PriceCent: 1000, Quantity: 1}}
	if err := applyQuoteAllocations(lines, nil); err != nil {
		t.Fatal(err)
	}
	if lines[0].WarehouseID != 1 || lines[0].GroupNo != "G1" || lines[0].LineNo != 1 {
		t.Fatalf("default allocation = %+v", lines[0])
	}
	if _, freight := buildQuoteGroups(lines, 0); freight != freightFor(1000) {
		t.Fatalf("single-group freight = %d", freight)
	}
	if err := applyQuoteAllocations(lines, []*pb.QuoteAllocation{{LineNo: 1, SkuId: 2, WarehouseId: 1}}); !apperr.IsCode(err, apperr.CodeInvalidArgument) {
		t.Fatalf("sku mismatch err = %v", err)
	}
	if err := applyQuoteAllocations(lines, []*pb.QuoteAllocation{{LineNo: 2, SkuId: 1, WarehouseId: 1}}); !apperr.IsCode(err, apperr.CodeInvalidArgument) {
		t.Fatalf("missing line err = %v", err)
	}
}
