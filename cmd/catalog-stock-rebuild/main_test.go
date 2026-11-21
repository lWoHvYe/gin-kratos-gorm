package main

import "testing"

func TestDecideProjectionNeverMovesBackwards(t *testing.T) {
	cases := []struct {
		name             string
		catalogStock     int64
		catalogVersion   int64
		inventoryStock   int64
		inventoryVersion int64
		want             projectionAction
		wantConflict     bool
	}{
		{"equal", 5, 3, 5, 3, projectionSkip, false},
		{"inventory newer", 5, 3, 7, 4, projectionUpdate, false},
		{"catalog newer is stale", 7, 5, 5, 3, projectionStale, false},
		{"same version different stock", 5, 3, 6, 3, projectionUpdate, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, conflict := decideProjection(tc.catalogStock, tc.catalogVersion, tc.inventoryStock, tc.inventoryVersion)
			if got != tc.want || conflict != tc.wantConflict {
				t.Fatalf("decideProjection = (%v,%v), want (%v,%v)", got, conflict, tc.want, tc.wantConflict)
			}
		})
	}
}
