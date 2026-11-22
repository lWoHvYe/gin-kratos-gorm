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

func TestMultiGroupActionsExposeTypedGroupActions(t *testing.T) {
	o := &orderRecord{Status: statusAllocated, RefundState: refundStateNone}
	groups := []*fulfillmentGroup{
		{GroupNo: "G1", Status: "SHIPPED"},
		{GroupNo: "G2", Status: statusAllocated, PermitState: permitRevoked},
	}
	actions := multiGroupActions(o, groups)
	want := map[[2]string]bool{
		{"G1", "receive"}: true, {"G1", "apply_after_sale"}: true,
	}
	for _, action := range actions {
		delete(want, [2]string{action.GroupNo, action.Action})
	}
	if len(want) != 0 {
		t.Fatalf("missing typed actions %v in %v", want, actions)
	}
	for _, action := range actions {
		if action.GroupNo == "G2" {
			t.Fatalf("revoked permit exposed an action: %v", action)
		}
	}

	legacy := groupActionStrings(actions)
	for _, action := range legacy {
		if action == "ship" || action == "receive" || action == "apply_after_sale" || action == "ship_group:G2" {
			t.Fatalf("order-level action leaked for split order: %v", legacy)
		}
	}
}

func TestRevokedGroupIsClosedInProjection(t *testing.T) {
	groups := []*fulfillmentGroup{
		{GroupNo: "G1", Status: "SHIPPED"},
		{GroupNo: "G2", Status: statusAllocated, PermitState: permitRevoked},
	}
	if got := shipmentStatusForGroups(statusAllocated, groups); got != "SHIPPED" {
		t.Fatalf("shipment status = %s", got)
	}
	if got := aggregateOrderStatus(statusAllocated, statusShipped, groups); got != statusShipped {
		t.Fatalf("aggregate status = %s", got)
	}
	if got := effectiveGroupStatus(groups[1]); got != "CANCELLED" {
		t.Fatalf("effective group status = %s", got)
	}
}
