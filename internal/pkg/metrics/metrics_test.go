package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegistryExportsPrometheusText(t *testing.T) {
	r := NewRegistry()
	r.Counter("requests_total", "requests", "method").Inc("GET")
	r.Gauge("in_flight", "in flight").Set(3)
	r.Histogram("latency_seconds", "latency", []float64{0.1, 1}, "route").Observe(0.5, "/api")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	r.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("unexpected status %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`requests_total{method="GET"} 1`,
		`in_flight 3`,
		`latency_seconds_bucket{route="/api",le="0.1"} 0`,
		`latency_seconds_bucket{route="/api",le="1"} 1`,
		`latency_seconds_count{route="/api"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics output missing %q:\n%s", want, body)
		}
	}
}
