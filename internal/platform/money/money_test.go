package money

import (
	"math"
	"testing"
)

func TestAllocateProportionalConservesTotal(t *testing.T) {
	cases := []struct {
		name    string
		total   int64
		weights []int64
		want    []int64
	}{
		{name: "exact", total: 100, weights: []int64{1, 1}, want: []int64{50, 50}},
		{name: "largest remainder", total: 100, weights: []int64{1, 2}, want: []int64{33, 67}},
		{name: "zero weights included", total: 10, weights: []int64{0, 3, 7}, want: []int64{0, 3, 7}},
		{name: "all zero", total: 9, weights: []int64{0, 0}, want: []int64{0, 9}},
		{name: "negative treated as zero", total: 5, weights: []int64{-10, 5}, want: []int64{0, 5}},
		{name: "empty", total: 0, weights: nil, want: nil},
		{name: "zero total", total: 0, weights: []int64{1, 1}, want: []int64{0, 0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := AllocateProportional(tc.total, tc.weights)
			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("allocation = %v, want %v", got, tc.want)
				}
			}
			var sum int64
			for _, value := range got {
				sum += value
			}
			if sum != tc.total {
				t.Fatalf("sum = %d, want %d", sum, tc.total)
			}
		})
	}
}

func TestAllocateProportionalLargeValuesDoNotOverflow(t *testing.T) {
	total := int64(math.MaxInt64)
	got := AllocateProportional(total, []int64{math.MaxInt64 / 4, math.MaxInt64 / 4})
	var sum int64
	for _, value := range got {
		if value < 0 {
			t.Fatalf("negative allocation: %v", got)
		}
		sum += value
	}
	if sum != total {
		t.Fatalf("sum = %d, want %d", sum, total)
	}
}

func TestAllocateProportionalStableOrder(t *testing.T) {
	first := AllocateProportional(7, []int64{1, 1, 1})
	for i := 0; i < 20; i++ {
		next := AllocateProportional(7, []int64{1, 1, 1})
		for j := range first {
			if next[j] != first[j] {
				t.Fatalf("allocation changed: %v -> %v", first, next)
			}
		}
	}
}
