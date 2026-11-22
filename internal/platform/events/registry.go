package events

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// OrderItem is the canonical order line carried by order events.
type OrderItem struct {
	SkuID        int64  `json:"sku_id"`
	Name         string `json:"name,omitempty"`
	PriceCent    int64  `json:"price_cent,omitempty"`
	Quantity     int32  `json:"quantity"`
	DiscountCent int64  `json:"discount_cent,omitempty"`
	WarehouseID  int64  `json:"warehouse_id,omitempty"`
	GroupNo      string `json:"group_no,omitempty"`
}

type orderEvent struct {
	OrderNo   string      `json:"order_no"`
	UserID    int64       `json:"user_id"`
	QuoteID   string      `json:"quote_id,omitempty"`
	PayCent   int64       `json:"pay_cent,omitempty"`
	PaidCent  int64       `json:"paid_cent,omitempty"`
	PaymentNo string      `json:"payment_no,omitempty"`
	Status    string      `json:"status,omitempty"`
	Reason    string      `json:"reason,omitempty"`
	Operator  string      `json:"operator,omitempty"`
	ExpireAt  int64       `json:"expire_at,omitempty"`
	PaidAt    int64       `json:"paid_at,omitempty"`
	Cancelled int64       `json:"cancelled_at,omitempty"`
	Created   int64       `json:"created_at,omitempty"`
	Items     []OrderItem `json:"items,omitempty"`
	ChangedAt int64       `json:"changed_at,omitempty"`
	GroupNo   string      `json:"group_no,omitempty"`
}

type inventoryItem struct {
	SkuID       int64  `json:"sku_id"`
	WarehouseID int64  `json:"warehouse_id"`
	Quantity    int64  `json:"quantity"`
	GroupNo     string `json:"group_no,omitempty"`
}

type inventoryEvent struct {
	ReservationNo string `json:"reservation_no,omitempty"`
	OrderNo       string `json:"order_no,omitempty"`
	SkuID         int64  `json:"sku_id,omitempty"`
	WarehouseID   int64  `json:"warehouse_id,omitempty"`
	Quantity      int64  `json:"quantity,omitempty"`
	Status        string `json:"status,omitempty"`
	OccurredAt    int64  `json:"occurred_at_unix,omitempty"`
	OnHand        int64  `json:"on_hand,omitempty"`
	Reserved      int64  `json:"reserved,omitempty"`
	Available     int64  `json:"available,omitempty"`
	Version       int64  `json:"version,omitempty"`
	ChangedAt     int64  `json:"changed_at,omitempty"`
	Delta         int64  `json:"delta,omitempty"`
	Reason        string `json:"reason,omitempty"`
	AdjustedAt    int64  `json:"adjusted_at,omitempty"`
}

type operationEvent struct {
	OperationID string           `json:"operation_id"`
	OrderNo     string           `json:"order_no,omitempty"`
	Status      string           `json:"status,omitempty"`
	Details     []map[string]any `json:"item_details,omitempty"`
	Items       []inventoryItem  `json:"items,omitempty"`
}

type paymentEvent struct {
	PaymentNo string `json:"payment_no"`
	OrderNo   string `json:"order_no"`
	UserID    int64  `json:"user_id,omitempty"`
	Amount    int64  `json:"amount_cent"`
	Channel   string `json:"channel,omitempty"`
	Status    string `json:"status,omitempty"`
}

type shipmentEvent struct {
	ShipmentNo string `json:"shipment_no"`
	OrderNo    string `json:"order_no"`
	GroupNo    string `json:"fulfillment_group_no,omitempty"`
	Warehouse  int64  `json:"warehouse_id,omitempty"`
	Carrier    string `json:"carrier,omitempty"`
	TrackingNo string `json:"tracking_no,omitempty"`
	Status     string `json:"status,omitempty"`
	Items      []struct {
		SkuID    int64 `json:"sku_id"`
		Quantity int32 `json:"quantity"`
	} `json:"items,omitempty"`
}

type afterSale struct {
	AfterSaleNo string `json:"after_sale_no"`
	OrderNo     string `json:"order_no"`
	UserID      int64  `json:"user_id"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	Amount      int64  `json:"amount_cent"`
	Reason      string `json:"reason,omitempty"`
	GroupNo     string `json:"group_no,omitempty"`
}

type couponEvent struct {
	QuoteID  string `json:"quote_id"`
	CouponID int64  `json:"coupon_id"`
	Code     string `json:"code,omitempty"`
	UserID   int64  `json:"user_id"`
	Amount   int64  `json:"amount_cent"`
}

type cartEvent struct {
	UserID int64 `json:"user_id"`
	SkuID  int64 `json:"sku_id"`
}

type productEvent struct {
	SpuID  int64  `json:"spu_id"`
	Status string `json:"status"`
}

type testEvent struct {
	OK bool `json:"ok"`
}

type schema struct {
	sample   any
	required []string
}

var schemas = map[string]schema{
	"order.created":                    {orderEvent{}, []string{"order_no", "user_id", "items"}},
	"order.cancelled":                  {orderEvent{}, []string{"order_no", "user_id", "status", "cancelled_at", "items"}},
	"order.paid":                       {orderEvent{}, []string{"order_no", "user_id", "payment_no", "paid_cent", "status", "paid_at", "items"}},
	"order.allocated":                  {orderEvent{}, []string{"order_no", "status"}},
	"order.shipped":                    {orderEvent{}, []string{"order_no", "status"}},
	"order.completed":                  {orderEvent{}, []string{"order_no", "status"}},
	"order.refund_requested":           {orderEvent{}, []string{"order_no", "payment_no", "status"}},
	"order.refund_projected":           {orderEvent{}, []string{"order_no", "payment_no", "refunded_cent", "captured_cent"}},
	"inventory.compensation_requested": {inventoryEvent{}, []string{"order_no", "items"}},
	"inventory.confirm_requested":      {inventoryEvent{}, []string{"order_no", "items"}},
	"inventory.reserved":               {inventoryEvent{}, []string{"reservation_no", "order_no", "sku_id", "warehouse_id", "quantity", "status"}},
	"inventory.confirmed":              {inventoryEvent{}, []string{"reservation_no", "order_no", "sku_id", "warehouse_id", "quantity", "status"}},
	"inventory.released":               {inventoryEvent{}, []string{"reservation_no", "order_no", "sku_id", "warehouse_id", "quantity", "status"}},
	"inventory.stock_changed":          {inventoryEvent{}, []string{"sku_id", "warehouse_id", "on_hand", "reserved", "version"}},
	"inventory.adjusted":               {inventoryEvent{}, []string{"sku_id", "warehouse_id", "delta", "on_hand", "reserved"}},
	"inventory.operation_completed":    {operationEvent{}, []string{"operation_id", "status"}},
	"inventory.operation_failed":       {operationEvent{}, []string{"operation_id", "status"}},
	"inventory.operation_item_failed":  {inventoryEvent{}, []string{"operation_id", "sku_id", "outcome"}},
	"payment.succeeded":                {paymentEvent{}, []string{"payment_no", "order_no", "amount_cent", "status"}},
	"payment.failed":                   {paymentEvent{}, []string{"payment_no", "order_no", "amount_cent", "status"}},
	"payment.refunded":                 {paymentEvent{}, []string{"payment_no", "order_no", "amount_cent", "refunded_cent", "captured_cent"}},
	"shipment.created":                 {shipmentEvent{}, []string{"shipment_no", "order_no", "status"}},
	"shipment.shipped":                 {shipmentEvent{}, []string{"shipment_no", "order_no", "status"}},
	"shipment.delivered":               {shipmentEvent{}, []string{"shipment_no", "order_no", "status"}},
	"aftersale.created":                {afterSale{}, []string{"after_sale_no", "order_no", "user_id", "type", "status", "amount_cent"}},
	"aftersale.approved":               {afterSale{}, []string{"after_sale_no", "order_no", "user_id", "type", "status", "amount_cent"}},
	"aftersale.rejected":               {afterSale{}, []string{"after_sale_no", "order_no", "user_id", "type", "status", "amount_cent"}},
	"aftersale.returning":              {afterSale{}, []string{"after_sale_no", "order_no", "user_id", "type", "status", "amount_cent"}},
	"aftersale.received":               {afterSale{}, []string{"after_sale_no", "order_no", "user_id", "type", "status", "amount_cent"}},
	"aftersale.restock_pending":        {afterSale{}, []string{"after_sale_no", "order_no", "user_id", "type", "status", "amount_cent"}},
	"aftersale.restocked":              {afterSale{}, []string{"after_sale_no", "order_no", "user_id", "type", "status", "amount_cent"}},
	"aftersale.restock_failed":         {afterSale{}, []string{"after_sale_no", "order_no", "user_id", "type", "status", "amount_cent"}},
	"aftersale.refund_requested":       {afterSale{}, []string{"after_sale_no", "order_no", "user_id", "type", "status", "amount_cent"}},
	"aftersale.completed":              {afterSale{}, []string{"after_sale_no", "order_no", "user_id", "type", "status", "amount_cent"}},
	"coupon.locked":                    {couponEvent{}, []string{"quote_id", "coupon_id", "amount_cent"}},
	"coupon.unlocked":                  {couponEvent{}, []string{"quote_id", "coupon_id", "amount_cent"}},
	"coupon.used":                      {couponEvent{}, []string{"quote_id", "coupon_id", "amount_cent"}},
	"cart.item_added":                  {cartEvent{}, []string{"user_id", "sku_id"}},
	"cart.item_updated":                {cartEvent{}, []string{"user_id", "sku_id"}},
	"cart.item_removed":                {cartEvent{}, []string{"user_id", "sku_id"}},
	"cart.selected_cleared":            {cartEvent{}, []string{"user_id", "sku_id"}},
	"product.created":                  {productEvent{}, []string{"spu_id"}},
	"product.updated":                  {productEvent{}, []string{"spu_id"}},
	"product.deleted":                  {productEvent{}, []string{"spu_id"}},
	"product.status_changed":           {productEvent{}, []string{"spu_id", "status"}},
	"sku.created":                      {productEvent{}, []string{"spu_id"}},
	"sku.updated":                      {productEvent{}, []string{"spu_id"}},
	"test.created":                     {testEvent{}, []string{"ok"}},
}

// ValidateJSON checks a serialized payload against the canonical schema for
// eventType. Unknown event types fail closed so a new event cannot silently
// bypass the contract registry.
func ValidateJSON(eventType string, payload []byte) error {
	sc, ok := schemas[eventType]
	if !ok {
		return fmt.Errorf("unregistered outbox event type %q", eventType)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return fmt.Errorf("event %s payload is not an object: %w", eventType, err)
	}
	for _, key := range sc.required {
		if _, ok := fields[key]; !ok {
			return fmt.Errorf("event %s missing required field %q", eventType, key)
		}
	}
	target := reflect.New(reflect.TypeOf(sc.sample)).Interface()
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("event %s does not match schema: %w", eventType, err)
	}
	return nil
}
