package order

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"time"

	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/events"
	"gin-kratos-gorm/internal/platform/outbox"
)

// INV-44 / T1.4b2a: Order owns the durable release intent, Inventory owns the
// execution. The event is appended in the same transaction as the refund
// admission, so a failed append rolls the admission back and a committed
// admission can never lack its release intent. Order never calls Inventory
// from the admission path.
//
// Only PRE_SHIP_REVOKED (unshipped group) produces a release request. A
// POST_SHIP_ALLOWED group left the warehouse; its stock comes back through the
// existing AfterSales restock flow, not this event.

// refundReleaseSource maps the AdmitRefund request source onto the closed
// event enum. T1.4b2 accepts exactly tcc|system_zero|system_shortage|aftersale;
// an empty source remains compatible as an ordinary after-sale refund, but any
// other explicit value (payment, legacy_expired, unknown) is rejected with
// REFUND_RELEASE_SOURCE_UNSUPPORTED so the admission rolls back with zero
// writes instead of mislabelling the release.
func refundReleaseSource(raw string) (string, string, error) {
	source := strings.TrimSpace(raw)
	if source == "" {
		source = events.RefundReleaseSourceAfterSale
	}
	reason, ok := events.RefundReleaseReasonForSource(source)
	if !ok {
		return "", "", apperr.Invalid("REFUND_RELEASE_SOURCE_UNSUPPORTED: source %q is not supported by the INV-44 release", source).
			WithDetails(map[string]any{
				"source": source,
				"supported_sources": []string{
					events.RefundReleaseSourceTCC, events.RefundReleaseSourceSystemZero,
					events.RefundReleaseSourceSystemShortage, events.RefundReleaseSourceAfterSale,
				},
			})
	}
	return source, reason, nil
}

// loadRefundReleaseItems reads the group composition from the frozen order
// lines and aggregates it per (sku, warehouse). Quantity-split orders may have
// several lines for the same reservation; sending them separately would not
// match the reservation quantity and would make Release fail closed.
func loadRefundReleaseItems(ctx context.Context, q querier, orderNo, groupNo string) ([]events.RefundReleaseItem, error) {
	rows, err := q.QueryContext(ctx, `SELECT sku_id, quantity, warehouse_id FROM order_items
		WHERE order_no = ? AND fulfillment_group_no = ? ORDER BY line_no`, orderNo, groupNo)
	if err != nil {
		return nil, apperr.Internal("load group items for refund release", err)
	}
	defer rows.Close()
	quantities := make(map[[2]int64]int64)
	for rows.Next() {
		var skuID, quantity, warehouseID int64
		if err := rows.Scan(&skuID, &quantity, &warehouseID); err != nil {
			return nil, apperr.Internal("scan group item for refund release", err)
		}
		if skuID <= 0 || quantity <= 0 {
			return nil, apperr.Failed("group item for refund release has an invalid sku/quantity").
				WithDetails(map[string]any{"order_no": orderNo, "group_no": groupNo, "sku_id": skuID, "quantity": quantity})
		}
		if warehouseID <= 0 {
			// AGENTS §4 / constitution Article III.3: a release must carry the
			// real warehouse, never default to W1.
			return nil, apperr.Failed("group item for refund release is missing warehouse_id").
				WithDetails(map[string]any{"order_no": orderNo, "group_no": groupNo, "sku_id": skuID})
		}
		quantities[[2]int64{skuID, warehouseID}] += quantity
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal("iterate group items for refund release", err)
	}
	if len(quantities) == 0 {
		return nil, apperr.Failed("unshipped group has no order items to release").
			WithDetails(map[string]any{"order_no": orderNo, "group_no": groupNo})
	}
	items := make([]events.RefundReleaseItem, 0, len(quantities))
	for key, quantity := range quantities {
		items = append(items, events.RefundReleaseItem{
			SkuID: key[0], WarehouseID: key[1], Quantity: quantity,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].SkuID == items[j].SkuID {
			return items[i].WarehouseID < items[j].WarehouseID
		}
		return items[i].SkuID < items[j].SkuID
	})
	return items, nil
}

// appendRefundReleaseTx appends the durable INV-44 release request. The caller
// owns the admission transaction, so any error here rolls the admission back.
func appendRefundReleaseTx(ctx context.Context, tx *sql.Tx, refundID, orderNo, groupNo, source string) error {
	releaseSource, reason, err := refundReleaseSource(source)
	if err != nil {
		return err
	}
	items, err := loadRefundReleaseItems(ctx, tx, orderNo, groupNo)
	if err != nil {
		return err
	}
	payload := events.RefundReleaseRequested{
		SchemaVersion: 1,
		ReleaseID:     events.RefundReleaseID(refundID),
		RefundID:      refundID,
		OrderNo:       orderNo,
		GroupNo:       groupNo,
		Source:        releaseSource,
		Reason:        reason,
		Items:         items,
		RequestedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	return outbox.Append(ctx, tx, outbox.Event{
		Topic:         events.RefundReleaseTopic,
		AggregateType: "order",
		AggregateID:   orderNo,
		EventType:     events.RefundReleaseEventType,
		Payload:       payload,
	})
}
