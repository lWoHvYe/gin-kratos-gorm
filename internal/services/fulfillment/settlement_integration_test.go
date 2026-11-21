package fulfillment

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"

	mysqldriver "github.com/go-sql-driver/mysql"
	"google.golang.org/grpc"
)

func fulfillmentTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run fulfillment permit tests")
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
	if _, err := db.Exec(`DROP TABLE IF EXISTS shipments`); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS shipments (
			id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, shipment_no VARCHAR(64) NOT NULL,
			order_no VARCHAR(64) NOT NULL, fulfillment_group_no VARCHAR(64) NOT NULL DEFAULT 'G1',
			warehouse_id BIGINT NOT NULL DEFAULT 0,
			carrier VARCHAR(64) NOT NULL DEFAULT '', tracking_no VARCHAR(128) NOT NULL DEFAULT '',
			status VARCHAR(16) NOT NULL DEFAULT 'CREATED', version BIGINT NOT NULL DEFAULT 1,
			created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
			UNIQUE KEY uk_shipments_no (shipment_no),
			UNIQUE KEY uk_shipments_order_group (order_no, fulfillment_group_no))`,
		`CREATE TABLE IF NOT EXISTS shipment_items (
			id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, shipment_id BIGINT NOT NULL,
			sku_id BIGINT NOT NULL, quantity INT NOT NULL,
			created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			UNIQUE KEY uk_shipment_items_sku (shipment_id, sku_id))`,
		`CREATE TABLE IF NOT EXISTS outbox_events (
			id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, event_id VARCHAR(64) NOT NULL,
			topic VARCHAR(128) NOT NULL, aggregate_type VARCHAR(64) NOT NULL,
			aggregate_id VARCHAR(64) NOT NULL, event_type VARCHAR(64) NOT NULL,
			payload JSON NOT NULL, attempts INT NOT NULL DEFAULT 0,
			last_error VARCHAR(1024) NOT NULL DEFAULT '', next_attempt_at DATETIME(3) NULL,
			published_at DATETIME(3) NULL, dead_lettered_at DATETIME(3) NULL,
			lease_owner VARCHAR(128) NULL, lease_until DATETIME(3) NULL,
			traceparent VARCHAR(256) NULL, created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			UNIQUE KEY uk_outbox_event_id (event_id))`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("create fulfillment test table: %v", err)
		}
	}
	for _, table := range []string{"shipment_items", "shipments", "outbox_events"} {
		if _, err := db.Exec("DELETE FROM " + table); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

type fakePermitOrder struct {
	pb.OrderServiceClient
	err    error
	calls  int
	groups []string
}

func (f *fakePermitOrder) AuthorizeShipment(_ context.Context, in *pb.ShipmentPermitRequest, _ ...grpc.CallOption) (*pb.Empty, error) {
	f.calls++
	f.groups = append(f.groups, in.GetFulfillmentGroupNo())
	return &pb.Empty{}, f.err
}

func TestShipRequiresOrderPermit(t *testing.T) {
	db := fulfillmentTestDB(t)
	if _, err := db.Exec(`INSERT INTO shipments
		(shipment_no, order_no, warehouse_id, carrier, tracking_no, status, version)
		VALUES ('SHIP-1', 'ORD-1', 1, 'SF', 'SF1', 'CREATED', 1)`); err != nil {
		t.Fatal(err)
	}
	permit := &fakePermitOrder{err: context.DeadlineExceeded}
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), permit)
	if _, err := svc.Ship(context.Background(), &pb.ShipRequest{ShipmentNo: "SHIP-1"}); err == nil {
		t.Fatal("ship without a permit must fail")
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM shipments WHERE shipment_no = 'SHIP-1'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != statusCreated {
		t.Fatalf("shipment advanced without permit: %s", status)
	}
	permit.err = nil
	if _, err := svc.Ship(context.Background(), &pb.ShipRequest{ShipmentNo: "SHIP-1"}); err != nil {
		t.Fatalf("ship with permit: %v", err)
	}
	if err := db.QueryRow(`SELECT status FROM shipments WHERE shipment_no = 'SHIP-1'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != statusShipped || permit.calls != 2 {
		t.Fatalf("status=%s permit calls=%d", status, permit.calls)
	}
}

func TestShipUsesShipmentGroup(t *testing.T) {
	db := fulfillmentTestDB(t)
	if _, err := db.Exec(`INSERT INTO shipments
		(shipment_no, order_no, fulfillment_group_no, warehouse_id, carrier, tracking_no, status, version)
		VALUES ('SHIP-G1', 'ORD-GROUPS', 'G1', 1, 'SF', 'SF1', 'CREATED', 1),
		       ('SHIP-G2', 'ORD-GROUPS', 'G2', 2, 'SF', 'SF2', 'CREATED', 1)`); err != nil {
		t.Fatal(err)
	}
	permit := &fakePermitOrder{}
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), permit)
	for _, shipmentNo := range []string{"SHIP-G1", "SHIP-G2"} {
		if _, err := svc.Ship(context.Background(), &pb.ShipRequest{ShipmentNo: shipmentNo}); err != nil {
			t.Fatalf("ship %s: %v", shipmentNo, err)
		}
	}
	if len(permit.groups) != 2 || permit.groups[0] != "G1" || permit.groups[1] != "G2" {
		t.Fatalf("permit groups = %v", permit.groups)
	}
	for _, shipmentNo := range []string{"SHIP-G1", "SHIP-G2"} {
		var status string
		if err := db.QueryRow(`SELECT status FROM shipments WHERE shipment_no = ?`, shipmentNo).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status != statusShipped {
			t.Fatalf("%s = %s", shipmentNo, status)
		}
	}
}
