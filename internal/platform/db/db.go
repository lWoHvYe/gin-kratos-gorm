package db

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/XSAM/otelsql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// DefaultMaxOpenConns bounds the pool per service replica. Sizing this for
	// the HA profile matters: MySQL max_connections must be at least
	// replicas * maxOpen across every service in the profile.
	DefaultMaxOpenConns = 30
	DefaultMaxIdleConns = 10
)

var (
	driverOnce   sync.Once
	driverName   = "mysql"
	driverErr    error
	statsMetrics sync.Once
)

// instrumentedDriver registers the MySQL driver once with OpenTelemetry spans.
// It falls back to the plain driver if registration fails.
func instrumentedDriver() (string, error) {
	driverOnce.Do(func() {
		driverName, driverErr = otelsql.Register("mysql")
	})
	return driverName, driverErr
}

// Open uses the default pool limits.
func Open(dsn string) (*sql.DB, error) {
	return OpenWithPool(dsn, DefaultMaxOpenConns, DefaultMaxIdleConns)
}

// OpenWithPool opens the database, applies bounded pool settings and waits for
// the first successful ping. Non-positive values fall back to the defaults and
// idle connections are clamped to the open limit.
func OpenWithPool(dsn string, maxOpen, maxIdle int) (*sql.DB, error) {
	return OpenWithPoolContext(context.Background(), dsn, maxOpen, maxIdle)
}

// OpenWithPoolContext is OpenWithPool with a cancellable startup: a SIGTERM
// during the retry loop aborts the wait instead of blocking shutdown.
func OpenWithPoolContext(ctx context.Context, dsn string, maxOpen, maxIdle int) (*sql.DB, error) {
	maxOpen, maxIdle = normalizePool(maxOpen, maxIdle)
	name, err := instrumentedDriver()
	if err != nil {
		return nil, fmt.Errorf("register otel mysql driver: %w", err)
	}
	db, err := sql.Open(name, dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	configurePool(db, maxOpen, maxIdle)
	var lastErr error
	for attempt := 0; attempt < 10; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		lastErr = db.PingContext(pingCtx)
		cancel()
		if lastErr == nil {
			registerPoolMetrics(db)
			return db, nil
		}
		select {
		case <-ctx.Done():
			_ = db.Close()
			return nil, fmt.Errorf("open mysql cancelled: %w", ctx.Err())
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

// registerPoolMetrics exposes sql.DBStats on /metrics once per process. Each
// service owns one database, so instance-level labels are added by Prometheus.
func registerPoolMetrics(db *sql.DB) {
	statsMetrics.Do(func() {
		prometheus.MustRegister(poolCollector{db: db})
	})
}

type poolCollector struct{ db *sql.DB }

var (
	poolOpenDesc = prometheus.NewDesc("mall_db_pool_open_connections",
		"Open connections in the pool.", nil, nil)
	poolInUseDesc = prometheus.NewDesc("mall_db_pool_in_use_connections",
		"Connections currently in use.", nil, nil)
	poolIdleDesc = prometheus.NewDesc("mall_db_pool_idle_connections",
		"Idle connections in the pool.", nil, nil)
	poolMaxOpenDesc = prometheus.NewDesc("mall_db_pool_max_open_connections",
		"Configured maximum open connections.", nil, nil)
	poolWaitCountDesc = prometheus.NewDesc("mall_db_pool_wait_count_total",
		"Total waits for a connection.", nil, nil)
	poolWaitDurationDesc = prometheus.NewDesc("mall_db_pool_wait_duration_seconds_total",
		"Total time blocked waiting for a connection.", nil, nil)
)

func (c poolCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- poolOpenDesc
	ch <- poolInUseDesc
	ch <- poolIdleDesc
	ch <- poolMaxOpenDesc
	ch <- poolWaitCountDesc
	ch <- poolWaitDurationDesc
}

func (c poolCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.db.Stats()
	ch <- prometheus.MustNewConstMetric(poolOpenDesc, prometheus.GaugeValue, float64(stats.OpenConnections))
	ch <- prometheus.MustNewConstMetric(poolInUseDesc, prometheus.GaugeValue, float64(stats.InUse))
	ch <- prometheus.MustNewConstMetric(poolIdleDesc, prometheus.GaugeValue, float64(stats.Idle))
	ch <- prometheus.MustNewConstMetric(poolMaxOpenDesc, prometheus.GaugeValue, float64(stats.MaxOpenConnections))
	ch <- prometheus.MustNewConstMetric(poolWaitCountDesc, prometheus.CounterValue, float64(stats.WaitCount))
	ch <- prometheus.MustNewConstMetric(poolWaitDurationDesc, prometheus.CounterValue, stats.WaitDuration.Seconds())
}
