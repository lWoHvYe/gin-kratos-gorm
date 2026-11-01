package cart

import (
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
)

func TestTotalCent(t *testing.T) {
	tests := []struct {
		name  string
		items []*pb.CartItem
		want  int64
	}{
		{name: "empty", items: nil, want: 0},
		{
			name: "selected only",
			items: []*pb.CartItem{
				{SkuId: 1, PriceCent: 1999, Quantity: 2, Selected: true},
				{SkuId: 2, PriceCent: 500, Quantity: 3, Selected: false},
				{SkuId: 3, PriceCent: 100, Quantity: 1, Selected: true},
			},
			want: 4098,
		},
		{
			name: "all selected",
			items: []*pb.CartItem{
				{SkuId: 1, PriceCent: 99, Quantity: 3, Selected: true},
				{SkuId: 2, PriceCent: 1, Quantity: 1, Selected: true},
			},
			want: 298,
		},
		{
			name:  "no selected items",
			items: []*pb.CartItem{{SkuId: 1, PriceCent: 100, Quantity: 1, Selected: false}},
			want:  0,
		},
		{
			name: "ignores non-positive quantity and negative price",
			items: []*pb.CartItem{
				{SkuId: 1, PriceCent: 100, Quantity: 0, Selected: true},
				{SkuId: 2, PriceCent: 100, Quantity: -1, Selected: true},
				{SkuId: 3, PriceCent: -5, Quantity: 1, Selected: true},
				{SkuId: 4, PriceCent: 0, Quantity: 5, Selected: true},
			},
			want: 0,
		},
		{
			name:  "nil item is skipped",
			items: []*pb.CartItem{nil, {SkuId: 1, PriceCent: 250, Quantity: 2, Selected: true}},
			want:  500,
		},
		{
			name:  "large values stay in int64",
			items: []*pb.CartItem{{SkuId: 1, PriceCent: 1_000_000_000, Quantity: 9999, Selected: true}},
			want:  9_999_000_000_000,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := totalCent(tc.items); got != tc.want {
				t.Fatalf("totalCent() = %d, want %d", got, tc.want)
			}
		})
	}
}
