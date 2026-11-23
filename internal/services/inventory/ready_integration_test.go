package inventory

import (
	"context"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
)

func TestCheckOrderReadyFailsClosedOnHeld(t *testing.T) {
	db := ledgerTestDB(t)
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS stock_reservations (
		id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, reservation_no VARCHAR(64) NOT NULL,
		order_no VARCHAR(64) NOT NULL, sku_id BIGINT NOT NULL, warehouse_id BIGINT NOT NULL,
		quantity INT NOT NULL, status VARCHAR(16) NOT NULL,
		expires_at DATETIME(3) NOT NULL, created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
		UNIQUE KEY uk_stock_reservations_order_sku_wh (order_no, sku_id, warehouse_id))`); err != nil {
		t.Fatal(err)
	}
	_, _ = db.Exec(`DELETE FROM stock_reservations`)
	_, _ = db.Exec(`INSERT INTO stock_reservations
		(reservation_no,order_no,sku_id,warehouse_id,quantity,status,expires_at)
		VALUES ('R1','ORD-R',1,1,1,'CONFIRMED',NOW(3)), ('R2','ORD-R',2,1,1,'HELD',NOW(3))`)
	svc := New(db, nil)
	ctx := context.Background()

	reply, err := svc.CheckOrderReady(ctx, &pb.CheckOrderReadyRequest{
		OrderNo: "ORD-R",
		Lines: []*pb.CheckOrderReadyLineRef{
			{LineNo: 1, SkuId: 1, WarehouseId: 1, Quantity: 1},
		},
	})
	if err != nil || !reply.GetReady() {
		t.Fatalf("confirmed line: ready=%v err=%v", reply.GetReady(), err)
	}
	reply, err = svc.CheckOrderReady(ctx, &pb.CheckOrderReadyRequest{
		OrderNo: "ORD-R",
		Lines: []*pb.CheckOrderReadyLineRef{
			{LineNo: 1, SkuId: 1, WarehouseId: 1, Quantity: 1},
			{LineNo: 2, SkuId: 2, WarehouseId: 1, Quantity: 1},
		},
	})
	if err == nil || reply.GetReady() {
		t.Fatalf("HELD line must fail closed: ready=%v err=%v", reply.GetReady(), err)
	}
}
