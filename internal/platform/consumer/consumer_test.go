package consumer

import (
	"context"
	"errors"
	"testing"

	"github.com/nats-io/nats.go"
)

func TestPermanentErrorsAreNotRetried(t *testing.T) {
	err := Permanent(errors.New("bad payload"))
	if !isPermanent(err) {
		t.Fatal("Permanent error must be detected")
	}
	if isPermanent(errors.New("transient")) {
		t.Fatal("plain error must stay retryable")
	}
	if Permanent(nil) != nil {
		t.Fatal("Permanent(nil) must stay nil")
	}
}

func TestRunRecoversPanics(t *testing.T) {
	err := run(context.Background(), func(context.Context, *nats.Msg) error {
		panic("boom")
	}, &nats.Msg{})
	if err == nil {
		t.Fatal("panic must surface as an error")
	}
}
