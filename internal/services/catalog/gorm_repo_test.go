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

func TestGormSPUToPBIncludesMediaAndAttributes(t *testing.T) {
	row := gormSPU{
		ID: 1, Name: "Widget", Status: "ON_SALE",
		Skus:       []gormSKU{{ID: 7, SpuID: 1, PriceCent: 1200, Status: "ENABLED"}},
		Images:     []gormImage{{ID: 2, SpuID: 1, URL: "https://example.invalid/a.png", Sort: 1}},
		Attributes: []gormAttribute{{ID: 3, SpuID: 1, Name: "color", Value: "red", Sort: 1}},
	}
	p := row.toPB()
	if p.GetMinPriceCent() != 1200 || len(p.GetImages()) != 1 || len(p.GetAttributes()) != 1 {
		t.Fatalf("unexpected product: %+v", p)
	}
	if p.GetImages()[0].GetUrl() == "" || p.GetAttributes()[0].GetName() != "color" {
		t.Fatalf("media not mapped: %+v", p)
	}
}
