package inventory

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/db"
	"gin-kratos-gorm/internal/platform/dtm"
	"gin-kratos-gorm/internal/platform/id"
	"gin-kratos-gorm/internal/platform/outbox"

	"github.com/go-sql-driver/mysql"
)

// Whole-order operation protocol. A reservation remains the per-SKU unit of
// truth; an operation records deterministic scope plus per-item progress so a
// redelivery can skip completed items instead of retrying the whole batch.
const (
	opTypeConfirm = "CONFIRM"
	opTypeCancel  = "CANCEL"

	opStatusPending    = "PENDING"
	opStatusInProgress = "IN_PROGRESS"
	opStatusSucceeded  = "SUCCEEDED"
	opStatusPartial    = "PARTIAL"

	itemStatusPending         = "PENDING"
	itemStatusDone            = "DONE"
	itemStatusFailedPermanent = "FAILED_PERMANENT"
	itemStatusMissing         = "MISSING"

	// maxOperationItemAttempts bounds per-item retries before it becomes a
	// permanent failure. A redelivery after the budget is exhausted must not
	// starve the remaining items forever.
	maxOperationItemAttempts = 8
	maxOperationErrorLength  = 512

	topicOperationCompleted  = "shop.inventory.operation.completed.v1"
	topicOperationFailed     = "shop.inventory.operation.failed.v1"
	topicOperationItemFailed = "shop.inventory.operation.item_failed.v1"

	eventOperationCompleted  = "inventory.operation_completed"
	eventOperationFailed     = "inventory.operation_failed"
	eventOperationItemFailed = "inventory.operation_item_failed"
)

var (
	// errReservationMissing distinguishes a missing reservation from a missing
	// stock row. Cancel treats the former as a terminal skip; confirm treats it
	// as a permanent failure.
	errReservationMissing = errors.New("inventory: reservation missing")
	// errReservationReleased marks a confirm against a released reservation.
	errReservationReleased = errors.New("inventory: reservation already released")
	// errReservationChanged marks a status CAS lost to a concurrent writer.
	errReservationChanged = errors.New("inventory: reservation changed concurrently")
	// errOperationProgressRace means an item/operation update did not apply
	// because another transaction changed it. The item transaction rolls back.
	errOperationProgressRace = errors.New("inventory: operation progress changed concurrently")
)

type operationItemInput struct {
	skuID       int64
	warehouseID int64
	quantity    int64
}

type operationSpec struct {
	typ         string
	orderNo     string
	scopeHash   string
	operationID string
	items       []operationItemInput
}

type operationItem struct {
	skuID       int64
	warehouseID int64
	quantity    int64
	status      string
	attempts    int
	lastError   string
}

type operation struct {
	id        string
	typ       string
	orderNo   string
	scopeHash string
	status    string

	total     int
	succeeded int
	failed    int
	skipped   int
	version   int64

	items []operationItem
}

type operationSummary struct {
	OperationID string
	Status      string
	Total       int
	Succeeded   int
	Failed      int
	Skipped     int
	Retryable   int
}

type itemOutcome string

const (
	outcomeDone      itemOutcome = "DONE"
	outcomeMissing   itemOutcome = "MISSING"
	outcomePermanent itemOutcome = "FAILED_PERMANENT"
	outcomeTransient itemOutcome = "TRANSIENT"
)

type itemResult struct {
	outcome itemOutcome
	reason  string
}

// newOperationSpec validates and canonicalizes the item set. The operation ID
// is derived from (type, order_no, scope_hash), never from order_no alone:
// a partial payment attempted set and a later whole-order cancel are different
// operations and must not share progress.
func newOperationSpec(typ, orderNo string, items []*pb.OrderItem) (operationSpec, error) {
	typ = strings.ToUpper(strings.TrimSpace(typ))
	if typ != opTypeConfirm && typ != opTypeCancel {
		return operationSpec{}, apperr.Invalid("operation type must be CONFIRM or CANCEL")
	}
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return operationSpec{}, apperr.Invalid("order_no is required")
	}
	if len(orderNo) > maxOrderNoLength {
		return operationSpec{}, apperr.Invalid("order_no must be at most %d characters", maxOrderNoLength)
	}
	if len(items) == 0 {
		return operationSpec{}, apperr.Invalid("items are required")
	}
	normalized := make([]operationItemInput, 0, len(items))
	index := make(map[[2]int64]int, len(items))
	for _, item := range items {
		if item == nil {
			return operationSpec{}, apperr.Invalid("inventory operation item is nil")
		}
		skuID := item.GetSkuId()
		warehouseID := normalizeWarehouse(item.GetWarehouseId())
		quantity := int64(item.GetQuantity())
		if skuID <= 0 {
			return operationSpec{}, apperr.Invalid("sku_id is required")
		}
		if quantity <= 0 {
			return operationSpec{}, apperr.Invalid("quantity must be positive for sku %d", skuID)
		}
		key := [2]int64{skuID, warehouseID}
		if i, ok := index[key]; ok {
			normalized[i].quantity += quantity
			continue
		}
		index[key] = len(normalized)
		normalized = append(normalized, operationItemInput{skuID: skuID, warehouseID: warehouseID, quantity: quantity})
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].skuID == normalized[j].skuID {
			return normalized[i].warehouseID < normalized[j].warehouseID
		}
		return normalized[i].skuID < normalized[j].skuID
	})
	var canonical strings.Builder
	for i, item := range normalized {
		if i > 0 {
			canonical.WriteByte(',')
		}
		fmt.Fprintf(&canonical, "%d:%d:%d", item.skuID, item.warehouseID, item.quantity)
	}
	sum := sha256.Sum256([]byte(canonical.String()))
	scopeHash := hex.EncodeToString(sum[:])
	return operationSpec{
		typ:         typ,
		orderNo:     orderNo,
		scopeHash:   scopeHash,
		operationID: operationIDFor(typ, orderNo, scopeHash),
		items:       normalized,
	}, nil
}

func operationIDFor(typ, orderNo, scopeHash string) string {
	// "inv:" + 7-char type + 64-char order + ":" + 32-char scope = at most 109.
	return "inv:v2:" + strings.ToLower(typ) + ":" + orderNo + ":" + scopeHash[:32]
}

// applyInventoryOperation is the entry point used by consumers. It routes DTM
// branches to one atomic branch transaction and everything else to the
// per-item resume loop.
func (s *Service) applyInventoryOperation(ctx context.Context, typ, orderNo string, items []*pb.OrderItem) (operationSummary, error) {
	spec, err := newOperationSpec(typ, orderNo, items)
	if err != nil {
		return operationSummary{}, err
	}
	return s.runOperation(ctx, spec)
}

func (s *Service) runOperation(ctx context.Context, spec operationSpec) (operationSummary, error) {
	if dtm.InBranch(ctx) {
		return s.runOperationInBranch(ctx, spec)
	}
	return s.runOperationLocal(ctx, spec)
}

// runOperationInBranch is the DTM path: one barrier transaction covers every
// item, so a partial branch cannot commit. A prior consumer may already have
// committed some items; those are skipped and the rest are applied atomically.
func (s *Service) runOperationInBranch(ctx context.Context, spec operationSpec) (operationSummary, error) {
	var summary operationSummary
	err := s.write(ctx, func(tx *sql.Tx) error {
		return s.runOperationTx(ctx, tx, spec, &summary)
	})
	if err != nil {
		return operationSummary{}, err
	}
	// A DTM barrier replay can skip the callback entirely, leaving the local
	// summary zeroed. Always read the committed operation back so a retried
	// confirm does not report success for a previous PARTIAL branch.
	committed, err := s.loadOperationSummary(ctx, spec.operationID)
	if err != nil {
		return operationSummary{}, apperr.Internal("load inventory operation after branch", err)
	}
	if committed.Retryable > 0 {
		return committed, apperr.Internal("terminal inventory operation still has pending items", nil)
	}
	return committed, nil
}

// runOperationTx applies every pending item inside one caller-owned
// transaction. It is the DTM branch body and is also used by the atomicity
// test; it never opens a nested transaction.
func (s *Service) runOperationTx(ctx context.Context, tx *sql.Tx, spec operationSpec, summary *operationSummary) error {
	op, err := s.ensureOperationTx(ctx, tx, spec)
	if err != nil {
		return err
	}
	if op.terminal() {
		*summary = op.summary()
		if summary.Retryable > 0 {
			return apperr.Internal("terminal inventory operation still has pending items", nil)
		}
		return nil
	}
	for i := range op.items {
		item := &op.items[i]
		if item.status != itemStatusPending {
			continue
		}
		result, err := s.applyItemBusinessTx(ctx, tx, op, item)
		if err != nil {
			return err
		}
		switch result.outcome {
		case outcomeDone:
			if err := s.markItemDoneTx(ctx, tx, op, item); err != nil {
				return err
			}
		case outcomeMissing:
			if op.typ != opTypeCancel {
				return apperr.NotFound(fmt.Sprintf("reservation missing for order %s sku %d", op.orderNo, item.skuID))
			}
			if err := s.markItemTerminalTx(ctx, tx, op, item, itemStatusMissing, result.reason); err != nil {
				return err
			}
		case outcomeTransient:
			return apperr.Unavailable("inventory operation item is retryable", errors.New(result.reason))
		default:
			return apperr.Failed("inventory operation item failed: " + result.reason)
		}
	}
	*summary = op.summary()
	return nil
}

// runOperationLocal processes each initially pending item once, in its own
// transaction. Business outcomes are committed as progress; only infrastructure
// errors abort the loop. A redelivery skips DONE/FAILED/MISSING items.
func (s *Service) runOperationLocal(ctx context.Context, spec operationSpec) (operationSummary, error) {
	op, err := s.ensureOperation(ctx, spec)
	if err != nil {
		return operationSummary{}, err
	}
	if op.terminal() {
		summary := op.summary()
		if summary.Retryable > 0 {
			return summary, apperr.Internal("terminal inventory operation still has pending items", nil)
		}
		return summary, nil
	}
	pending := make([]operationItem, 0, len(op.items))
	for _, item := range op.items {
		if item.status == itemStatusPending {
			pending = append(pending, item)
		}
	}
	var firstErr error
	for _, item := range pending {
		txErr := s.withLocalTxRetry(ctx, func(tx *sql.Tx) error {
			current, err := s.loadOperationTx(ctx, tx, spec.operationID, true)
			if err != nil {
				return err
			}
			if current.terminal() {
				return nil
			}
			target := current.item(item.skuID, item.warehouseID)
			if target == nil || target.status != itemStatusPending {
				return nil
			}
			if _, err := tx.ExecContext(ctx, "SAVEPOINT inventory_operation_item"); err != nil {
				return apperr.Internal("create operation item savepoint", err)
			}
			result, err := s.applyItemBusinessTx(ctx, tx, current, target)
			if err != nil {
				return err
			}
			if result.outcome == outcomeDone {
				if _, err := tx.ExecContext(ctx, "RELEASE SAVEPOINT inventory_operation_item"); err != nil {
					return apperr.Internal("release operation item savepoint", err)
				}
			} else {
				// A transient/permanent outcome may have written part of the
				// business mutation before failing; keep the progress update
				// but roll the item's stock/ledger writes back.
				if _, err := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT inventory_operation_item"); err != nil {
					return apperr.Internal("rollback operation item savepoint", err)
				}
				if _, err := tx.ExecContext(ctx, "RELEASE SAVEPOINT inventory_operation_item"); err != nil {
					return apperr.Internal("release operation item savepoint", err)
				}
			}
			return s.recordOutcomeTx(ctx, tx, current, target, result)
		})
		if txErr != nil {
			firstErr = txErr
			break
		}
	}
	summary, err := s.loadOperationSummary(ctx, spec.operationID)
	if err != nil {
		return operationSummary{}, err
	}
	if firstErr != nil {
		return summary, firstErr
	}
	if summary.Retryable > 0 {
		return summary, apperr.Unavailable("inventory operation has retryable items", nil)
	}
	if summary.Status != opStatusSucceeded && summary.Status != opStatusPartial {
		return summary, apperr.Internal("inventory operation is not terminal but has no pending items", nil)
	}
	return summary, nil
}

func (s *Service) applyItemBusinessTx(ctx context.Context, tx *sql.Tx, op *operation, item *operationItem) (itemResult, error) {
	switch op.typ {
	case opTypeConfirm:
		_, err := s.confirmTx(ctx, tx, reserveInput{
			orderNo: op.orderNo, skuID: item.skuID, warehouseID: item.warehouseID, quantity: item.quantity,
		})
		if err == nil {
			return itemResult{outcome: outcomeDone}, nil
		}
		return classifyConfirmItemError(err)
	case opTypeCancel:
		_, err := s.cancelReservationTx(ctx, tx, op.orderNo, item.skuID, item.warehouseID, item.quantity)
		if err == nil {
			return itemResult{outcome: outcomeDone}, nil
		}
		return classifyCancelItemError(err)
	default:
		return itemResult{}, apperr.Internal("unknown inventory operation type", fmt.Errorf("%q", op.typ))
	}
}

func classifyConfirmItemError(err error) (itemResult, error) {
	reason := operationErrorReason(err)
	switch {
	case errors.Is(err, errReservationMissing):
		return itemResult{outcome: outcomePermanent, reason: reason}, nil
	case errors.Is(err, errReservationReleased):
		return itemResult{outcome: outcomePermanent, reason: reason}, nil
	case errors.Is(err, errReservationChanged):
		return itemResult{outcome: outcomeTransient, reason: reason}, nil
	case apperr.IsCode(err, apperr.CodeInvalidArgument):
		return itemResult{outcome: outcomePermanent, reason: reason}, nil
	case apperr.IsCode(err, apperr.CodeConflict):
		return itemResult{outcome: outcomePermanent, reason: reason}, nil
	case apperr.IsCode(err, apperr.CodeNotFound):
		return itemResult{outcome: outcomePermanent, reason: reason}, nil
	case apperr.IsCode(err, apperr.CodeFailedPrecondition):
		return itemResult{outcome: outcomeTransient, reason: reason}, nil
	default:
		// Internal/unavailable/DB errors must roll the item transaction back.
		return itemResult{}, err
	}
}

func classifyCancelItemError(err error) (itemResult, error) {
	reason := operationErrorReason(err)
	switch {
	case errors.Is(err, errReservationMissing):
		return itemResult{outcome: outcomeMissing, reason: reason}, nil
	case errors.Is(err, errReservationChanged):
		return itemResult{outcome: outcomeTransient, reason: reason}, nil
	case apperr.IsCode(err, apperr.CodeInvalidArgument):
		return itemResult{outcome: outcomePermanent, reason: reason}, nil
	case apperr.IsCode(err, apperr.CodeConflict):
		return itemResult{outcome: outcomePermanent, reason: reason}, nil
	case apperr.IsCode(err, apperr.CodeNotFound):
		// The reservation exists but its stock row does not: a data problem,
		// not an idempotent "nothing to release".
		return itemResult{outcome: outcomePermanent, reason: reason}, nil
	case apperr.IsCode(err, apperr.CodeFailedPrecondition):
		return itemResult{outcome: outcomeTransient, reason: reason}, nil
	default:
		return itemResult{}, err
	}
}

func (s *Service) recordOutcomeTx(ctx context.Context, tx *sql.Tx, op *operation, item *operationItem, result itemResult) error {
	switch result.outcome {
	case outcomeDone:
		return s.markItemDoneTx(ctx, tx, op, item)
	case outcomeMissing:
		return s.markItemTerminalTx(ctx, tx, op, item, itemStatusMissing, result.reason)
	case outcomePermanent:
		return s.markItemTerminalTx(ctx, tx, op, item, itemStatusFailedPermanent, result.reason)
	case outcomeTransient:
		return s.markItemRetryTx(ctx, tx, op, item, result.reason)
	default:
		return apperr.Internal("unknown operation item outcome", fmt.Errorf("%q", result.outcome))
	}
}

func (s *Service) markItemDoneTx(ctx context.Context, tx *sql.Tx, op *operation, item *operationItem) error {
	attempts := item.attempts + 1
	res, err := tx.ExecContext(ctx, `UPDATE inventory_operation_items
		SET status = ?, attempts = ?, last_error = '', updated_at = NOW(3)
		WHERE operation_id = ? AND sku_id = ? AND warehouse_id = ? AND status = ?`,
		itemStatusDone, attempts, op.id, item.skuID, item.warehouseID, itemStatusPending)
	if err != nil {
		return apperr.Internal("mark operation item done", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return errOperationProgressRace
	}
	res, err = tx.ExecContext(ctx, `UPDATE inventory_operations
		SET succeeded_items = succeeded_items + 1,
		    status = CASE WHEN status = ? THEN ? ELSE status END,
		    version = version + 1, updated_at = NOW(3)
		WHERE operation_id = ? AND status IN (?, ?)`,
		opStatusPending, opStatusInProgress, op.id, opStatusPending, opStatusInProgress)
	if err != nil {
		return apperr.Internal("mark operation item succeeded", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return errOperationProgressRace
	}
	item.status = itemStatusDone
	item.attempts = attempts
	item.lastError = ""
	op.succeeded++
	if op.status == opStatusPending {
		op.status = opStatusInProgress
	}
	op.version++
	return s.finalizeIfCompleteTx(ctx, tx, op)
}

func (s *Service) markItemTerminalTx(ctx context.Context, tx *sql.Tx, op *operation, item *operationItem, status, reason string) error {
	attempts := item.attempts + 1
	reason = truncateOperationError(reason)
	res, err := tx.ExecContext(ctx, `UPDATE inventory_operation_items
		SET status = ?, attempts = ?, last_error = ?, updated_at = NOW(3)
		WHERE operation_id = ? AND sku_id = ? AND warehouse_id = ? AND status = ?`,
		status, attempts, reason, op.id, item.skuID, item.warehouseID, itemStatusPending)
	if err != nil {
		return apperr.Internal("mark operation item terminal", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return errOperationProgressRace
	}
	if status == itemStatusMissing {
		res, err = tx.ExecContext(ctx, `UPDATE inventory_operations
			SET skipped_items = skipped_items + 1,
			    status = CASE WHEN status = ? THEN ? ELSE status END,
			    version = version + 1, updated_at = NOW(3)
			WHERE operation_id = ? AND status IN (?, ?)`,
			opStatusPending, opStatusInProgress, op.id, opStatusPending, opStatusInProgress)
	} else {
		res, err = tx.ExecContext(ctx, `UPDATE inventory_operations
			SET failed_items = failed_items + 1,
			    status = CASE WHEN status = ? THEN ? ELSE status END,
			    version = version + 1, updated_at = NOW(3)
			WHERE operation_id = ? AND status IN (?, ?)`,
			opStatusPending, opStatusInProgress, op.id, opStatusPending, opStatusInProgress)
	}
	if err != nil {
		return apperr.Internal("mark operation item failure", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return errOperationProgressRace
	}
	item.status = status
	item.attempts = attempts
	item.lastError = reason
	if status == itemStatusMissing {
		op.skipped++
	} else {
		op.failed++
	}
	if op.status == opStatusPending {
		op.status = opStatusInProgress
	}
	op.version++
	if err := s.appendItemFailedEvent(ctx, tx, op, item, status, reason, attempts); err != nil {
		return err
	}
	return s.finalizeIfCompleteTx(ctx, tx, op)
}

func (s *Service) markItemRetryTx(ctx context.Context, tx *sql.Tx, op *operation, item *operationItem, reason string) error {
	attempts := item.attempts + 1
	reason = truncateOperationError(reason)
	if attempts >= maxOperationItemAttempts {
		return s.markItemTerminalTx(ctx, tx, op, item, itemStatusFailedPermanent,
			"retry budget exhausted: "+reason)
	}
	res, err := tx.ExecContext(ctx, `UPDATE inventory_operation_items
		SET attempts = ?, last_error = ?, updated_at = NOW(3)
		WHERE operation_id = ? AND sku_id = ? AND warehouse_id = ? AND status = ?`,
		attempts, reason, op.id, item.skuID, item.warehouseID, itemStatusPending)
	if err != nil {
		return apperr.Internal("mark operation item retry", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return errOperationProgressRace
	}
	res, err = tx.ExecContext(ctx, `UPDATE inventory_operations
		SET status = CASE WHEN status = ? THEN ? ELSE status END,
		    version = version + 1, updated_at = NOW(3)
		WHERE operation_id = ? AND status IN (?, ?)`,
		opStatusPending, opStatusInProgress, op.id, opStatusPending, opStatusInProgress)
	if err != nil {
		return apperr.Internal("mark operation in progress", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return errOperationProgressRace
	}
	item.attempts = attempts
	item.lastError = reason
	if op.status == opStatusPending {
		op.status = opStatusInProgress
	}
	op.version++
	return nil
}

func (s *Service) finalizeIfCompleteTx(ctx context.Context, tx *sql.Tx, op *operation) error {
	if op.terminal() {
		return nil
	}
	if op.succeeded+op.failed+op.skipped != op.total {
		return nil
	}
	for _, item := range op.items {
		if item.status == itemStatusPending {
			return nil
		}
	}
	status := opStatusSucceeded
	if op.failed+op.skipped > 0 {
		status = opStatusPartial
	}
	res, err := tx.ExecContext(ctx, `UPDATE inventory_operations
		SET status = ?, version = version + 1, updated_at = NOW(3)
		WHERE operation_id = ? AND status IN (?, ?)`,
		status, op.id, opStatusPending, opStatusInProgress)
	if err != nil {
		return apperr.Internal("finalize inventory operation", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// Another transaction finalized first; it also emitted the event.
		return nil
	}
	op.status = status
	op.version++
	return s.appendOperationTerminalEvent(ctx, tx, op, status)
}

func (s *Service) appendOperationTerminalEvent(ctx context.Context, tx *sql.Tx, op *operation, status string) error {
	topic, eventType := topicOperationCompleted, eventOperationCompleted
	if status == opStatusPartial {
		topic, eventType = topicOperationFailed, eventOperationFailed
	}
	failedItems := make([]map[string]any, 0, op.failed+op.skipped)
	for _, item := range op.items {
		if item.status == itemStatusDone {
			continue
		}
		failedItems = append(failedItems, map[string]any{
			"sku_id": item.skuID, "quantity": item.quantity,
			"status": item.status, "reason": item.lastError, "attempts": item.attempts,
		})
	}
	return outbox.Append(ctx, tx, outbox.Event{
		ID:            id.Event(),
		Topic:         topic,
		AggregateType: "inventory_operation",
		AggregateID:   operationAggregateID(op.id),
		EventType:     eventType,
		Payload: map[string]any{
			"operation_id": op.id, "operation_type": op.typ, "order_no": op.orderNo,
			"scope_hash": op.scopeHash, "status": status,
			"total_items": op.total, "succeeded_items": op.succeeded,
			"failed_items": op.failed, "skipped_items": op.skipped,
			"item_details": failedItems, "completed_at": time.Now().Unix(),
		},
	})
}

func (s *Service) appendItemFailedEvent(ctx context.Context, tx *sql.Tx, op *operation, item *operationItem, status, reason string, attempts int) error {
	return outbox.Append(ctx, tx, outbox.Event{
		ID:            id.Event(),
		Topic:         topicOperationItemFailed,
		AggregateType: "inventory_operation",
		AggregateID:   operationAggregateID(op.id),
		EventType:     eventOperationItemFailed,
		Payload: map[string]any{
			"operation_id": op.id, "operation_type": op.typ, "order_no": op.orderNo,
			"sku_id": item.skuID, "warehouse_id": item.warehouseID, "quantity": item.quantity,
			"outcome": status, "reason": reason, "attempts": attempts,
			"failed_at": time.Now().Unix(),
		},
	})
}

// operationAggregateID keeps outbox_events.aggregate_id within VARCHAR(64)
// while the payload carries the full operation ID.
func operationAggregateID(operationID string) string {
	sum := sha256.Sum256([]byte(operationID))
	return "invop:" + hex.EncodeToString(sum[:])[:32]
}

func operationErrorReason(err error) string {
	if err == nil {
		return ""
	}
	return truncateOperationError(err.Error())
}

func truncateOperationError(reason string) string {
	reason = strings.TrimSpace(reason)
	if len(reason) <= maxOperationErrorLength {
		return reason
	}
	runes := []rune(reason)
	if len(runes) > maxOperationErrorLength {
		runes = runes[:maxOperationErrorLength]
	}
	return string(runes)
}

func (o *operation) terminal() bool {
	return o.status == opStatusSucceeded || o.status == opStatusPartial
}

func (o *operation) item(skuID, warehouseID int64) *operationItem {
	for i := range o.items {
		if o.items[i].skuID == skuID && (warehouseID == 0 || o.items[i].warehouseID == warehouseID) {
			return &o.items[i]
		}
	}
	return nil
}

func (o *operation) summary() operationSummary {
	retryable := 0
	for _, item := range o.items {
		if item.status == itemStatusPending {
			retryable++
		}
	}
	return operationSummary{
		OperationID: o.id, Status: o.status, Total: o.total,
		Succeeded: o.succeeded, Failed: o.failed, Skipped: o.skipped, Retryable: retryable,
	}
}

func (s *Service) ensureOperation(ctx context.Context, spec operationSpec) (*operation, error) {
	var op *operation
	err := s.withLocalTxRetry(ctx, func(tx *sql.Tx) error {
		var err error
		op, err = s.ensureOperationTx(ctx, tx, spec)
		return err
	})
	if err != nil {
		return nil, err
	}
	return op, nil
}

// withLocalTxRetry retries the two transient MySQL outcomes that are safe to
// replay because the callback is a full transaction: deadlock (1213) and lock
// wait timeout (1205). The operation/item updates are idempotent; DTM branches
// do not use this helper because DTM owns their retry loop.
func (s *Service) withLocalTxRetry(ctx context.Context, fn func(*sql.Tx) error) error {
	for attempt := 0; ; attempt++ {
		err := db.WithTx(ctx, s.db, fn)
		if err == nil || !isRetryableMySQLError(err) || attempt >= 2 {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 20 * time.Millisecond):
		}
	}
}

func isRetryableMySQLError(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && (me.Number == 1213 || me.Number == 1205)
}

func (s *Service) ensureOperationTx(ctx context.Context, tx *sql.Tx, spec operationSpec) (*operation, error) {
	if _, err := tx.ExecContext(ctx, `INSERT INTO inventory_operations
		(operation_id, operation_type, order_no, scope_hash, status, total_items,
		 succeeded_items, failed_items, skipped_items, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, 0, 0, 1, NOW(3), NOW(3))
		ON DUPLICATE KEY UPDATE operation_id = operation_id`,
		spec.operationID, spec.typ, spec.orderNo, spec.scopeHash, opStatusPending, len(spec.items)); err != nil {
		return nil, apperr.Internal("ensure inventory operation", err)
	}
	op, err := s.loadOperationTx(ctx, tx, spec.operationID, true)
	if err != nil {
		if apperr.IsCode(err, apperr.CodeNotFound) {
			return nil, apperr.Conflict("inventory operation scope is bound to a different operation id")
		}
		return nil, err
	}
	if op.typ != spec.typ || op.orderNo != spec.orderNo || op.scopeHash != spec.scopeHash || op.total != len(spec.items) {
		return nil, apperr.Conflict("inventory operation scope mismatch")
	}
	for _, item := range spec.items {
		if _, err := tx.ExecContext(ctx, `INSERT INTO inventory_operation_items
			(operation_id, sku_id, warehouse_id, quantity, status, attempts, last_error, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 0, '', NOW(3), NOW(3))
			ON DUPLICATE KEY UPDATE operation_id = operation_id`,
			spec.operationID, item.skuID, item.warehouseID, item.quantity, itemStatusPending); err != nil {
			return nil, apperr.Internal("ensure inventory operation item", err)
		}
	}
	op, err = s.loadOperationTx(ctx, tx, spec.operationID, true)
	if err != nil {
		return nil, err
	}
	if len(op.items) != len(spec.items) {
		return nil, apperr.Conflict("inventory operation item set does not match its scope")
	}
	byKey := make(map[[2]int64]operationItem, len(op.items))
	for _, item := range op.items {
		byKey[[2]int64{item.skuID, item.warehouseID}] = item
	}
	for _, want := range spec.items {
		got, ok := byKey[[2]int64{want.skuID, want.warehouseID}]
		if !ok || got.quantity != want.quantity {
			return nil, apperr.Conflict("inventory operation item set does not match its scope")
		}
	}
	return op, nil
}

func (s *Service) loadOperation(ctx context.Context, operationID string) (*operation, error) {
	var op operation
	if err := s.db.QueryRowContext(ctx, `SELECT operation_id, operation_type, order_no, scope_hash,
		status, total_items, succeeded_items, failed_items, skipped_items, version
		FROM inventory_operations WHERE operation_id = ?`, operationID).
		Scan(&op.id, &op.typ, &op.orderNo, &op.scopeHash, &op.status,
			&op.total, &op.succeeded, &op.failed, &op.skipped, &op.version); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("inventory operation not found")
		}
		return nil, apperr.Internal("load inventory operation", err)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT sku_id, warehouse_id, quantity, status, attempts, last_error
		FROM inventory_operation_items WHERE operation_id = ? ORDER BY sku_id, warehouse_id`, operationID)
	if err != nil {
		return nil, apperr.Internal("load inventory operation items", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item operationItem
		if err := rows.Scan(&item.skuID, &item.warehouseID, &item.quantity, &item.status, &item.attempts, &item.lastError); err != nil {
			return nil, apperr.Internal("scan inventory operation item", err)
		}
		op.items = append(op.items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal("iterate inventory operation items", err)
	}
	return &op, nil
}

func (s *Service) loadOperationSummary(ctx context.Context, operationID string) (operationSummary, error) {
	op, err := s.loadOperation(ctx, operationID)
	if err != nil {
		return operationSummary{}, err
	}
	return op.summary(), nil
}

func (s *Service) loadOperationTx(ctx context.Context, tx *sql.Tx, operationID string, forUpdate bool) (*operation, error) {
	query := `SELECT operation_id, operation_type, order_no, scope_hash,
		status, total_items, succeeded_items, failed_items, skipped_items, version
		FROM inventory_operations WHERE operation_id = ?`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var op operation
	if err := tx.QueryRowContext(ctx, query, operationID).
		Scan(&op.id, &op.typ, &op.orderNo, &op.scopeHash, &op.status,
			&op.total, &op.succeeded, &op.failed, &op.skipped, &op.version); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("inventory operation not found")
		}
		return nil, apperr.Internal("load inventory operation", err)
	}
	itemQuery := `SELECT sku_id, warehouse_id, quantity, status, attempts, last_error
		FROM inventory_operation_items WHERE operation_id = ? ORDER BY sku_id, warehouse_id`
	if forUpdate {
		itemQuery += " FOR UPDATE"
	}
	rows, err := tx.QueryContext(ctx, itemQuery, operationID)
	if err != nil {
		return nil, apperr.Internal("load inventory operation items", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item operationItem
		if err := rows.Scan(&item.skuID, &item.warehouseID, &item.quantity, &item.status, &item.attempts, &item.lastError); err != nil {
			return nil, apperr.Internal("scan inventory operation item", err)
		}
		op.items = append(op.items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal("iterate inventory operation items", err)
	}
	return &op, nil
}

// ReplayInventoryOperation is the explicit recovery path for permanently failed
// or missing items. It never resets a DONE item: stock already moved must not be
// applied twice. There is intentionally no background scanner.
func (s *Service) replayOperation(ctx context.Context, operationID string, skus []int64) error {
	operationID = strings.TrimSpace(operationID)
	if operationID == "" {
		return apperr.Invalid("operation_id is required")
	}
	var spec operationSpec
	err := s.withLocalTxRetry(ctx, func(tx *sql.Tx) error {
		op, err := s.loadOperationTx(ctx, tx, operationID, true)
		if err != nil {
			return err
		}
		selected := skus
		if len(selected) == 0 {
			for _, item := range op.items {
				if item.status != itemStatusDone {
					selected = append(selected, item.skuID)
				}
			}
		}
		if len(selected) == 0 {
			return apperr.Conflict("inventory operation has no replayable items")
		}
		for _, sku := range selected {
			item := op.item(sku, 0)
			if item == nil {
				return apperr.NotFound(fmt.Sprintf("sku %d is not part of operation %s", sku, operationID))
			}
			if item.status == itemStatusDone {
				return apperr.Conflict(fmt.Sprintf("sku %d already completed; replay would double-apply stock", sku))
			}
			if item.status == itemStatusPending {
				continue
			}
			var counterSQL string
			switch item.status {
			case itemStatusFailedPermanent:
				counterSQL = `UPDATE inventory_operations SET failed_items = failed_items - 1, version = version + 1, updated_at = NOW(3) WHERE operation_id = ? AND failed_items > 0`
			case itemStatusMissing:
				counterSQL = `UPDATE inventory_operations SET skipped_items = skipped_items - 1, version = version + 1, updated_at = NOW(3) WHERE operation_id = ? AND skipped_items > 0`
			default:
				return apperr.Conflict(fmt.Sprintf("sku %d is in unknown item status %s", sku, item.status))
			}
			res, err := tx.ExecContext(ctx, counterSQL, operationID)
			if err != nil {
				return apperr.Internal("decrement operation counter for replay", err)
			}
			if n, _ := res.RowsAffected(); n != 1 {
				return apperr.Conflict("inventory operation counters are inconsistent; replay refused")
			}
			res, err = tx.ExecContext(ctx, `UPDATE inventory_operation_items
				SET status = ?, attempts = 0, last_error = '', updated_at = NOW(3)
				WHERE operation_id = ? AND sku_id = ? AND warehouse_id = ? AND status = ?`,
				itemStatusPending, operationID, sku, item.warehouseID, item.status)
			if err != nil {
				return apperr.Internal("reset operation item for replay", err)
			}
			if n, _ := res.RowsAffected(); n != 1 {
				return errOperationProgressRace
			}
			item.status = itemStatusPending
			item.attempts = 0
			item.lastError = ""
		}
		if _, err := tx.ExecContext(ctx, `UPDATE inventory_operations
			SET status = ?, version = version + 1, updated_at = NOW(3)
			WHERE operation_id = ?`, opStatusInProgress, operationID); err != nil {
			return apperr.Internal("mark operation replay in progress", err)
		}
		inputs := make([]operationItemInput, 0, len(op.items))
		for _, item := range op.items {
			inputs = append(inputs, operationItemInput{skuID: item.skuID, warehouseID: item.warehouseID, quantity: item.quantity})
		}
		spec = operationSpec{
			typ: op.typ, orderNo: op.orderNo, scopeHash: op.scopeHash,
			operationID: op.id, items: inputs,
		}
		return nil
	})
	if err != nil {
		return err
	}
	summary, err := s.runOperationLocal(ctx, spec)
	if err != nil {
		return err
	}
	if summary.Retryable > 0 {
		return apperr.Unavailable("inventory operation replay has retryable items", nil)
	}
	if summary.Failed > 0 {
		return apperr.Failed(fmt.Sprintf("inventory operation replay left %d failed item(s)", summary.Failed))
	}
	return nil
}
