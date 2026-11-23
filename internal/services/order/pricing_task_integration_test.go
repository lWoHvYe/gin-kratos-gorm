package order

import (
	"context"
	"testing"
)

func TestPricingConfirmationTaskCreatedOnSettlement(t *testing.T) {
	db := orderTestDB(t)
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := ensurePricingConfirmationTask(ctx, tx, &orderRecord{
		OrderNo: "ORD-PRICE", QuoteID: "QUOTE-PRICE", UserID: 7,
	}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var state, payload string
	if err := db.QueryRow(`SELECT state, payload FROM pricing_confirmation_tasks WHERE order_no='ORD-PRICE'`).
		Scan(&state, &payload); err != nil {
		t.Fatal(err)
	}
	if state != pricingTaskPending || payload == "" {
		t.Fatalf("state=%q payload=%q", state, payload)
	}
}
