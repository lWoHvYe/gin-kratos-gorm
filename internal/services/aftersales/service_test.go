package aftersales

import (
	"math"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

func TestValidateCreate(t *testing.T) {
	tests := []struct {
		name     string
		orderNo  string
		userID   int64
		typ      string
		reason   string
		amount   int64
		items    []*pb.AfterSaleItem
		wantErr  bool
		wantType string
	}{
		{name: "empty order", orderNo: " ", userID: 1, typ: typeRefundOnly, amount: 100, wantErr: true},
		{name: "missing user", orderNo: "ORD1", userID: 0, typ: typeRefundOnly, amount: 100, wantErr: true},
		{name: "bad type", orderNo: "ORD1", userID: 1, typ: "EXCHANGE", amount: 100, wantErr: true},
		{name: "zero amount", orderNo: "ORD1", userID: 1, typ: typeRefundOnly, amount: 0, wantErr: true},
		{name: "negative amount", orderNo: "ORD1", userID: 1, typ: typeRefundOnly, amount: -5, wantErr: true},
		{name: "nil item", orderNo: "ORD1", userID: 1, typ: typeReturnRefund, amount: 100, items: []*pb.AfterSaleItem{nil}, wantErr: true},
		{name: "bad item sku", orderNo: "ORD1", userID: 1, typ: typeReturnRefund, amount: 100, items: []*pb.AfterSaleItem{{SkuId: -1, Quantity: 1}}, wantErr: true},
		{name: "bad item quantity", orderNo: "ORD1", userID: 1, typ: typeReturnRefund, amount: 100, items: []*pb.AfterSaleItem{{SkuId: 1, Quantity: 0}}, wantErr: true},
		{name: "overflow", orderNo: "ORD1", userID: 1, typ: typeReturnRefund, amount: 100, items: []*pb.AfterSaleItem{
			{SkuId: 1, Quantity: math.MaxInt32},
			{SkuId: 1, Quantity: 1},
		}, wantErr: true},
		{name: "refund only valid", orderNo: "ORD1", userID: 1, typ: typeRefundOnly, amount: 100},
		{name: "return refund valid", orderNo: "ORD1", userID: 1, typ: typeReturnRefund, reason: "  broken  ", amount: 2500, items: []*pb.AfterSaleItem{{SkuId: 3, Quantity: 2}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in, _, err := validateCreate(tt.orderNo, tt.userID, tt.typ, tt.reason, tt.amount, tt.items)
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
			if in.amountCent != tt.amount || in.typ != tt.typ {
				t.Fatalf("unexpected input: %+v", in)
			}
			if in.reason != "broken" && tt.reason != "" {
				t.Fatalf("reason not trimmed: %q", in.reason)
			}
		})
	}
}

func TestValidateCreateMergesDuplicateSKUs(t *testing.T) {
	_, items, err := validateCreate("ORD1", 1, typeReturnRefund, "", 500, []*pb.AfterSaleItem{
		{SkuId: 2, Quantity: 1},
		{SkuId: 2, Quantity: 2},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].SkuID != 2 || items[0].Quantity != 3 {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestApproveOutcome(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		approved bool
		next     string
		changed  bool
		wantErr  bool
	}{
		{name: "approve requested", current: statusRequested, approved: true, next: statusApproved, changed: true},
		{name: "reject requested", current: statusRequested, approved: false, next: statusRejected, changed: true},
		{name: "approve twice", current: statusApproved, approved: true, next: statusApproved, changed: false},
		{name: "reject approved", current: statusApproved, approved: false, wantErr: true},
		{name: "reject twice", current: statusRejected, approved: false, next: statusRejected, changed: false},
		{name: "approve rejected", current: statusRejected, approved: true, wantErr: true},
		{name: "approve refunded", current: statusRefunded, approved: true, wantErr: true},
		{name: "approve refunding", current: statusRefunding, approved: true, wantErr: true},
		{name: "unknown status", current: "BOGUS", approved: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next, changed, err := approveOutcome(tt.current, tt.approved)
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

func TestRefundOutcome(t *testing.T) {
	tests := []struct {
		name    string
		current string
		next    string
		changed bool
		wantErr bool
	}{
		{name: "approved", current: statusApproved, next: statusRefunding, changed: true},
		{name: "refunding", current: statusRefunding, next: statusRefunded, changed: true},
		{name: "already refunded", current: statusRefunded, next: statusRefunded, changed: false},
		{name: "requested", current: statusRequested, wantErr: true},
		{name: "rejected", current: statusRejected, wantErr: true},
		{name: "unknown", current: "BOGUS", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next, changed, err := refundOutcome(tt.current)
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

func TestRefundableStatus(t *testing.T) {
	cases := []struct {
		name    string
		typ     string
		status  string
		wantErr bool
	}{
		{"refund only approved", typeRefundOnly, statusApproved, false},
		{"refund only requested", typeRefundOnly, statusRequested, true},
		{"return received", typeReturnRefund, statusReceived, false},
		{"return approved", typeReturnRefund, statusApproved, true},
		{"return returning", typeReturnRefund, statusReturning, true},
		{"retry refunding", typeReturnRefund, statusRefunding, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := refundableStatus(&afterSaleRow{typ: tc.typ, status: tc.status})
			if tc.wantErr != (err != nil) {
				t.Fatalf("wantErr=%v got %v", tc.wantErr, err)
			}
		})
	}
}
