package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	db.SetMaxOpenConns(30)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(time.Hour)
	db.SetConnMaxIdleTime(10 * time.Minute)
	var lastErr error
	for attempt := 0; attempt < 10; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		lastErr = db.PingContext(ctx)
		cancel()
		if lastErr == nil {
			return db, nil
		}
		select {
		case <-time.After(time.Duration(attempt+1) * time.Second):
		}
	}
	_ = db.Close()
	return nil, fmt.Errorf("ping mysql after retries: %w", lastErr)
}

func WithTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
