package catalog

import "testing"

func TestGormSPUToPB(t *testing.T) {
	row := gormSPU{
		ID: 9, Name: "Keyboard", Status: "ON_SALE",
		Skus: []gormSKU{
			{ID: 1, SpuID: 9, SkuCode: "K1", PriceCent: 39900, Status: "ENABLED"},
			{ID: 2, SpuID: 9, SkuCode: "K2", PriceCent: 29900, Status: "ENABLED"},
		},
	}
	p := row.toPB()
	if p.GetId() != 9 || len(p.GetSkus()) != 2 || p.GetMinPriceCent() != 29900 {
		t.Fatalf("unexpected product: %+v", p)
	}
	if p.GetSkus()[0].GetSkuCode() != "K1" || p.GetSkus()[1].GetPriceCent() != 29900 {
		t.Fatalf("unexpected skus: %+v", p.GetSkus())
	}
}
