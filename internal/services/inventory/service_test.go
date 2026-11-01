package inventory

import (
	"strings"
	"testing"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

func TestNormalizeReserveDefaults(t *testing.T) {
	in, err := normalizeReserve(&pb.ReserveRequest{
		OrderNo:  "  ORD-1  ",
		SkuId:    42,
		Quantity: 3,
	})
	if err != nil {
		t.Fatalf("normalizeReserve: %v", err)
	}
	if in.orderNo != "ORD-1" {
		t.Fatalf("order_no = %q, want trimmed ORD-1", in.orderNo)
	}
	if in.skuID != 42 || in.quantity != 3 {
		t.Fatalf("unexpected sku/quantity: %+v", in)
	}
	if in.warehouseID != defaultWarehouseID {
		t.Fatalf("warehouse_id = %d, want %d", in.warehouseID, defaultWarehouseID)
	}
	if in.ttl != 900*time.Second {
		t.Fatalf("ttl = %v, want 900s", in.ttl)
	}
}

func TestNormalizeReserveExplicitValues(t *testing.T) {
	in, err := normalizeReserve(&pb.ReserveRequest{
		OrderNo:     "ORD-2",
		SkuId:       7,
		WarehouseId: 3,
		Quantity:    1,
		TtlSeconds:  60,
	})
	if err != nil {
		t.Fatalf("normalizeReserve: %v", err)
	}
	if in.warehouseID != 3 {
		t.Fatalf("warehouse_id = %d, want 3", in.warehouseID)
	}
	if in.ttl != time.Minute {
		t.Fatalf("ttl = %v, want 1m", in.ttl)
	}
}

func TestNormalizeReserveRejectsInvalidInput(t *testing.T) {
	tooLong := strings.Repeat("o", maxOrderNoLength+1)
	cases := []struct {
		name string
		req  *pb.ReserveRequest
	}{
		{"empty order", &pb.ReserveRequest{SkuId: 1, Quantity: 1}},
		{"blank order", &pb.ReserveRequest{OrderNo: "   ", SkuId: 1, Quantity: 1}},
		{"long order", &pb.ReserveRequest{OrderNo: tooLong, SkuId: 1, Quantity: 1}},
		{"missing sku", &pb.ReserveRequest{OrderNo: "ORD-3", Quantity: 1}},
		{"zero quantity", &pb.ReserveRequest{OrderNo: "ORD-3", SkuId: 1}},
		{"negative quantity", &pb.ReserveRequest{OrderNo: "ORD-3", SkuId: 1, Quantity: -1}},
		{"negative ttl", &pb.ReserveRequest{OrderNo: "ORD-3", SkuId: 1, Quantity: 1, TtlSeconds: -1}},
		{"huge ttl", &pb.ReserveRequest{OrderNo: "ORD-3", SkuId: 1, Quantity: 1, TtlSeconds: 86401}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := normalizeReserve(tc.req); !apperr.IsCode(err, apperr.CodeInvalidArgument) {
				t.Fatalf("err = %v, want INVALID_ARGUMENT", err)
			}
		})
	}
}

func TestNormalizeReservationKeyAllowsZeroQuantity(t *testing.T) {
	in, err := normalizeReservationKey(&pb.ReserveRequest{OrderNo: "ORD-4", SkuId: 9})
	if err != nil {
		t.Fatalf("normalizeReservationKey: %v", err)
	}
	if in.quantity != 0 {
		t.Fatalf("quantity = %d, want 0", in.quantity)
	}
	if _, err := normalizeReservationKey(&pb.ReserveRequest{OrderNo: "ORD-4", SkuId: 9, Quantity: -2}); !apperr.IsCode(err, apperr.CodeInvalidArgument) {
		t.Fatalf("negative quantity err = %v, want INVALID_ARGUMENT", err)
	}
}

func TestReservationReplyStatuses(t *testing.T) {
	expires := time.Unix(1700000000, 0)
	held := &reservation{reservationNo: "RSV-1", orderNo: "ORD-5", status: statusHeld, expiresAt: expires}
	reply, err := reservationReply(held)
	if err != nil {
		t.Fatalf("HELD reply: %v", err)
	}
	if reply.GetReservationNo() != "RSV-1" || reply.GetExpiresAtUnix() != 1700000000 {
		t.Fatalf("unexpected HELD reply: %+v", reply)
	}

	confirmed := &reservation{reservationNo: "RSV-2", orderNo: "ORD-5", status: statusConfirmed, expiresAt: expires}
	if _, err := reservationReply(confirmed); err != nil {
		t.Fatalf("CONFIRMED reply: %v", err)
	}

	released := &reservation{reservationNo: "RSV-3", orderNo: "ORD-5", status: statusReleased, expiresAt: expires}
	if _, err := reservationReply(released); !apperr.IsCode(err, apperr.CodeConflict) {
		t.Fatalf("RELEASED err = %v, want CONFLICT", err)
	}

	unknown := &reservation{reservationNo: "RSV-4", orderNo: "ORD-5", status: "BROKEN"}
	if _, err := reservationReply(unknown); !apperr.IsCode(err, apperr.CodeInternal) {
		t.Fatalf("unknown status err = %v, want INTERNAL", err)
	}
}

func TestStockAvailable(t *testing.T) {
	st := &stock{onHand: 10, reserved: 4}
	if got := st.available(); got != 6 {
		t.Fatalf("available = %d, want 6", got)
	}
}

func TestEventIDDeterministicAndBounded(t *testing.T) {
	a := eventID(eventReserved, "RSV123")
	b := eventID(eventReserved, "RSV123")
	if a != b {
		t.Fatalf("event id is not deterministic: %q vs %q", a, b)
	}
	if a == eventID(eventConfirmed, "RSV123") {
		t.Fatal("different event types must have different ids")
	}
	long := eventID(eventReserved, strings.Repeat("x", 100))
	if len(long) > 64 {
		t.Fatalf("event id length = %d, want <= 64", len(long))
	}
}
