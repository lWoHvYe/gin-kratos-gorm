package order

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"

	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/events"
	"gin-kratos-gorm/internal/platform/outbox"
)

// INV-44 / T1.4b2a (design Rev 1.2, P0): only a single-group order may request
// a physical release. Inventory reservations are keyed by
// (order_no, sku_id, warehouse_id), not by group, so a full release is only
// safe when the order has exactly one fulfillment group. Multi-group orders
// keep their reservations (P1/manual); Order only logs an alert and never
// writes a partial/group-scoped event.
//
// The event is appended inside the admission transaction for a single-group
// PRE admission, so a failed append rolls the admission back. There is no
// deferred scan, last-active decision, watermark, cursor or manual queue.

const releaseGroupCountSingle = 1

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

// orderGroupCount counts the distinct fulfillment groups of an order from the
// group projection and the frozen lines. The union covers legacy G1 orders that
// have no order_fulfillment_groups row while still failing closed toward
// "multi-group" if the two projections disagree.
func orderGroupCount(ctx context.Context, q querier, orderNo string) (int, error) {
	rows, err := q.QueryContext(ctx, `SELECT group_no FROM order_fulfillment_groups WHERE order_no = ?
		UNION
		SELECT fulfillment_group_no FROM order_items WHERE order_no = ?`, orderNo, orderNo)
	if err != nil {
		return 0, apperr.Internal("count fulfillment groups for refund release", err)
	}
	defer rows.Close()
	groups := make(map[string]struct{})
	for rows.Next() {
		var groupNo string
		if err := rows.Scan(&groupNo); err != nil {
			return 0, apperr.Internal("scan fulfillment group for refund release", err)
		}
		groupNo = strings.TrimSpace(groupNo)
		if groupNo != "" {
			groups[groupNo] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return 0, apperr.Internal("iterate fulfillment groups for refund release", err)
	}
	if len(groups) == 0 {
		// No persisted group and no grouped lines: the order projection is
		// broken. Fail closed rather than guess G1 (design §2.1).
		return 0, apperr.Failed("order has no fulfillment group for refund release").
			WithDetails(map[string]any{"order_no": orderNo})
	}
	return len(groups), nil
}

// loadOrderReleaseItems builds the order-level items for the full release.
// Quantities are aggregated per (sku, warehouse) across the frozen lines. For a
// single-group order this is the group's complete reservation; the event must
// never carry a partial/group quantity.
func loadOrderReleaseItems(ctx context.Context, q querier, orderNo string) ([]events.RefundReleaseItem, error) {
	rows, err := q.QueryContext(ctx, `SELECT sku_id, quantity, warehouse_id, fulfillment_group_no
		FROM order_items WHERE order_no = ? ORDER BY line_no`, orderNo)
	if err != nil {
		return nil, apperr.Internal("load order items for refund release", err)
	}
	defer rows.Close()
	quantities := make(map[[2]int64]int64)
	for rows.Next() {
		var skuID, quantity, warehouseID int64
		var groupNo string
		if err := rows.Scan(&skuID, &quantity, &warehouseID, &groupNo); err != nil {
			return nil, apperr.Internal("scan order item for refund release", err)
		}
		if skuID <= 0 || quantity <= 0 {
			return nil, apperr.Failed("order item for refund release has an invalid sku/quantity").
				WithDetails(map[string]any{"order_no": orderNo, "sku_id": skuID, "quantity": quantity})
		}
		if warehouseID <= 0 {
			// AGENTS §4 / constitution Article III.3: a release must carry the
			// real warehouse, never default to W1.
			return nil, apperr.Failed("order item for refund release is missing warehouse_id").
				WithDetails(map[string]any{"order_no": orderNo, "sku_id": skuID})
		}
		if strings.TrimSpace(groupNo) == "" {
			// A line without a group would force a guessed G1 and hide a broken
			// group projection. Fail closed instead (design §2.1).
			return nil, apperr.Failed("order item for refund release is missing fulfillment_group_no").
				WithDetails(map[string]any{"order_no": orderNo, "sku_id": skuID})
		}
		quantities[[2]int64{skuID, warehouseID}] += quantity
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal("iterate order items for refund release", err)
	}
	if len(quantities) == 0 {
		return nil, apperr.Failed("order has no order items to release").
			WithDetails(map[string]any{"order_no": orderNo})
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

// hasOrderReleaseEventTx reports whether this order already has a full-release
// event. Admission writers hold the order row lock, so a plain read serializes
// per order without taking a table-wide lock on the outbox.
func hasOrderReleaseEventTx(ctx context.Context, q querier, orderNo string) (bool, error) {
	var id int64
	err := q.QueryRowContext(ctx, `SELECT id FROM outbox_events
		WHERE event_type = ? AND aggregate_id = ? ORDER BY id LIMIT 1`,
		events.RefundReleaseEventType, orderNo).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, apperr.Internal("check order refund release event", err)
	}
	return true, nil
}

// appendRefundReleaseTx emits the single-group full release. It is a no-op for
// a POST admission and for every multi-group order: the admission still
// commits, but Inventory must keep the reservations (P1/manual).
func (s *Service) appendRefundReleaseTx(ctx context.Context, tx *sql.Tx, refundID, orderNo, groupNo, source string) error {
	releaseSource, reason, err := refundReleaseSource(source)
	if err != nil {
		return err
	}
	// Validate the frozen line identity first: a missing group/warehouse is a
	// data-integrity failure regardless of group count (design §2.1).
	items, err := loadOrderReleaseItems(ctx, tx, orderNo)
	if err != nil {
		return err
	}
	groupCount, err := orderGroupCount(ctx, tx, orderNo)
	if err != nil {
		return err
	}
	if groupCount != releaseGroupCountSingle {
		s.log.Warn("multi-group refund keeps inventory reservations (P1/manual)",
			"order_no", orderNo, "group_no", groupNo, "refund_id", refundID, "group_count", groupCount)
		return nil
	}
	exists, err := hasOrderReleaseEventTx(ctx, tx, orderNo)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	payload := events.RefundReleaseRequested{
		SchemaVersion: 1,
		ReleaseScope:  events.RefundReleaseScopeOrderReservationFull,
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

// B3 is deferred: there is no production backfill, receipt table or scan
// worker. Existing PRE admissions are not converged by code; multi-group
// reservations intentionally stay reserved for P1/manual handling.
