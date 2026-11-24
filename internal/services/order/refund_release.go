package order

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"time"

	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/events"
	"gin-kratos-gorm/internal/platform/outbox"
)

// INV-44 / T1.4b2a (design Rev 1.1): Order owns the durable order-level full
// release intent, Inventory owns the execution.
//
// Inventory reservations are keyed by (order_no, sku_id, warehouse_id), not by
// fulfillment group. A group-scoped refund therefore cannot release only its
// own quantity. Order defers the release until the refunding group is the last
// active group of the order, then requests `order_reservation_full`. The event
// items are the order-level aggregates, not the triggering group quantity.
//
// The event is appended in the same transaction as the refund admission when
// the group is already last, so a failed append rolls the admission back. When
// the group is not last, nothing is written; a bounded reconcile scan re-checks
// group state and emits the same idempotent release later.
//
// A POST/shipped group makes a full release unsafe (it would touch inventory
// that already left the warehouse or was restocked by AfterSales). Those orders
// are never auto-released; they stay for manual/AfterSales compensation and are
// logged by the scan.

const (
	refundReleaseScanLimit       = 100
	refundReleaseDefaultInterval = time.Minute
	groupStatusCancelled         = "CANCELLED"
)

// refundReleaseGroupState is the Order-side view used to decide whether the
// triggering group is the last active one.
type refundReleaseGroupState struct {
	groupNo     string
	status      string
	refundState string
	permitState string
}

type refundReleasePlan struct {
	emit            bool
	manual          bool
	blockedGroups   []string
	coveredGroupNos []string
	items           []events.RefundReleaseItem
}

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

// loadOrderReleaseItems builds the order-level items for the full release and
// the set of groups represented by the frozen lines. Quantities are aggregated
// per (sku, warehouse) across every group because Inventory's reservation is
// order-scoped, not group-scoped.
func loadOrderReleaseItems(ctx context.Context, q querier, orderNo string) ([]events.RefundReleaseItem, []string, error) {
	rows, err := q.QueryContext(ctx, `SELECT sku_id, quantity, warehouse_id, fulfillment_group_no
		FROM order_items WHERE order_no = ? ORDER BY line_no`, orderNo)
	if err != nil {
		return nil, nil, apperr.Internal("load order items for refund release", err)
	}
	defer rows.Close()
	quantities := make(map[[2]int64]int64)
	groups := make(map[string]struct{})
	for rows.Next() {
		var skuID, quantity, warehouseID int64
		var groupNo string
		if err := rows.Scan(&skuID, &quantity, &warehouseID, &groupNo); err != nil {
			return nil, nil, apperr.Internal("scan order item for refund release", err)
		}
		if skuID <= 0 || quantity <= 0 {
			return nil, nil, apperr.Failed("order item for refund release has an invalid sku/quantity").
				WithDetails(map[string]any{"order_no": orderNo, "sku_id": skuID, "quantity": quantity})
		}
		if warehouseID <= 0 {
			// AGENTS §4 / constitution Article III.3: a release must carry the
			// real warehouse, never default to W1.
			return nil, nil, apperr.Failed("order item for refund release is missing warehouse_id").
				WithDetails(map[string]any{"order_no": orderNo, "sku_id": skuID})
		}
		if strings.TrimSpace(groupNo) == "" {
			// A line without a group would force a guessed G1 and hide a broken
			// group projection. Fail closed instead (design §2.1/§7).
			return nil, nil, apperr.Failed("order item for refund release is missing fulfillment_group_no").
				WithDetails(map[string]any{"order_no": orderNo, "sku_id": skuID})
		}
		groupNo = permitGroup(groupNo)
		groups[groupNo] = struct{}{}
		quantities[[2]int64{skuID, warehouseID}] += quantity
	}
	if err := rows.Err(); err != nil {
		return nil, nil, apperr.Internal("iterate order items for refund release", err)
	}
	if len(quantities) == 0 {
		return nil, nil, apperr.Failed("order has no order items to release").
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
	groupNos := make([]string, 0, len(groups))
	for groupNo := range groups {
		groupNos = append(groupNos, groupNo)
	}
	sort.Strings(groupNos)
	return items, groupNos, nil
}

// loadRefundReleaseGroups merges the persisted fulfillment groups with the
// groups observed on the frozen lines. A group without a row is treated as
// active (fail-safe): we must not release on behalf of an unknown group.
func loadRefundReleaseGroups(ctx context.Context, q querier, orderNo string, itemGroups []string) ([]refundReleaseGroupState, error) {
	rows, err := q.QueryContext(ctx, `SELECT g.group_no, g.status, g.refund_state,
			COALESCE(p.state, 'NONE')
		FROM order_fulfillment_groups g
		LEFT JOIN order_shipping_permits p
		  ON p.order_no = g.order_no AND p.fulfillment_group_no = g.group_no
		WHERE g.order_no = ? ORDER BY g.group_no`, orderNo)
	if err != nil {
		return nil, apperr.Internal("load fulfillment groups for refund release", err)
	}
	defer rows.Close()
	states := make(map[string]refundReleaseGroupState)
	for rows.Next() {
		var state refundReleaseGroupState
		if err := rows.Scan(&state.groupNo, &state.status, &state.refundState, &state.permitState); err != nil {
			return nil, apperr.Internal("scan fulfillment group for refund release", err)
		}
		states[state.groupNo] = state
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal("iterate fulfillment groups for refund release", err)
	}
	for _, groupNo := range itemGroups {
		if _, ok := states[groupNo]; !ok {
			states[groupNo] = refundReleaseGroupState{groupNo: groupNo, status: "UNKNOWN", refundState: refundStateNone, permitState: permitNone}
		}
	}
	out := make([]refundReleaseGroupState, 0, len(states))
	for _, state := range states {
		out = append(out, state)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].groupNo < out[j].groupNo })
	return out, nil
}

func refundReleaseGroupIsPOST(g refundReleaseGroupState) bool {
	switch g.status {
	case statusShipped, "RECEIVED", statusCompleted:
		return true
	}
	return g.permitState == permitConsumed
}

// refundReleaseGroupIsActive follows the design's active definition (status
// not CANCELLED and not fully refunded). REFUNDING stays active: its refund was
// admitted but has not completed, so its reservation may still need the stock.
func refundReleaseGroupIsActive(g refundReleaseGroupState) bool {
	if g.status == groupStatusCancelled {
		return false
	}
	switch g.refundState {
	case groupRefundStateRefunded, refundStateFull:
		return false
	}
	return true
}

// planRefundRelease decides whether the triggering group is the last active
// group of the order and therefore allowed to request the full release.
func planRefundRelease(ctx context.Context, q querier, orderNo, currentGroup string) (refundReleasePlan, error) {
	items, itemGroups, err := loadOrderReleaseItems(ctx, q, orderNo)
	if err != nil {
		return refundReleasePlan{}, err
	}
	groups, err := loadRefundReleaseGroups(ctx, q, orderNo, itemGroups)
	if err != nil {
		return refundReleasePlan{}, err
	}
	covered := make([]string, 0, len(groups))
	activeOthers := make([]string, 0, len(groups))
	blocked := make([]string, 0)
	current := refundReleaseGroupState{groupNo: currentGroup, status: "UNKNOWN", refundState: refundStateNone, permitState: permitNone}
	foundCurrent := false
	for _, group := range groups {
		covered = append(covered, group.groupNo)
		if group.groupNo == currentGroup {
			current = group
			foundCurrent = true
		}
	}
	if !foundCurrent {
		covered = append(covered, currentGroup)
	}
	sort.Strings(covered)
	if refundReleaseGroupIsPOST(current) {
		// The admission result was PRE but the group looks shipped now. A full
		// release could touch already departed/restocked inventory.
		return refundReleasePlan{manual: true, blockedGroups: []string{currentGroup}, coveredGroupNos: covered, items: items}, nil
	}
	for _, group := range groups {
		if group.groupNo == currentGroup {
			continue
		}
		if refundReleaseGroupIsPOST(group) {
			blocked = append(blocked, group.groupNo)
			continue
		}
		if refundReleaseGroupIsActive(group) {
			activeOthers = append(activeOthers, group.groupNo)
		}
	}
	if len(blocked) > 0 {
		return refundReleasePlan{manual: true, blockedGroups: blocked, coveredGroupNos: covered, items: items}, nil
	}
	if len(activeOthers) > 0 {
		// Deferred: another active group may still ship, so Inventory must keep
		// the order reservation until the last active group refunds.
		return refundReleasePlan{coveredGroupNos: covered, items: items}, nil
	}
	return refundReleasePlan{emit: true, coveredGroupNos: covered, items: items}, nil
}

// hasOrderReleaseEventTx reports whether this order already has a full-release
// event. Every writer holds the order row lock first (admission and scan), so a
// plain read is serialized per order without taking a table-wide lock on the
// outbox.
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

// appendRefundReleaseTx emits the order-level full release when this admission
// is allowed to. It is a no-op for a deferred group (the scan retries later)
// and for an order that already has a release event. It never errors for the
// manual/POST case: the admission must still commit, but the scan logs the
// blocked groups for manual/AfterSales handling.
func (s *Service) appendRefundReleaseTx(ctx context.Context, tx *sql.Tx, refundID, orderNo, groupNo, source string) error {
	releaseSource, reason, err := refundReleaseSource(source)
	if err != nil {
		return err
	}
	plan, err := planRefundRelease(ctx, tx, orderNo, groupNo)
	if err != nil {
		return err
	}
	if plan.manual {
		s.log.Warn("refund release requires manual/after-sales handling",
			"order_no", orderNo, "group_no", groupNo, "refund_id", refundID,
			"blocked_groups", plan.blockedGroups)
		return nil
	}
	if !plan.emit {
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
		SchemaVersion:   1,
		ReleaseScope:    events.RefundReleaseScopeOrderReservationFull,
		ReleaseID:       events.RefundReleaseID(refundID),
		RefundID:        refundID,
		OrderNo:         orderNo,
		GroupNo:         groupNo,
		CoveredGroupNos: plan.coveredGroupNos,
		Source:          releaseSource,
		Reason:          reason,
		Items:           plan.items,
		RequestedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	return outbox.Append(ctx, tx, outbox.Event{
		Topic:         events.RefundReleaseTopic,
		AggregateType: "order",
		AggregateID:   orderNo,
		EventType:     events.RefundReleaseEventType,
		Payload:       payload,
	})
}

// deferredRefundReleaseSource reconstructs the source of a PRE admission that
// did not emit at admission time. TCC and zero refunds have a
// refund_applications row; a resolved shortage case identifies system_shortage;
// everything else is an ordinary after-sale refund.
func (s *Service) deferredRefundReleaseSource(ctx context.Context, q querier, refundID, orderNo, groupNo string) (string, error) {
	var source string
	err := q.QueryRowContext(ctx, `SELECT source FROM refund_applications WHERE refund_id = ?`, refundID).Scan(&source)
	if err == nil {
		return source, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", apperr.Internal("load refund application source for deferred release", err)
	}
	var shortageStatus string
	err = q.QueryRowContext(ctx, `SELECT status FROM order_shortage_cases
		WHERE order_no = ? AND group_no = ? ORDER BY id DESC LIMIT 1`, orderNo, groupNo).Scan(&shortageStatus)
	if err == nil && shortageStatus == shortageResolvedRefund {
		return events.RefundReleaseSourceSystemShortage, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", apperr.Internal("load shortage case for deferred release", err)
	}
	return events.RefundReleaseSourceAfterSale, nil
}

type pendingRefundRelease struct {
	refundID string
	orderNo  string
	groupNo  string
}

// scanPendingRefundReleases finds PRE admissions whose order has no release
// event yet. It is bounded and idempotent; the per-order transaction re-checks
// the event and group state before appending.
//
// B3 is deliberately not a production backfill: no release receipt table and no
// migration exist. Existing PRE admissions are converged by this scan once their
// group is last active. Dev/UAT may run one manual pass (restart the worker or
// call the scan) instead of a data migration.
func (s *Service) scanPendingRefundReleases(ctx context.Context) ([]pendingRefundRelease, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT a.refund_id, a.order_no, a.group_no
		FROM order_refund_admissions a
		WHERE a.result = ?
		  AND NOT EXISTS (
			SELECT 1 FROM outbox_events e
			WHERE e.event_type = ? AND e.aggregate_id = a.order_no
		  )
		ORDER BY a.updated_at, a.refund_id
		LIMIT ?`, refundResultPre, events.RefundReleaseEventType, refundReleaseScanLimit)
	if err != nil {
		return nil, apperr.Internal("scan pending refund releases", err)
	}
	defer rows.Close()
	var pending []pendingRefundRelease
	for rows.Next() {
		var item pendingRefundRelease
		if err := rows.Scan(&item.refundID, &item.orderNo, &item.groupNo); err != nil {
			return nil, apperr.Internal("scan pending refund release row", err)
		}
		pending = append(pending, item)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal("iterate pending refund releases", err)
	}
	return pending, nil
}

func (s *Service) reconcileRefundRelease(ctx context.Context, pending pendingRefundRelease) error {
	// READ COMMITTED: after waiting for the order row lock, the re-check must see
	// a release event committed by the other admission/scan.
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return apperr.Internal("begin deferred refund release", err)
	}
	defer func() { _ = tx.Rollback() }()
	locked, err := findOrderForUpdate(ctx, tx, pending.orderNo)
	if err != nil {
		return err
	}
	if locked == nil {
		return nil
	}
	exists, err := hasOrderReleaseEventTx(ctx, tx, pending.orderNo)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	admission, err := findRefundAdmissionLocked(ctx, tx, pending.refundID)
	if err != nil {
		return apperr.Internal("load pending refund admission", err)
	}
	if admission == nil || admission.Result != refundResultPre {
		return nil
	}
	source, err := s.deferredRefundReleaseSource(ctx, tx, pending.refundID, pending.orderNo, pending.groupNo)
	if err != nil {
		return err
	}
	if err := s.appendRefundReleaseTx(ctx, tx, pending.refundID, pending.orderNo, pending.groupNo, source); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return apperr.Internal("commit deferred refund release", err)
	}
	return nil
}

// runRefundReleaseReconcileOnce scans a bounded page and emits releases for
// orders where the triggering group has become the last active group. A failure
// on one order is logged and does not stop the others.
func (s *Service) runRefundReleaseReconcileOnce(ctx context.Context) error {
	pending, err := s.scanPendingRefundReleases(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, item := range pending {
		if err := s.reconcileRefundRelease(ctx, item); err != nil {
			s.log.Error("deferred refund release failed",
				"order_no", item.orderNo, "group_no", item.groupNo, "refund_id", item.refundID, "error", err)
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// RunRefundReleaseReconcileWorker periodically retries deferred order-level
// releases. It is safe on multiple replicas: every scan locks the order row and
// re-checks the outbox, so only one full release can be appended per order.
func RunRefundReleaseReconcileWorker(ctx context.Context, svc *Service, logger *slog.Logger, interval time.Duration) {
	if svc == nil {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	if interval <= 0 {
		interval = refundReleaseDefaultInterval
	}
	if err := svc.runRefundReleaseReconcileOnce(ctx); err != nil {
		logger.Error("refund release reconcile worker", "error", err)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := svc.runRefundReleaseReconcileOnce(ctx); err != nil {
				logger.Error("refund release reconcile worker", "error", err)
			}
		}
	}
}
