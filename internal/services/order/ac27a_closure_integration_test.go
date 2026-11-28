package order

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
)

// AC-27A clause 5 through the production write path: a CONSUMED permit whose
// Fulfillment.CancelIfCreated confirms the waybill was cancelled converges to
// RefundAdmission result=PRE_SHIP_REVOKED. The read model then closes the group
// (effective CANCELLED, excluded from the remaining groups) while the persisted
// orders.status stays ALLOCATED and the permit stays CONSUMED (SPEC-0001 v1.5
// AC-27 EARS-1/EARS-2, AC-27A).
func TestAC27AConsumedPreAdmissionClosesFromRealRows(t *testing.T) {
	db := orderTestDB(t)
	fulfillment := &fakeFulfillmentPort{status: "CANCELLED"}
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 1000}, fulfillment)
	ctx := context.Background()
	const orderNo = "ORD-AC27A-PRE"

	seedAC27ATwoGroupOrder(t, db, orderNo)
	if _, err := db.Exec(`INSERT INTO order_items
		(order_no,line_no,sku_id,name,quantity,warehouse_id,fulfillment_group_no,line_paid_cent,unit_paid_cent)
		VALUES (?,1,1,'g1',1,1,'G1',500,500),(?,2,2,'g2',1,2,'G2',500,500)`,
		orderNo, orderNo); err != nil {
		t.Fatal(err)
	}
	for _, permit := range []struct{ group, shipment string }{{"G1", "SHIP-G1"}, {"G2", "SHIP-G2"}} {
		if _, err := db.Exec(`INSERT INTO order_shipping_permits
			(order_no, fulfillment_group_no, state, shipment_no) VALUES (?,?,?,?)`,
			orderNo, permit.group, permitConsumed, permit.shipment); err != nil {
			t.Fatal(err)
		}
	}

	reply, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-AC27A", OrderNo: orderNo, GroupNo: "G2", AmountCent: 500, Source: "aftersale",
	})
	if err != nil {
		t.Fatalf("AdmitRefund: %v", err)
	}
	if got := reply.GetResult(); got != refundResultPre {
		t.Fatalf("admission result = %s, want %s", got, refundResultPre)
	}
	var admissionResult, admissionState string
	if err := db.QueryRow(`SELECT result, state FROM order_refund_admissions
		WHERE refund_id = 'R-AC27A'`).Scan(&admissionResult, &admissionState); err != nil {
		t.Fatal(err)
	}
	if admissionResult != refundResultPre || admissionState != refundResultPre {
		t.Fatalf("persisted admission = %s/%s, want PRE/PRE", admissionResult, admissionState)
	}

	groups, err := loadFulfillmentGroups(ctx, db, orderNo)
	if err != nil {
		t.Fatal(err)
	}
	g1 := groupedByNo(t, groups, "G1")
	g2 := groupedByNo(t, groups, "G2")
	if g2.PermitState != permitConsumed {
		t.Fatalf("G2 permit = %s, want CONSUMED (INV-45: never rewritten to REVOKED)", g2.PermitState)
	}
	if g2.Status != statusAllocated {
		t.Fatalf("persisted G2 status = %s, want ALLOCATED", g2.Status)
	}
	if !g2.PreShipAdmission {
		t.Fatal("G2 pre-ship admission was not derived from order_refund_admissions")
	}
	if !groupIsClosed(g2) {
		t.Fatal("G2 must be closed after CONSUMED + PRE_SHIP_REVOKED")
	}
	if got := effectiveGroupStatus(g2); got != statusCancelled {
		t.Fatalf("effective G2 status = %s, want CANCELLED", got)
	}
	if groupIsClosed(g1) {
		t.Fatal("shipped G1 must stay open")
	}

	record, err := findOrder(ctx, db, orderNo)
	if err != nil || record == nil {
		t.Fatalf("load order: %v", err)
	}
	if record.Status != statusAllocated {
		t.Fatalf("orders.status = %s before projection, want ALLOCATED", record.Status)
	}
	out := record.toProto(nil, groups)
	if got := out.GetStatus(); got != statusAllocated {
		t.Fatalf("projected lifecycle = %s, want ALLOCATED (closure must not write orders.status)", got)
	}
	if got := out.GetShipmentStatus(); got != "SHIPPED" {
		t.Fatalf("read-model shipment status = %s, want SHIPPED", got)
	}
	if projected := projectedGroup(t, out, "G2"); projected.GetStatus() != statusCancelled ||
		projected.GetRefundState() != refundStateNone {
		t.Fatalf("projected G2 = %s/%s, want CANCELLED/NONE",
			projected.GetStatus(), projected.GetRefundState())
	}

	// The refund-completion projection only writes the group refund state; it
	// must not advance or rewrite the persisted lifecycle (AC-25/INV-01).
	if err := applyAfterSaleCompleted(ctx, svc, afterSaleCompletedEvent{
		OrderNo: orderNo, GroupNo: "G2", AfterSaleNo: "AS-AC27A",
	}); err != nil {
		t.Fatal(err)
	}
	record, err = findOrder(ctx, db, orderNo)
	if err != nil || record == nil {
		t.Fatalf("reload order: %v", err)
	}
	if record.Status != statusAllocated {
		t.Fatalf("orders.status = %s after refund completion, want ALLOCATED", record.Status)
	}
	groups, err = loadFulfillmentGroups(ctx, db, orderNo)
	if err != nil {
		t.Fatal(err)
	}
	g2 = groupedByNo(t, groups, "G2")
	if g2.RefundState != groupRefundStateRefunded {
		t.Fatalf("G2 refund_state = %s, want REFUNDED", g2.RefundState)
	}
	if g2.Status != statusAllocated || !g2.PreShipAdmission || !groupIsClosed(g2) {
		t.Fatalf("G2 after completion = status %s/pre %v/closed %v",
			g2.Status, g2.PreShipAdmission, groupIsClosed(g2))
	}
	if err := db.QueryRow(`SELECT result, state FROM order_refund_admissions
		WHERE refund_id = 'R-AC27A'`).Scan(&admissionResult, &admissionState); err != nil {
		t.Fatal(err)
	}
	if admissionResult != refundResultPre || admissionState != refundResultPre {
		t.Fatalf("admission rewritten to %s/%s, want PRE/PRE", admissionResult, admissionState)
	}
	out = record.toProto(nil, groups)
	if got := out.GetStatus(); got != statusAllocated {
		t.Fatalf("projected lifecycle = %s after completion, want ALLOCATED", got)
	}
	if got := out.GetShipmentStatus(); got != "SHIPPED" {
		t.Fatalf("read-model shipment status = %s after completion, want SHIPPED", got)
	}
	if projected := projectedGroup(t, out, "G2"); projected.GetStatus() != statusCancelled ||
		projected.GetRefundState() != groupRefundStateRefunded {
		t.Fatalf("projected G2 = %s/%s, want CANCELLED/REFUNDED",
			projected.GetStatus(), projected.GetRefundState())
	}
}

// AC-27A: only a *valid* PRE closes a pre-ship group: result=PRE_SHIP_REVOKED
// with state=result (SPEC-0001 v1.6). REVIEW_REQUIRED, a PENDING_REMOTE
// placeholder, POST_SHIP_ALLOWED, a missing row and a PRE row whose state did
// not land on the result all stay open; a late SHIPPED status wins over the PRE
// audit row.
func TestAC27AAdmissionResultDerivation(t *testing.T) {
	db := orderTestDB(t)
	ctx := context.Background()
	cases := []struct {
		name          string
		groupStatus   string
		permit        string
		result        string
		state         string
		wantAdmission bool
		wantClosed    bool
	}{
		{"pre_ship_revoked closes", statusAllocated, permitConsumed, refundResultPre, refundResultPre, true, true},
		{"pre_ship_revoked with mismatched state stays open", statusAllocated, permitConsumed, refundResultPre, refundPending, false, false},
		{"pre_ship_revoked without consumed permit stays open", statusAllocated, permitNone, refundResultPre, refundResultPre, true, false},
		{"review_required stays open", statusAllocated, permitConsumed, refundReviewRequired, refundPending, false, false},
		{"pending_remote placeholder stays open", statusAllocated, permitConsumed, refundResultPost, refundPending, false, false},
		{"post_ship_allowed stays open", statusAllocated, permitConsumed, refundResultPost, refundResultPost, false, false},
		{"late shipped status wins over pre-ship admission", "SHIPPED", permitConsumed, refundResultPre, refundResultPre, true, false},
		{"no admission row stays open", statusAllocated, permitConsumed, "", "", false, false},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orderNo := fmt.Sprintf("ORD-AC27A-DERIVE-%d", i)
			seedSettlementOrder(t, db, orderNo, statusAllocated, 1000, 0, refundStateNone)
			if _, err := db.Exec(`INSERT INTO order_fulfillment_groups
				(group_no,order_no,seq,warehouse_id,status,refund_state,reservation_state,
				 freight_cent,freight_share_cent,version)
				VALUES ('G1',?,1,1,?,'NONE','HELD',0,0,1)`, orderNo, tc.groupStatus); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`INSERT INTO order_shipping_permits
				(order_no, fulfillment_group_no, state) VALUES (?,'G1',?)`, orderNo, tc.permit); err != nil {
				t.Fatal(err)
			}
			if tc.result != "" {
				if _, err := db.Exec(`INSERT INTO order_refund_admissions
					(refund_id,order_no,group_no,result,state,amount_cent)
					VALUES (?,?,'G1',?,?,500)`, "R-"+orderNo, orderNo, tc.result, tc.state); err != nil {
					t.Fatal(err)
				}
			}
			groups, err := loadFulfillmentGroups(ctx, db, orderNo)
			if err != nil {
				t.Fatal(err)
			}
			group := groupedByNo(t, groups, "G1")
			if got := group.PreShipAdmission; got != tc.wantAdmission {
				t.Fatalf("PreShipAdmission = %v, want %v", got, tc.wantAdmission)
			}
			if got := groupIsClosed(group); got != tc.wantClosed {
				t.Fatalf("groupIsClosed = %v, want %v", got, tc.wantClosed)
			}
			wantEffective := tc.groupStatus
			if tc.wantClosed {
				wantEffective = statusCancelled
			}
			if got := effectiveGroupStatus(group); got != wantEffective {
				t.Fatalf("effectiveGroupStatus = %s, want %s", got, wantEffective)
			}
		})
	}
}

func seedAC27ATwoGroupOrder(t *testing.T, db *sql.DB, orderNo string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO orders
		(order_no,user_id,status,total_cent,discount_cent,promotion_discount_cent,applied_promotions_json,
		 freight_cent,pay_cent,paid_cent,settlement_payment_no,refunded_cent,refund_status,refund_state,
		 receiver_name,receiver_phone,receiver_address,quote_id,client_request_id,
		 allocation_frozen_at,allocation_hash,package_count,warehouse_count,expire_at,created_at)
		VALUES (?,1,?,1000,0,0,'[]',0,1000,1000,'PAY-AC27A',0,'','NONE',
		 'R','1','Addr','Q','',NOW(3),'h',2,2,NOW(3),NOW(3))`, orderNo, statusAllocated); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := insertFulfillmentGroups(context.Background(), tx, orderNo, []*pb.OrderFulfillmentGroup{
		{GroupNo: "G1", WarehouseId: 1},
		{GroupNo: "G2", WarehouseId: 2},
	}, 0); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if _, err := tx.Exec(`UPDATE order_fulfillment_groups
		SET status = CASE group_no WHEN 'G1' THEN 'SHIPPED' ELSE 'ALLOCATED' END
		WHERE order_no = ?`, orderNo); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func projectedGroup(t *testing.T, order *pb.Order, groupNo string) *pb.OrderFulfillmentGroup {
	t.Helper()
	for _, group := range order.GetFulfillmentGroups() {
		if group.GetGroupNo() == groupNo {
			return group
		}
	}
	t.Fatalf("projected group %s not found", groupNo)
	return nil
}
