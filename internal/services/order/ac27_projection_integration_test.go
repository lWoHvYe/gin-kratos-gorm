package order

import (
	"context"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
)

// AC-27: the EARS-2 read model is produced by the production writers, not by a
// hand-built fixture. BlockShipmentForRefund moves the group to REFUNDING and
// revokes the permit; applyAfterSaleCompleted moves it to REFUNDED. Reading the
// rows back through loadFulfillmentGroups + toProto (the order RPC path) must
// expose G2 as CANCELLED/REFUNDED without advancing or polluting orders.status,
// and G1 SHIPPED must still drive the shipment projection.
func TestAC27RefundedGroupProjectionFromRows(t *testing.T) {
	db := orderTestDB(t)
	svc := settlementService(db)
	ctx := context.Background()
	const orderNo = "ORD-AC27-ROWS"

	seedSettlementOrder(t, db, orderNo, statusAllocated, 1000, 0, refundStateNone)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := insertFulfillmentGroups(ctx, tx, orderNo, []*pb.OrderFulfillmentGroup{
		{GroupNo: "G1", WarehouseId: 1},
		{GroupNo: "G2", WarehouseId: 2},
	}, 0); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE order_fulfillment_groups
		SET status = CASE group_no WHEN 'G1' THEN 'SHIPPED' ELSE 'ALLOCATED' END
		WHERE order_no = ?`, orderNo); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.BlockShipmentForRefund(ctx, &pb.ShipmentPermitRequest{
		OrderNo: orderNo, AfterSaleNo: "AS-AC27", FulfillmentGroupNo: "G2",
	}); err != nil {
		t.Fatal(err)
	}
	groups, err := loadFulfillmentGroups(ctx, db, orderNo)
	if err != nil {
		t.Fatal(err)
	}
	g2 := groupedByNo(t, groups, "G2")
	if g2.RefundState != refundStateRefunding {
		t.Fatalf("admitted G2 refund_state = %s", g2.RefundState)
	}
	if g2.PermitState != permitRevoked {
		t.Fatalf("admitted G2 permit = %s", g2.PermitState)
	}
	// The refund is not complete yet, so AC-27 EARS-2's "已退款完成" branch has
	// not been reached; the test must not claim it here.
	if err := applyAfterSaleCompleted(ctx, svc, afterSaleCompletedEvent{
		OrderNo: orderNo, GroupNo: "G2", AfterSaleNo: "AS-AC27",
	}); err != nil {
		t.Fatal(err)
	}
	groups, err = loadFulfillmentGroups(ctx, db, orderNo)
	if err != nil {
		t.Fatal(err)
	}
	g2 = groupedByNo(t, groups, "G2")
	if g2.RefundState != groupRefundStateRefunded {
		t.Fatalf("completed G2 refund_state = %s", g2.RefundState)
	}
	if g2.Status != statusAllocated {
		t.Fatalf("persisted G2 status = %s, refund must not rewrite the group row", g2.Status)
	}
	if got := effectiveGroupStatus(g2); got != "CANCELLED" {
		t.Fatalf("effective G2 status = %s", got)
	}

	record, err := findOrder(ctx, db, orderNo)
	if err != nil || record == nil {
		t.Fatalf("load order: %v", err)
	}
	if record.Status != statusAllocated {
		t.Fatalf("orders.status = %s, refund progress must not be written into the lifecycle", record.Status)
	}
	out := record.toProto(nil, groups)
	if got := out.GetStatus(); got != statusAllocated {
		t.Fatalf("projected order status = %s", got)
	}
	if got := out.GetShipmentStatus(); got != "SHIPPED" {
		t.Fatalf("projected shipment status = %s, G1 SHIPPED must not be blocked by refunded G2", got)
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
}

func groupedByNo(t *testing.T, groups []*fulfillmentGroup, groupNo string) *fulfillmentGroup {
	t.Helper()
	for _, group := range groups {
		if group.GroupNo == groupNo {
			return group
		}
	}
	t.Fatalf("fulfillment group %s not found", groupNo)
	return nil
}
