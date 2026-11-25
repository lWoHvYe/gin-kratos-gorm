package inventory

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"gin-kratos-gorm/internal/platform/consumer"
	"gin-kratos-gorm/internal/platform/mq"

	"github.com/nats-io/nats.go"
)

func TestOrderCancelledConsumerNATS(t *testing.T) {
	if os.Getenv("TEST_MYSQL_DSN") == "" || os.Getenv("TEST_NATS_URL") == "" {
		t.Skip("set TEST_MYSQL_DSN and TEST_NATS_URL")
	}
	db := seckillTestDB(t)
	if _, err := db.Exec(`INSERT INTO inventory_stocks (sku_id, warehouse_id, on_hand, reserved, version) VALUES (1,1,10,0,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO stock_reservations
		(reservation_no, order_no, sku_id, warehouse_id, quantity, status, expires_at, created_at, updated_at)
		VALUES ('RES-CANCEL-NATS','ORD-CANCEL-NATS',1,1,1,'CONFIRMED',DATE_ADD(NOW(3), INTERVAL 1 HOUR),NOW(3),NOW(3))`); err != nil {
		t.Fatal(err)
	}
	client, err := mq.Connect(os.Getenv("TEST_NATS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	// See the confirm consumer test: drop the durable's stale backlog first.
	_ = client.JS.DeleteConsumer("MALL", "inventory-order-cancelled")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc := &Service{db: db, log: logger}
	go consumer.Subscribe(ctx, client, logger, consumer.Config{
		Subject: "shop.order.cancelled.v1", Durable: "inventory-order-cancelled-test",
	}, func(hctx context.Context, msg *nats.Msg) error {
		return handleOrderCancelled(hctx, svc, msg.Data)
	})
	time.Sleep(500 * time.Millisecond)
	if err := client.Publish(context.Background(), "shop.order.cancelled.v1",
		[]byte(`{"order_no":"ORD-CANCEL-NATS","items":[{"sku_id":1,"warehouse_id":1,"quantity":1}]}`)); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var status string
		var onHand int64
		if err := db.QueryRow(`SELECT status FROM stock_reservations WHERE order_no='ORD-CANCEL-NATS'`).Scan(&status); err == nil && status == statusReleased {
			_ = db.QueryRow(`SELECT on_hand FROM inventory_stocks WHERE sku_id=1`).Scan(&onHand)
			if onHand == 11 {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("order.cancelled did not restock the confirmed reservation")
}
