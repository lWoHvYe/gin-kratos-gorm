package order

import (
	"testing"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

func validSplitQuote() *pb.Quote {
	quote := &pb.Quote{
		QuoteId: "QTE-1", TotalCent: 2000, DiscountCent: 0, FreightCent: 2000, PayCent: 4000,
		PackageCount: 2, WarehouseCount: 2,
		Groups: []*pb.QuoteGroup{
			{GroupNo: "G1", WarehouseId: 1, GoodsCent: 1000, FreightShareCent: 1000},
			{GroupNo: "G2", WarehouseId: 2, GoodsCent: 1000, FreightShareCent: 1000},
		},
		Lines: []*pb.QuoteLine{
			{LineNo: 1, SkuId: 1, Name: "a", PriceCent: 1000, Quantity: 1, WarehouseId: 1, GroupNo: "G1",
				LinePaidCent: 1000, UnitPaidCent: 1000},
			{LineNo: 2, SkuId: 2, Name: "b", PriceCent: 1000, Quantity: 1, WarehouseId: 2, GroupNo: "G2",
				LinePaidCent: 1000, UnitPaidCent: 1000},
		},
	}
	quote.AllocationHash = quoteAllocationHash(quote)
	return quote
}

func TestValidateQuoteForOrderSplit(t *testing.T) {
	if err := validateQuoteForOrder(validSplitQuote(), true); err != nil {
		t.Fatalf("valid split quote rejected: %v", err)
	}
	if err := validateQuoteForOrder(validSplitQuote(), false); !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("split quote accepted with split disabled: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*pb.Quote)
	}{
		{"missing group", func(q *pb.Quote) { q.Groups = nil }},
		{"duplicate group", func(q *pb.Quote) { q.Groups[1].GroupNo = "G1" }},
		{"line group mismatch", func(q *pb.Quote) { q.Lines[0].GroupNo = "G2" }},
		{"line paid mismatch", func(q *pb.Quote) { q.Lines[0].LinePaidCent = 900 }},
		{"discount sum mismatch", func(q *pb.Quote) { q.DiscountCent = 1; q.Lines[0].DiscountCent = 1; q.Lines[0].LinePaidCent = 999 }},
		{"freight share mismatch", func(q *pb.Quote) { q.Groups[0].FreightShareCent = 500; q.Groups[1].FreightShareCent = 500 }},
		{"pay mismatch", func(q *pb.Quote) { q.PayCent = 3999 }},
		{"too many groups", func(q *pb.Quote) {
			q.Groups = append(q.Groups, &pb.QuoteGroup{GroupNo: "G3", WarehouseId: 3})
			q.Lines = append(q.Lines, &pb.QuoteLine{LineNo: 3, SkuId: 3, PriceCent: 1000, Quantity: 1,
				WarehouseId: 3, GroupNo: "G3", LinePaidCent: 1000, UnitPaidCent: 1000})
			q.TotalCent, q.FreightCent, q.PayCent = 3000, 3000, 6000
			q.Groups[0].FreightShareCent, q.Groups[1].FreightShareCent, q.Groups[2].FreightShareCent = 1000, 1000, 1000
			q.PackageCount, q.WarehouseCount = 3, 3
		}},
		{"too many lines per sku", func(q *pb.Quote) {
			q.Lines[1].SkuId = 1
		}},
		{"hash drift", func(q *pb.Quote) { q.AllocationHash = "deadbeef" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			quote := validSplitQuote()
			tc.mutate(quote)
			if err := validateQuoteForOrder(quote, true); err == nil {
				t.Fatalf("invalid quote accepted: %+v", quote)
			}
		})
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
	policy := newOrderActionPolicy(o, groups)
	actions := policy.groupActions()
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

	legacy := policy.legacyActions()
	for _, action := range legacy {
		if action == "ship" || action == "receive" || action == "apply_after_sale" || action == "ship_group:G2" {
			t.Fatalf("order-level action leaked for split order: %v", legacy)
		}
	}
}

// AC-27: a refunded unshipped group is closed in the projection and does not
// block the remaining group. The EARS-2 precondition is refund_state=REFUNDED
// plus a revoked permit (the reservation was released by refund admission), so
// the fixture and the assertions use exactly those read-model fields.
func TestAC27RevokedGroupIsClosedInProjection(t *testing.T) {
	groups := []*fulfillmentGroup{
		{GroupNo: "G1", Status: "SHIPPED", RefundState: refundStateNone,
			ReservationState: "HELD", PermitState: permitConsumed},
		{GroupNo: "G2", Status: statusAllocated, RefundState: groupRefundStateRefunded,
			ReservationState: "RELEASED", PermitState: permitRevoked},
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

	// AC-27 EARS-2: refund progress is carried only by refund_state; the
	// lifecycle stays inside PAID/ALLOCATED/SHIPPED. Asserted through the same
	// read model the order RPCs return (repo.go toProto).
	record := &orderRecord{
		OrderNo: "ORD-AC27", Status: statusAllocated,
		PaidCent: 1000, RefundedCent: 400, RefundState: refundStatePartial,
	}
	out := record.toProto(nil, groups)
	if got := out.GetStatus(); got != statusAllocated {
		t.Fatalf("order lifecycle polluted by refund progress: %s", got)
	}
	if got := out.GetShipmentStatus(); got != "SHIPPED" {
		t.Fatalf("order shipment projection = %s", got)
	}
	var projected *pb.OrderFulfillmentGroup
	for _, group := range out.GetFulfillmentGroups() {
		if group.GetGroupNo() == "G2" {
			projected = group
		}
	}
	if projected == nil {
		t.Fatal("G2 missing from order projection")
	}
	if got := projected.GetStatus(); got != "CANCELLED" {
		t.Fatalf("projected G2 status = %s", got)
	}
	if got := projected.GetRefundState(); got != groupRefundStateRefunded {
		t.Fatalf("projected G2 refund_state = %s", got)
	}
	for _, action := range out.GetGroupActions() {
		if action.GetGroupNo() == "G2" {
			t.Fatalf("closed refunded G2 still exposes action %v", action)
		}
	}
}
