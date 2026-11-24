// Package events contains the typed payloads for cross-service events that
// carry money, inventory or fulfillment-group facts. Legacy events still use
// maps where no consumer contract has been stabilized yet.
package events

// AfterSaleRefundRequested is emitted when a refund intent is persisted.
type AfterSaleRefundRequested struct {
	AfterSaleNo         string `json:"after_sale_no"`
	OrderNo             string `json:"order_no"`
	GroupNo             string `json:"group_no,omitempty"`
	UserID              int64  `json:"user_id"`
	Type                string `json:"type"`
	Status              string `json:"status"`
	AmountCent          int64  `json:"amount_cent"`
	EffectiveAmountCent int64  `json:"effective_amount_cent,omitempty"`
	AmountState         string `json:"amount_state,omitempty"`
	RestockState        string `json:"restock_state,omitempty"`
	Reason              string `json:"reason"`
	RefundIntent        bool   `json:"refund_intent,omitempty"`
}

// AfterSaleCompleted is emitted after Payment has completed the refund.
type AfterSaleCompleted struct {
	AfterSaleNo         string `json:"after_sale_no"`
	OrderNo             string `json:"order_no"`
	GroupNo             string `json:"group_no,omitempty"`
	UserID              int64  `json:"user_id"`
	Type                string `json:"type"`
	Status              string `json:"status"`
	AmountCent          int64  `json:"amount_cent"`
	EffectiveAmountCent int64  `json:"effective_amount_cent,omitempty"`
	AmountState         string `json:"amount_state,omitempty"`
	RestockState        string `json:"restock_state,omitempty"`
	Reason              string `json:"reason"`
}

// Event identity of the INV-44 release request produced by Order and executed
// by Inventory (T1.4b2a). The subject is versioned because this is an additive
// protocol: the legacy order-level inventory.compensation_requested keeps its
// own semantics.
const (
	RefundReleaseTopic     = "shop.inventory.refund_release_requested.v1"
	RefundReleaseEventType = "inventory.refund_release_requested"

	// RefundReleaseScopeOrderReservationFull is the only T1.4b2 release
	// scope: Inventory releases every reservation of the order, not one group.
	RefundReleaseScopeOrderReservationFull = "order_reservation_full"

	RefundReleaseSourceTCC            = "tcc"
	RefundReleaseSourceSystemZero     = "system_zero"
	RefundReleaseSourceSystemShortage = "system_shortage"
	RefundReleaseSourceAfterSale      = "aftersale"

	RefundReleaseReasonTCCCompensation = "TCC_COMPENSATION"
	RefundReleaseReasonZeroAfterSale   = "ZERO_AFTER_SALE"
	RefundReleaseReasonShortageRefund  = "SHORTAGE_REFUND"
	RefundReleaseReasonAfterSaleRefund = "AFTER_SALE_REFUND"
)

// RefundReleaseID is the deterministic idempotency key of one order-level full
// release: the refund identity of the admission that triggered it, prefixed
// once. Replaying the same refund must converge on the same release instead of
// creating a second one.
func RefundReleaseID(refundID string) string { return "rr:" + refundID }

// RefundReleaseItem is one (sku, warehouse) quantity released by Inventory.
// WarehouseID is mandatory: multi-warehouse operations must never default to
// W1 (AGENTS §4 / constitution Article III.3).
type RefundReleaseItem struct {
	SkuID       int64 `json:"sku_id"`
	Quantity    int64 `json:"quantity"`
	WarehouseID int64 `json:"warehouse_id"`
}

// RefundReleaseRequested is the durable INV-44 release intent. Order writes it
// inside the refund admission transaction; Inventory consumes it and performs
// Release (HELD) or TccCancelConfirm (CONFIRMED). It carries no money/permit
// decision and must not be interpreted as an order status change.
type RefundReleaseRequested struct {
	SchemaVersion int32  `json:"schema_version"`
	ReleaseScope  string `json:"release_scope"`
	ReleaseID     string `json:"release_id"`
	RefundID      string `json:"refund_id"`
	OrderNo       string `json:"order_no"`
	// GroupNo is the triggering group and is audit-only. It does not identify
	// the Inventory reservation (the full order reservation does).
	GroupNo string `json:"group_no"`
	// CoveredGroupNos records the groups covered by the order-level release.
	CoveredGroupNos []string            `json:"covered_group_nos,omitempty"`
	Source          string              `json:"source"`
	Reason          string              `json:"reason"`
	Items           []RefundReleaseItem `json:"items"`
	RequestedAt     string              `json:"requested_at"`
}

// PaymentRefunded is the canonical payment refund fact consumed by Order.
type PaymentRefunded struct {
	PaymentNo           string `json:"payment_no"`
	OrderNo             string `json:"order_no"`
	RefundNo            string `json:"refund_no"`
	RefundID            string `json:"refund_id,omitempty"`
	UserID              int64  `json:"user_id,omitempty"`
	AmountCent          int64  `json:"amount_cent"`
	RefundedCent        int64  `json:"refunded_cent"`
	OrderRefundedCent   int64  `json:"order_refunded_cent,omitempty"`
	CapturedCent        int64  `json:"captured_cent"`
	Reason              string `json:"reason,omitempty"`
	Channel             string `json:"channel,omitempty"`
	Status              string `json:"status,omitempty"`
	IdempotencyKey      string `json:"idempotency_key,omitempty"`
	NonEffectiveCapture bool   `json:"non_effective_capture,omitempty"`
}
