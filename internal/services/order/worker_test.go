package order

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// AC-72/O18 (T5b): the refund-review gauges ride the existing order tick (no
// new worker). They count only persisted REVIEW_REQUIRED/PENDING_REMOTE rows,
// expose the oldest age for the 7-day SLA, and exclude zero-amount conflicts by
// construction (those paths write no admission row).
func TestRefundReviewGauges(t *testing.T) {
	db := orderTestDB(t)
	ctx := context.Background()
	if _, err := db.Exec(`INSERT INTO order_refund_admissions
		(refund_id, order_no, group_no, result, state, amount_cent, version, created_at, updated_at)
		VALUES ('R-GAUGE-NEW','ORD-GAUGE','G1','REVIEW_REQUIRED','PENDING_REMOTE',1000,1,NOW(3),NOW(3)),
		       ('R-GAUGE-OLD','ORD-GAUGE','G1','REVIEW_REQUIRED','PENDING_REMOTE',2000,1,DATE_SUB(NOW(3), INTERVAL 10 DAY),DATE_SUB(NOW(3), INTERVAL 10 DAY)),
		       ('R-GAUGE-PRE','ORD-GAUGE','G1','PRE_SHIP_REVOKED','PRE_SHIP_REVOKED',1000,1,NOW(3),NOW(3)),
		       ('R-GAUGE-PENDING','ORD-GAUGE','G1','PENDING_REMOTE','PENDING_REMOTE',1000,1,NOW(3),NOW(3))`); err != nil {
		t.Fatal(err)
	}
	if err := refreshRefundReviewGauges(ctx, db); err != nil {
		t.Fatal(err)
	}
	if got := gaugeValue(t, "mall_refund_review_backlog"); got != 2 {
		t.Fatalf("backlog = %v, want 2", got)
	}
	if got := gaugeValue(t, "mall_refund_review_oldest_seconds"); got < 10*24*3600 {
		t.Fatalf("oldest seconds = %v, want >= 10 days", got)
	}
	// The 7-day overdue query used by the ops runbook.
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_admissions
		WHERE result = 'REVIEW_REQUIRED' AND state = 'PENDING_REMOTE'
		  AND updated_at < DATE_SUB(NOW(3), INTERVAL 7 DAY)`); got != 1 {
		t.Fatalf("7-day overdue rows = %d, want 1", got)
	}
}

func gaugeValue(t *testing.T, name string) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			return metric.GetGauge().GetValue()
		}
	}
	t.Fatalf("gauge %s not registered", name)
	return 0
}
