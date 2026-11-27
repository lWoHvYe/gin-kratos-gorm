package order

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"

	"google.golang.org/grpc"
)

type ac47Inventory struct {
	pb.InventoryServiceClient
	confirmErr   error
	confirmCalls int
	cancelCalls  int
}

func (f *ac47Inventory) Confirm(context.Context, *pb.ReserveRequest, ...grpc.CallOption) (*pb.Empty, error) {
	f.confirmCalls++
	if f.confirmErr != nil {
		return nil, f.confirmErr
	}
	return &pb.Empty{}, nil
}

func (f *ac47Inventory) TccCancelConfirm(context.Context, *pb.TccPaymentRequest, ...grpc.CallOption) (*pb.Empty, error) {
	f.cancelCalls++
	return &pb.Empty{}, nil
}

func (f *ac47Inventory) CheckOrderReady(context.Context, *pb.CheckOrderReadyRequest, ...grpc.CallOption) (*pb.CheckOrderReadyReply, error) {
	return &pb.CheckOrderReadyReply{Ready: true}, nil
}

func TestAC47MarkPaidConfirmFailureCompensates(t *testing.T) {
	db := orderTestDB(t)
	inventory := &ac47Inventory{confirmErr: errors.New("inventory confirm failed")}
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.inventory = inventory
	seedTccPendingOrder(t, db, "ORD-AC47", 10000, 0, 0)
	seedLegacyItem(t, db, "ORD-AC47", 10000)
	_, err := svc.MarkPaid(context.Background(), &pb.MarkPaidRequest{
		OrderNo: "ORD-AC47", PaymentNo: "PAY-AC47", PaidCent: 10000,
	})
	if err == nil || !strings.Contains(err.Error(), "confirm inventory before payment") {
		t.Fatalf("MarkPaid error = %v", err)
	}
	if got := orderStatus(t, db, "ORD-AC47"); got != statusPendingPayment {
		t.Fatalf("status = %s, want PENDING_PAYMENT", got)
	}
	if inventory.confirmCalls != 1 || inventory.cancelCalls != 1 {
		t.Fatalf("inventory calls = confirm:%d cancel:%d", inventory.confirmCalls, inventory.cancelCalls)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-AC47' AND event_type = 'order.paid'`); got != 0 {
		t.Fatalf("paid events = %d, want 0", got)
	}
}
