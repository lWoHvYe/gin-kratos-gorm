package order

import "testing"

// AC-27A (SPEC-0001 v1.5) fixes the closure predicate order and its inputs:
// nil -> cancelled -> shipped guard -> permit REVOKED -> pre-ship
// RefundAdmission -> open. The table pins that order, including the cases where
// a fact must NOT decide closure.
func TestAC27AClosurePredicateOrder(t *testing.T) {
	cases := []struct {
		name  string
		group *fulfillmentGroup
		want  bool
	}{
		{name: "missing group is open", group: nil, want: false},
		{name: "persisted CANCELLED closes", group: &fulfillmentGroup{Status: statusCancelled}, want: true},
		{
			name:  "CANCELLED wins over a pre-ship admission",
			group: &fulfillmentGroup{Status: statusCancelled, PreShipAdmission: true},
			want:  true,
		},
		{
			name:  "SHIPPED guard precedes permit REVOKED and admission",
			group: &fulfillmentGroup{Status: "SHIPPED", PermitState: permitRevoked, PreShipAdmission: true},
			want:  false,
		},
		{
			name:  "RECEIVED guard precedes a pre-ship admission",
			group: &fulfillmentGroup{Status: "RECEIVED", PreShipAdmission: true},
			want:  false,
		},
		{
			name:  "permit REVOKED closes an allocated group",
			group: &fulfillmentGroup{Status: statusAllocated, PermitState: permitRevoked},
			want:  true,
		},
		{
			name:  "CONSUMED plus pre-ship admission closes",
			group: &fulfillmentGroup{Status: statusAllocated, PermitState: permitConsumed, PreShipAdmission: true},
			want:  true,
		},
		{
			name:  "CONSUMED without a pre-ship admission stays open",
			group: &fulfillmentGroup{Status: statusAllocated, PermitState: permitConsumed},
			want:  false,
		},
		{
			name:  "pre-ship admission without a CONSUMED permit stays open",
			group: &fulfillmentGroup{Status: statusAllocated, PermitState: permitNone, PreShipAdmission: true},
			want:  false,
		},
		{
			name:  "permit NONE stays open",
			group: &fulfillmentGroup{Status: statusAllocated, PermitState: permitNone},
			want:  false,
		},
		{
			name:  "permit GRANTED stays open",
			group: &fulfillmentGroup{Status: statusAllocated, PermitState: permitGranted},
			want:  false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := groupIsClosed(tc.group); got != tc.want {
				t.Fatalf("groupIsClosed(%+v) = %v, want %v", tc.group, got, tc.want)
			}
		})
	}
}

// AC-27A: refund_state (S9) and reservation_state never participate in the
// closure predicate; pre-ship REFUNDING still closes when the Order-owned
// admission/permit terminal fact is present, and neither REFUNDED nor RELEASED
// alone closes. The release fact belongs to Inventory (INV-44).
func TestAC27AClosureIgnoresRefundStateAndReservationState(t *testing.T) {
	for _, refundState := range []string{refundStateNone, refundStateRefunding, groupRefundStateRefunded} {
		for _, reservation := range []string{"HELD", "RELEASED"} {
			closed := &fulfillmentGroup{
				Status: statusAllocated, PermitState: permitConsumed,
				RefundState: refundState, ReservationState: reservation, PreShipAdmission: true,
			}
			if !groupIsClosed(closed) {
				t.Fatalf("pre-ship admission must close regardless of refund/reservation: %+v", closed)
			}
			open := &fulfillmentGroup{
				Status: statusAllocated, PermitState: permitConsumed,
				RefundState: refundState, ReservationState: reservation,
			}
			if groupIsClosed(open) {
				t.Fatalf("refund/reservation state alone must not close: %+v", open)
			}
		}
	}
	released := &fulfillmentGroup{Status: statusAllocated, ReservationState: "RELEASED", PermitState: permitNone}
	if groupIsClosed(released) {
		t.Fatalf("released reservation alone must not close a group: %+v", released)
	}
	refunded := &fulfillmentGroup{Status: statusAllocated, RefundState: groupRefundStateRefunded, PermitState: permitConsumed}
	if groupIsClosed(refunded) {
		t.Fatalf("refund completion alone must not close an unshipped group: %+v", refunded)
	}
}

// AC-27A: a late S3-confirmed departure wins over the pre-ship admission (the
// shipped/received guard is first); the PRE row stays audit-only and must not
// re-close the group.
func TestAC27ALateShippedStatusWinsOverPreAdmission(t *testing.T) {
	for _, status := range []string{"SHIPPED", "RECEIVED"} {
		group := &fulfillmentGroup{
			GroupNo: "G2", Status: status, PermitState: permitConsumed,
			RefundState: groupRefundStateRefunded, PreShipAdmission: true,
		}
		if groupIsClosed(group) {
			t.Fatalf("late %s status must keep the group open: %+v", status, group)
		}
		if got := effectiveGroupStatus(group); got != status {
			t.Fatalf("effective group status = %s, want %s", got, status)
		}
	}
	groups := []*fulfillmentGroup{
		{GroupNo: "G1", Status: "SHIPPED", PermitState: permitConsumed},
		{GroupNo: "G2", Status: "SHIPPED", PermitState: permitConsumed, PreShipAdmission: true},
	}
	if got := shipmentStatusForGroups(statusShipped, groups); got != "SHIPPED" {
		t.Fatalf("shipment status = %s, want SHIPPED", got)
	}
}

// AC-27A: a closed group coexists with an existing CANCELLED lifecycle without
// rewriting it (AC-27 v1.5 EARS-3).
func TestAC27AClosedGroupKeepsCancelledLifecycle(t *testing.T) {
	groups := []*fulfillmentGroup{{
		GroupNo: "G1", Status: statusAllocated, PermitState: permitConsumed, PreShipAdmission: true,
	}}
	record := &orderRecord{OrderNo: "ORD-AC27A-CANCELLED", Status: statusCancelled, PaidCent: 1000}
	out := record.toProto(nil, groups)
	if got := out.GetStatus(); got != statusCancelled {
		t.Fatalf("lifecycle = %s, want CANCELLED preserved", got)
	}
	if got := out.GetShipmentStatus(); got != "UNSHIPPED" {
		t.Fatalf("shipment status = %s, want UNSHIPPED", got)
	}
	if projected := projectedGroup(t, out, "G1"); projected.GetStatus() != statusCancelled {
		t.Fatalf("projected group = %s, want CANCELLED", projected.GetStatus())
	}
}

// AC-27A / AC-27 EARS-1 three-group boundary: a still-open G3 keeps the read
// model at PARTIALLY_SHIPPED even when G2 is closed; only when every remaining
// group has shipped does shipment_status become SHIPPED. orders.status (S1) is
// never advanced by closure itself.
func TestAC27AThreeGroupBoundary(t *testing.T) {
	groups := []*fulfillmentGroup{
		{GroupNo: "G1", Status: "SHIPPED", PermitState: permitConsumed},
		{GroupNo: "G2", Status: statusAllocated, PermitState: permitConsumed,
			RefundState: groupRefundStateRefunded, PreShipAdmission: true},
		{GroupNo: "G3", Status: statusAllocated, PermitState: permitGranted},
	}
	if got := effectiveGroupStatus(groups[1]); got != statusCancelled {
		t.Fatalf("closed G2 projection = %s, want CANCELLED", got)
	}
	if groupIsClosed(groups[2]) {
		t.Fatal("still-planned G3 must stay open")
	}
	if got := shipmentStatusForGroups(statusAllocated, groups); got != "PARTIALLY_SHIPPED" {
		t.Fatalf("three-group shipment status = %s, want PARTIALLY_SHIPPED", got)
	}
	if got := aggregateOrderStatus(statusAllocated, statusShipped, groups); got != "" {
		t.Fatalf("three-group aggregate advanced to %q, want no transition", got)
	}
	record := &orderRecord{OrderNo: "ORD-AC27A-3G", Status: statusAllocated, PaidCent: 1000}
	out := record.toProto(nil, groups)
	if got := out.GetStatus(); got != statusAllocated {
		t.Fatalf("persisted lifecycle projection = %s, want ALLOCATED", got)
	}
	if got := out.GetShipmentStatus(); got != "PARTIALLY_SHIPPED" {
		t.Fatalf("read-model shipment status = %s, want PARTIALLY_SHIPPED", got)
	}

	// Once G3 also closes (same Order-owned terminal fact), the read-model
	// projection advances while orders.status keeps its existing value.
	groups[2].PermitState = permitConsumed
	groups[2].PreShipAdmission = true
	if got := shipmentStatusForGroups(statusAllocated, groups); got != "SHIPPED" {
		t.Fatalf("all-remaining-shipped status = %s, want SHIPPED", got)
	}
	if got := aggregateOrderStatus(statusAllocated, statusShipped, groups); got != statusShipped {
		t.Fatalf("aggregate = %s, want SHIPPED", got)
	}
	out = record.toProto(nil, groups)
	if got := out.GetStatus(); got != statusAllocated {
		t.Fatalf("closure wrote the lifecycle: %s", got)
	}
	if got := out.GetShipmentStatus(); got != "SHIPPED" {
		t.Fatalf("read-model shipment status = %s, want SHIPPED", got)
	}
}
