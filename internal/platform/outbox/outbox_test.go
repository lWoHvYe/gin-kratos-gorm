package outbox

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestEncodePayloadAcceptsRawJSON(t *testing.T) {
	for _, in := range []any{
		[]byte(`{"order_no":"ORD1"}`),
		json.RawMessage(`{"order_no":"ORD1"}`),
		map[string]any{"order_no": "ORD1"},
	} {
		got, err := encodePayload(in)
		if err != nil {
			t.Fatalf("encodePayload(%T): %v", in, err)
		}
		if string(got) != `{"order_no":"ORD1"}` {
			t.Fatalf("encodePayload(%T) = %s", in, got)
		}
	}
}

func TestEncodePayloadRejectsInvalidRawJSON(t *testing.T) {
	if _, err := encodePayload([]byte(`not json`)); err == nil {
		t.Fatal("expected error for invalid raw payload")
	}
}

func TestBackoffIsCapped(t *testing.T) {
	cases := []struct {
		attempts int
		want     time.Duration
	}{
		{0, 2 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{4, 16 * time.Second},
		{8, 256 * time.Second},
		{20, 256 * time.Second},
	}
	for _, tc := range cases {
		if got := backoff(tc.attempts); got != tc.want {
			t.Fatalf("backoff(%d) = %s, want %s", tc.attempts, got, tc.want)
		}
	}
}

func TestTruncateErrorFitsColumn(t *testing.T) {
	long := errors.New(strings.Repeat("x", 2000))
	if got := truncateError(long); len(got) != 1024 {
		t.Fatalf("len = %d, want 1024", len(got))
	}
	short := errors.New("boom")
	if got := truncateError(short); got != "boom" {
		t.Fatalf("short error = %q", got)
	}
}
