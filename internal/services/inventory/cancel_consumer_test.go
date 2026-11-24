package inventory

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
)

// T1.6-R B1: order.cancelled must compensate the exact warehouse carried by the
// event (HELD release / CONFIRMED restock); there is no implicit W1 fallback.
func TestOrderCancelledCompensatesHeldAndConfirmed(t *testing.T) {
	cases := []struct {
		name        string
		status      string
		warehouse   int64
		reserved    int64
		wantOnHand  int64
		wantReserve int64
	}{
		{name: "held W1 releases reserved", status: statusHeld, warehouse: 1, reserved: 1, wantOnHand: 10, wantReserve: 0},
		{name: "confirmed W1 restocks on_hand", status: statusConfirmed, warehouse: 1, reserved: 0, wantOnHand: 11, wantReserve: 0},
		{name: "held W2 releases reserved", status: statusHeld, warehouse: 2, reserved: 1, wantOnHand: 10, wantReserve: 0},
		{name: "confirmed W2 restocks on_hand", status: statusConfirmed, warehouse: 2, reserved: 0, wantOnHand: 11, wantReserve: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := seckillTestDB(t)
			orderNo := fmt.Sprintf("ORD-T16-CANCEL-%s-W%d", tc.status, tc.warehouse)
			// A different warehouse must remain untouched.
			if _, err := db.Exec(`INSERT INTO inventory_stocks (sku_id, warehouse_id, on_hand, reserved, version)
				VALUES (1, 3, 5, 5, 1)`); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`INSERT INTO inventory_stocks (sku_id, warehouse_id, on_hand, reserved, version)
				VALUES (1, ?, 10, ?, 1)`, tc.warehouse, tc.reserved); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`INSERT INTO stock_reservations
				(reservation_no, order_no, sku_id, warehouse_id, quantity, status, expires_at, created_at, updated_at)
				VALUES (?, ?, 1, ?, 1, ?, DATE_ADD(NOW(3), INTERVAL 1 HOUR), NOW(3), NOW(3))`,
				"RES-"+orderNo, orderNo, tc.warehouse, tc.status); err != nil {
				t.Fatal(err)
			}
			svc := &Service{db: db, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
			payload := []byte(fmt.Sprintf(
				`{"order_no":%q,"items":[{"sku_id":1,"warehouse_id":%d,"quantity":1}]}`, orderNo, tc.warehouse))
			if err := handleOrderCancelled(context.Background(), svc, payload); err != nil {
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
			if err := db.QueryRow(`SELECT on_hand, reserved FROM inventory_stocks WHERE sku_id = 1 AND warehouse_id = ?`, tc.warehouse).
				Scan(&onHand, &reserved); err != nil {
				t.Fatal(err)
			}
			if onHand != tc.wantOnHand || reserved != tc.wantReserve {
				t.Fatalf("W%d stock = on_hand %d reserved %d, want %d/%d",
					tc.warehouse, onHand, reserved, tc.wantOnHand, tc.wantReserve)
			}
			var otherOnHand, otherReserved int64
			if err := db.QueryRow(`SELECT on_hand, reserved FROM inventory_stocks WHERE sku_id = 1 AND warehouse_id = 3`).
				Scan(&otherOnHand, &otherReserved); err != nil {
				t.Fatal(err)
			}
			if otherOnHand != 5 || otherReserved != 5 {
				t.Fatalf("W3 stock changed by W%d compensation: %d/%d", tc.warehouse, otherOnHand, otherReserved)
			}
		})
	}
}

func TestOrderCancelledMissingWarehouseFailsClosed(t *testing.T) {
	db := seckillTestDB(t)
	if _, err := db.Exec(`INSERT INTO inventory_stocks (sku_id, warehouse_id, on_hand, reserved, version)
		VALUES (1, 1, 10, 1, 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO stock_reservations
		(reservation_no, order_no, sku_id, warehouse_id, quantity, status, expires_at, created_at, updated_at)
		VALUES ('RES-T16-NOWH','ORD-T16-NOWH',1,1,1,'HELD',DATE_ADD(NOW(3), INTERVAL 1 HOUR),NOW(3),NOW(3))`); err != nil {
		t.Fatal(err)
	}
	svc := &Service{db: db, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if err := handleOrderCancelled(context.Background(), svc,
		[]byte(`{"order_no":"ORD-T16-NOWH","items":[{"sku_id":1,"quantity":1}]}`)); err == nil {
		t.Fatal("order.cancelled without warehouse_id was accepted")
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM stock_reservations WHERE order_no='ORD-T16-NOWH'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != statusHeld {
		t.Fatalf("missing-warehouse event mutated reservation: %s", status)
	}
}

// B1: the durable inventory compensation path must use the event warehouse too.
func TestCompensationConsumerUsesWarehouse(t *testing.T) {
	db := seckillTestDB(t)
	if _, err := db.Exec(`INSERT INTO inventory_stocks (sku_id, warehouse_id, on_hand, reserved, version)
		VALUES (1, 2, 10, 1, 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO stock_reservations
		(reservation_no, order_no, sku_id, warehouse_id, quantity, status, expires_at, created_at, updated_at)
		VALUES ('RES-T16-COMP-W2','ORD-T16-COMP-W2',1,2,1,'HELD',DATE_ADD(NOW(3), INTERVAL 1 HOUR),NOW(3),NOW(3))`); err != nil {
		t.Fatal(err)
	}
	svc := &Service{db: db, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if err := handleCompensation(context.Background(), svc,
		[]byte(`{"order_no":"ORD-T16-COMP-W2","items":[{"sku_id":1,"warehouse_id":2,"quantity":1}]}`)); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM stock_reservations WHERE order_no='ORD-T16-COMP-W2'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != statusReleased {
		t.Fatalf("compensation status = %s, want RELEASED", status)
	}
	var onHand, reserved int64
	if err := db.QueryRow(`SELECT on_hand, reserved FROM inventory_stocks WHERE sku_id=1 AND warehouse_id=2`).
		Scan(&onHand, &reserved); err != nil {
		t.Fatal(err)
	}
	if onHand != 10 || reserved != 0 {
		t.Fatalf("W2 stock = %d/%d, want 10/0", onHand, reserved)
	}
}

func TestCompensationConsumerMissingWarehouseFailsClosed(t *testing.T) {
	db := seckillTestDB(t)
	svc := &Service{db: db, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if err := handleCompensation(context.Background(), svc,
		[]byte(`{"order_no":"ORD-T16-COMP-NOWH","items":[{"sku_id":1,"quantity":1}]}`)); err == nil {
		t.Fatal("compensation without warehouse_id was accepted")
	}
}
