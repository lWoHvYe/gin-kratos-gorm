package biz

import (
	"context"
	"time"

	"gin-kratos-gorm/internal/pkg/idgen"
)

const (
	EventOrderCreated   = "order.created"
	EventOrderPaid      = "order.paid"
	EventOrderCancelled = "order.cancelled"
)

// DomainEvent is persisted to the outbox in the same transaction as the state
// change, then dispatched asynchronously (at-least-once).
type DomainEvent struct {
	ID            string
	Type          string
	AggregateType string
	AggregateID   string
	OccurredAt    time.Time
	Payload       any
}

type OrderItemPayload struct {
	SkuID   int64   `json:"sku_id"`
	SkuName string  `json:"sku_name"`
	Num     int32   `json:"num"`
	Price   float64 `json:"price"`
}

type OrderEventPayload struct {
	OrderSN   string             `json:"order_sn"`
	UserID    int64              `json:"user_id"`
	Status    int32              `json:"status"`
	PayAmount float64            `json:"pay_amount"`
	Reason    string             `json:"reason,omitempty"`
	Items     []OrderItemPayload `json:"items,omitempty"`
}

func NewOrderEvent(eventType string, orderSN string, userID int64, status int32, payAmount float64, reason string, items []OrderItemPayload, now time.Time) *DomainEvent {
	return &DomainEvent{
		ID:            idgen.EventID(),
		Type:          eventType,
		AggregateType: "order",
		AggregateID:   orderSN,
		OccurredAt:    now,
		Payload: OrderEventPayload{
			OrderSN:   orderSN,
			UserID:    userID,
			Status:    status,
			PayAmount: payAmount,
			Reason:    reason,
			Items:     items,
		},
	}
}

// OutboxWriter persists an event transactionally with the caller's transaction.
type OutboxWriter interface {
	Add(ctx context.Context, event *DomainEvent) error
}
