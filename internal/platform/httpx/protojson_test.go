package httpx

import (
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
)

func TestProtoToMapEmitsZeroValues(t *testing.T) {
	msg := &pb.Quote{
		QuoteId:   "Q1",
		TotalCent: 10_000,
		// DiscountCent, FreightCent, promotion/coupon discounts stay zero.
		Lines: []*pb.QuoteLine{{SkuId: 1, PriceCent: 1_000, Quantity: 2}},
	}
	out, ok := ProtoToMap(msg).(map[string]any)
	if !ok {
		t.Fatalf("expected a map, got %T", ProtoToMap(msg))
	}
	cases := map[string]any{
		"discount_cent":           int64(0),
		"promotion_discount_cent": int64(0),
		"coupon_discount_cent":    int64(0),
		"freight_cent":            int64(0),
		"coupon_id":               "",
		"applied_promotions_json": "",
	}
	for field, want := range cases {
		got, present := out[field]
		if !present {
			t.Fatalf("field %q missing from response: %v", field, out)
		}
		if got != want {
			t.Fatalf("field %q = %#v, want %#v", field, got, want)
		}
	}
	if out["total_cent"] != int64(10_000) {
		t.Fatalf("int64 should stay numeric, got %#v", out["total_cent"])
	}
	lines, ok := out["lines"].([]any)
	if !ok || len(lines) != 1 {
		t.Fatalf("lines = %#v", out["lines"])
	}
	line := lines[0].(map[string]any)
	if line["discount_cent"] != int64(0) {
		t.Fatalf("nested zero value missing: %#v", line)
	}
}

func TestProtoToMapEmitsEmptyCollectionsAndStrings(t *testing.T) {
	out := ProtoToMap(&pb.Order{OrderNo: "ORD1"}).(map[string]any)
	if items, ok := out["items"].([]any); !ok || len(items) != 0 {
		t.Fatalf("items = %#v, want empty list", out["items"])
	}
	if out["refund_status"] != "" {
		t.Fatalf("refund_status = %#v, want empty string", out["refund_status"])
	}
	if out["paid_cent"] != int64(0) || out["refunded_cent"] != int64(0) {
		t.Fatalf("money zeroes missing: %#v", out)
	}
}

func TestProtoToMapNilMessage(t *testing.T) {
	if got := ProtoToMap(nil); got != nil {
		t.Fatalf("nil message = %#v", got)
	}
}
