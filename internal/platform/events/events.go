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

// PaymentRefunded is the canonical payment refund fact consumed by Order.
type PaymentRefunded struct {
	PaymentNo         string `json:"payment_no"`
	OrderNo           string `json:"order_no"`
	RefundNo          string `json:"refund_no"`
	UserID            int64  `json:"user_id,omitempty"`
	AmountCent        int64  `json:"amount_cent"`
	RefundedCent      int64  `json:"refunded_cent"`
	OrderRefundedCent int64  `json:"order_refunded_cent,omitempty"`
	CapturedCent      int64  `json:"captured_cent"`
	Reason            string `json:"reason,omitempty"`
	Channel           string `json:"channel,omitempty"`
	Status            string `json:"status,omitempty"`
	IdempotencyKey    string `json:"idempotency_key,omitempty"`
}
