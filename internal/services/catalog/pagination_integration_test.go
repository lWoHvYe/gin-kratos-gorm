package catalog

import (
	"context"
	"database/sql"
	"os"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"

	mysqldriver "github.com/go-sql-driver/mysql"
)

func pageTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN")
	}
	cfg, err := mysqldriver.ParseDSN(dsn)
	if err != nil || cfg.DBName == "" {
		t.Fatalf("bad DSN: %v", err)
	}
	admin := *cfg
	admin.DBName = ""
	a, _ := sql.Open("mysql", admin.FormatDSN())
	_, _ = a.Exec("CREATE DATABASE IF NOT EXISTS `" + cfg.DBName + "`")
	_ = a.Close()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, q := range []string{
		`DROP TABLE IF EXISTS spu_attributes`, `DROP TABLE IF EXISTS spu_images`,
		`DROP TABLE IF EXISTS skus`, `DROP TABLE IF EXISTS spus`,
		`CREATE TABLE spus (id BIGINT PRIMARY KEY, category_id BIGINT, brand_id BIGINT, name VARCHAR(128) NOT NULL DEFAULT '',
		 subtitle VARCHAR(128) NOT NULL DEFAULT '', description TEXT NOT NULL, main_image VARCHAR(255) NOT NULL DEFAULT '', status VARCHAR(16), min_price_cent BIGINT)`,
		`CREATE TABLE skus (id BIGINT PRIMARY KEY, spu_id BIGINT, sku_code VARCHAR(64), specs_json VARCHAR(255) NOT NULL DEFAULT '',
		 price_cent BIGINT NOT NULL DEFAULT 0, stock BIGINT NOT NULL DEFAULT 0, stock_version BIGINT NOT NULL DEFAULT 0, status VARCHAR(16),
		 image_url VARCHAR(255) NOT NULL DEFAULT '', category_id BIGINT NOT NULL DEFAULT 0, brand_id BIGINT NOT NULL DEFAULT 0)`,
		`CREATE TABLE spu_images (id BIGINT PRIMARY KEY, spu_id BIGINT, url VARCHAR(255), sort INT)`,
		`CREATE TABLE spu_attributes (id BIGINT PRIMARY KEY, spu_id BIGINT, name VARCHAR(64), value VARCHAR(255), sort INT)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i <= 6; i++ {
		if _, err := db.Exec(`INSERT INTO spus (id,category_id,brand_id,name,description,main_image,status,min_price_cent)
			VALUES (?,1,1,?,'','', 'ON_SALE', ?)`, i, "P", i*100); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO skus (id,spu_id,sku_code,price_cent,status) VALUES (?,?,?,?, 'ENABLED')`,
			i, i, "SKU-"+string(rune('0'+i)), i*100); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestKeysetPaginationBothReaders(t *testing.T) {
	db := pageTestDB(t)
	readers := map[string]*Service{}
	g, err := newGormReader(db)
	if err != nil {
		t.Fatal(err)
	}
	readers["gorm"] = &Service{db: db, gorm: g}
	readers["sql"] = &Service{db: db}
	for name, svc := range readers {
		t.Run(name, func(t *testing.T) {
			first, err := svc.ListProducts(context.Background(), &pb.ListProductsRequest{Page: 1, PageSize: 2, Sort: "price_asc"})
			if err != nil {
				t.Fatal(err)
			}
			if len(first.GetProducts()) != 2 || first.GetNextCursor() == "" {
				t.Fatalf("first = %+v", first)
			}
			second, err := svc.ListProducts(context.Background(), &pb.ListProductsRequest{
				Page: 1, PageSize: 2, Sort: "price_asc", Cursor: first.GetNextCursor(),
			})
			if err != nil {
				t.Fatal(err)
			}
			seen := map[int64]bool{}
			for _, p := range first.GetProducts() {
				seen[p.GetId()] = true
			}
			for _, p := range second.GetProducts() {
				if seen[p.GetId()] {
					t.Fatalf("overlap at id %d", p.GetId())
				}
			}
			if _, err := svc.ListProducts(context.Background(), &pb.ListProductsRequest{Page: 1, PageSize: 2, Cursor: "not-a-cursor"}); err == nil {
				t.Fatal("invalid cursor accepted")
			}
			if _, err := svc.ListProducts(context.Background(), &pb.ListProductsRequest{Page: 1001, PageSize: 2}); err == nil {
				t.Fatal("page>1000 accepted")
			}
		})
	}
}
