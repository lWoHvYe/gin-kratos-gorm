package pricing

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
)

func quoteTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run quote cleanup integration tests")
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
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS quotes (
			quote_id VARCHAR(64) NOT NULL PRIMARY KEY,
			expires_at DATETIME(3) NOT NULL
		) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS quote_lines (
			id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			quote_id VARCHAR(64) NOT NULL
		) ENGINE=InnoDB`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestCleanupQuotesRemovesOnlyExpiredRetention(t *testing.T) {
	db := quoteTestDB(t)
	if _, err := db.Exec(`DELETE FROM quote_lines`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM quotes`); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	_, err := db.Exec(`INSERT INTO quotes (quote_id, expires_at) VALUES (?, ?), (?, ?)`,
		"old", now.Add(-quoteRetention-time.Hour), "fresh", now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO quote_lines (quote_id) VALUES ('old'), ('fresh')`); err != nil {
		t.Fatal(err)
	}
	cleanupQuotes(context.Background(), db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	var quotes, lines int
	if err := db.QueryRow(`SELECT COUNT(*) FROM quotes`).Scan(&quotes); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM quote_lines`).Scan(&lines); err != nil {
		t.Fatal(err)
	}
	if quotes != 1 || lines != 1 {
		t.Fatalf("quotes=%d lines=%d, want 1/1", quotes, lines)
	}
}
