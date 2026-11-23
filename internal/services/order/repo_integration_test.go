package order

import (
	"context"
	"testing"
	"time"
)

// TestListOrdersScansAllColumns is the regression guard for the list-read path:
// orderColumns declares every orders column, so listOrders must hand Scan one
// destination per column. Before the shared scan helper, listOrders scanned 20
// of 24 columns and every call failed with
// "expected 24 destination arguments in Scan, not 20".
func TestListOrdersScansAllColumns(t *testing.T) {
	db := orderTestDB(t)
	ctx := context.Background()

	const (
		orderNo        = "ORD-LIST-ALLCOLS"
		userID         = int64(424242)
		allocationHash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
		totalCent      = int64(19900)
		discountCent   = int64(1500)
		promotionCent  = int64(500)
		freightCent    = int64(800)
		payCent        = int64(19200)
		paidCent       = int64(19200)
		refundedCent   = int64(1200)
		packageCount   = int32(3)
		warehouseCount = int32(2)
	)
	frozenAt := time.Date(2026, 11, 23, 9, 15, 30, 0, time.UTC)
	createdAt := time.Date(2026, 11, 23, 9, 0, 0, 0, time.UTC)
	expireAt := createdAt.Add(30 * time.Minute)

	if _, err := db.ExecContext(ctx, `INSERT INTO orders
		(order_no, user_id, status, total_cent, discount_cent, promotion_discount_cent,
		 applied_promotions_json, freight_cent, pay_cent, paid_cent, settlement_payment_no, refunded_cent, refund_status,
		 refund_state, receiver_name, receiver_phone, receiver_address, quote_id, client_request_id,
		 allocation_frozen_at, allocation_hash, package_count, warehouse_count, created_at, expire_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		orderNo, userID, statusAllocated, totalCent, discountCent, promotionCent,
		`[{"promotion_id":7,"discount_cent":500}]`, freightCent, payCent, paidCent, "PAY-LIST", refundedCent, "PARTIAL",
		"NONE", "Ada Lovelace", "13800000000", "1 Analytical Engine Way", "QUOTE-1", "client-req-1",
		frozenAt, allocationHash, packageCount, warehouseCount, createdAt, expireAt); err != nil {
		t.Fatalf("insert order fixture: %v", err)
	}

	records, err := listOrders(ctx, db, "WHERE user_id = ?", []any{userID}, "created_at DESC, id DESC", 20, 0)
	if err != nil {
		t.Fatalf("listOrders must scan every column declared in orderColumns: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("listOrders returned %d orders, want 1", len(records))
	}
	got := records[0]

	if got.OrderNo != orderNo || got.UserID != userID || got.Status != statusAllocated {
		t.Fatalf("identity fields mismatch: %+v", got)
	}
	if got.TotalCent != totalCent || got.DiscountCent != discountCent ||
		got.PromotionDiscountCent != promotionCent || got.FreightCent != freightCent ||
		got.PayCent != payCent || got.PaidCent != paidCent || got.RefundedCent != refundedCent {
		t.Fatalf("amount fields mismatch: total=%d discount=%d promotion=%d freight=%d pay=%d paid=%d refunded=%d",
			got.TotalCent, got.DiscountCent, got.PromotionDiscountCent, got.FreightCent,
			got.PayCent, got.PaidCent, got.RefundedCent)
	}
	if got.AppliedPromotionsJSON != `[{"promotion_id":7,"discount_cent":500}]` {
		t.Fatalf("applied_promotions_json mismatch: %q", got.AppliedPromotionsJSON)
	}
	if got.RefundStatus != "PARTIAL" || got.RefundState != "NONE" {
		t.Fatalf("refund projection mismatch: status=%q state=%q", got.RefundStatus, got.RefundState)
	}
	if got.ReceiverName != "Ada Lovelace" || got.ReceiverPhone != "13800000000" ||
		got.ReceiverAddress != "1 Analytical Engine Way" {
		t.Fatalf("receiver fields mismatch: %+v", got)
	}
	if got.QuoteID != "QUOTE-1" || got.ClientRequestID != "client-req-1" {
		t.Fatalf("quote/idempotency fields mismatch: quote=%q client_request=%q", got.QuoteID, got.ClientRequestID)
	}
	// The four allocation columns were the ones dropped by the old literal Scan.
	if !got.AllocationFrozenAt.Valid {
		t.Fatal("allocation_frozen_at was not scanned (NULL or missing destination)")
	}
	if !got.AllocationFrozenAt.Time.Equal(frozenAt) {
		t.Fatalf("allocation_frozen_at = %s, want %s", got.AllocationFrozenAt.Time, frozenAt)
	}
	if got.AllocationHash != allocationHash {
		t.Fatalf("allocation_hash = %q, want %q", got.AllocationHash, allocationHash)
	}
	if got.PackageCount != packageCount || got.WarehouseCount != warehouseCount {
		t.Fatalf("allocation counts = (%d,%d), want (%d,%d)",
			got.PackageCount, got.WarehouseCount, packageCount, warehouseCount)
	}
	if !got.CreatedAt.Equal(createdAt) || !got.ExpireAt.Equal(expireAt) {
		t.Fatalf("timestamps mismatch: created=%s expire=%s", got.CreatedAt, got.ExpireAt)
	}
}
