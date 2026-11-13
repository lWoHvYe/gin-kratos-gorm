package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const (
	// DefaultMaxOpenConns bounds the pool per service replica. Sizing this for
	// the HA profile matters: MySQL max_connections must be at least
	// replicas * maxOpen across every service in the profile.
	DefaultMaxOpenConns = 30
	DefaultMaxIdleConns = 10
)

// Open uses the default pool limits.
func Open(dsn string) (*sql.DB, error) {
	return OpenWithPool(dsn, DefaultMaxOpenConns, DefaultMaxIdleConns)
}

// OpenWithPool opens the database, applies bounded pool settings and waits for
// the first successful ping. Non-positive values fall back to the defaults and
// idle connections are clamped to the open limit.
func OpenWithPool(dsn string, maxOpen, maxIdle int) (*sql.DB, error) {
	maxOpen, maxIdle = normalizePool(maxOpen, maxIdle)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	configurePool(db, maxOpen, maxIdle)
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

func configurePool(db *sql.DB, maxOpen, maxIdle int) {
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(time.Hour)
	db.SetConnMaxIdleTime(10 * time.Minute)
}

func normalizePool(maxOpen, maxIdle int) (int, int) {
	if maxOpen <= 0 {
		maxOpen = DefaultMaxOpenConns
	}
	if maxIdle < 0 {
		maxIdle = 0
	}
	if maxIdle > maxOpen {
		maxIdle = maxOpen
	}
	return maxOpen, maxIdle
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
