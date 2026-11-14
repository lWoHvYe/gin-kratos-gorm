package db

import (
	"database/sql"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestPoolCollectorExposesStats(t *testing.T) {
	conn, err := sql.Open("mysql", "user:pass@tcp(127.0.0.1:1)/mall")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	configurePool(conn, 12, 4)

	registry := prometheus.NewRegistry()
	if err := registry.Register(poolCollector{db: conn}); err != nil {
		t.Fatal(err)
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, family := range families {
		seen[family.GetName()] = true
	}
	for _, name := range []string{
		"mall_db_pool_open_connections",
		"mall_db_pool_in_use_connections",
		"mall_db_pool_idle_connections",
		"mall_db_pool_max_open_connections",
		"mall_db_pool_wait_count_total",
		"mall_db_pool_wait_duration_seconds_total",
	} {
		if !seen[name] {
			t.Fatalf("metric %s missing from gather", name)
		}
	}
}
