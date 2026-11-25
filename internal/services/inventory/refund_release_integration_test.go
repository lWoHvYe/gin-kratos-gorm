package inventory

import (
	"context"
	"database/sql"
	"sync"
	"testing"

	"gin-kratos-gorm/internal/platform/events"
)

func seedReleaseStock(t *testing.T, db *sql.DB, sku, warehouse, onHand, reserved int64) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO inventory_stocks (sku_id, warehouse_id, on_hand, reserved, version)
		VALUES (?, ?, ?, ?, 1)`, sku, warehouse, onHand, reserved); err != nil {
		t.Fatalf("seed stock %d/%d: %v", sku, warehouse, err)
	}
}

func seedReleaseReservation(t *testing.T, db *sql.DB, orderNo string, sku, warehouse, quantity int64, status string) {
	t.Helper()
	resNo := "RES-" + orderNo + "-" + itoa(sku) + "-" + itoa(warehouse)
	if _, err := db.Exec(`INSERT INTO stock_reservations
		(reservation_no, order_no, sku_id, warehouse_id, quantity, status, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, DATE_ADD(NOW(3), INTERVAL 1 HOUR), NOW(3), NOW(3))`,
		resNo, orderNo, sku, warehouse, quantity, status); err != nil {
		t.Fatalf("seed reservation %s/%d/%d: %v", orderNo, sku, warehouse, err)
	}
}

func releaseReservationStatus(t *testing.T, db *sql.DB, orderNo string, sku, warehouse int64) string {
	t.Helper()
	var status string
	if err := db.QueryRow(`SELECT status FROM stock_reservations
		WHERE order_no = ? AND sku_id = ? AND warehouse_id = ?`, orderNo, sku, warehouse).Scan(&status); err != nil {
		t.Fatalf("load reservation %s/%d/%d: %v", orderNo, sku, warehouse, err)
	}
	return status
}

func releaseStockCounters(t *testing.T, db *sql.DB, sku, warehouse int64) (onHand, reserved int64) {
	t.Helper()
	if err := db.QueryRow(`SELECT on_hand, reserved FROM inventory_stocks
		WHERE sku_id = ? AND warehouse_id = ?`, sku, warehouse).Scan(&onHand, &reserved); err != nil {
		t.Fatalf("load stock %d/%d: %v", sku, warehouse, err)
	}
	return onHand, reserved
}

func releaseLedgerCount(t *testing.T, db *sql.DB, orderNo string, sku, warehouse int64, entry string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM inventory_ledger
		WHERE order_no = ? AND sku_id = ? AND warehouse_id = ? AND entry_type = ?`,
		orderNo, sku, warehouse, entry).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// AC-50 / INV-44: PRE + HELD releases the hold.
func TestRefundReleaseConsumerReleasesHeldOnce(t *testing.T) {
	db := seckillTestDB(t)
	seedReleaseStock(t, db, 1, 1, 10, 1)
	seedReleaseReservation(t, db, "ORD-REL", 1, 1, 1, statusHeld)
	svc := &Service{db: db}
	// Event quantity is deliberately wrong: order_reservation_full ignores it.
	data := releasePayload(t, "ORD-REL", "rr:R-REL", "R-REL",
		events.RefundReleaseItem{SkuID: 1, Quantity: 99, WarehouseID: 1})
	for i := 0; i < 2; i++ {
		if err := handleRefundRelease(context.Background(), svc, data); err != nil {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
	}
	if got := releaseReservationStatus(t, db, "ORD-REL", 1, 1); got != statusReleased {
		t.Fatalf("reservation=%s, want RELEASED", got)
	}
	onHand, reserved := releaseStockCounters(t, db, 1, 1)
	if onHand != 10 || reserved != 0 {
		t.Fatalf("stock=%d/%d, want 10/0", onHand, reserved)
	}
	if got := releaseLedgerCount(t, db, "ORD-REL", 1, 1, entryRelease); got != 1 {
		t.Fatalf("RELEASE ledger rows=%d, want 1", got)
	}
}

// AC-50 / INV-44: PRE + CONFIRMED compensates through TccCancelConfirm.
func TestRefundReleaseConsumerRestocksConfirmedOnce(t *testing.T) {
	db := seckillTestDB(t)
	seedReleaseStock(t, db, 1, 1, 10, 0)
	seedReleaseReservation(t, db, "ORD-CONF-REL", 1, 1, 1, statusConfirmed)
	svc := &Service{db: db}
	// Event quantity is deliberately wrong; TccCancelConfirm uses the
	// authoritative reservation quantity.
	data := releasePayload(t, "ORD-CONF-REL", "rr:R-CONF-REL", "R-CONF-REL",
		events.RefundReleaseItem{SkuID: 1, Quantity: 99, WarehouseID: 1})
	for i := 0; i < 2; i++ {
		if err := handleRefundRelease(context.Background(), svc, data); err != nil {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
	}
	if got := releaseReservationStatus(t, db, "ORD-CONF-REL", 1, 1); got != statusReleased {
		t.Fatalf("reservation=%s, want RELEASED", got)
	}
	onHand, reserved := releaseStockCounters(t, db, 1, 1)
	if onHand != 11 || reserved != 0 {
		t.Fatalf("stock=%d/%d, want 11/0", onHand, reserved)
	}
	if got := releaseLedgerCount(t, db, "ORD-CONF-REL", 1, 1, entryRestock); got != 1 {
		t.Fatalf("RESTOCK ledger rows=%d, want 1", got)
	}
}

func TestRefundReleaseConsumerMultiWarehouse(t *testing.T) {
	db := seckillTestDB(t)
	seedReleaseStock(t, db, 1, 1, 10, 2)
	seedReleaseStock(t, db, 2, 2, 20, 1)
	seedReleaseReservation(t, db, "ORD-MW", 1, 1, 2, statusHeld)
	seedReleaseReservation(t, db, "ORD-MW", 2, 2, 1, statusHeld)
	svc := &Service{db: db}
	data := releasePayload(t, "ORD-MW", "rr:R-MW", "R-MW",
		events.RefundReleaseItem{SkuID: 1, Quantity: 2, WarehouseID: 1},
		events.RefundReleaseItem{SkuID: 2, Quantity: 1, WarehouseID: 2})
	if err := handleRefundRelease(context.Background(), svc, data); err != nil {
		t.Fatal(err)
	}
	if onHand, reserved := releaseStockCounters(t, db, 1, 1); onHand != 10 || reserved != 0 {
		t.Fatalf("sku1 stock=%d/%d, want 10/0", onHand, reserved)
	}
	if onHand, reserved := releaseStockCounters(t, db, 2, 2); onHand != 20 || reserved != 0 {
		t.Fatalf("sku2 stock=%d/%d, want 20/0", onHand, reserved)
	}
	for _, sku := range []int64{1, 2} {
		if got := releaseReservationStatus(t, db, "ORD-MW", sku, sku); got != statusReleased {
			t.Fatalf("sku %d reservation=%s", sku, got)
		}
	}
}

// Several workers consuming the same release must converge on one physical
// release; reservation row locks make the losers no-ops.
func TestRefundReleaseConsumerConcurrentWorkers(t *testing.T) {
	db := seckillTestDB(t)
	seedReleaseStock(t, db, 1, 1, 10, 1)
	seedReleaseReservation(t, db, "ORD-WORKERS", 1, 1, 1, statusHeld)
	svc := &Service{db: db}
	data := releasePayload(t, "ORD-WORKERS", "rr:R-WORKERS", "R-WORKERS",
		events.RefundReleaseItem{SkuID: 1, Quantity: 1, WarehouseID: 1})
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = handleRefundRelease(context.Background(), svc, data)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("worker %d: %v", i, err)
		}
	}
	onHand, reserved := releaseStockCounters(t, db, 1, 1)
	if onHand != 10 || reserved != 0 {
		t.Fatalf("stock=%d/%d, want 10/0", onHand, reserved)
	}
	if got := releaseLedgerCount(t, db, "ORD-WORKERS", 1, 1, entryRelease); got != 1 {
		t.Fatalf("RELEASE ledger rows=%d, want 1", got)
	}
}

// A later item failing must not ACK the event; the first release is durable,
// the retry skips it and the second item is not released twice.
func TestRefundReleasePartialFailureDoesNotAck(t *testing.T) {
	db := seckillTestDB(t)
	seedReleaseStock(t, db, 1, 1, 10, 1)
	seedReleaseReservation(t, db, "ORD-PART", 1, 1, 1, statusHeld)
	seedReleaseReservation(t, db, "ORD-PART", 2, 1, 1, statusHeld)
	svc := &Service{db: db}
	data := releasePayload(t, "ORD-PART", "rr:R-PART", "R-PART",
		events.RefundReleaseItem{SkuID: 1, Quantity: 1, WarehouseID: 1},
		events.RefundReleaseItem{SkuID: 2, Quantity: 1, WarehouseID: 1})
	if err := handleRefundRelease(context.Background(), svc, data); err == nil {
		t.Fatal("partial failure was ACKed")
	}
	if got := releaseReservationStatus(t, db, "ORD-PART", 1, 1); got != statusReleased {
		t.Fatalf("first item=%s, want RELEASED", got)
	}
	if got := releaseReservationStatus(t, db, "ORD-PART", 2, 1); got != statusHeld {
		t.Fatalf("second item=%s, want HELD", got)
	}
	if err := handleRefundRelease(context.Background(), svc, data); err == nil {
		t.Fatal("retry of the still-broken event was ACKed")
	}
	if got := releaseLedgerCount(t, db, "ORD-PART", 1, 1, entryRelease); got != 1 {
		t.Fatalf("first item RELEASE ledger rows=%d, want 1 (no double release)", got)
	}
}

func TestRefundReleaseConsumerSkipsMissingReservation(t *testing.T) {
	db := seckillTestDB(t)
	seedReleaseStock(t, db, 1, 1, 10, 0)
	svc := &Service{db: db}
	data := releasePayload(t, "ORD-MISS", "rr:R-MISS", "R-MISS",
		events.RefundReleaseItem{SkuID: 1, Quantity: 1, WarehouseID: 1})
	if err := handleRefundRelease(context.Background(), svc, data); err != nil {
		t.Fatalf("missing reservation is an idempotent no-op: %v", err)
	}
	if onHand, reserved := releaseStockCounters(t, db, 1, 1); onHand != 10 || reserved != 0 {
		t.Fatalf("stock changed for a missing reservation: %d/%d", onHand, reserved)
	}
}

func TestRefundReleaseConsumerRejectsMissingWarehouseAndMismatchedReleaseID(t *testing.T) {
	db := seckillTestDB(t)
	seedReleaseStock(t, db, 1, 1, 10, 1)
	seedReleaseReservation(t, db, "ORD-BAD", 1, 1, 1, statusHeld)
	svc := &Service{db: db}
	cases := [][]byte{
		releasePayload(t, "ORD-BAD", "rr:R-BAD", "R-BAD",
			events.RefundReleaseItem{SkuID: 1, Quantity: 1}),
		releasePayload(t, "ORD-BAD", "rr:R-OTHER", "R-BAD",
			events.RefundReleaseItem{SkuID: 1, Quantity: 1, WarehouseID: 1}),
	}
	for i, data := range cases {
		if err := handleRefundRelease(context.Background(), svc, data); err == nil {
			t.Fatalf("case %d: malformed event was accepted", i)
		}
	}
	if got := releaseReservationStatus(t, db, "ORD-BAD", 1, 1); got != statusHeld {
		t.Fatalf("reservation=%s, want HELD", got)
	}
	if _, reserved := releaseStockCounters(t, db, 1, 1); reserved != 1 {
		t.Fatalf("reserved=%d, want 1", reserved)
	}
}

// A canceled handler context must not partially apply the event.
func TestRefundReleaseConsumerCanceledContextDoesNotRelease(t *testing.T) {
	db := seckillTestDB(t)
	seedReleaseStock(t, db, 1, 1, 10, 1)
	seedReleaseReservation(t, db, "ORD-CTX", 1, 1, 1, statusHeld)
	svc := &Service{db: db}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	data := releasePayload(t, "ORD-CTX", "rr:R-CTX", "R-CTX",
		events.RefundReleaseItem{SkuID: 1, Quantity: 1, WarehouseID: 1})
	if err := handleRefundRelease(ctx, svc, data); err == nil {
		t.Fatal("canceled context was ACKed")
	}
	if got := releaseReservationStatus(t, db, "ORD-CTX", 1, 1); got != statusHeld {
		t.Fatalf("reservation=%s, want HELD", got)
	}
}
