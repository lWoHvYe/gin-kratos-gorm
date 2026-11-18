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

	SeckillAdmissions = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mall_seckill_admissions_total",
		Help: "Seckill admission outcomes.",
	}, []string{"result"})

	SeckillOrderCommands = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mall_seckill_order_commands_total",
		Help: "Seckill JetStream command outcomes.",
	}, []string{"result"})

	SeckillReconcileMismatch = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mall_seckill_reconcile_mismatch_total",
		Help: "Seckill stock reconciliation mismatches by reason.",
	}, []string{"reason"})

	OutboxPublishErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mall_outbox_publish_errors_total",
		Help: "Outbox publish attempts that failed and will be retried.",
	})

	// ConsumerOutcomes tracks durable consumer results per subject.
	ConsumerOutcomes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mall_consumer_messages_total",
		Help: "Consumer message outcomes by subject.",
	}, []string{"subject", "result"})

	// OutboxPending/OutboxOldestSeconds expose relay backlog.
	OutboxPending = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mall_outbox_pending_events",
		Help: "Unpublished, non-dead-lettered outbox events.",
	})
	OutboxOldestSeconds = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mall_outbox_oldest_pending_seconds",
		Help: "Age of the oldest pending outbox event in seconds.",
	})

	// OutboxDeadLettered counts events that exhausted their retry budget.
	OutboxDeadLettered = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mall_outbox_dead_lettered_total",
		Help: "Outbox events moved to the dead-letter state after max attempts.",
	})

	// AfterSaleRefundRecovery tracks the stuck-REFUNDING retry worker.
	AfterSaleRefundRecovery = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mall_aftersale_refund_recovery_total",
		Help: "Stuck after-sale refunds retried by result.",
	}, []string{"result"})

	// PaymentRefundEvents tracks compensation events consumed by payment.
	PaymentRefundEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mall_payment_refund_events_total",
		Help: "order.refund_requested events handled by payment, by result.",
	}, []string{"result"})

	// GRPCDuration tracks gRPC handler latency by operation and outcome.
	GRPCDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "mall_grpc_request_duration_seconds",
		Help:    "gRPC handler latency in seconds.",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5},
	}, []string{"operation", "code"})

	// Errors counts handled errors by surface (grpc/http) and domain code.
	Errors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mall_errors_total",
		Help: "Errors returned to callers, by surface and domain code.",
	}, []string{"surface", "code"})

	// AIRequests counts assistant turns by outcome.
	AIRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mall_ai_requests_total",
		Help: "Assistant chat requests by result.",
	}, []string{"result"})

	// AIDuration tracks assistant request latency.
	AIDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "mall_ai_request_duration_seconds",
		Help:    "Assistant chat latency in seconds.",
		Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 20, 40, 60},
	}, []string{"result"})

	// AIToolCalls counts tool invocations by tool and outcome.
	AIToolCalls = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mall_ai_tool_calls_total",
		Help: "Assistant tool calls by tool and result.",
	}, []string{"tool", "result"})

	// AITokens counts model tokens by kind (prompt/completion).
	AITokens = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mall_ai_tokens_total",
		Help: "Model tokens consumed by the assistant.",
	}, []string{"kind"})

	// AIRateLimited counts requests rejected by the per-user quota.
	AIRateLimited = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mall_ai_rate_limited_total",
		Help: "Assistant requests rejected by the per-user quota.",
	})

	// AIInfo exposes the active provider/model/prompt version.
	AIInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mall_ai_info",
		Help: "Active assistant provider, model and prompt version.",
	}, []string{"provider", "model", "prompt_version"})

	// AIBudgetExceeded counts rejected requests by budget scope.
	AIBudgetExceeded = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mall_ai_budget_exceeded_total",
		Help: "Assistant requests rejected by a token budget.",
	}, []string{"scope"})

	// QuotesCleaned counts expired quote rows removed by retention.
	QuotesCleaned = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mall_quotes_cleaned_total",
		Help: "Expired quotes removed by the retention worker.",
	})

	// AICostCents tracks estimated model cost when prices are configured.
	AICostCents = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mall_ai_cost_cents_total",
		Help: "Estimated assistant cost in cents by token kind.",
	}, []string{"kind"})

	// AIRAGUpdates tracks incremental catalog index updates.
	AIRAGUpdates = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mall_ai_rag_updates_total",
		Help: "Incremental RAG index updates by result.",
	}, []string{"result"})

	// AICircuitState is 1 when the provider circuit is open, 0 otherwise.
	AICircuitState = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mall_ai_circuit_open",
		Help: "Whether the assistant provider circuit is open.",
	})
)

func init() {
	prometheus.MustRegister(HTTPRequests, HTTPDuration, OutboxPublished, OutboxPublishErrors,
		SeckillAdmissions, SeckillOrderCommands, SeckillReconcileMismatch, PaymentRefundEvents,
		OutboxPending, OutboxOldestSeconds, OutboxDeadLettered, ConsumerOutcomes, AfterSaleRefundRecovery, GRPCDuration, Errors,
		AIRequests, AIDuration, AIToolCalls, AITokens, AIRateLimited, AIInfo,
		AIBudgetExceeded, AICostCents, AICircuitState, AIRAGUpdates, QuotesCleaned)
}
