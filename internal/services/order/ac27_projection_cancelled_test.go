package order

import "testing"

// AC-27 EARS-3 / SPEC §7.2: a group row that is already CANCELLED (the order
// cancellation path releases its reservation through
// setFulfillmentGroupStatus) is closed by the existing read-model fields. No
// new waybill field is invented; the persisted group status is the only
// order-side cancellation fact.
func TestAC27CancelledGroupIsClosedInProjection(t *testing.T) {
	groups := []*fulfillmentGroup{
		{GroupNo: "G1", Status: statusCancelled, RefundState: refundStateNone,
			ReservationState: "RELEASED", PermitState: permitNone},
		{GroupNo: "G2", Status: statusCancelled, RefundState: refundStateNone,
			ReservationState: "RELEASED", PermitState: permitNone},
	}
	for _, group := range groups {
		if !groupIsClosed(group) {
			t.Fatalf("cancelled group %s is not closed", group.GroupNo)
		}
		if got := effectiveGroupStatus(group); got != statusCancelled {
			t.Fatalf("effective group %s = %s", group.GroupNo, got)
		}
	}
	if got := shipmentStatusForGroups(statusCancelled, groups); got != "UNSHIPPED" {
		t.Fatalf("all-cancelled shipment status = %s", got)
	}
	if got := aggregateOrderStatus(statusCancelled, statusShipped, groups); got != "" {
		t.Fatalf("all-cancelled order advanced to %s", got)
	}
}

// AC-27 / SPEC §7.5 "全部退款（未发）": every unshipped group refunded to
// completion is closed in the projection while the order lifecycle stays
// ALLOCATED. Refund progress lives in refund_state, never in orders.status.
func TestAC27AllRefundedUnshippedGroupsStayAllocated(t *testing.T) {
	groups := []*fulfillmentGroup{
		{GroupNo: "G1", Status: statusAllocated, RefundState: groupRefundStateRefunded,
			ReservationState: "RELEASED", PermitState: permitRevoked},
		{GroupNo: "G2", Status: statusAllocated, RefundState: groupRefundStateRefunded,
			ReservationState: "RELEASED", PermitState: permitRevoked},
	}
	if got := shipmentStatusForGroups(statusAllocated, groups); got != "UNSHIPPED" {
		t.Fatalf("fully refunded unshipped projection = %s", got)
	}
	if got := aggregateOrderStatus(statusAllocated, statusShipped, groups); got != "" {
		t.Fatalf("fully refunded unshipped order advanced to %s", got)
	}
	out := (&orderRecord{OrderNo: "ORD-AC27-ALL", Status: statusAllocated, PaidCent: 1000,
		RefundedCent: 1000, RefundState: refundStateFull}).toProto(nil, groups)
	if got := out.GetStatus(); got != statusAllocated {
		t.Fatalf("fully refunded order lifecycle = %s, want ALLOCATED", got)
	}
	for _, group := range out.GetFulfillmentGroups() {
		if got := group.GetStatus(); got != statusCancelled {
			t.Fatalf("projected %s = %s, want CANCELLED", group.GetGroupNo(), got)
		}
		if got := group.GetRefundState(); got != groupRefundStateRefunded {
			t.Fatalf("projected %s refund_state = %s", group.GetGroupNo(), got)
		}
	}
}
