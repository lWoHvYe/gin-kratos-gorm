package outbox

import (
	"encoding/json"
	"testing"
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
