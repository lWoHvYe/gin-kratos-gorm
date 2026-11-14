package id

import (
	"strings"
	"testing"
)

func TestOrderForSeckillIsDeterministic(t *testing.T) {
	first := OrderForSeckill("token-hash-1")
	second := OrderForSeckill("token-hash-1")
	if first != second {
		t.Fatalf("same token produced different order numbers: %q vs %q", first, second)
	}
	if !strings.HasPrefix(first, "ORD") {
		t.Fatalf("order number %q lacks the ORD prefix", first)
	}
	if len(first) > 64 {
		t.Fatalf("order number %q exceeds the column width", first)
	}
	if other := OrderForSeckill("token-hash-2"); other == first {
		t.Fatal("different tokens must not share an order number")
	}
}
