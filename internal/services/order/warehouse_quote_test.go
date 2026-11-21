package order

import (
	"testing"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

func TestValidateSingleWarehouseQuote(t *testing.T) {
	if err := validateSingleWarehouseQuote(&pb.Quote{}); err != nil {
		t.Fatalf("legacy quote rejected: %v", err)
	}
	if err := validateSingleWarehouseQuote(&pb.Quote{
		Groups: []*pb.QuoteGroup{{GroupNo: "G1", WarehouseId: 1}},
		Lines:  []*pb.QuoteLine{{WarehouseId: 1, GroupNo: "G1"}},
	}); err != nil {
		t.Fatalf("single group rejected: %v", err)
	}
	for _, quote := range []*pb.Quote{
		{Groups: []*pb.QuoteGroup{{GroupNo: "G1"}, {GroupNo: "G2"}}},
		{Lines: []*pb.QuoteLine{{WarehouseId: 2}}},
		{Lines: []*pb.QuoteLine{{WarehouseId: 1, GroupNo: "G2"}}},
	} {
		if err := validateSingleWarehouseQuote(quote); !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
			t.Fatalf("split quote accepted: %+v err=%v", quote, err)
		}
	}
}

func TestValidateSplitQuote(t *testing.T) {
	valid := &pb.Quote{
		Groups: []*pb.QuoteGroup{{GroupNo: "G1", WarehouseId: 1}, {GroupNo: "G2", WarehouseId: 2}},
		Lines: []*pb.QuoteLine{
			{WarehouseId: 1, GroupNo: "G1"},
			{WarehouseId: 2, GroupNo: "G2"},
		},
	}
	if err := validateSplitQuote(valid); err != nil {
		t.Fatalf("valid split quote rejected: %v", err)
	}
	for _, quote := range []*pb.Quote{
		{Groups: []*pb.QuoteGroup{{GroupNo: "", WarehouseId: 1}}},
		{Groups: []*pb.QuoteGroup{{GroupNo: "G1", WarehouseId: 1}, {GroupNo: "G1", WarehouseId: 2}}},
		{Lines: []*pb.QuoteLine{{WarehouseId: 0, GroupNo: "G1"}}},
		{Groups: []*pb.QuoteGroup{{GroupNo: "G1", WarehouseId: 1}}, Lines: []*pb.QuoteLine{{WarehouseId: 2, GroupNo: "G1"}}},
	} {
		if err := validateSplitQuote(quote); err == nil {
			t.Fatalf("invalid split quote accepted: %+v", quote)
		}
	}
}

func TestBuildCreateOrderInnerCarriesGroups(t *testing.T) {
	quote := &pb.Quote{
		TotalCent: 2000, DiscountCent: 0, FreightCent: 2000, PayCent: 4000,
		Groups: []*pb.QuoteGroup{{GroupNo: "G1", WarehouseId: 1, FreightCent: 1000}, {GroupNo: "G2", WarehouseId: 2, FreightCent: 1000}},
		Lines: []*pb.QuoteLine{
			{SkuId: 1, Name: "a", PriceCent: 1000, Quantity: 1, WarehouseId: 1, GroupNo: "G1"},
			{SkuId: 2, Name: "b", PriceCent: 1000, Quantity: 1, WarehouseId: 2, GroupNo: "G2"},
		},
	}
	inner := buildCreateOrderInner("ORD-1", createOrderInput{UserID: 1, QuoteID: "QTE-1"}, quote, time.Unix(100, 0))
	if len(inner.GetFulfillmentGroups()) != 2 {
		t.Fatalf("groups = %+v", inner.GetFulfillmentGroups())
	}
	if inner.GetItems()[1].GetWarehouseId() != 2 || inner.GetItems()[1].GetGroupNo() != "G2" {
		t.Fatalf("item = %+v", inner.GetItems()[1])
	}
}
