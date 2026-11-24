package events

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
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
	OrderNo        string      `json:"order_no"`
	UserID         int64       `json:"user_id"`
	QuoteID        string      `json:"quote_id,omitempty"`
	PayCent        int64       `json:"pay_cent,omitempty"`
	PaidCent       int64       `json:"paid_cent,omitempty"`
	PaymentNo      string      `json:"payment_no,omitempty"`
	Status         string      `json:"status,omitempty"`
	Reason         string      `json:"reason,omitempty"`
	Operator       string      `json:"operator,omitempty"`
	ExpireAt       int64       `json:"expire_at,omitempty"`
	PaidAt         int64       `json:"paid_at,omitempty"`
	Cancelled      int64       `json:"cancelled_at,omitempty"`
	Created        int64       `json:"created_at,omitempty"`
	Items          []OrderItem `json:"items,omitempty"`
	ChangedAt      int64       `json:"changed_at,omitempty"`
	GroupNo        string      `json:"group_no,omitempty"`
	AllocationHash string      `json:"allocation_hash,omitempty"`
	PackageCount   int32       `json:"package_count,omitempty"`
	WarehouseCount int32       `json:"warehouse_count,omitempty"`
	FrozenAt       int64       `json:"frozen_at,omitempty"`
	Resolution     string      `json:"resolution,omitempty"`
	ChooseDeadline int64       `json:"choose_deadline_unix,omitempty"`
	ReasonCode     string      `json:"reason_code,omitempty"`
	DetectedAt     int64       `json:"detected_at_unix,omitempty"`
	NotifyDeadline int64       `json:"notify_deadline_unix,omitempty"`
	NotifiedAt     int64       `json:"notified_at_unix,omitempty"`
	ResolvedAt     int64       `json:"resolved_at,omitempty"`
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
	PaymentNo           string `json:"payment_no"`
	OrderNo             string `json:"order_no"`
	UserID              int64  `json:"user_id,omitempty"`
	Amount              int64  `json:"amount_cent"`
	Channel             string `json:"channel,omitempty"`
	Status              string `json:"status,omitempty"`
	RefundID            string `json:"refund_id,omitempty"`
	RefundedCent        int64  `json:"refunded_cent,omitempty"`
	CapturedCent        int64  `json:"captured_cent,omitempty"`
	OrderCapturedCent   int64  `json:"order_captured_cent,omitempty"`
	IntentCapturedCent  int64  `json:"intent_captured_cent,omitempty"`
	IntentRefundedCent  int64  `json:"intent_refunded_cent,omitempty"`
	NonEffectiveCapture bool   `json:"non_effective_capture,omitempty"`
	Source              string `json:"source,omitempty"`
}

type orderSettledEvent struct {
	OrderNo        string   `json:"order_no"`
	PaymentNo      string   `json:"payment_no"`
	SettledAt      int64    `json:"settled_at"`
	AllocationHash string   `json:"allocation_hash"`
	PackageCount   int32    `json:"package_count"`
	WarehouseCount int32    `json:"warehouse_count"`
	Groups         []string `json:"groups"`
	// SchemaVersion is optional on the v1 subject; absent means 1. Shadow
	// consumers reject versions greater than 1 instead of guessing.
	SchemaVersion int32 `json:"schema_version,omitempty"`
}

type orderRefundAdmittedEvent struct {
	RefundID   string `json:"refund_id"`
	OrderNo    string `json:"order_no"`
	GroupNo    string `json:"group_no"`
	Result     string `json:"result"`
	AmountCent int64  `json:"amount_cent"`
	Version    int64  `json:"version"`
}

type shipmentEvent struct {
	ShipmentNo    string `json:"shipment_no"`
	OrderNo       string `json:"order_no"`
	GroupNo       string `json:"fulfillment_group_no,omitempty"`
	Warehouse     int64  `json:"warehouse_id,omitempty"`
	Carrier       string `json:"carrier,omitempty"`
	CarrierCode   string `json:"carrier_code,omitempty"`
	TrackingNo    string `json:"tracking_no,omitempty"`
	Status        string `json:"status,omitempty"`
	TrackStatus   string `json:"track_status,omitempty"`
	ExceptionType string `json:"exception_type,omitempty"`
	EventID       string `json:"event_id,omitempty"`
	Source        string `json:"source,omitempty"`
	Items         []struct {
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
	LineNo      int32  `json:"line_no,omitempty"`
	Quantity    int64  `json:"quantity,omitempty"`
	Refunded    int64  `json:"refunded_amount_cent,omitempty"`
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

// validateSample enforces contract rules that a typed unmarshal cannot
// express: required nested fields, non-zero identities and closed enums. It is
// keyed by event type so the legacy positional schema literals keep working.
// The decoded sample (not the raw JSON) is passed so implementations stay
// typed.
type validateSample func(sample any) error

// refundReleaseReasons binds each release source to its canonical reason. Both
// are closed enums in the T1.4b2 design; a mismatched pair is a producer bug.
var refundReleaseReasons = map[string]string{
	RefundReleaseSourceTCC:            RefundReleaseReasonTCCCompensation,
	RefundReleaseSourceSystemZero:     RefundReleaseReasonZeroAfterSale,
	RefundReleaseSourceSystemShortage: RefundReleaseReasonShortageRefund,
	RefundReleaseSourceAfterSale:      RefundReleaseReasonAfterSaleRefund,
}

// RefundReleaseReasonForSource returns the canonical reason for a T1.4b2
// release source. Producers use it so the source/reason pair cannot drift from
// the registry contract; ok=false means the source is unsupported.
func RefundReleaseReasonForSource(source string) (reason string, ok bool) {
	reason, ok = refundReleaseReasons[source]
	return reason, ok
}

// validators holds the per-event contract checks that the sample unmarshal
// cannot express.
var validators = map[string]validateSample{
	"inventory.refund_release_requested": validateRefundRelease,
}

// validateRefundRelease enforces INV-44/AGENTS: group identity and per-item
// warehouse identity are mandatory (never defaulted), release_id is derived
// from refund_id, schema_version is exactly 1, and items are unique per
// (sku, warehouse) so one reservation cannot be represented twice.
func validateRefundRelease(sample any) error {
	ev, ok := sample.(RefundReleaseRequested)
	if !ok {
		return fmt.Errorf("refund release validator received %T", sample)
	}
	if ev.SchemaVersion != 1 {
		return fmt.Errorf("schema_version must be 1, got %d", ev.SchemaVersion)
	}
	refundID := strings.TrimSpace(ev.RefundID)
	if refundID == "" {
		return fmt.Errorf("refund_id is required")
	}
	if strings.TrimSpace(ev.ReleaseID) != RefundReleaseID(refundID) {
		return fmt.Errorf("release_id must be %q, got %q", RefundReleaseID(refundID), ev.ReleaseID)
	}
	if strings.TrimSpace(ev.OrderNo) == "" {
		return fmt.Errorf("order_no is required")
	}
	if strings.TrimSpace(ev.GroupNo) == "" {
		return fmt.Errorf("group_no is required")
	}
	wantReason, ok := refundReleaseReasons[ev.Source]
	if !ok {
		return fmt.Errorf("source %q is not a release source", ev.Source)
	}
	if ev.Reason != wantReason {
		return fmt.Errorf("reason for source %q must be %q, got %q", ev.Source, wantReason, ev.Reason)
	}
	if _, err := time.Parse(time.RFC3339, ev.RequestedAt); err != nil {
		return fmt.Errorf("requested_at must be RFC3339: %w", err)
	}
	if len(ev.Items) == 0 {
		return fmt.Errorf("items must not be empty")
	}
	seen := make(map[[2]int64]struct{}, len(ev.Items))
	for i, item := range ev.Items {
		if item.SkuID <= 0 {
			return fmt.Errorf("items[%d].sku_id is required", i)
		}
		if item.Quantity <= 0 {
			return fmt.Errorf("items[%d].quantity must be positive", i)
		}
		if item.WarehouseID <= 0 {
			return fmt.Errorf("items[%d].warehouse_id is required", i)
		}
		key := [2]int64{item.SkuID, item.WarehouseID}
		if _, dup := seen[key]; dup {
			return fmt.Errorf("items[%d] duplicates sku %d warehouse %d", i, item.SkuID, item.WarehouseID)
		}
		seen[key] = struct{}{}
	}
	return nil
}

var schemas = map[string]schema{
	"order.created":                    {orderEvent{}, []string{"order_no", "user_id", "items"}},
	"order.cancelled":                  {orderEvent{}, []string{"order_no", "user_id", "status", "cancelled_at", "items"}},
	"order.paid":                       {orderEvent{}, []string{"order_no", "user_id", "payment_no", "paid_cent", "status", "paid_at", "items"}},
	"order.allocation_frozen":          {orderEvent{}, []string{"order_no", "allocation_hash", "package_count", "warehouse_count", "frozen_at"}},
	"order.settled":                    {orderSettledEvent{}, []string{"order_no", "payment_no", "settled_at", "allocation_hash", "package_count", "warehouse_count", "groups"}},
	"order.refund_admitted":            {orderRefundAdmittedEvent{}, []string{"refund_id", "order_no", "group_no", "result", "amount_cent", "version"}},
	"order.shortage_reported":          {orderEvent{}, []string{"order_no", "group_no", "status", "detected_at_unix", "notify_deadline_unix", "notified_at_unix", "choose_deadline_unix", "reason_code"}},
	"order.shortage_resolved":          {orderEvent{}, []string{"order_no", "group_no", "status", "resolution", "reason_code", "resolved_at", "items"}},
	"order.allocated":                  {orderEvent{}, []string{"order_no", "status"}},
	"order.shipped":                    {orderEvent{}, []string{"order_no", "status"}},
	"order.completed":                  {orderEvent{}, []string{"order_no", "status"}},
	"order.refund_requested":           {orderEvent{}, []string{"order_no", "payment_no", "status"}},
	"order.refund_projected":           {orderEvent{}, []string{"order_no", "payment_no", "refunded_cent", "captured_cent"}},
	"inventory.compensation_requested": {inventoryEvent{}, []string{"order_no", "items"}},
	"inventory.confirm_requested":      {inventoryEvent{}, []string{"order_no", "items"}},
	"inventory.refund_release_requested": {RefundReleaseRequested{}, []string{
		"schema_version", "release_id", "refund_id", "order_no", "group_no",
		"source", "reason", "items", "requested_at",
	}},
	"inventory.reserved":              {inventoryEvent{}, []string{"reservation_no", "order_no", "sku_id", "warehouse_id", "quantity", "status"}},
	"inventory.confirmed":             {inventoryEvent{}, []string{"reservation_no", "order_no", "sku_id", "warehouse_id", "quantity", "status"}},
	"inventory.released":              {inventoryEvent{}, []string{"reservation_no", "order_no", "sku_id", "warehouse_id", "quantity", "status"}},
	"inventory.stock_changed":         {inventoryEvent{}, []string{"sku_id", "warehouse_id", "on_hand", "reserved", "version"}},
	"inventory.adjusted":              {inventoryEvent{}, []string{"sku_id", "warehouse_id", "delta", "on_hand", "reserved"}},
	"inventory.operation_completed":   {operationEvent{}, []string{"operation_id", "status"}},
	"inventory.operation_failed":      {operationEvent{}, []string{"operation_id", "status"}},
	"inventory.operation_item_failed": {inventoryEvent{}, []string{"operation_id", "sku_id", "outcome"}},
	"payment.succeeded":               {paymentEvent{}, []string{"payment_no", "order_no", "amount_cent", "status"}},
	"payment.failed":                  {paymentEvent{}, []string{"payment_no", "order_no", "amount_cent", "status"}},
	"payment.refunded":                {paymentEvent{}, []string{"payment_no", "order_no", "amount_cent", "refunded_cent", "captured_cent"}},
	"shipment.created":                {shipmentEvent{}, []string{"shipment_no", "order_no", "status"}},
	"shipment.shipped":                {shipmentEvent{}, []string{"shipment_no", "order_no", "status"}},
	"shipment.delivered":              {shipmentEvent{}, []string{"shipment_no", "order_no", "status"}},
	"shipment.tracked":                {shipmentEvent{}, []string{"shipment_no", "order_no", "status"}},
	"shipment.exception":              {shipmentEvent{}, []string{"shipment_no", "order_no", "exception_type"}},
	"shipment.received":               {shipmentEvent{}, []string{"shipment_no", "order_no", "source"}},
	"shipment.dispute_opened":         {shipmentEvent{}, []string{"shipment_no", "order_no"}},
	"shipment.dispute_resolved":       {shipmentEvent{}, []string{"shipment_no", "order_no"}},
	"aftersale.created":               {afterSale{}, []string{"after_sale_no", "order_no", "user_id", "type", "status", "amount_cent"}},
	"aftersale.approved":              {afterSale{}, []string{"after_sale_no", "order_no", "user_id", "type", "status", "amount_cent"}},
	"aftersale.rejected":              {afterSale{}, []string{"after_sale_no", "order_no", "user_id", "type", "status", "amount_cent"}},
	"aftersale.returning":             {afterSale{}, []string{"after_sale_no", "order_no", "user_id", "type", "status", "amount_cent"}},
	"aftersale.received":              {afterSale{}, []string{"after_sale_no", "order_no", "user_id", "type", "status", "amount_cent"}},
	"aftersale.restock_pending":       {afterSale{}, []string{"after_sale_no", "order_no", "user_id", "type", "status", "amount_cent"}},
	"aftersale.restocked":             {afterSale{}, []string{"after_sale_no", "order_no", "user_id", "type", "status", "amount_cent"}},
	"aftersale.restock_failed":        {afterSale{}, []string{"after_sale_no", "order_no", "user_id", "type", "status", "amount_cent"}},
	"aftersale.refund_requested":      {afterSale{}, []string{"after_sale_no", "order_no", "user_id", "type", "status", "amount_cent"}},
	"aftersale.completed":             {afterSale{}, []string{"after_sale_no", "order_no", "user_id", "type", "status", "amount_cent"}},
	"aftersale.line_refunded":         {afterSale{}, []string{"after_sale_no", "order_no", "line_no", "quantity", "refunded_amount_cent"}},
	"coupon.locked":                   {couponEvent{}, []string{"quote_id", "coupon_id", "amount_cent"}},
	"coupon.unlocked":                 {couponEvent{}, []string{"quote_id", "coupon_id", "amount_cent"}},
	"coupon.used":                     {couponEvent{}, []string{"quote_id", "coupon_id", "amount_cent"}},
	"cart.item_added":                 {cartEvent{}, []string{"user_id", "sku_id"}},
	"cart.item_updated":               {cartEvent{}, []string{"user_id", "sku_id"}},
	"cart.item_removed":               {cartEvent{}, []string{"user_id", "sku_id"}},
	"cart.selected_cleared":           {cartEvent{}, []string{"user_id", "sku_id"}},
	"product.created":                 {productEvent{}, []string{"spu_id"}},
	"product.updated":                 {productEvent{}, []string{"spu_id"}},
	"product.deleted":                 {productEvent{}, []string{"spu_id"}},
	"product.status_changed":          {productEvent{}, []string{"spu_id", "status"}},
	"sku.created":                     {productEvent{}, []string{"spu_id"}},
	"sku.updated":                     {productEvent{}, []string{"spu_id"}},
	"test.created":                    {testEvent{}, []string{"ok"}},
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
	if validate := validators[eventType]; validate != nil {
		if err := validate(reflect.ValueOf(target).Elem().Interface()); err != nil {
			return fmt.Errorf("event %s violates its contract: %w", eventType, err)
		}
	}
	return nil
}
