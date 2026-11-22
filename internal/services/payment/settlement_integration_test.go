package payment

import (
	"context"
	"database/sql"
	"os"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"

	mysqldriver "github.com/go-sql-driver/mysql"
)

func paymentTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run payment integration tests")
	}
	cfg, err := mysqldriver.ParseDSN(dsn)
	if err != nil || cfg.DBName == "" {
		t.Fatalf("TEST_MYSQL_DSN must name a database: %v", err)
	}
	adminCfg := *cfg
	adminCfg.DBName = ""
	admin, err := sql.Open("mysql", adminCfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec("CREATE DATABASE IF NOT EXISTS `" + cfg.DBName + "` CHARACTER SET utf8mb4"); err != nil {
		t.Fatalf("create test database: %v", err)
	}
	_ = admin.Close()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`DROP TABLE IF EXISTS payment_intents`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS payment_intents (
		id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, payment_no VARCHAR(64) NOT NULL,
		order_no VARCHAR(64) NOT NULL, user_id BIGINT NOT NULL DEFAULT 0,
		amount_cent BIGINT NOT NULL, captured_cent BIGINT NOT NULL DEFAULT 0,
		refunded_cent BIGINT NOT NULL DEFAULT 0, channel VARCHAR(32) NOT NULL DEFAULT 'mock',
		payment_method VARCHAR(32) NOT NULL DEFAULT '', status VARCHAR(32) NOT NULL DEFAULT 'PENDING',
		pay_url VARCHAR(255) NOT NULL DEFAULT '', expires_at DATETIME(3) NOT NULL,
		version BIGINT NOT NULL DEFAULT 1,
		created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
		updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
		UNIQUE KEY uk_payment_intents_no (payment_no))`); err != nil {
		t.Fatalf("create payment_intents: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS refunds`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE refunds (
		id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, refund_no VARCHAR(64) NOT NULL,
		payment_no VARCHAR(64) NOT NULL, order_no VARCHAR(64) NOT NULL,
		amount_cent BIGINT NOT NULL, reason VARCHAR(255) NOT NULL DEFAULT '',
		status VARCHAR(32) NOT NULL DEFAULT 'REFUNDED', channel VARCHAR(32) NOT NULL DEFAULT '',
		idempotency_key VARCHAR(128) NOT NULL DEFAULT '', completed_at DATETIME(3) NULL,
		version BIGINT NOT NULL DEFAULT 1,
		created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
		updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
		UNIQUE KEY uk_refunds_no (refund_no), UNIQUE KEY uk_refunds_key (payment_no, idempotency_key))`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS outbox_events`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE outbox_events (
		id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, event_id VARCHAR(64) NOT NULL,
		topic VARCHAR(128) NOT NULL, aggregate_type VARCHAR(64) NOT NULL,
		aggregate_id VARCHAR(64) NOT NULL, event_type VARCHAR(64) NOT NULL,
		payload JSON NOT NULL, attempts INT NOT NULL DEFAULT 0,
		last_error VARCHAR(1024) NOT NULL DEFAULT '', next_attempt_at DATETIME(3) NULL,
		published_at DATETIME(3) NULL, dead_lettered_at DATETIME(3) NULL,
		lease_owner VARCHAR(128) NULL, lease_until DATETIME(3) NULL,
		traceparent VARCHAR(256) NULL, created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
		UNIQUE KEY uk_outbox_event_id (event_id))`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM payment_intents`); err != nil {
		t.Fatal(err)
	}
	return db
}

func seedPaymentIntent(t *testing.T, db *sql.DB, paymentNo, orderNo, status string, amount, captured, refunded int64) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO payment_intents
		(payment_no, order_no, user_id, amount_cent, captured_cent, refunded_cent, channel, status, expires_at, version)
		VALUES (?, ?, 1, ?, ?, ?, 'mock', ?, DATE_ADD(NOW(3), INTERVAL 1 HOUR), 1)`,
		paymentNo, orderNo, amount, captured, refunded, status); err != nil {
		t.Fatalf("seed payment intent: %v", err)
	}
}

func TestGetPaymentByOrderReturnsCapturedFacts(t *testing.T) {
	db := paymentTestDB(t)
	seedPaymentIntent(t, db, "PAY-1", "ORD-1", statusSucceeded, 1000, 1000, 0)
	svc := New(db, nil, "", nil)
	got, err := svc.GetPaymentByOrder(context.Background(), &pb.GetPaymentByOrderRequest{OrderNo: "ORD-1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.GetPaymentNo() != "PAY-1" || got.GetCapturedCent() != 1000 || got.GetStatus() != statusSucceeded {
		t.Fatalf("intent = %+v", got)
	}
}

func TestGetPaymentByOrderIgnoresPendingAndMissing(t *testing.T) {
	db := paymentTestDB(t)
	seedPaymentIntent(t, db, "PAY-PENDING", "ORD-PENDING", statusPending, 1000, 0, 0)
	svc := New(db, nil, "", nil)
	if _, err := svc.GetPaymentByOrder(context.Background(), &pb.GetPaymentByOrderRequest{OrderNo: "ORD-PENDING"}); !apperr.IsCode(err, apperr.CodeNotFound) {
		t.Fatalf("pending err = %v", err)
	}
	if _, err := svc.GetPaymentByOrder(context.Background(), &pb.GetPaymentByOrderRequest{OrderNo: "ORD-MISSING"}); !apperr.IsCode(err, apperr.CodeNotFound) {
		t.Fatalf("missing err = %v", err)
	}
}

func TestGetPaymentByOrderRejectsEmptyOrder(t *testing.T) {
	svc := New(nil, nil, "", nil)
	if _, err := svc.GetPaymentByOrder(context.Background(), &pb.GetPaymentByOrderRequest{}); !apperr.IsCode(err, apperr.CodeInvalidArgument) {
		t.Fatalf("empty order err = %v", err)
	}
}

func TestGetPaymentByOrderAggregatesCapturedIntents(t *testing.T) {
	db := paymentTestDB(t)
	seedPaymentIntent(t, db, "PAY-A", "ORD-MULTI", statusSucceeded, 600, 600, 0)
	seedPaymentIntent(t, db, "PAY-B", "ORD-MULTI", statusPartiallyRefunded, 400, 400, 100)
	svc := New(db, nil, "", nil)
	got, err := svc.GetPaymentByOrder(context.Background(), &pb.GetPaymentByOrderRequest{OrderNo: "ORD-MULTI"})
	if err != nil {
		t.Fatal(err)
	}
	if got.GetCapturedCent() != 1000 || got.GetRefundedCent() != 100 || got.GetAmountCent() != 1000 {
		t.Fatalf("aggregate = %+v", got)
	}
	if got.GetStatus() != statusPartiallyRefunded {
		t.Fatalf("status = %s", got.GetStatus())
	}
}

func TestRefundOrderAllocatesAcrossCapturedIntents(t *testing.T) {
	db := paymentTestDB(t)
	seedPaymentIntent(t, db, "PAY-M1", "ORD-REFUND-MULTI", statusSucceeded, 600, 600, 0)
	seedPaymentIntent(t, db, "PAY-M2", "ORD-REFUND-MULTI", statusSucceeded, 400, 400, 0)
	svc := New(db, nil, "", nil)
	reply, err := svc.RefundOrder(context.Background(), &pb.RefundOrderRequest{
		OrderNo: "ORD-REFUND-MULTI", AmountCent: 700, Reason: "multi intent", IdempotencyKey: "multi-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetRefundedCent() != 700 {
		t.Fatalf("refunded = %d", reply.GetRefundedCent())
	}
	var total int64
	if err := db.QueryRow(`SELECT COALESCE(SUM(refunded_cent),0) FROM payment_intents WHERE order_no='ORD-REFUND-MULTI'`).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != 700 {
		t.Fatalf("intent refunded sum = %d", total)
	}
	var count int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM refunds WHERE order_no='ORD-REFUND-MULTI'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("refund rows = %d", count)
	}
	again, err := svc.RefundOrder(context.Background(), &pb.RefundOrderRequest{
		OrderNo: "ORD-REFUND-MULTI", AmountCent: 700, Reason: "multi intent", IdempotencyKey: "multi-1",
	})
	if err != nil {
		t.Fatalf("idempotent retry: %v", err)
	}
	if again.GetRefundedCent() != 700 {
		t.Fatalf("retry refunded = %d", again.GetRefundedCent())
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM refunds WHERE order_no='ORD-REFUND-MULTI'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("refund rows after retry = %d", count)
	}
}
