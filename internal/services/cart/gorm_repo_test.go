package cart

import (
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
)

func TestCartGormRows(t *testing.T) {
	items := []*cartItemRow{{SkuID: 1, Name: "K1", PriceCent: 100, Quantity: 2, Selected: true}}
	converted := make([]*pb.CartItem, 0, len(items))
	for _, item := range items {
		converted = append(converted, &pb.CartItem{SkuId: item.SkuID, Name: item.Name, PriceCent: item.PriceCent, Quantity: item.Quantity, Selected: item.Selected})
	}
	if totalCent(converted) != 200 {
		t.Fatalf("unexpected total: %d", totalCent(converted))
	}
}
