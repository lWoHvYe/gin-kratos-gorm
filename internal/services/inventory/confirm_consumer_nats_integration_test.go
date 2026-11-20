package inventory

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"gin-kratos-gorm/internal/platform/mq"
)

func TestConfirmConsumerNATS(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	natsURL := os.Getenv("TEST_NATS_URL")
	if dsn == "" || natsURL == "" {
		t.Skip("set TEST_MYSQL_DSN and TEST_NATS_URL")
	}
	db := seckillTestDB(t)
	if _, err := db.Exec(`INSERT INTO inventory_stocks (sku_id, warehouse_id, on_hand, reserved, version) VALUES (1,1,10,1,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO stock_reservations
		(reservation_no, order_no, sku_id, warehouse_id, quantity, status, expires_at, created_at, updated_at)
		VALUES ('RES-NATS','ORD-NATS',1,1,1,'HELD',DATE_ADD(NOW(3), INTERVAL 1 HOUR),NOW(3),NOW(3))`); err != nil {
		t.Fatal(err)
	}
	client, err := mq.Connect(natsURL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go RunConfirmConsumer(ctx, client, &Service{db: db, log: logger}, logger)
	time.Sleep(500 * time.Millisecond)
	payload := []byte(`{"order_no":"ORD-NATS","items":[{"sku_id":1,"quantity":1}]}`)
	if err := client.Publish(context.Background(), "shop.inventory.confirm.requested.v1", payload); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var status string
		if err := db.QueryRow(`SELECT status FROM stock_reservations WHERE order_no='ORD-NATS'`).Scan(&status); err == nil && status == statusConfirmed {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("confirm consumer did not confirm the reservation")
}
