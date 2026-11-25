package order

import "testing"

// AC-27 reverse case: a group that already shipped must keep its shipped fact
// while its refund is still in progress (refund_state=REFUNDING, e.g. a
// post-ship after-sales refund). It must not be projected as CANCELLED and
// must not be counted as an unshipped group.
func TestAC27PostShipRefundingGroupStaysShipped(t *testing.T) {
	groups := []*fulfillmentGroup{
		{GroupNo: "G1", Status: "SHIPPED", RefundState: refundStateNone, PermitState: permitConsumed},
		{GroupNo: "G2", Status: "SHIPPED", RefundState: refundStateRefunding, PermitState: permitConsumed},
	}
	if got := effectiveGroupStatus(groups[1]); got != "SHIPPED" {
		t.Fatalf("refunding shipped group projected as %s", got)
	}
	if got := shipmentStatusForGroups(statusShipped, groups); got != "SHIPPED" {
		t.Fatalf("shipment status = %s, a refunding shipped group must still count as shipped", got)
	}
	out := (&orderRecord{OrderNo: "ORD-AC27-POST", Status: statusShipped, PaidCent: 1000,
		RefundState: refundStateRefunding}).toProto(nil, groups)
	if got := out.GetStatus(); got != statusShipped {
		t.Fatalf("order lifecycle polluted by refund progress: %s", got)
	}
	if got := out.GetShipmentStatus(); got != "SHIPPED" {
		t.Fatalf("order shipment projection = %s", got)
	}
	receive := false
	for _, action := range out.GetGroupActions() {
		if action.GetGroupNo() == "G2" && action.GetAction() == actionReceive {
			receive = true
		}
	}
	if !receive {
		t.Fatalf("shipped G2 lost its receive action while refunding: %v", out.GetGroupActions())
	}
}

// AC-27 reverse case: after the refund completes on a shipped group, the
// shipped/received facts still drive the fulfillment projection. A refunded
// group is never rewritten to CANCELLED and never counted as unshipped.
func TestAC27PostShipRefundedGroupStillCountsAsShipped(t *testing.T) {
	groups := []*fulfillmentGroup{
		{GroupNo: "G1", Status: "RECEIVED", RefundState: refundStateNone, PermitState: permitConsumed},
		{GroupNo: "G2", Status: "SHIPPED", RefundState: groupRefundStateRefunded, PermitState: permitConsumed},
	}
	if got := effectiveGroupStatus(groups[1]); got != "SHIPPED" {
		t.Fatalf("refunded shipped group projected as %s", got)
	}
	if got := shipmentStatusForGroups(statusShipped, groups); got != "PARTIALLY_RECEIVED" {
		t.Fatalf("shipment status = %s, refunded shipped group must not become unshipped", got)
	}
	if got := aggregateOrderStatus(statusShipped, statusCompleted, groups); got != "" {
		t.Fatalf("aggregate completed early = %s", got)
	}
	// Once the shipped group is received too, the order completes; the refund
	// on the group does not erase the receipt fact.
	groups[1].Status = "RECEIVED"
	if got := effectiveGroupStatus(groups[1]); got != "RECEIVED" {
		t.Fatalf("refunded received group projected as %s", got)
	}
	if got := shipmentStatusForGroups(statusShipped, groups); got != "RECEIVED" {
		t.Fatalf("shipment status = %s", got)
	}
	if got := aggregateOrderStatus(statusShipped, statusCompleted, groups); got != statusCompleted {
		t.Fatalf("aggregate status = %s", got)
	}
}
