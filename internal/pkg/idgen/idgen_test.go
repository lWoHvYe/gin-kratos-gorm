package idgen

import (
	"testing"
	"time"
)

func TestOrderSNSortableAndUnique(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	g := NewOrderSN("node-a").WithClock(func() time.Time { return now })
	first := g.Next()
	second := g.Next()
	if first == second {
		t.Fatal("expected unique order numbers")
	}
	if len(first) != 23 {
		t.Fatalf("unexpected length %d: %s", len(first), first)
	}
	if first[:14] != "20260102030405" {
		t.Fatalf("unexpected timestamp prefix: %s", first)
	}
	if second <= first {
		t.Fatalf("order numbers should be sortable: %s <= %s", second, first)
	}
}

func TestEventID(t *testing.T) {
	a, b := EventID(), EventID()
	if len(a) != 32 || len(b) != 32 {
		t.Fatalf("unexpected event id lengths: %d %d", len(a), len(b))
	}
	if a == b {
		t.Fatal("expected random event ids")
	}
}
