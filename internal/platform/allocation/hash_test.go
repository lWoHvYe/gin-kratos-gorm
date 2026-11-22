package allocation

import "testing"

// AC-38/B-08: allocation:v1 golden vector shared by Pricing and Order.
func TestAllocationHashV1Golden(t *testing.T) {
	lines := []Line{{
		LineNo: 1, SkuID: 10, WarehouseID: 1, GroupNo: "G1", Quantity: 2,
		PriceCent: 1000, DiscountCent: 100, LinePaidCent: 1900, UnitPaidCent: 950,
	}}
	groups := []Group{{
		GroupNo: "G1", WarehouseID: 1, GoodsCent: 2000, DiscountCent: 100, FreightShareCent: 1000,
	}}
	const want = "853669d236f45b55fea56ac1276390774ae2532fc6f2239d94f378d906627dfd"
	got := Hash(lines, groups)
	if got == "" {
		t.Fatal("empty hash")
	}
	if got != want {
		t.Fatalf("hash = %s, want %s", got, want)
	}
}
