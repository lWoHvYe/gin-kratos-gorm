package db

import (
	"database/sql"
	"testing"
)

func TestNormalizePool(t *testing.T) {
	cases := []struct{ open, idle, wantOpen, wantIdle int }{
		{30, 10, 30, 10},
		{0, 0, DefaultMaxOpenConns, 0},
		{-5, -1, DefaultMaxOpenConns, 0},
		{80, 200, 80, 80},
	}
	for _, tc := range cases {
		open, idle := normalizePool(tc.open, tc.idle)
		if open != tc.wantOpen || idle != tc.wantIdle {
			t.Fatalf("normalizePool(%d,%d) = %d,%d want %d,%d", tc.open, tc.idle, open, idle, tc.wantOpen, tc.wantIdle)
		}
	}
}

func TestConfigurePoolAppliesLimits(t *testing.T) {
	// sql.Open does not dial, so this exercises the pool configuration only.
	conn, err := sql.Open("mysql", "user:pass@tcp(127.0.0.1:1)/mall")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	configurePool(conn, 42, 7)
	if got := conn.Stats().MaxOpenConnections; got != 42 {
		t.Fatalf("MaxOpenConnections = %d", got)
	}
}
