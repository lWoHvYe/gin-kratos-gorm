package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// AC-72/O18 (T5b): the refund-review gauges are registered with the fixed
// names the alert/report layer consumes.
func TestRefundReviewGaugesRegistered(t *testing.T) {
	want := map[string]bool{
		"mall_refund_review_backlog":        false,
		"mall_refund_review_oldest_seconds": false,
	}
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if _, ok := want[family.GetName()]; ok {
			want[family.GetName()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Fatalf("gauge %s is not registered", name)
		}
	}
}
