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
