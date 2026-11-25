package inventory

import (
	"context"
	"testing"
)

func TestCompensationHandlerRestocksOnce(t *testing.T) {
	db := seckillTestDB(t)
	if _, err := db.Exec(`INSERT INTO inventory_stocks
		(sku_id, warehouse_id, on_hand, reserved, version) VALUES (1,1,10,1,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO stock_reservations
		(reservation_no, order_no, sku_id, warehouse_id, quantity, status, expires_at, created_at, updated_at)
		VALUES ('RES-COMP','ORD-COMP',1,1,1,'HELD',DATE_ADD(NOW(3), INTERVAL 1 HOUR),NOW(3),NOW(3))`); err != nil {
		t.Fatal(err)
	}
	svc := &Service{db: db}
	payload := []byte(`{"order_no":"ORD-COMP","items":[{"sku_id":1,"warehouse_id":1,"quantity":1}]}`)
	for i := 0; i < 2; i++ {
		if err := handleCompensation(context.Background(), svc, payload); err != nil {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
	}
	var onHand, reserved int64
	if err := db.QueryRow(`SELECT on_hand, reserved FROM inventory_stocks WHERE sku_id=1`).Scan(&onHand, &reserved); err != nil {
		t.Fatal(err)
	}
	if onHand != 10 || reserved != 0 {
		t.Fatalf("on_hand=%d reserved=%d, want 10/0", onHand, reserved)
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM stock_reservations WHERE order_no='ORD-COMP'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != statusReleased {
		t.Fatalf("reservation status=%s", status)
	}
}
