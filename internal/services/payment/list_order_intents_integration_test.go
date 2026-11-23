package payment

import (
	"context"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
)

func TestListOrderIntentsReturnsAllStatuses(t *testing.T) {
	db := paymentTestDB(t)
	svc := New(db, nil, "", nil)
	ctx := context.Background()
	for _, row := range []struct {
		no, status string
	}{
		{"PAY-L1", "PENDING"},
		{"PAY-L2", "SUCCEEDED"},
		{"PAY-L3", "FAILED"},
	} {
		if _, err := db.Exec(`INSERT INTO payment_intents
			(payment_no, order_no, amount_cent, status, expires_at)
			VALUES (?, 'ORD-L', 100, ?, NOW(3))`, row.no, row.status); err != nil {
			t.Fatal(err)
		}
	}
	reply, err := svc.ListOrderIntents(ctx, &pb.ListOrderIntentsRequest{OrderNo: "ORD-L"})
	if err != nil {
		t.Fatal(err)
	}
	if len(reply.GetIntents()) != 3 {
		t.Fatalf("intents = %d, want 3", len(reply.GetIntents()))
	}
	if reply.GetIntents()[0].GetPaymentNo() != "PAY-L1" || reply.GetIntents()[2].GetStatus() != "FAILED" {
		t.Fatalf("unexpected intents: %+v", reply.GetIntents())
	}
}
