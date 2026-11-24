package events

import (
	"encoding/json"
	"testing"
)

func refundReleasePayload() map[string]any {
	return map[string]any{
		"schema_version": 1,
		"release_scope":  RefundReleaseScopeOrderReservationFull,
		"release_id":     "rr:AS-1",
		"refund_id":      "AS-1",
		"order_no":       "ORD-1",
		"group_no":       "G1",
		"source":         RefundReleaseSourceAfterSale,
		"reason":         RefundReleaseReasonAfterSaleRefund,
		"items": []any{
			map[string]any{"sku_id": 1, "quantity": 2, "warehouse_id": 1},
			map[string]any{"sku_id": 2, "quantity": 1, "warehouse_id": 3},
		},
		"requested_at": "2026-11-25T00:00:00Z",
	}
}

func marshalPayload(t *testing.T, payload map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestRefundReleaseContractAcceptsValidEvent(t *testing.T) {
	if err := ValidateJSON(RefundReleaseEventType, marshalPayload(t, refundReleasePayload())); err != nil {
		t.Fatalf("valid refund release event rejected: %v", err)
	}
}

func TestRefundReleaseContractRejectsIncompleteOrIncoherentEvents(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"missing group_no", func(p map[string]any) { delete(p, "group_no") }},
		{"missing warehouse_id", func(p map[string]any) {
			p["items"] = []any{map[string]any{"sku_id": 1, "quantity": 1}}
		}},
		{"zero warehouse_id", func(p map[string]any) {
			p["items"] = []any{map[string]any{"sku_id": 1, "quantity": 1, "warehouse_id": 0}}
		}},
		{"missing quantity", func(p map[string]any) {
			p["items"] = []any{map[string]any{"sku_id": 1, "warehouse_id": 1}}
		}},
		{"empty items", func(p map[string]any) { p["items"] = []any{} }},
		{"release_id mismatch", func(p map[string]any) { p["release_id"] = "rr:AS-2" }},
		{"missing refund_id", func(p map[string]any) { delete(p, "refund_id") }},
		{"schema_version 2", func(p map[string]any) { p["schema_version"] = 2 }},
		{"missing release_scope", func(p map[string]any) { delete(p, "release_scope") }},
		{"wrong release_scope", func(p map[string]any) { p["release_scope"] = "group_reservation" }},
		{"empty covered group", func(p map[string]any) { p["covered_group_nos"] = []any{"G1", ""} }},
		{"duplicate covered group", func(p map[string]any) { p["covered_group_nos"] = []any{"G1", "G1"} }},
		{"unknown source", func(p map[string]any) { p["source"] = "payment" }},
		{"reason does not match source", func(p map[string]any) {
			p["source"] = RefundReleaseSourceTCC
		}},
		{"bad requested_at", func(p map[string]any) { p["requested_at"] = "yesterday" }},
		{"duplicate sku and warehouse", func(p map[string]any) {
			p["items"] = []any{
				map[string]any{"sku_id": 1, "quantity": 1, "warehouse_id": 1},
				map[string]any{"sku_id": 1, "quantity": 2, "warehouse_id": 1},
			}
		}},
		{"quantity is not a number", func(p map[string]any) {
			p["items"] = []any{map[string]any{"sku_id": 1, "quantity": "2", "warehouse_id": 1}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := refundReleasePayload()
			tc.mutate(payload)
			if err := ValidateJSON(RefundReleaseEventType, marshalPayload(t, payload)); err == nil {
				t.Fatalf("invalid refund release event accepted: %s", marshalPayload(t, payload))
			}
		})
	}
}

func TestRefundReleaseSourceReasonPairs(t *testing.T) {
	pairs := map[string]string{
		RefundReleaseSourceTCC:            RefundReleaseReasonTCCCompensation,
		RefundReleaseSourceSystemZero:     RefundReleaseReasonZeroAfterSale,
		RefundReleaseSourceSystemShortage: RefundReleaseReasonShortageRefund,
		RefundReleaseSourceAfterSale:      RefundReleaseReasonAfterSaleRefund,
	}
	for source, reason := range pairs {
		payload := refundReleasePayload()
		payload["source"] = source
		payload["reason"] = reason
		if err := ValidateJSON(RefundReleaseEventType, marshalPayload(t, payload)); err != nil {
			t.Fatalf("source %s / reason %s rejected: %v", source, reason, err)
		}
	}
}

func TestRefundReleaseIDIsDeterministic(t *testing.T) {
	if got := RefundReleaseID("tcc:PAY-1:G2"); got != "rr:tcc:PAY-1:G2" {
		t.Fatalf("RefundReleaseID = %q", got)
	}
}
