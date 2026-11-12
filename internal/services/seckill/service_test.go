package seckill

import "testing"

func TestValidateFields(t *testing.T) {
	if err := validateFields(1, 2, 100, 10, 1, 100, 200); err != nil {
		t.Fatalf("valid activity rejected: %v", err)
	}
	cases := []struct {
		name string
		err  error
	}{
		{"campaign", validateFields(0, 2, 100, 10, 1, 100, 200)},
		{"sku", validateFields(1, 0, 100, 10, 1, 100, 200)},
		{"price", validateFields(1, 2, 0, 10, 1, 100, 200)},
		{"stock", validateFields(1, 2, 100, 0, 1, 100, 200)},
		{"limit", validateFields(1, 2, 100, 10, 0, 100, 200)},
		{"window", validateFields(1, 2, 100, 10, 1, 200, 200)},
	}
	for _, tc := range cases {
		if tc.err == nil {
			t.Fatalf("%s: expected validation error", tc.name)
		}
	}
}

func TestWarmupKeysUseClusterHashTag(t *testing.T) {
	meta, stock := warmupKeys(42)
	if meta != "seckill:{42}:meta" || stock != "seckill:{42}:stock" {
		t.Fatalf("unexpected keys: %q %q", meta, stock)
	}
}
