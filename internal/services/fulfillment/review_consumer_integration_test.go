package fulfillment

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
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

// T7-R1 (F7b) partial evidence: build a REAL placeholder through the normal
// order.paid consumer path (not a fixture INSERT), then prove that a persisted
// REVIEW decision (and a non-PRE event) never cancels/recreates it. The Order
// side re-probe (PENDING_REMOTE -> CANCELLED -> PRE) needs the Order service and
// its own DB, so the cross-service half is recorded as [BLOCKED] in the
// selfcheck rather than faked here.
func TestOrderPaidCreatesRealPlaceholderThenReviewNoop(t *testing.T) {
	if os.Getenv("TEST_MYSQL_DSN") == "" || os.Getenv("TEST_NATS_URL") == "" {
		t.Skip("set TEST_MYSQL_DSN and TEST_NATS_URL")
	}
	db := fulfillmentTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	// Unique order per run: the JetStream history is replayed when a durable is
	// recreated, so a reused id would see an older PRE/POST and taint this case.
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	orderNo := "ORD-LATE-" + suffix
	refundID := "R-LATE-" + suffix
	orderClient := &fakePermitOrder{order: &pb.Order{
		OrderNo: orderNo, UserId: 1, Status: "PAID", PaidCent: 1000,
		Items:             []*pb.OrderItem{{SkuId: 1, Quantity: 1, GroupNo: "G1"}},
		FulfillmentGroups: []*pb.OrderFulfillmentGroup{{GroupNo: "G1", WarehouseId: 1}},
		GroupActions:      []*pb.OrderGroupAction{{GroupNo: "G1", Action: "ship"}},
	}}
	svc := New(db, nil, logger, orderClient)
	client, err := mq.Connect(os.Getenv("TEST_NATS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	_ = client.JS.DeleteConsumer("MALL", "fulfillment-order-paid")
	_ = client.JS.DeleteConsumer("MALL", "fulfillment-refund-admitted")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	orderCtx, orderCancel := context.WithCancel(ctx)
	defer orderCancel()
	go RunOrderPaidConsumer(orderCtx, client, svc, logger)
	time.Sleep(500 * time.Millisecond)

	publish := func(subject, payload string) {
		t.Helper()
		if err := client.Publish(ctx, subject, []byte(payload)); err != nil {
			t.Fatal(err)
		}
	}
	shipmentStatus := func() (string, bool) {
		t.Helper()
		var status string
		err := db.QueryRow(`SELECT status FROM shipments WHERE order_no = ?`, orderNo).Scan(&status)
		if err != nil {
			return "", false
		}
		return status, true
	}

	// Publish repeatedly: JetStream durable creation can race the first publish
	// on a cold broker; duplicate order.paid events are idempotent (CreateShipment
	// returns the existing CREATED row).
	deadline := time.Now().Add(10 * time.Second)
	created := false
	for time.Now().Before(deadline) {
		publish("shop.order.paid.v1", `{"order_no":"`+orderNo+`","items":[{"sku_id":1,"quantity":1}]}`)
		for i := 0; i < 10; i++ {
			if status, ok := shipmentStatus(); ok && status == statusCreated {
				created = true
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if created {
			break
		}
	}
	if !created {
		t.Fatal("order.paid consumer did not create the real CREATED placeholder")
	}
	// Stop the order.paid consumer before starting the refund consumer: the two
	// subscriptions are independent in production, but keeping one broker
	// subscription active at a time makes this test deterministic.
	orderCancel()
	_ = client.JS.DeleteConsumer("MALL", "fulfillment-order-paid")
	time.Sleep(200 * time.Millisecond)
	_ = client.JS.DeleteConsumer("MALL", "fulfillment-refund-admitted")
	go RunRefundAdmittedConsumer(ctx, client, svc, logger)
	time.Sleep(500 * time.Millisecond)

	// REVIEW_REQUIRED is not an event Order emits for a review, and even a
	// hand-crafted one must not cancel the real placeholder.
	publish("shop.order.refund_admitted.v1",
		`{"refund_id":"`+refundID+`","order_no":"`+orderNo+`","group_no":"G1","result":"REVIEW_REQUIRED","state":"PENDING_REMOTE"}`)
	time.Sleep(500 * time.Millisecond)
	if status, _ := shipmentStatus(); status != statusCreated {
		t.Fatalf("REVIEW event mutated the real placeholder: %s", status)
	}

	// A real terminal PRE (what an audited re-arm can later produce) cancels
	// exactly this real CREATED row.
	publish("shop.order.refund_admitted.v1",
		`{"refund_id":"`+refundID+`","order_no":"`+orderNo+`","group_no":"G1","result":"PRE_SHIP_REVOKED","state":"PRE_SHIP_REVOKED"}`)
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if status, _ := shipmentStatus(); status == statusCancelled {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	status, _ := shipmentStatus()
	t.Fatalf("real placeholder was not cancelled by PRE: %s", status)
}
