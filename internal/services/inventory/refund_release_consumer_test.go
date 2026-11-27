package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/events"
)

type fakeRefundRelease struct {
	statuses     map[[2]int64]string
	quantities   map[[2]int64]int64
	releaseErr   map[int64]error
	cancelErr    map[int64]error
	releaseCalls []*pb.ReserveRequest
	cancelCalls  []*pb.TccPaymentRequest
}

func newFakeRefundRelease() *fakeRefundRelease {
	return &fakeRefundRelease{
		statuses:   make(map[[2]int64]string),
		quantities: make(map[[2]int64]int64),
		releaseErr: make(map[int64]error),
		cancelErr:  make(map[int64]error),
	}
}

func (f *fakeRefundRelease) refundReleaseReservation(_ context.Context, _ string, skuID, warehouseID int64) (string, int64, error) {
	key := [2]int64{skuID, warehouseID}
	return f.statuses[key], f.quantities[key], nil
}

func (f *fakeRefundRelease) Release(_ context.Context, req *pb.ReserveRequest) (*pb.Empty, error) {
	f.releaseCalls = append(f.releaseCalls, req)
	if err := f.releaseErr[req.GetSkuId()]; err != nil {
		return nil, err
	}
	f.statuses[[2]int64{req.GetSkuId(), req.GetWarehouseId()}] = statusReleased
	return &pb.Empty{}, nil
}

func (f *fakeRefundRelease) TccCancelConfirm(_ context.Context, req *pb.TccPaymentRequest) (*pb.Empty, error) {
	f.cancelCalls = append(f.cancelCalls, req)
	item := req.GetItems()[0]
	if err := f.cancelErr[item.GetSkuId()]; err != nil {
		return nil, err
	}
	f.statuses[[2]int64{item.GetSkuId(), item.GetWarehouseId()}] = statusReleased
	return &pb.Empty{}, nil
}

func releasePayload(t *testing.T, orderNo, releaseID, refundID string, items ...events.RefundReleaseItem) []byte {
	t.Helper()
	raw, err := json.Marshal(events.RefundReleaseRequested{
		SchemaVersion: 1,
		ReleaseScope:  events.RefundReleaseScopeOrderReservationFull,
		GroupCount:    1,
		ReleaseID:     releaseID,
		RefundID:      refundID,
		OrderNo:       orderNo,
		GroupNo:       "G2",
		Source:        events.RefundReleaseSourceAfterSale,
		Reason:        events.RefundReleaseReasonAfterSaleRefund,
		Items:         items,
		RequestedAt:   time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestRefundReleaseHandlerDispatchesHeldAndConfirmed(t *testing.T) {
	fake := newFakeRefundRelease()
	fake.statuses[[2]int64{1, 1}] = statusHeld
	fake.statuses[[2]int64{2, 3}] = statusConfirmed
	fake.quantities[[2]int64{2, 3}] = 5
	data := releasePayload(t, "ORD-REL", "rr:R-1", "R-1",
		events.RefundReleaseItem{SkuID: 1, Quantity: 2, WarehouseID: 1},
		events.RefundReleaseItem{SkuID: 2, Quantity: 1, WarehouseID: 3},
	)
	if err := handleRefundRelease(context.Background(), fake, data); err != nil {
		t.Fatal(err)
	}
	if len(fake.releaseCalls) != 1 {
		t.Fatalf("release calls=%d, want 1", len(fake.releaseCalls))
	}
	release := fake.releaseCalls[0]
	if release.GetOrderNo() != "ORD-REL" || release.GetSkuId() != 1 ||
		release.GetWarehouseId() != 1 || release.GetQuantity() != 0 {
		t.Fatalf("release request = %+v", release)
	}
	if len(fake.cancelCalls) != 1 {
		t.Fatalf("cancel calls=%d, want 1", len(fake.cancelCalls))
	}
	cancel := fake.cancelCalls[0]
	if cancel.GetOrderNo() != "ORD-REL" || len(cancel.GetItems()) != 1 ||
		cancel.GetItems()[0].GetSkuId() != 2 || cancel.GetItems()[0].GetWarehouseId() != 3 ||
		cancel.GetItems()[0].GetQuantity() != 5 {
		t.Fatalf("cancel request = %+v", cancel)
	}
}

func TestRefundReleaseHandlerSkipsReleasedAndMissingReservations(t *testing.T) {
	fake := newFakeRefundRelease()
	fake.statuses[[2]int64{1, 1}] = statusReleased
	data := releasePayload(t, "ORD-REL", "rr:R-1", "R-1",
		events.RefundReleaseItem{SkuID: 1, Quantity: 1, WarehouseID: 1},
		events.RefundReleaseItem{SkuID: 2, Quantity: 1, WarehouseID: 1},
	)
	for i := 0; i < 2; i++ {
		if err := handleRefundRelease(context.Background(), fake, data); err != nil {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
	}
	if len(fake.releaseCalls) != 0 || len(fake.cancelCalls) != 0 {
		t.Fatalf("released/missing reservations were touched: release=%d cancel=%d",
			len(fake.releaseCalls), len(fake.cancelCalls))
	}
}

func TestRefundReleaseHandlerRejectsMalformedEvents(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"missing warehouse", releasePayload(t, "ORD-REL", "rr:R-1", "R-1",
			events.RefundReleaseItem{SkuID: 1, Quantity: 1})},
		{"release_id mismatch", releasePayload(t, "ORD-REL", "rr:R-OTHER", "R-1",
			events.RefundReleaseItem{SkuID: 1, Quantity: 1, WarehouseID: 1})},
		{"not json", []byte(`not-json`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := newFakeRefundRelease()
			fake.statuses[[2]int64{1, 1}] = statusHeld
			if err := handleRefundRelease(context.Background(), fake, tc.data); err == nil {
				t.Fatal("malformed release event was accepted")
			}
			if len(fake.releaseCalls) != 0 || len(fake.cancelCalls) != 0 {
				t.Fatal("malformed release event reached the executor")
			}
		})
	}
}

// A transient failure on a later item aborts the message: the first release
// stays committed (it is a HELD -> RELEASED fact), the handler returns an error
// so the framework retries, and the redelivery skips the released item instead
// of releasing it twice.
func TestRefundReleaseHandlerRetriesWithoutDoubleRelease(t *testing.T) {
	fake := newFakeRefundRelease()
	fake.statuses[[2]int64{1, 1}] = statusHeld
	fake.statuses[[2]int64{2, 1}] = statusHeld
	fake.releaseErr[2] = context.DeadlineExceeded
	data := releasePayload(t, "ORD-REL", "rr:R-1", "R-1",
		events.RefundReleaseItem{SkuID: 1, Quantity: 1, WarehouseID: 1},
		events.RefundReleaseItem{SkuID: 2, Quantity: 1, WarehouseID: 1},
	)
	err := handleRefundRelease(context.Background(), fake, data)
	if err == nil {
		t.Fatal("partial failure was ACKed")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want the transient cause", err)
	}
	if fake.statuses[[2]int64{1, 1}] != statusReleased || fake.statuses[[2]int64{2, 1}] != statusHeld {
		t.Fatalf("partial state = %+v", fake.statuses)
	}
	delete(fake.releaseErr, 2)
	if err := handleRefundRelease(context.Background(), fake, data); err != nil {
		t.Fatal(err)
	}
	if fake.statuses[[2]int64{2, 1}] != statusReleased {
		t.Fatal("retry did not release the remaining item")
	}
	if len(fake.releaseCalls) != 3 {
		t.Fatalf("release calls=%d, want 3 (item1 once, item2 twice)", len(fake.releaseCalls))
	}
}

func TestRefundReleaseHandlerPropagatesPermanentDataError(t *testing.T) {
	fake := newFakeRefundRelease()
	fake.statuses[[2]int64{1, 1}] = statusConfirmed
	fake.cancelErr[1] = apperr.Failed("stock row missing while restocking")
	data := releasePayload(t, "ORD-REL", "rr:R-1", "R-1",
		events.RefundReleaseItem{SkuID: 1, Quantity: 1, WarehouseID: 1})
	if err := handleRefundRelease(context.Background(), fake, data); err == nil {
		t.Fatal("permanent data error was ACKed")
	}
	if fake.statuses[[2]int64{1, 1}] != statusConfirmed {
		t.Fatal("permanent failure changed the reservation state")
	}
}

func TestRefundReleaseHandlerRejectsUnknownReservationStatus(t *testing.T) {
	fake := newFakeRefundRelease()
	fake.statuses[[2]int64{1, 1}] = "BROKEN"
	data := releasePayload(t, "ORD-REL", "rr:R-1", "R-1",
		events.RefundReleaseItem{SkuID: 1, Quantity: 1, WarehouseID: 1})
	if err := handleRefundRelease(context.Background(), fake, data); err == nil {
		t.Fatal("unknown reservation status was ACKed")
	}
}

// The consumer must fail closed when group_count is missing/0/!=1 or when the
// optional audit list names a different group. Malformed events never reach the
// executor.
func TestRefundReleaseHandlerRejectsInvalidGroupCount(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"missing group_count", func(p map[string]any) { delete(p, "group_count") }},
		{"zero group_count", func(p map[string]any) { p["group_count"] = 0 }},
		{"multi group_count", func(p map[string]any) { p["group_count"] = 2 }},
		{"covered other group", func(p map[string]any) { p["covered_group_nos"] = []any{"G1"} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := newFakeRefundRelease()
			fake.statuses[[2]int64{1, 1}] = statusHeld
			var payload map[string]any
			if err := json.Unmarshal(releasePayload(t, "ORD-REL", "rr:R-1", "R-1",
				events.RefundReleaseItem{SkuID: 1, Quantity: 1, WarehouseID: 1}), &payload); err != nil {
				t.Fatal(err)
			}
			tc.mutate(payload)
			raw, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			if err := handleRefundRelease(context.Background(), fake, raw); err == nil {
				t.Fatalf("%s was accepted", tc.name)
			}
			if len(fake.releaseCalls) != 0 || len(fake.cancelCalls) != 0 {
				t.Fatalf("%s reached the executor", tc.name)
			}
		})
	}
}

func TestRefundReleaseHandlerAcceptsSingleGroupCoveredAudit(t *testing.T) {
	fake := newFakeRefundRelease()
	fake.statuses[[2]int64{1, 1}] = statusHeld
	raw := releasePayload(t, "ORD-REL", "rr:R-1", "R-1",
		events.RefundReleaseItem{SkuID: 1, Quantity: 1, WarehouseID: 1})
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	payload["covered_group_nos"] = []any{"G2"}
	// releasePayload uses GroupNo G2 in the unit helper.
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := handleRefundRelease(context.Background(), fake, raw); err != nil {
		t.Fatalf("single-group covered audit rejected: %v", err)
	}
	if len(fake.releaseCalls) != 1 {
		t.Fatalf("release calls=%d, want 1", len(fake.releaseCalls))
	}
}

func TestRefundReleaseLegacy24hAcceptedByConsumer(t *testing.T) {
	fake := newFakeRefundRelease()
	fake.statuses[[2]int64{1, 1}] = statusHeld
	raw, err := json.Marshal(events.RefundReleaseRequested{
		SchemaVersion: 1,
		ReleaseScope:  events.RefundReleaseScopeOrderReservationFull,
		GroupCount:    1,
		ReleaseID:     events.RefundReleaseID("pay:PAY-1:legacy_expired"),
		RefundID:      "pay:PAY-1:legacy_expired",
		OrderNo:       "ORD-LEGACY",
		GroupNo:       "G1",
		Source:        events.RefundReleaseSourceLegacy24h,
		Reason:        events.RefundReleaseReasonLegacy24hExpiry,
		Items:         []events.RefundReleaseItem{{SkuID: 1, Quantity: 1, WarehouseID: 1}},
		RequestedAt:   time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := handleRefundRelease(context.Background(), fake, raw); err != nil {
		t.Fatal(err)
	}
	if len(fake.releaseCalls) != 1 || fake.releaseCalls[0].GetOrderNo() != "ORD-LEGACY" {
		t.Fatalf("legacy_24h release calls = %+v", fake.releaseCalls)
	}
}
