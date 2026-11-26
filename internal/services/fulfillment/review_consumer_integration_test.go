package fulfillment

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"gin-kratos-gorm/internal/platform/mq"

	"github.com/nats-io/nats.go"
)

// T6b/T6c (F7/O14/F7b, W9): the refund-admitted consumer only acts on a
// terminal PRE result. Duplicate PRE delivery cancels once; a persisted
// REVIEW_REQUIRED decision is never emitted as an event, and even a hand-crafted
// REVIEW/POST event is a no-op (no cancel, no placeholder recreation). This is
// the broker-level complement to the order-side no-auto-PRE test.
func TestRefundAdmittedConsumerReviewAndDuplicate(t *testing.T) {
	if os.Getenv("TEST_MYSQL_DSN") == "" || os.Getenv("TEST_NATS_URL") == "" {
		t.Skip("set TEST_MYSQL_DSN and TEST_NATS_URL")
	}
	db := fulfillmentTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := New(db, nil, logger, nil)
	if _, err := db.Exec(`INSERT INTO shipments (shipment_no, order_no, fulfillment_group_no, status)
		VALUES ('SHIP-REV1','ORD-REV1','G1','CREATED'), ('SHIP-REV2','ORD-REV2','G1','CREATED')`); err != nil {
		t.Fatal(err)
	}
	client, err := mq.Connect(os.Getenv("TEST_NATS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	_ = client.JS.DeleteConsumer("MALL", "fulfillment-refund-admitted")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var dlqCount atomic.Int64
	var dlqPayload atomic.Value
	if _, err := client.QueueSubscribe("shop.order.refund_admitted.v1.dlq", "", func(msg *nats.Msg) {
		dlqCount.Add(1)
		dlqPayload.Store(append([]byte(nil), msg.Data...))
	}); err != nil {
		t.Fatal(err)
	}
	go RunRefundAdmittedConsumer(ctx, client, svc, logger)
	time.Sleep(500 * time.Millisecond)

	publish := func(payload string) {
		t.Helper()
		if err := client.Publish(ctx, "shop.order.refund_admitted.v1", []byte(payload)); err != nil {
			t.Fatal(err)
		}
	}
	shipmentStatus := func(no string) string {
		t.Helper()
		var status string
		if err := db.QueryRow(`SELECT status FROM shipments WHERE shipment_no = ?`, no).Scan(&status); err != nil {
			t.Fatal(err)
		}
		return status
	}
	waitStatus := func(no, want string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if shipmentStatus(no) == want {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatalf("shipment %s = %s, want %s", no, shipmentStatus(no), want)
	}

	pre := `{"refund_id":"R-REV1","order_no":"ORD-REV1","group_no":"G1","result":"PRE_SHIP_REVOKED","state":"PRE_SHIP_REVOKED"}`
	publish(pre)
	waitStatus("SHIP-REV1", statusCancelled)
	// Duplicate delivery is idempotent (CANCELLED -> CANCELLED, no new row).
	publish(pre)
	time.Sleep(300 * time.Millisecond)
	if got := shipmentStatus("SHIP-REV1"); got != statusCancelled {
		t.Fatalf("duplicate PRE changed status to %s", got)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM shipments WHERE order_no = 'ORD-REV1'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("duplicate PRE created %d shipment rows", count)
	}

	// A REVIEW decision is not a terminal event; a hand-crafted event must be a
	// no-op and must not create/cancel a placeholder for ORD-REV2.
	publish(`{"refund_id":"R-REV2","order_no":"ORD-REV2","group_no":"G1","result":"REVIEW_REQUIRED","state":"PENDING_REMOTE"}`)
	publish(`{"refund_id":"R-REV2","order_no":"ORD-REV2","group_no":"G1","result":"POST_SHIP_ALLOWED","state":"POST_SHIP_ALLOWED"}`)
	time.Sleep(500 * time.Millisecond)
	if got := shipmentStatus("SHIP-REV2"); got != statusCreated {
		t.Fatalf("non-PRE event mutated the placeholder: %s", got)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM shipments WHERE order_no = 'ORD-REV2'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("non-PRE event created %d rows", count)
	}

	// T6c: an unparseable delivery is permanent and lands in the DLQ; replaying
	// it from the DLQ stays a no-op (no cancel/refund side effect).
	publish(`{`)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && dlqCount.Load() == 0 {
		time.Sleep(100 * time.Millisecond)
	}
	if dlqCount.Load() < 1 {
		t.Fatalf("DLQ messages = %d, want at least 1", dlqCount.Load())
	}
	if payload, _ := dlqPayload.Load().([]byte); len(payload) > 0 {
		if err := client.Publish(ctx, "shop.order.refund_admitted.v1", payload); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(300 * time.Millisecond)
	if got := shipmentStatus("SHIP-REV2"); got != statusCreated {
		t.Fatalf("DLQ replay mutated the placeholder: %s", got)
	}
}
