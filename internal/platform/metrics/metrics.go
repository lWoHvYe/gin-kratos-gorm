// Package metrics owns the process-wide Prometheus collectors. The default
// registry is exposed by every service's /metrics endpoint.
package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	HTTPRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mall_http_requests_total",
		Help: "Total HTTP requests handled by BFFs.",
	}, []string{"method", "path", "status"})

	HTTPDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "mall_http_request_duration_seconds",
		Help:    "HTTP request latency in seconds.",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
	}, []string{"method", "path"})

	OutboxPublished = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mall_outbox_published_total",
		Help: "Outbox events successfully published to NATS.",
	})

	OutboxPublishErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mall_outbox_publish_errors_total",
		Help: "Outbox publish attempts that failed and will be retried.",
	})
)

func init() {
	prometheus.MustRegister(HTTPRequests, HTTPDuration, OutboxPublished, OutboxPublishErrors)
}
