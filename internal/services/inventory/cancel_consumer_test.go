package inventory

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// T1.6 relies on the existing order.cancelled compensation for both reservation
// states: HELD releases the reserved quantity, CONFIRMED restocks on_hand. No
// new event/consumer was added; this test binds that reuse explicitly.
func TestOrderCancelledCompensatesHeldAndConfirmed(t *testing.T) {
	cases := []struct {
		name        string
		status      string
		reserved    int64
		wantOnHand  int64
		wantReserve int64
	}{
		{name: "held releases reserved", status: statusHeld, reserved: 1, wantOnHand: 10, wantReserve: 0},
		{name: "confirmed restocks on_hand", status: statusConfirmed, reserved: 0, wantOnHand: 11, wantReserve: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := seckillTestDB(t)
			orderNo := "ORD-T16-CANCEL-" + tc.status
			if _, err := db.Exec(`INSERT INTO inventory_stocks (sku_id, warehouse_id, on_hand, reserved, version)
				VALUES (1, 1, 10, ?, 1)`, tc.reserved); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`INSERT INTO stock_reservations
				(reservation_no, order_no, sku_id, warehouse_id, quantity, status, expires_at, created_at, updated_at)
				VALUES (?, ?, 1, 1, 1, ?, DATE_ADD(NOW(3), INTERVAL 1 HOUR), NOW(3), NOW(3))`,
				"RES-"+orderNo, orderNo, tc.status); err != nil {
				t.Fatal(err)
			}
			svc := &Service{db: db, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
			if err := handleOrderCancelled(context.Background(), svc,
				[]byte(`{"order_no":"`+orderNo+`","items":[{"sku_id":1,"quantity":1}]}`)); err != nil {
				t.Fatalf("order.cancelled compensation: %v", err)
			}
			var status string
			if err := db.QueryRow(`SELECT status FROM stock_reservations WHERE order_no = ?`, orderNo).Scan(&status); err != nil {
				t.Fatal(err)
			}
			if status != statusReleased {
				t.Fatalf("reservation status = %s, want RELEASED", status)
			}
			var onHand, reserved int64
			if err := db.QueryRow(`SELECT on_hand, reserved FROM inventory_stocks WHERE sku_id = 1`).
				Scan(&onHand, &reserved); err != nil {
				t.Fatal(err)
			}
			if onHand != tc.wantOnHand || reserved != tc.wantReserve {
				t.Fatalf("stock = on_hand %d reserved %d, want %d/%d",
					onHand, reserved, tc.wantOnHand, tc.wantReserve)
			}
		})
	}
}
