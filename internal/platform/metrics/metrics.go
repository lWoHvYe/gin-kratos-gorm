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

	// AfterSaleRestockBacklog/Oldest expose durable restock work.
	AfterSaleRestockBacklog = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mall_aftersale_restock_backlog",
		Help: "Pending or processing after-sale restock tasks.",
	})
	AfterSaleRestockOldestSeconds = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mall_aftersale_restock_oldest_seconds",
		Help: "Age of the oldest pending after-sale restock task in seconds.",
	})

	// RefundReviewBacklog/OldestSeconds expose the AC-72 refund review queue
	// (order_refund_admissions result=REVIEW_REQUIRED, state=PENDING_REMOTE).
	// Zero-amount conflicts never write that row and are therefore not counted.
	RefundReviewBacklog = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mall_refund_review_backlog",
		Help: "Persisted refund admission reviews awaiting manual handling.",
	})
	RefundReviewOldestSeconds = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mall_refund_review_oldest_seconds",
		Help: "Age of the oldest persisted refund admission review in seconds.",
	})

	// AfterSaleRefundBacklog/Oldest expose durable refund work.
	AfterSaleRefundBacklog = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mall_aftersale_refund_backlog",
		Help: "Pending or processing after-sale refund tasks.",
	})
	AfterSaleRefundOldestSeconds = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mall_aftersale_refund_oldest_seconds",
		Help: "Age of the oldest pending after-sale refund task in seconds.",
	})

	// PaymentRefundEvents tracks compensation events consumed by payment.
	PaymentRefundEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mall_payment_refund_events_total",
		Help: "order.refund_requested events handled by payment, by result.",
	}, []string{"result"})

	// PaymentCancelCASLost counts PENDING_PAYMENT->CANCELLED CAS losses on the
	// T1.6 payment-level cancel path. The metric name is fixed by the accepted
	// T1.6 Addendum §4.2 ruling 1 (`CANCEL_CAS_LOST`); reason must match the log
	// and details.reason, result records how the loss converged.
	PaymentCancelCASLost = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "payment_cancel_cas_lost_total",
		Help: "PENDING_PAYMENT->CANCELLED CAS losses in the T1.6 payment-level cancel path, by reason and result.",
	}, []string{"reason", "result"})

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

	// AIFeedback counts explicit helpful/not-helpful ratings.
	AIFeedback = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mall_ai_feedback_total",
		Help: "Assistant feedback by rating.",
	}, []string{"rating"})

	// AIRAGSearch counts retrieval mode; AIRAGEmpty counts empty result sets.
	AIRAGSearch = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mall_ai_rag_search_total",
		Help: "RAG searches by retrieval mode.",
	}, []string{"mode"})
	AIRAGEmpty = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mall_ai_rag_empty_total",
		Help: "RAG searches that returned no documents.",
	})

	// AIRAGConsumerPending/Truncated/RebuildFailures expose index operations.
	AIRAGConsumerPending = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mall_ai_rag_consumer_pending",
		Help: "Pending catalog-index consumer messages.",
	})
	AIRAGTruncated = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mall_ai_rag_truncated",
		Help: "1 when the startup catalog scan hit its page safety cap.",
	})
	AIRAGRebuildFailures = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mall_ai_rag_rebuild_failures_total",
		Help: "RAG startup build failures.",
	})

	// AIRAGDocuments/BuildSeconds/Versions expose RAG index health.
	AIRAGDocuments = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mall_ai_rag_documents",
		Help: "Documents in the current RAG index.",
	})
	AIRAGBuildSeconds = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mall_ai_rag_build_seconds",
		Help: "Duration of the last RAG index build.",
	})
	AIRAGVersions = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mall_ai_rag_versions",
		Help: "Active RAG versions in the dual-write registry.",
	})

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

	// InventoryLedgerRows/InventoryLedgerOldestSeconds quantify ledger growth.
	InventoryLedgerRows = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mall_inventory_ledger_rows",
		Help: "Rows in inventory_ledger.",
	})
	InventoryLedgerOldestSeconds = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mall_inventory_ledger_oldest_seconds",
		Help: "Age of the oldest inventory_ledger row.",
	})

	// InventoryLedgerArchived counts rows moved out of the hot ledger.
	InventoryLedgerArchived = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mall_inventory_ledger_archived_total",
		Help: "Inventory ledger rows moved to the archive.",
	})

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
		PaymentCancelCASLost,
		OutboxPending, OutboxOldestSeconds, OutboxDeadLettered, ConsumerOutcomes, AfterSaleRefundRecovery, GRPCDuration, Errors,
		AIRequests, AIDuration, AIToolCalls, AITokens, AIRateLimited, AIInfo,
		AIBudgetExceeded, AICostCents, AICircuitState, AIRAGUpdates, AIRAGSearch, AIRAGEmpty,
		AIRAGDocuments, AIRAGBuildSeconds, AIRAGVersions, AIRAGConsumerPending,
		AIRAGTruncated, AIRAGRebuildFailures, QuotesCleaned,
		InventoryLedgerRows, InventoryLedgerOldestSeconds, InventoryLedgerArchived, AIFeedback,
		AfterSaleRestockBacklog, AfterSaleRestockOldestSeconds,
		AfterSaleRefundBacklog, AfterSaleRefundOldestSeconds,
		RefundReviewBacklog, RefundReviewOldestSeconds)
}
