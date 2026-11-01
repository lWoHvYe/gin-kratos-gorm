package fulfillment

import (
	"math"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

func TestValidateCreateShipment(t *testing.T) {
	tests := []struct {
		name    string
		orderNo string
		items   []*pb.ShipmentItem
		wantErr bool
	}{
		{name: "empty order", orderNo: "", items: []*pb.ShipmentItem{{SkuId: 1, Quantity: 1}}, wantErr: true},
		{name: "no items", orderNo: "ORD1", items: nil, wantErr: true},
		{name: "nil item", orderNo: "ORD1", items: []*pb.ShipmentItem{nil}, wantErr: true},
		{name: "zero sku", orderNo: "ORD1", items: []*pb.ShipmentItem{{SkuId: 0, Quantity: 1}}, wantErr: true},
		{name: "zero quantity", orderNo: "ORD1", items: []*pb.ShipmentItem{{SkuId: 1, Quantity: 0}}, wantErr: true},
		{name: "negative quantity", orderNo: "ORD1", items: []*pb.ShipmentItem{{SkuId: 1, Quantity: -1}}, wantErr: true},
		{name: "quantity overflow", orderNo: "ORD1", items: []*pb.ShipmentItem{
			{SkuId: 1, Quantity: math.MaxInt32},
			{SkuId: 1, Quantity: 1},
		}, wantErr: true},
		{name: "valid", orderNo: "ORD1", items: []*pb.ShipmentItem{{SkuId: 1, Quantity: 2}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateCreateShipment(tt.orderNo, tt.items)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				if !apperr.IsCode(err, apperr.CodeInvalidArgument) {
					t.Fatalf("expected INVALID_ARGUMENT, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateCreateShipmentMergesDuplicateSKUs(t *testing.T) {
	items, err := validateCreateShipment("ORD1", []*pb.ShipmentItem{
		{SkuId: 7, Quantity: 1},
		{SkuId: 9, Quantity: 3},
		{SkuId: 7, Quantity: 4},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 merged items, got %d: %+v", len(items), items)
	}
	if items[0].SkuID != 7 || items[0].Quantity != 5 {
		t.Fatalf("unexpected first item: %+v", items[0])
	}
	if items[1].SkuID != 9 || items[1].Quantity != 3 {
		t.Fatalf("unexpected second item: %+v", items[1])
	}
}

func TestShipOutcome(t *testing.T) {
	tests := []struct {
		current string
		next    string
		changed bool
		wantErr bool
	}{
		{current: statusCreated, next: statusShipped, changed: true},
		{current: statusShipped, next: statusShipped, changed: false},
		{current: statusReceived, wantErr: true},
		{current: "BOGUS", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.current, func(t *testing.T) {
			next, changed, err := shipOutcome(tt.current)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				if !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
					t.Fatalf("expected FAILED_PRECONDITION, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if next != tt.next || changed != tt.changed {
				t.Fatalf("got (%q,%v), want (%q,%v)", next, changed, tt.next, tt.changed)
			}
		})
	}
}

func TestReceiveOutcome(t *testing.T) {
	tests := []struct {
		current string
		next    string
		changed bool
		wantErr bool
	}{
		{current: statusShipped, next: statusReceived, changed: true},
		{current: statusReceived, next: statusReceived, changed: false},
		{current: statusCreated, wantErr: true},
		{current: "BOGUS", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.current, func(t *testing.T) {
			next, changed, err := receiveOutcome(tt.current)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				if !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
					t.Fatalf("expected FAILED_PRECONDITION, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if next != tt.next || changed != tt.changed {
				t.Fatalf("got (%q,%v), want (%q,%v)", next, changed, tt.next, tt.changed)
			}
		})
	}
}
