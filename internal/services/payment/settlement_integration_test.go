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
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS payment_intents (
		id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, payment_no VARCHAR(64) NOT NULL,
		order_no VARCHAR(64) NOT NULL, user_id BIGINT NOT NULL DEFAULT 0,
		amount_cent BIGINT NOT NULL, captured_cent BIGINT NOT NULL DEFAULT 0,
		refunded_cent BIGINT NOT NULL DEFAULT 0, channel VARCHAR(32) NOT NULL DEFAULT 'mock',
		payment_method VARCHAR(32) NOT NULL DEFAULT '', status VARCHAR(16) NOT NULL DEFAULT 'PENDING',
		pay_url VARCHAR(255) NOT NULL DEFAULT '', expires_at DATETIME(3) NOT NULL,
		version BIGINT NOT NULL DEFAULT 1,
		created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
		updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
		UNIQUE KEY uk_payment_intents_no (payment_no))`); err != nil {
		t.Fatalf("create payment_intents: %v", err)
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
