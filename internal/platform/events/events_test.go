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
		"order.created", "order.cancelled", "order.paid", "order.settled", "order.refund_admitted", "order.allocated",
		"order.shipped", "order.completed", "order.refund_requested", "order.refund_projected",
		"inventory.compensation_requested", "inventory.confirm_requested",
		"inventory.refund_release_requested",
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

// T1.6 / C-04: order.refund_requested keeps accepting legacy payloads while
// opting into the payment-level cancel fields. The fields are optional so an
// old producer/consumer cannot be broken by the new command carrier.
func TestOrderRefundRequestedPaymentCancelFieldsAreOptional(t *testing.T) {
	legacy := []byte(`{"order_no":"O","payment_no":"P","status":"CANCELLED","reason":"order cancelled after inventory confirm","paid_cent":500}`)
	if err := ValidateJSON("order.refund_requested", legacy); err != nil {
		t.Fatalf("legacy order.refund_requested rejected: %v", err)
	}
	command := []byte(`{"order_no":"O","payment_no":"P","status":"CANCELLED","refund_id":"pay:P:cancel","source":"payment","non_effective_capture":true,"amount_cent":500,"paid_cent":500}`)
	if err := ValidateJSON("order.refund_requested", command); err != nil {
		t.Fatalf("payment-level order.refund_requested rejected: %v", err)
	}
	// The identity fields are still optional at the registry; the consumer is
	// the component that decides command vs observe-only.
	withoutCommandFields := []byte(`{"order_no":"O","payment_no":"P","status":"CANCELLED"}`)
	if err := ValidateJSON("order.refund_requested", withoutCommandFields); err != nil {
		t.Fatalf("minimal legacy order.refund_requested rejected: %v", err)
	}
}
