package mq

import "testing"

func TestStreamReplicas(t *testing.T) {
	cases := []struct {
		env  string
		want int
	}{
		{"", 1},
		{"3", 3},
		{"0", 1},
		{"-2", 1},
		{"three", 1},
	}
	for _, tc := range cases {
		t.Setenv("NATS_STREAM_REPLICAS", tc.env)
		if got := streamReplicas(); got != tc.want {
			t.Fatalf("NATS_STREAM_REPLICAS=%q -> %d, want %d", tc.env, got, tc.want)
		}
	}
}
