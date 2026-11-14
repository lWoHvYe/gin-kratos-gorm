package consumer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"gin-kratos-gorm/internal/platform/mq"

	"github.com/nats-io/nats.go"
)

func TestConsumerDeadLettersPermanentFailure(t *testing.T) {
	url := os.Getenv("TEST_NATS_URL")
	if url == "" {
		t.Skip("set TEST_NATS_URL to run the consumer integration test")
	}
	client, err := mq.Connect(url)
	if err != nil {
		t.Fatalf("connect nats: %v", err)
	}
	defer client.Close()

	subject := fmt.Sprintf("shop.test.consumer.%d", time.Now().UnixNano())
	dlq := subject + ".dlq"
	sub, err := client.Conn.SubscribeSync(dlq)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Unsubscribe() }()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := Config{Subject: subject, Durable: "test-dlq-" + fmt.Sprint(time.Now().UnixNano()), MaxDeliver: 2}
	if err := Subscribe(ctx, client, logger, cfg, func(context.Context, *nats.Msg) error {
		return Permanent(errors.New("poison payload"))
	}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	if err := client.Publish(context.Background(), subject, []byte(`{"bad":true}`)); err != nil {
		t.Fatal(err)
	}
	msg, err := sub.NextMsg(10 * time.Second)
	if err != nil {
		t.Fatalf("dead-letter message not received: %v", err)
	}
	if len(msg.Data) == 0 {
		t.Fatal("DLQ payload is empty")
	}
}
