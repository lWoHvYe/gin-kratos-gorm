package events

import (
	"encoding/json"
	"testing"
)

func TestCoreEventContractsEncodeRequiredFields(t *testing.T) {
	cases := []struct {
		name string
		val  any
		keys []string
	}{
		{"refund_requested", AfterSaleRefundRequested{AfterSaleNo: "AS", OrderNo: "O", GroupNo: "G2"}, []string{"after_sale_no", "order_no", "group_no"}},
		{"aftersale_completed", AfterSaleCompleted{AfterSaleNo: "AS", OrderNo: "O", GroupNo: "G2"}, []string{"after_sale_no", "order_no", "group_no"}},
		{"payment_refunded", PaymentRefunded{PaymentNo: "P", OrderNo: "O", RefundNo: "R"}, []string{"payment_no", "order_no", "refund_no"}},
	}
	for _, tc := range cases {
		raw, err := json.Marshal(tc.val)
		if err != nil {
			t.Fatalf("%s marshal: %v", tc.name, err)
		}
		var got map[string]json.RawMessage
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("%s unmarshal: %v", tc.name, err)
		}
		for _, key := range tc.keys {
			if _, ok := got[key]; !ok {
				t.Fatalf("%s missing %s in %s", tc.name, key, raw)
			}
		}
	}
}

func TestRegistryCoversKnownEventTypes(t *testing.T) {
	known := []string{
		"order.created", "order.cancelled", "order.paid", "order.allocated",
		"order.shipped", "order.completed", "order.refund_requested", "order.refund_projected",
		"inventory.compensation_requested", "inventory.confirm_requested",
		"inventory.reserved", "inventory.confirmed", "inventory.released",
		"inventory.stock_changed", "inventory.adjusted",
		"inventory.operation_completed", "inventory.operation_failed", "inventory.operation_item_failed",
		"payment.succeeded", "payment.failed", "payment.refunded",
		"shipment.created", "shipment.shipped", "shipment.delivered",
		"aftersale.created", "aftersale.approved", "aftersale.rejected", "aftersale.returning",
		"aftersale.received", "aftersale.restock_pending", "aftersale.restocked",
		"aftersale.restock_failed", "aftersale.refund_requested", "aftersale.completed",
		"coupon.locked", "coupon.unlocked", "coupon.used",
		"cart.item_added", "cart.item_updated", "cart.item_removed", "cart.selected_cleared",
		"product.created", "product.updated", "product.deleted", "product.status_changed",
		"sku.created", "sku.updated",
	}
	for _, eventType := range known {
		if _, ok := schemas[eventType]; !ok {
			t.Fatalf("event type %q has no registered schema", eventType)
		}
	}
}

func TestValidateJSONFailsClosed(t *testing.T) {
	if err := ValidateJSON("unknown.event", []byte(`{}`)); err == nil {
		t.Fatal("unknown event type accepted")
	}
	if err := ValidateJSON("order.paid", []byte(`{"order_no":"O"}`)); err == nil {
		t.Fatal("missing required fields accepted")
	}
	if err := ValidateJSON("test.created", []byte(`{"ok":true}`)); err != nil {
		t.Fatal(err)
	}
}
