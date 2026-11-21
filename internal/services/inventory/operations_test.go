package inventory

import (
	"strings"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

func TestNewOperationSpecIsCanonicalAndScoped(t *testing.T) {
	items := []*pb.OrderItem{
		{SkuId: 3, Quantity: 1},
		{SkuId: 1, Quantity: 2},
		{SkuId: 2, Quantity: 5},
	}
	a, err := newOperationSpec(opTypeConfirm, "ORD-SCOPE", items)
	if err != nil {
		t.Fatal(err)
	}
	// Input order must not change the scope or operation id.
	reordered, err := newOperationSpec(opTypeConfirm, "ORD-SCOPE", []*pb.OrderItem{
		{SkuId: 2, Quantity: 5},
		{SkuId: 3, Quantity: 1},
		{SkuId: 1, Quantity: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if a.operationID != reordered.operationID || a.scopeHash != reordered.scopeHash {
		t.Fatalf("scope depends on item order: %+v vs %+v", a, reordered)
	}
	// Different quantity is a different operation, even for the same order.
	different, err := newOperationSpec(opTypeConfirm, "ORD-SCOPE", []*pb.OrderItem{
		{SkuId: 1, Quantity: 2}, {SkuId: 2, Quantity: 4}, {SkuId: 3, Quantity: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if different.operationID == a.operationID || different.scopeHash == a.scopeHash {
		t.Fatal("quantity change reused the operation scope")
	}
	// Confirm and cancel of the same items are different operations.
	cancel, err := newOperationSpec(opTypeCancel, "ORD-SCOPE", items)
	if err != nil {
		t.Fatal(err)
	}
	if cancel.operationID == a.operationID {
		t.Fatal("confirm and cancel reused the operation id")
	}
	if !strings.HasPrefix(a.operationID, "inv:confirm:ORD-SCOPE:") {
		t.Fatalf("operation id = %q", a.operationID)
	}
	if len(a.operationID) > 128 {
		t.Fatalf("operation id too long: %d", len(a.operationID))
	}
}

func TestNewOperationSpecRejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name  string
		typ   string
		order string
		items []*pb.OrderItem
	}{
		{"type", "REFUND", "ORD-1", []*pb.OrderItem{{SkuId: 1, Quantity: 1}}},
		{"order", opTypeConfirm, "  ", []*pb.OrderItem{{SkuId: 1, Quantity: 1}}},
		{"empty", opTypeConfirm, "ORD-1", nil},
		{"sku", opTypeConfirm, "ORD-1", []*pb.OrderItem{{SkuId: 0, Quantity: 1}}},
		{"zero qty", opTypeConfirm, "ORD-1", []*pb.OrderItem{{SkuId: 1, Quantity: 0}}},
		{"negative qty", opTypeCancel, "ORD-1", []*pb.OrderItem{{SkuId: 1, Quantity: -1}}},
		{"duplicate", opTypeCancel, "ORD-1", []*pb.OrderItem{{SkuId: 1, Quantity: 1}, {SkuId: 1, Quantity: 2}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newOperationSpec(tc.typ, tc.order, tc.items)
			if !apperr.IsCode(err, apperr.CodeInvalidArgument) {
				t.Fatalf("err = %v, want INVALID_ARGUMENT", err)
			}
		})
	}
}

func TestOperationAggregateIDFitsOutboxColumn(t *testing.T) {
	opID := operationIDFor(opTypeConfirm, strings.Repeat("o", 64), strings.Repeat("a", 64))
	aggregate := operationAggregateID(opID)
	if len(aggregate) > 64 {
		t.Fatalf("aggregate id length = %d, want <= 64", len(aggregate))
	}
	if operationAggregateID(opID) != aggregate {
		t.Fatal("aggregate id is not deterministic")
	}
	if operationAggregateID(opID+"x") == aggregate {
		t.Fatal("aggregate id collision for different operations")
	}
}

func TestOperationSummaryCountsPending(t *testing.T) {
	op := &operation{
		status: opStatusInProgress, total: 3, succeeded: 1, failed: 1,
		items: []operationItem{
			{skuID: 1, status: itemStatusDone},
			{skuID: 2, status: itemStatusFailedPermanent},
			{skuID: 3, status: itemStatusPending},
		},
	}
	summary := op.summary()
	if summary.Retryable != 1 || summary.Succeeded != 1 || summary.Failed != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	if op.terminal() {
		t.Fatal("in-progress operation reported terminal")
	}
	op.status = opStatusPartial
	if !op.terminal() {
		t.Fatal("PARTIAL must be terminal")
	}
}
