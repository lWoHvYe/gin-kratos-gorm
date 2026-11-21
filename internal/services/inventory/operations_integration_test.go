package inventory

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	platformdb "gin-kratos-gorm/internal/platform/db"
)

func operationItems(skus ...int64) []*pb.OrderItem {
	items := make([]*pb.OrderItem, 0, len(skus))
	for _, sku := range skus {
		items = append(items, &pb.OrderItem{SkuId: sku, Quantity: 1})
	}
	return items
}

func seedStock(t *testing.T, db *sql.DB, sku, onHand, reserved int64) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO inventory_stocks
		(sku_id, warehouse_id, on_hand, reserved, version) VALUES (?, 1, ?, ?, 1)`,
		sku, onHand, reserved); err != nil {
		t.Fatalf("seed stock sku %d: %v", sku, err)
	}
}

func seedReservation(t *testing.T, db *sql.DB, orderNo string, sku, quantity int64, status string) {
	t.Helper()
	resNo := "RES-" + orderNo + "-" + itoa(sku)
	if _, err := db.Exec(`INSERT INTO stock_reservations
		(reservation_no, order_no, sku_id, warehouse_id, quantity, status, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, 1, ?, ?, DATE_ADD(NOW(3), INTERVAL 1 HOUR), NOW(3), NOW(3))`,
		resNo, orderNo, sku, quantity, status); err != nil {
		t.Fatalf("seed reservation sku %d: %v", sku, err)
	}
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

func stockCounters(t *testing.T, db *sql.DB, sku int64) (onHand, reserved int64) {
	t.Helper()
	if err := db.QueryRow(`SELECT on_hand, reserved FROM inventory_stocks WHERE sku_id = ? AND warehouse_id = 1`, sku).
		Scan(&onHand, &reserved); err != nil {
		t.Fatalf("load stock sku %d: %v", sku, err)
	}
	return onHand, reserved
}

func reservationStatus(t *testing.T, db *sql.DB, orderNo string, sku int64) string {
	t.Helper()
	var status string
	if err := db.QueryRow(`SELECT status FROM stock_reservations WHERE order_no = ? AND sku_id = ?`, orderNo, sku).
		Scan(&status); err != nil {
		t.Fatalf("load reservation %s/%d: %v", orderNo, sku, err)
	}
	return status
}

func ledgerCount(t *testing.T, db *sql.DB, orderNo string, sku int64, entry string) int {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM inventory_ledger WHERE order_no = ? AND sku_id = ? AND entry_type = ?`,
		orderNo, sku, entry).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func operationRow(t *testing.T, db *sql.DB, operationID string) (status string, total, succeeded, failed, skipped int) {
	t.Helper()
	if err := db.QueryRow(`SELECT status, total_items, succeeded_items, failed_items, skipped_items
		FROM inventory_operations WHERE operation_id = ?`, operationID).
		Scan(&status, &total, &succeeded, &failed, &skipped); err != nil {
		t.Fatalf("load operation %s: %v", operationID, err)
	}
	return status, total, succeeded, failed, skipped
}

func operationItemRow(t *testing.T, db *sql.DB, operationID string, sku int64) (status string, attempts int, lastError string) {
	t.Helper()
	if err := db.QueryRow(`SELECT status, attempts, last_error FROM inventory_operation_items
		WHERE operation_id = ? AND sku_id = ?`, operationID, sku).Scan(&status, &attempts, &lastError); err != nil {
		t.Fatalf("load operation item %s/%d: %v", operationID, sku, err)
	}
	return status, attempts, lastError
}

func outboxPayloads(t *testing.T, db *sql.DB, topic string) []map[string]any {
	t.Helper()
	rows, err := db.Query(`SELECT payload FROM outbox_events WHERE topic = ? ORDER BY id`, topic)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var payload map[string]any
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatalf("decode outbox payload: %v", err)
		}
		out = append(out, payload)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func assertStockInvariants(t *testing.T, db *sql.DB) {
	t.Helper()
	var bad int
	if err := db.QueryRow(`SELECT COUNT(*) FROM inventory_stocks
		WHERE on_hand < 0 OR reserved < 0 OR reserved > on_hand`).Scan(&bad); err != nil {
		t.Fatal(err)
	}
	if bad != 0 {
		t.Fatalf("%d stock rows violate 0 <= reserved <= on_hand", bad)
	}
	var badOps int
	if err := db.QueryRow(`SELECT COUNT(*) FROM inventory_operations
		WHERE succeeded_items + failed_items + skipped_items > total_items`).Scan(&badOps); err != nil {
		t.Fatal(err)
	}
	if badOps != 0 {
		t.Fatalf("%d operation rows have counters above total_items", badOps)
	}
	var inconsistent int
	if err := db.QueryRow(`SELECT COUNT(*) FROM inventory_operations o WHERE
		o.succeeded_items <> (SELECT COUNT(*) FROM inventory_operation_items i
			WHERE i.operation_id = o.operation_id AND i.status = 'DONE')
		OR o.failed_items <> (SELECT COUNT(*) FROM inventory_operation_items i
			WHERE i.operation_id = o.operation_id AND i.status = 'FAILED_PERMANENT')
		OR o.skipped_items <> (SELECT COUNT(*) FROM inventory_operation_items i
			WHERE i.operation_id = o.operation_id AND i.status = 'MISSING')`).Scan(&inconsistent); err != nil {
		t.Fatal(err)
	}
	if inconsistent != 0 {
		t.Fatalf("%d operation rows have counters inconsistent with their items", inconsistent)
	}
	var terminalPending int
	if err := db.QueryRow(`SELECT COUNT(*) FROM inventory_operations o WHERE
		o.status IN ('SUCCEEDED','PARTIAL') AND EXISTS (
			SELECT 1 FROM inventory_operation_items i
			WHERE i.operation_id = o.operation_id AND i.status = 'PENDING')`).Scan(&terminalPending); err != nil {
		t.Fatal(err)
	}
	if terminalPending != 0 {
		t.Fatalf("%d terminal operations still have pending items", terminalPending)
	}
}

func TestConfirmOperationResumesAfterMiddleItemFailure(t *testing.T) {
	db := seckillTestDB(t)
	ctx := context.Background()
	const orderNo = "ORD-OPS-1"
	seedStock(t, db, 1, 10, 1)
	seedStock(t, db, 2, 10, 0) // broken: reservation is HELD but stock reserved was not incremented
	seedStock(t, db, 3, 10, 1)
	for _, sku := range []int64{1, 2, 3} {
		seedReservation(t, db, orderNo, sku, 1, statusHeld)
	}
	svc := &Service{db: db}

	summary, err := svc.applyInventoryOperation(ctx, opTypeConfirm, orderNo, operationItems(1, 2, 3))
	if err == nil {
		t.Fatal("first pass should report a retryable item")
	}
	if summary.Succeeded != 2 || summary.Retryable != 1 || summary.Failed != 0 {
		t.Fatalf("first summary = %+v", summary)
	}
	if got := reservationStatus(t, db, orderNo, 1); got != statusConfirmed {
		t.Fatalf("sku1 status = %s", got)
	}
	if got := reservationStatus(t, db, orderNo, 2); got != statusHeld {
		t.Fatalf("sku2 status = %s", got)
	}
	if got := reservationStatus(t, db, orderNo, 3); got != statusConfirmed {
		t.Fatalf("sku3 status = %s", got)
	}
	if got, attempts, _ := operationItemRow(t, db, summary.OperationID, 2); got != itemStatusPending || attempts != 1 {
		t.Fatalf("sku2 item = %s attempts=%d", got, attempts)
	}
	if got, _, _, _, _ := operationRow(t, db, summary.OperationID); got != opStatusInProgress {
		t.Fatalf("operation status = %s", got)
	}
	if got := ledgerCount(t, db, orderNo, 1, entryConfirm); got != 1 {
		t.Fatalf("sku1 confirm ledger rows = %d", got)
	}
	if got := ledgerCount(t, db, orderNo, 2, entryConfirm); got != 0 {
		t.Fatalf("sku2 confirm ledger rows = %d", got)
	}

	// Heal the transient stock inconsistency; the redelivery must skip sku1/3
	// and only apply sku2. Use a fresh Service to prove progress is in MySQL,
	// not in process memory (restart/crash recovery).
	if _, err := db.Exec(`UPDATE inventory_stocks SET reserved = 1 WHERE sku_id = 2 AND warehouse_id = 1`); err != nil {
		t.Fatal(err)
	}
	svc = &Service{db: db}
	summary, err = svc.applyInventoryOperation(ctx, opTypeConfirm, orderNo, operationItems(1, 2, 3))
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if summary.Status != opStatusSucceeded || summary.Succeeded != 3 || summary.Retryable != 0 {
		t.Fatalf("resume summary = %+v", summary)
	}
	for _, sku := range []int64{1, 2, 3} {
		if got := reservationStatus(t, db, orderNo, sku); got != statusConfirmed {
			t.Fatalf("sku%d status = %s", sku, got)
		}
		if got := ledgerCount(t, db, orderNo, sku, entryConfirm); got != 1 {
			t.Fatalf("sku%d confirm ledger rows = %d", sku, got)
		}
		onHand, reserved := stockCounters(t, db, sku)
		if onHand != 9 || reserved != 0 {
			t.Fatalf("sku%d stock = %d/%d", sku, onHand, reserved)
		}
	}
	if got := len(outboxPayloads(t, db, topicOperationCompleted)); got != 1 {
		t.Fatalf("completed events = %d", got)
	}
	if got := len(outboxPayloads(t, db, topicConfirmed)); got != 3 {
		t.Fatalf("confirmed events = %d", got)
	}

	// A third delivery is a terminal no-op: no duplicate ledger or events.
	if _, err := svc.applyInventoryOperation(ctx, opTypeConfirm, orderNo, operationItems(1, 2, 3)); err != nil {
		t.Fatalf("idempotent replay: %v", err)
	}
	if got := len(outboxPayloads(t, db, topicConfirmed)); got != 3 {
		t.Fatalf("confirmed events after replay = %d", got)
	}
	if got := len(outboxPayloads(t, db, topicOperationCompleted)); got != 1 {
		t.Fatalf("completed events after replay = %d", got)
	}
	assertStockInvariants(t, db)
}

func TestConfirmOperationPermanentFailureIsPerItem(t *testing.T) {
	db := seckillTestDB(t)
	ctx := context.Background()
	const orderNo = "ORD-OPS-2"
	seedStock(t, db, 1, 10, 1)
	seedStock(t, db, 2, 10, 0)
	seedStock(t, db, 3, 10, 1)
	seedReservation(t, db, orderNo, 1, 1, statusHeld)
	seedReservation(t, db, orderNo, 2, 1, statusReleased)
	seedReservation(t, db, orderNo, 3, 1, statusHeld)
	svc := &Service{db: db}

	summary, err := svc.applyInventoryOperation(ctx, opTypeConfirm, orderNo, operationItems(1, 2, 3))
	if err != nil {
		t.Fatalf("terminal partial must be Ackable: %v", err)
	}
	if summary.Status != opStatusPartial || summary.Succeeded != 2 || summary.Failed != 1 || summary.Retryable != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	if got, attempts, reason := operationItemRow(t, db, summary.OperationID, 2); got != itemStatusFailedPermanent || attempts != 1 || reason == "" {
		t.Fatalf("sku2 item = %s attempts=%d reason=%q", got, attempts, reason)
	}
	if got := ledgerCount(t, db, orderNo, 2, entryConfirm); got != 0 {
		t.Fatalf("released reservation was confirmed: ledger=%d", got)
	}
	if got := len(outboxPayloads(t, db, topicOperationItemFailed)); got != 1 {
		t.Fatalf("item-failed events = %d", got)
	}
	if got := len(outboxPayloads(t, db, topicOperationFailed)); got != 1 {
		t.Fatalf("operation-failed events = %d", got)
	}

	// A redelivery stays terminal and does not retry the permanent item.
	again, err := svc.applyInventoryOperation(ctx, opTypeConfirm, orderNo, operationItems(1, 2, 3))
	if err != nil || again.Status != opStatusPartial || again.Failed != 1 {
		t.Fatalf("second pass = %+v err=%v", again, err)
	}
	if got := len(outboxPayloads(t, db, topicOperationItemFailed)); got != 1 {
		t.Fatalf("item-failed events after replay = %d", got)
	}
	// The synchronous TCC path must report the failure to DTM/payment.
	if _, err := svc.TccConfirm(ctx, &pb.TccPaymentRequest{OrderNo: orderNo, Items: operationItems(1, 2, 3)}); !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("TccConfirm err = %v, want FAILED_PRECONDITION", err)
	}
	assertStockInvariants(t, db)
}

func TestCancelOperationMixedStates(t *testing.T) {
	db := seckillTestDB(t)
	ctx := context.Background()
	const orderNo = "ORD-OPS-3"
	seedStock(t, db, 1, 10, 1)
	seedStock(t, db, 2, 9, 0)
	seedStock(t, db, 3, 10, 0)
	seedReservation(t, db, orderNo, 1, 1, statusHeld)
	seedReservation(t, db, orderNo, 2, 1, statusConfirmed)
	// sku3 has no reservation at all.
	svc := &Service{db: db}

	summary, err := svc.applyInventoryOperation(ctx, opTypeCancel, orderNo, operationItems(1, 2, 3))
	if err != nil {
		t.Fatalf("cancel terminal partial must be Ackable: %v", err)
	}
	if summary.Status != opStatusPartial || summary.Succeeded != 2 || summary.Skipped != 1 || summary.Failed != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	if got := reservationStatus(t, db, orderNo, 1); got != statusReleased {
		t.Fatalf("sku1 status = %s", got)
	}
	if got := reservationStatus(t, db, orderNo, 2); got != statusReleased {
		t.Fatalf("sku2 status = %s", got)
	}
	if onHand, reserved := stockCounters(t, db, 1); onHand != 10 || reserved != 0 {
		t.Fatalf("sku1 stock = %d/%d", onHand, reserved)
	}
	if onHand, reserved := stockCounters(t, db, 2); onHand != 10 || reserved != 0 {
		t.Fatalf("sku2 restock = %d/%d", onHand, reserved)
	}
	if got := ledgerCount(t, db, orderNo, 1, entryRelease); got != 1 {
		t.Fatalf("sku1 release ledger = %d", got)
	}
	if got := ledgerCount(t, db, orderNo, 2, entryRestock); got != 1 {
		t.Fatalf("sku2 restock ledger = %d", got)
	}
	if got, _, reason := operationItemRow(t, db, summary.OperationID, 3); got != itemStatusMissing || reason == "" {
		t.Fatalf("sku3 item = %s reason=%q", got, reason)
	}
	if got := len(outboxPayloads(t, db, topicOperationItemFailed)); got != 1 {
		t.Fatalf("item-failed events = %d", got)
	}
	released := outboxPayloads(t, db, topicReleased)
	if len(released) != 2 {
		t.Fatalf("released events = %d", len(released))
	}
	for _, payload := range released {
		if payload["status"] != statusReleased {
			t.Fatalf("released event status = %#v", payload["status"])
		}
	}
	if _, err := svc.TccCancelConfirm(ctx, &pb.TccPaymentRequest{OrderNo: orderNo, Items: operationItems(1, 2, 3)}); err != nil {
		t.Fatalf("TccCancelConfirm should accept a terminal skip: %v", err)
	}
	// Repeating cancel is a no-op: no duplicate ledger or events.
	if _, err := svc.applyInventoryOperation(ctx, opTypeCancel, orderNo, operationItems(1, 2, 3)); err != nil {
		t.Fatalf("repeat cancel: %v", err)
	}
	if got := len(outboxPayloads(t, db, topicReleased)); got != 2 {
		t.Fatalf("released events after replay = %d", got)
	}
	assertStockInvariants(t, db)
}

func TestOperationTxAllOrNothing(t *testing.T) {
	db := seckillTestDB(t)
	ctx := context.Background()
	const orderNo = "ORD-OPS-4"
	seedStock(t, db, 1, 10, 1)
	seedStock(t, db, 2, 10, 0) // broken
	seedStock(t, db, 3, 10, 1)
	for _, sku := range []int64{1, 2, 3} {
		seedReservation(t, db, orderNo, sku, 1, statusHeld)
	}
	svc := &Service{db: db}
	spec, err := newOperationSpec(opTypeConfirm, orderNo, operationItems(1, 2, 3))
	if err != nil {
		t.Fatal(err)
	}
	var summary operationSummary
	err = platformdb.WithTx(ctx, db, func(tx *sql.Tx) error {
		return svc.runOperationTx(ctx, tx, spec, &summary)
	})
	if err == nil {
		t.Fatal("atomic branch should fail on the broken item")
	}
	// The whole transaction rolled back: no operation row, no confirm ledger.
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM inventory_operations WHERE operation_id = ?`, spec.operationID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("operation row survived a rolled-back branch: %d", count)
	}
	for _, sku := range []int64{1, 2, 3} {
		if got := reservationStatus(t, db, orderNo, sku); got != statusHeld {
			t.Fatalf("sku%d status = %s after rollback", sku, got)
		}
		if got := ledgerCount(t, db, orderNo, sku, entryConfirm); got != 0 {
			t.Fatalf("sku%d confirm ledger = %d after rollback", sku, got)
		}
	}

	// Heal and retry the same branch: all items commit in one transaction.
	if _, err := db.Exec(`UPDATE inventory_stocks SET reserved = 1 WHERE sku_id = 2 AND warehouse_id = 1`); err != nil {
		t.Fatal(err)
	}
	err = platformdb.WithTx(ctx, db, func(tx *sql.Tx) error {
		return svc.runOperationTx(ctx, tx, spec, &summary)
	})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Status != opStatusSucceeded || summary.Succeeded != 3 {
		t.Fatalf("summary = %+v", summary)
	}
	for _, sku := range []int64{1, 2, 3} {
		if got := reservationStatus(t, db, orderNo, sku); got != statusConfirmed {
			t.Fatalf("sku%d status = %s", sku, got)
		}
		if got := ledgerCount(t, db, orderNo, sku, entryConfirm); got != 1 {
			t.Fatalf("sku%d confirm ledger = %d", sku, got)
		}
	}
	assertStockInvariants(t, db)
}

func TestReplayMissingCancelItem(t *testing.T) {
	db := seckillTestDB(t)
	ctx := context.Background()
	const orderNo = "ORD-OPS-5"
	seedStock(t, db, 1, 10, 1)
	seedStock(t, db, 2, 10, 0)
	seedReservation(t, db, orderNo, 1, 1, statusHeld)
	svc := &Service{db: db}

	summary, err := svc.applyInventoryOperation(ctx, opTypeCancel, orderNo, operationItems(1, 2))
	if err != nil {
		t.Fatal(err)
	}
	if summary.Status != opStatusPartial || summary.Skipped != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	// The missing item is now backed by a real reservation; replay must release
	// it and promote the operation to SUCCEEDED.
	if _, err := db.Exec(`UPDATE inventory_stocks SET reserved = 1 WHERE sku_id = 2 AND warehouse_id = 1`); err != nil {
		t.Fatal(err)
	}
	seedReservation(t, db, orderNo, 2, 1, statusHeld)
	if err := svc.ReplayInventoryOperation(ctx, summary.OperationID, []int64{2}); err != nil {
		t.Fatalf("replay: %v", err)
	}
	if got := reservationStatus(t, db, orderNo, 2); got != statusReleased {
		t.Fatalf("sku2 status = %s after replay", got)
	}
	status, _, succeeded, failed, skipped := operationRow(t, db, summary.OperationID)
	if status != opStatusSucceeded || succeeded != 2 || failed != 0 || skipped != 0 {
		t.Fatalf("operation after replay = %s %d/%d/%d", status, succeeded, failed, skipped)
	}
	// A DONE item cannot be replayed: stock already moved.
	if err := svc.ReplayInventoryOperation(ctx, summary.OperationID, []int64{1}); !apperr.IsCode(err, apperr.CodeConflict) {
		t.Fatalf("replay DONE item err = %v, want CONFLICT", err)
	}
	assertStockInvariants(t, db)
}

func TestOperationRetryBudgetBecomesPermanent(t *testing.T) {
	db := seckillTestDB(t)
	ctx := context.Background()
	const orderNo = "ORD-OPS-6"
	seedStock(t, db, 1, 10, 0) // permanently inconsistent reserved counter
	seedReservation(t, db, orderNo, 1, 1, statusHeld)
	svc := &Service{db: db}
	var summary operationSummary
	for attempt := 1; attempt <= maxOperationItemAttempts; attempt++ {
		var err error
		summary, err = svc.applyInventoryOperation(ctx, opTypeConfirm, orderNo, operationItems(1))
		if attempt < maxOperationItemAttempts {
			if err == nil {
				t.Fatalf("attempt %d unexpectedly terminal", attempt)
			}
			if got, attempts, _ := operationItemRow(t, db, summary.OperationID, 1); got != itemStatusPending || attempts != attempt {
				t.Fatalf("attempt %d item = %s attempts=%d", attempt, got, attempts)
			}
			continue
		}
		if err != nil {
			t.Fatalf("final attempt should be terminal: %v", err)
		}
	}
	if summary.Status != opStatusPartial || summary.Failed != 1 || summary.Retryable != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	if got, attempts, reason := operationItemRow(t, db, summary.OperationID, 1); got != itemStatusFailedPermanent || attempts != maxOperationItemAttempts {
		t.Fatalf("final item = %s attempts=%d reason=%q", got, attempts, reason)
	}
}

func TestConcurrentConfirmAndCancelKeepInvariants(t *testing.T) {
	db := seckillTestDB(t)
	ctx := context.Background()
	const orderNo = "ORD-OPS-7"
	skus := []int64{1, 2, 3, 4}
	for _, sku := range skus {
		seedStock(t, db, sku, 10, 1)
		seedReservation(t, db, orderNo, sku, 1, statusHeld)
	}
	svc := &Service{db: db}
	var wg sync.WaitGroup
	var confirmSummary, cancelSummary operationSummary
	var confirmErr, cancelErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		confirmSummary, confirmErr = svc.applyInventoryOperation(ctx, opTypeConfirm, orderNo, operationItems(skus...))
	}()
	go func() {
		defer wg.Done()
		cancelSummary, cancelErr = svc.applyInventoryOperation(ctx, opTypeCancel, orderNo, operationItems(skus...))
	}()
	wg.Wait()

	if cancelErr != nil {
		t.Fatalf("cancel error: %v summary=%+v", cancelErr, cancelSummary)
	}
	// Cancel always wins the final state: either it released HELD reservations
	// or restocked CONFIRMED ones. Confirm may be PARTIAL if cancel won a SKU.
	if confirmErr != nil && confirmSummary.Status != opStatusPartial {
		t.Fatalf("confirm error: %v summary=%+v", confirmErr, confirmSummary)
	}
	for _, sku := range skus {
		if got := reservationStatus(t, db, orderNo, sku); got != statusReleased {
			t.Fatalf("sku%d final status = %s", sku, got)
		}
		if onHand, reserved := stockCounters(t, db, sku); onHand != 10 || reserved != 0 {
			t.Fatalf("sku%d stock = %d/%d", sku, onHand, reserved)
		}
		if got := ledgerCount(t, db, orderNo, sku, entryRelease) + ledgerCount(t, db, orderNo, sku, entryRestock); got != 1 {
			t.Fatalf("sku%d release/restock ledger = %d", sku, got)
		}
	}
	assertStockInvariants(t, db)
}

func TestOperationScopeIsBoundToItsID(t *testing.T) {
	db := seckillTestDB(t)
	ctx := context.Background()
	const orderNo = "ORD-OPS-8"
	spec, err := newOperationSpec(opTypeConfirm, orderNo, operationItems(1, 2))
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a collision: the unique (order,type,scope) row uses another id.
	if _, err := db.Exec(`INSERT INTO inventory_operations
		(operation_id, operation_type, order_no, scope_hash, status, total_items,
		 succeeded_items, failed_items, skipped_items, version, created_at, updated_at)
		VALUES ('foreign-operation', ?, ?, ?, 'PENDING', 2, 0, 0, 0, 1, NOW(3), NOW(3))`,
		spec.typ, spec.orderNo, spec.scopeHash); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO inventory_operation_items
		(operation_id, sku_id, quantity, status, attempts, last_error, created_at, updated_at)
		VALUES ('foreign-operation', 1, 1, 'PENDING', 0, '', NOW(3), NOW(3)),
		       ('foreign-operation', 2, 1, 'PENDING', 0, '', NOW(3), NOW(3))`); err != nil {
		t.Fatal(err)
	}
	if _, err := newOperationSpec(opTypeConfirm, orderNo, operationItems(1, 2)); err != nil {
		t.Fatal(err)
	}
	svc := &Service{db: db}
	if _, err := svc.ensureOperation(ctx, spec); !apperr.IsCode(err, apperr.CodeConflict) {
		t.Fatalf("scope collision err = %v, want CONFLICT", err)
	}
}
