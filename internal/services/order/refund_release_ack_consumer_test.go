package order

import (
	"context"
	"database/sql"
	"testing"

	"gin-kratos-gorm/internal/platform/events"
)

func seedRefundReleaseReceipt(t *testing.T, db *sql.DB, releaseID, refundID, orderNo, state string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO refund_release_receipts
		(release_id, refund_id, order_no, state, version) VALUES (?, ?, ?, ?, 1)`,
		releaseID, refundID, orderNo, state); err != nil {
		t.Fatal(err)
	}
}

func TestRefundReleaseAckAdvancesReceipt(t *testing.T) {
	db := orderTestDB(t)
	svc := legacyService(db)
	seedRefundReleaseReceipt(t, db, "rr:pay:PAY-1:legacy_expired", "pay:PAY-1:legacy_expired", "ORD-ACK", refundReleaseReceiptPending)
	event := events.RefundReleaseCompleted{
		SchemaVersion: 1,
		ReleaseID:     "rr:pay:PAY-1:legacy_expired",
		RefundID:      "pay:PAY-1:legacy_expired",
		OrderNo:       "ORD-ACK",
		Status:        events.RefundReleaseStatusReleased,
		ReleasedAt:    "2026-11-27T00:00:00Z",
	}
	if err := applyRefundReleaseAck(context.Background(), svc, event); err != nil {
		t.Fatal(err)
	}
	var state string
	var version int64
	if err := db.QueryRow(`SELECT state, version FROM refund_release_receipts WHERE release_id = ?`, event.ReleaseID).
		Scan(&state, &version); err != nil {
		t.Fatal(err)
	}
	if state != refundReleaseReceiptReleased || version != 2 {
		t.Fatalf("receipt = %s/%d", state, version)
	}
	// Duplicate ack is a no-op.
	if err := applyRefundReleaseAck(context.Background(), svc, event); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT state, version FROM refund_release_receipts WHERE release_id = ?`, event.ReleaseID).
		Scan(&state, &version); err != nil {
		t.Fatal(err)
	}
	if state != refundReleaseReceiptReleased || version != 2 {
		t.Fatalf("duplicate receipt = %s/%d", state, version)
	}
}

func TestRefundReleaseAckFailedAndUnknown(t *testing.T) {
	db := orderTestDB(t)
	svc := legacyService(db)
	seedRefundReleaseReceipt(t, db, "rr:pay:PAY-2:legacy_expired", "pay:PAY-2:legacy_expired", "ORD-ACK2", refundReleaseReceiptPending)
	failed := events.RefundReleaseCompleted{
		SchemaVersion: 1,
		ReleaseID:     "rr:pay:PAY-2:legacy_expired",
		RefundID:      "pay:PAY-2:legacy_expired",
		OrderNo:       "ORD-ACK2",
		Status:        events.RefundReleaseStatusFailed,
		ReleasedAt:    "2026-11-27T00:00:00Z",
		Error:         "inventory unavailable",
	}
	if err := applyRefundReleaseAck(context.Background(), svc, failed); err != nil {
		t.Fatal(err)
	}
	var state, lastError string
	if err := db.QueryRow(`SELECT state, last_error FROM refund_release_receipts WHERE release_id = ?`, failed.ReleaseID).
		Scan(&state, &lastError); err != nil {
		t.Fatal(err)
	}
	if state != refundReleaseReceiptFailed || lastError == "" {
		t.Fatalf("failed receipt = %s/%q", state, lastError)
	}
	unknown := failed
	unknown.ReleaseID = "rr:missing"
	unknown.RefundID = "missing"
	if err := applyRefundReleaseAck(context.Background(), svc, unknown); err == nil {
		t.Fatal("unknown release receipt was accepted")
	}
}
