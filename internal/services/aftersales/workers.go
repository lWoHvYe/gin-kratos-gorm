package aftersales

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/events"
	"gin-kratos-gorm/internal/platform/id"
	"gin-kratos-gorm/internal/platform/metrics"
	"gin-kratos-gorm/internal/platform/outbox"
)

// taskLease bounds a worker's claim. A crashed worker's task becomes claimable
// after the lease expires; no status scanner over business states is involved.
const taskLease = time.Minute

// T1.4b2b / SPEC-0002 v1.9 AC-67/AC-68: zero-amount after-sales never call
// Payment. They are admitted through Order's zero short-circuit, whose source
// and DONE state are contract values (S19). AfterSales must only use these
// constants; it must not infer a zero refund from the requested amount alone.
const (
	zeroAfterSaleSource        = "system_zero"
	zeroSettlementPaymentNoPre = "zero:"
	zeroRefundApplicationDone  = "DONE"
	// zeroCaptureSource is only an audit label for a zero-settled order; the
	// zero path never writes the after_sale_amount_ledger or a refund
	// commitment, so no amount is ever reserved.
	zeroCaptureSource = "zero_settlement"
)

// isZeroSettlementOrder reports whether Order's authoritative facts describe a
// zero-amount settlement: pay_cent=0 and the synthetic `zero:<order_no>`
// settlement identity (AC-67). A pay_cent=0 order whose settlement identity is
// something else may still have a captured intent and must not be turned into a
// zero refund.
func isZeroSettlementOrder(order *pb.Order) bool {
	return order != nil && order.GetPayCent() == 0 &&
		order.GetSettlementPaymentNo() == zeroSettlementPaymentNoPre+order.GetOrderNo()
}

type restockTask struct {
	afterSaleNo string
	skuID       int64
	warehouseID int64
	groupNo     string
	quantity    int64
	condition   string
	attempts    int
}

// RunRestockWorker processes durable per-SKU restock tasks. Inventory.Restock
// is idempotent by after_sale_no + sku, so a replay after a crash is safe.
func RunRestockWorker(ctx context.Context, database *sql.DB, svc *Service, logger *slog.Logger, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		runRestockBatch(ctx, database, svc, logger)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func runRestockBatch(ctx context.Context, database *sql.DB, svc *Service, logger *slog.Logger) {
	observeTaskBacklog(ctx, database)
	rows, err := database.QueryContext(ctx, `SELECT after_sale_no, sku_id, warehouse_id, group_no
		FROM after_sale_restock_tasks
		WHERE status IN ('PENDING', 'PROCESSING')
		  AND (status = 'PENDING' OR lease_until IS NULL OR lease_until < NOW(3))
		ORDER BY updated_at, after_sale_no, sku_id, warehouse_id LIMIT 50`)
	if err != nil {
		logger.Warn("scan restock tasks", "error", err)
		return
	}
	var tasks []restockTask
	for rows.Next() {
		var t restockTask
		if err := rows.Scan(&t.afterSaleNo, &t.skuID, &t.warehouseID, &t.groupNo); err == nil {
			tasks = append(tasks, t)
		}
	}
	_ = rows.Close()
	for _, task := range tasks {
		if err := svc.processRestockTaskForGroup(ctx, task.afterSaleNo, task.skuID, task.warehouseID, task.groupNo); err != nil {
			logger.Warn("restock task failed", "after_sale_no", task.afterSaleNo, "sku_id", task.skuID,
				"warehouse_id", task.warehouseID, "group_no", task.groupNo, "error", err)
		}
	}
}

func (s *Service) processRestockTask(ctx context.Context, afterSaleNo string, skuID int64) error {
	return s.processRestockTaskForGroup(ctx, afterSaleNo, skuID, 1, "G1")
}

func (s *Service) processRestockTaskForGroup(ctx context.Context, afterSaleNo string, skuID, warehouseID int64, groupNo string) error {
	task, owner, ok, err := s.claimRestockTask(ctx, afterSaleNo, skuID, warehouseID, groupNo)
	if err != nil || !ok {
		return err
	}
	row, err := s.getAfterSaleRow(ctx, afterSaleNo)
	if err != nil {
		s.releaseRestockTask(ctx, afterSaleNo, skuID, warehouseID, groupNo, owner, err)
		return err
	}
	if row.restockState == restockDone {
		return s.completeRestockTask(ctx, afterSaleNo, skuID, warehouseID, groupNo, owner, row)
	}
	if s.inventory == nil {
		err := apperr.Unavailable("inventory client is not configured", nil)
		s.releaseRestockTask(ctx, afterSaleNo, skuID, warehouseID, groupNo, owner, err)
		return err
	}
	_, err = s.inventory.Restock(ctx, &pb.RestockRequest{
		OrderNo: row.orderNo, SkuId: skuID, WarehouseId: task.warehouseID,
		Quantity: task.quantity, Condition: task.condition,
		Reason: "after-sale " + afterSaleNo + " " + task.groupNo, RestockNo: afterSaleNo,
	})
	if err != nil {
		s.releaseRestockTask(ctx, afterSaleNo, skuID, warehouseID, groupNo, owner, err)
		return apperr.Unavailable("restock returned goods", err)
	}
	return s.completeRestockTask(ctx, afterSaleNo, skuID, warehouseID, groupNo, owner, row)
}

func (s *Service) claimRestockTask(ctx context.Context, afterSaleNo string, skuID, warehouseID int64, groupNo string) (restockTask, string, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return restockTask{}, "", false, apperr.Internal("begin restock claim", err)
	}
	defer func() { _ = tx.Rollback() }()
	var task restockTask
	var status, owner string
	var lease sql.NullTime
	task.afterSaleNo = afterSaleNo
	task.skuID = skuID
	task.warehouseID = warehouseID
	task.groupNo = groupNo
	if err := tx.QueryRowContext(ctx, `SELECT quantity, `+"`condition`"+`, status, attempts, lease_owner, lease_until
		FROM after_sale_restock_tasks WHERE after_sale_no = ? AND sku_id = ? AND warehouse_id = ? AND group_no = ? FOR UPDATE`,
		afterSaleNo, skuID, warehouseID, groupNo).Scan(&task.quantity, &task.condition, &status, &task.attempts, &owner, &lease); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return restockTask{}, "", false, nil
		}
		return restockTask{}, "", false, apperr.Internal("load restock task", err)
	}
	if status == taskDone || status == taskFailed {
		return restockTask{}, "", false, nil
	}
	if status == taskProcessing && lease.Valid && lease.Time.After(time.Now()) {
		return restockTask{}, "", false, nil
	}
	owner = id.Event()
	if _, err := tx.ExecContext(ctx, `UPDATE after_sale_restock_tasks
		SET status = ?, attempts = attempts + 1, lease_owner = ?, lease_until = DATE_ADD(NOW(3), INTERVAL 1 MINUTE), updated_at = NOW(3)
		WHERE after_sale_no = ? AND sku_id = ? AND warehouse_id = ? AND group_no = ?`,
		taskProcessing, owner, afterSaleNo, skuID, warehouseID, groupNo); err != nil {
		return restockTask{}, "", false, apperr.Internal("claim restock task", err)
	}
	if err := tx.Commit(); err != nil {
		return restockTask{}, "", false, apperr.Internal("commit restock claim", err)
	}
	task.attempts++
	return task, owner, true, nil
}

func (s *Service) completeRestockTask(ctx context.Context, afterSaleNo string, skuID, warehouseID int64, groupNo, owner string, row *afterSaleRow) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return apperr.Internal("begin complete restock", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE after_sale_restock_tasks
		SET status = ?, last_error = '', lease_owner = '', lease_until = NULL, updated_at = NOW(3)
		WHERE after_sale_no = ? AND sku_id = ? AND warehouse_id = ? AND group_no = ? AND status = ?`,
		taskDone, afterSaleNo, skuID, warehouseID, groupNo, taskProcessing); err != nil {
		return apperr.Internal("complete restock task", err)
	}
	var remaining int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM after_sale_restock_tasks
		WHERE after_sale_no = ? AND status <> ?`, afterSaleNo, taskDone).Scan(&remaining); err != nil {
		return apperr.Internal("count remaining restock tasks", err)
	}
	if remaining == 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE after_sales
			SET restock_state = ?, version = version + 1 WHERE after_sale_no = ?`, restockDone, afterSaleNo); err != nil {
			return apperr.Internal("mark restock done", err)
		}
		if row.status == statusRefunding {
			// Legacy REFUNDING rows queued restock before refund; now it is safe
			// to create the durable refund task.
			if _, err := tx.ExecContext(ctx, `INSERT INTO after_sale_refund_tasks
				(after_sale_no, status, attempts, last_error, created_at, updated_at)
				VALUES (?, ?, 0, '', NOW(3), NOW(3))
				ON DUPLICATE KEY UPDATE after_sale_no = after_sale_no`, afterSaleNo, taskPending); err != nil {
				return apperr.Internal("queue refund after restock", err)
			}
		}
		payload, _ := json.Marshal(afterSaleEvent{
			AfterSaleNo: afterSaleNo, OrderNo: row.orderNo, UserID: row.userID,
			Type: row.typ, Status: row.status, AmountCent: row.amountCent,
			EffectiveAmountCent: row.effectiveAmountCent, AmountState: row.amountState,
			RestockState: restockDone, Reason: row.reason,
		})
		if err := outbox.Append(ctx, tx, outbox.Event{
			Topic: topicAfterSaleRestocked, AggregateType: aggregateAfterSale,
			AggregateID: afterSaleNo, EventType: "aftersale.restocked", Payload: payload,
		}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Service) releaseRestockTask(ctx context.Context, afterSaleNo string, skuID, warehouseID int64, groupNo, owner string, cause error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer func() { _ = tx.Rollback() }()
	var attempts int
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT attempts, status FROM after_sale_restock_tasks
		WHERE after_sale_no = ? AND sku_id = ? AND warehouse_id = ? AND group_no = ? FOR UPDATE`,
		afterSaleNo, skuID, warehouseID, groupNo).Scan(&attempts, &status); err != nil {
		return
	}
	if status != taskProcessing {
		return
	}
	next := taskPending
	permanent := attempts >= maxTaskAttempts
	if permanent {
		next = taskFailed
		if _, err := tx.ExecContext(ctx, `UPDATE after_sales
			SET restock_state = ?, version = version + 1 WHERE after_sale_no = ?`, restockFailed, afterSaleNo); err != nil {
			return
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE after_sale_restock_tasks
		SET status = ?, last_error = ?, lease_owner = '', lease_until = NULL, updated_at = NOW(3)
		WHERE after_sale_no = ? AND sku_id = ? AND warehouse_id = ? AND group_no = ?`,
		next, cutRunes(cause.Error(), 512), afterSaleNo, skuID, warehouseID, groupNo); err != nil {
		return
	}
	if permanent {
		var row afterSaleRow
		if err := tx.QueryRowContext(ctx, `SELECT id, order_no, user_id, `+"`type`"+`, status, amount_cent,
			effective_amount_cent, amount_state, restock_state, reason
			FROM after_sales WHERE after_sale_no = ?`, afterSaleNo).
			Scan(&row.id, &row.orderNo, &row.userID, &row.typ, &row.status, &row.amountCent,
				&row.effectiveAmountCent, &row.amountState, &row.restockState, &row.reason); err != nil {
			return
		}
		payload, _ := json.Marshal(afterSaleEvent{
			AfterSaleNo: afterSaleNo, OrderNo: row.orderNo, UserID: row.userID,
			Type: row.typ, Status: row.status, AmountCent: row.amountCent,
			EffectiveAmountCent: row.effectiveAmountCent, AmountState: row.amountState,
			RestockState: restockFailed, Reason: cutRunes(cause.Error(), 512),
		})
		if err := outbox.Append(ctx, tx, outbox.Event{
			Topic: topicAfterSaleRestockFailed, AggregateType: aggregateAfterSale,
			AggregateID: afterSaleNo, EventType: "aftersale.restock_failed", Payload: payload,
		}); err != nil {
			return
		}
	}
	_ = tx.Commit()
}

// RunRefundWorker processes durable refund tasks. It never scans after_sales
// status combinations; REFUNDING is an intent and the task table is the queue.
func RunRefundWorker(ctx context.Context, database *sql.DB, svc *Service, logger *slog.Logger, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		runRefundBatch(ctx, database, svc, logger)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func runRefundBatch(ctx context.Context, database *sql.DB, svc *Service, logger *slog.Logger) {
	observeTaskBacklog(ctx, database)
	rows, err := database.QueryContext(ctx, `SELECT after_sale_no FROM after_sale_refund_tasks
		WHERE status IN ('PENDING', 'PROCESSING')
		  AND (status = 'PENDING' OR lease_until IS NULL OR lease_until < NOW(3))
		ORDER BY updated_at LIMIT 50`)
	if err != nil {
		logger.Warn("scan refund tasks", "error", err)
		return
	}
	var numbers []string
	for rows.Next() {
		var no string
		if err := rows.Scan(&no); err == nil {
			numbers = append(numbers, no)
		}
	}
	_ = rows.Close()
	for _, no := range numbers {
		if err := svc.processRefundTask(ctx, no); err != nil {
			logger.Warn("refund task failed", "after_sale_no", no, "error", err)
		}
	}
}

func (s *Service) processRefundTask(ctx context.Context, afterSaleNo string) error {
	owner, ok, err := s.claimRefundTask(ctx, afterSaleNo)
	if err != nil || !ok {
		return err
	}
	row, err := s.getAfterSaleRow(ctx, afterSaleNo)
	if err != nil {
		s.releaseRefundTask(ctx, afterSaleNo, owner, err)
		return err
	}
	if row.status == statusRefunded {
		return s.completeRefundTask(ctx, afterSaleNo, owner)
	}
	if row.status != statusRefunding && !(row.status == statusApproved && row.source == sourceSystem) {
		err := apperr.Conflict(fmt.Sprintf("after sale is %s, not REFUNDING", row.status))
		s.releaseRefundTask(ctx, afterSaleNo, owner, err)
		return err
	}
	if row.typ == typeReturnRefund && row.restockState != restockDone {
		err := apperr.Failed("return-refund stock must be restocked before refunding")
		s.releaseRefundTask(ctx, afterSaleNo, owner, err)
		return err
	}
	groupNo, err := s.groupForAfterSale(ctx, afterSaleNo)
	if err != nil {
		s.releaseRefundTask(ctx, afterSaleNo, owner, err)
		return err
	}
	order, err := s.loadOrder(ctx, row.orderNo, row.userID)
	if err != nil {
		s.releaseRefundTask(ctx, afterSaleNo, owner, err)
		return err
	}
	items, err := s.loadItems(ctx, afterSaleNo)
	if err != nil {
		s.releaseRefundTask(ctx, afterSaleNo, owner, err)
		return err
	}
	// T1.4b2b: a zero-settled order takes the zero short-circuit, so the
	// Payment dependency and the positive-amount guard below are never
	// reached. The task was already claimed (persisted intent + lease) before
	// this call, satisfying the R-C precondition for CancelIfCreated.
	if isZeroSettlementOrder(order) {
		return s.processZeroRefundTask(ctx, afterSaleNo, owner, row, items, groupNo)
	}
	if s.payments == nil {
		err := apperr.Unavailable("payment client is not configured", nil)
		s.releaseRefundTask(ctx, afterSaleNo, owner, err)
		return err
	}
	// Refund admission is a remote Order call; do it before opening the local
	// line-ledger transaction. It is idempotent by after_sale_no, so the
	// ordinary Refund path and this worker converge on one admission result.
	if s.orders != nil {
		amount := row.effectiveAmountCent
		if amount <= 0 {
			amount = row.amountCent
		}
		if _, err := s.orders.AdmitRefund(ctx, &pb.AdmitRefundRequest{
			RefundId: afterSaleNo, OrderNo: row.orderNo, GroupNo: groupNo,
			AmountCent: amount, Source: "aftersale",
		}); err != nil {
			s.releaseRefundTask(ctx, afterSaleNo, owner, err)
			return apperr.Unavailable("refund admission", err)
		}
	}
	// Hold the line ledger row locks across the idempotent Payment call. This
	// serializes concurrent after-sales on the same line; a timeout rolls back
	// and the durable task retries with the same after_sale_no.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		s.releaseRefundTask(ctx, afterSaleNo, owner, apperr.Internal("begin refund completion", err))
		return apperr.Internal("begin refund completion", err)
	}
	defer func() { _ = tx.Rollback() }()
	current, err := lockAfterSale(ctx, tx, afterSaleNo)
	if err != nil {
		_ = tx.Rollback()
		s.releaseRefundTask(ctx, afterSaleNo, owner, err)
		return err
	}
	if current.status == statusRefunded {
		_ = tx.Rollback()
		return s.completeRefundTask(ctx, afterSaleNo, owner)
	}
	if current.status == statusApproved && current.source == sourceSystem {
		if _, err := tx.ExecContext(ctx, `UPDATE after_sales SET status = ?, version = version + 1 WHERE id = ?`,
			statusRefunding, current.id); err != nil {
			return apperr.Internal("system refund to REFUNDING", err)
		}
		current.status = statusRefunding
	}
	if current.status != statusRefunding {
		return apperr.Conflict(fmt.Sprintf("after sale is %s, not REFUNDING", current.status))
	}
	if err := s.clearPendingForAfterSaleTx(ctx, tx, afterSaleNo); err != nil {
		return err
	}
	refunds, goodsCent, freightCent, ruleID, err := s.computeLineRefunds(ctx, tx, order, items, current.typ, true)
	if err != nil {
		return err
	}
	if !s.freightResponsibilityEnabled {
		// computedFreight/computedRule came from computeLineRefunds (legacy).
	} else if current.source == sourceSystem {
		share := groupFreightShare(order, groupNo)
		freightCent, err = s.systemFreightRefund(ctx, tx, order, groupNo, share)
		if err != nil {
			return err
		}
		ruleID = shortageFreightRule
	} else if len(items) > 0 && goodsCent > 0 {
		policyFreight, hold, policyRule, ferr := s.freightRefundFor(ctx, tx, order, groupNo, current.responsibilityParty)
		if ferr != nil {
			return ferr
		}
		if hold {
			freightCent = 0
			ruleID = "FREIGHT_HOLD"
		} else {
			freightCent = policyFreight
			ruleID = policyRule
		}
	}
	refundCent := goodsCent + freightCent
	if refundCent == 0 {
		// Legacy row without line snapshots keeps the approved effective amount.
		refundCent = current.effectiveAmountCent
		ruleID = "LEGACY"
	}
	if current.effectiveAmountCent > 0 && refundCent > current.effectiveAmountCent {
		refundCent = current.effectiveAmountCent
	}
	// Zero-amount after-sales were completed by the zero short-circuit above;
	// reaching this guard with <= 0 is a positive-path anomaly, never a
	// "nothing to refund" success.
	if refundCent <= 0 {
		return apperr.Failed("after sale has no positive refund amount")
	}
	// If the captured/approved cap is below the line formula, cap the line
	// amounts deterministically in line order and keep the breakdown equal to
	// the amount actually paid. Otherwise the ledger would record more than
	// Payment refunded.
	if goodsCent+freightCent > refundCent {
		remaining := refundCent
		var cappedGoods int64
		for i := range refunds {
			if refunds[i].AmountCent > remaining {
				refunds[i].AmountCent = remaining
			}
			if refunds[i].AmountCent < 0 {
				refunds[i].AmountCent = 0
			}
			remaining -= refunds[i].AmountCent
			cappedGoods += refunds[i].AmountCent
		}
		goodsCent = cappedGoods
		freightCent = refundCent - goodsCent
		if freightCent < 0 {
			freightCent = 0
		}
	}
	reply, err := s.payments.RefundOrder(ctx, &pb.RefundOrderRequest{
		OrderNo: current.orderNo, AmountCent: refundCent, Reason: "after-sale " + afterSaleNo,
		IdempotencyKey: afterSaleNo, GroupNo: groupNo, AfterSaleNo: afterSaleNo,
		GoodsCent: goodsCent, FreightCent: freightCent, RuleId: ruleID,
	})
	if err != nil {
		_ = tx.Rollback()
		s.releaseRefundTask(ctx, afterSaleNo, owner, err)
		return apperr.Unavailable("payment refund failed; retry is safe", err)
	}
	refundNo := ""
	if reply != nil {
		refundNo = reply.GetRefundNo()
	}
	if _, err := tx.ExecContext(ctx, `UPDATE after_sales
		SET status = ?, effective_amount_cent = ?, refund_no = ?, refunded_cent = ?, amount_state = ?, version = version + 1
		WHERE id = ?`, statusRefunded, refundCent, refundNo, refundCent, amountConsumed, current.id); err != nil {
		return apperr.Internal("mark after sale refunded", err)
	}
	for _, item := range items {
		if err := s.clearPendingLineTx(ctx, tx, current.orderNo, item.GetLineNo(), int64(item.GetQuantity())); err != nil {
			return err
		}
	}
	for _, refund := range refunds {
		if refund.LineNo <= 0 {
			continue
		}
		if err := s.recordLineRefundTx(ctx, tx, current.orderNo, refund); err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{
			"after_sale_no": afterSaleNo, "order_no": current.orderNo, "line_no": refund.LineNo,
			"quantity": refund.Quantity, "refunded_amount_cent": refund.AmountCent,
		})
		if err := outbox.Append(ctx, tx, outbox.Event{
			Topic: "shop.aftersale.line_refunded.v1", AggregateType: aggregateAfterSale,
			AggregateID: afterSaleNo, EventType: "aftersale.line_refunded", Payload: payload,
		}); err != nil {
			return apperr.Internal("append line refunded", err)
		}
	}
	if err := s.recordRefundBreakdownTx(ctx, tx, afterSaleNo, goodsCent, freightCent, ruleID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE after_sale_refund_tasks
		SET status = ?, last_error = '', lease_owner = '', lease_until = NULL, updated_at = NOW(3)
		WHERE after_sale_no = ?`, taskDone, afterSaleNo); err != nil {
		return apperr.Internal("complete refund task", err)
	}
	if err := appendLog(ctx, tx, current.id, "REFUND", statusRefunding, statusRefunded, "payment refund completed"); err != nil {
		return err
	}
	payload, _ := json.Marshal(events.AfterSaleCompleted{
		AfterSaleNo: afterSaleNo, OrderNo: current.orderNo, GroupNo: groupNo, UserID: current.userID,
		Type: current.typ, Status: statusRefunded, AmountCent: current.amountCent,
		EffectiveAmountCent: refundCent, AmountState: amountConsumed,
		RestockState: current.restockState, Reason: current.reason,
	})
	if err := outbox.Append(ctx, tx, outbox.Event{
		Topic: topicAfterSaleCompleted, AggregateType: aggregateAfterSale,
		AggregateID: afterSaleNo, EventType: "aftersale.completed", Payload: payload,
	}); err != nil {
		return apperr.Internal("append aftersale.completed", err)
	}
	return tx.Commit()
}

// processZeroRefundTask completes a zero-amount after-sale through Order's
// zero short-circuit (SPEC-0002 v1.9 AC-67/AC-68, AZ-02/AZ-03/AZ-05):
//
//  1. the durable refund task is already claimed (PENDING -> PROCESSING +
//     lease), so a crash after the remote admission can always be retried;
//  2. Order.AdmitRefund(source=system_zero, amount=0) performs the group
//     admission and writes S19 DONE in one transaction; a failed retry returns
//     the existing S19 instead of a second one;
//  3. only a DONE reply may mark the local after-sale REFUNDED. S19 is the
//     execution fact; if it is missing, this function releases the task for
//     retry and never fabricates REFUNDED/refunded_cent;
//  4. the local transaction writes REFUNDED, refunded_cent=0,
//     amount_state=CONSUMED, refund_no=” and task DONE together, then emits
//     aftersale.completed. Payment is never called.
func (s *Service) processZeroRefundTask(ctx context.Context, afterSaleNo, owner string, row *afterSaleRow, items []*pb.AfterSaleItem, groupNo string) error {
	if row.amountCent != 0 || row.effectiveAmountCent != 0 {
		// A zero-settled order cannot legitimately carry a refund amount. Fail
		// closed instead of silently discarding the requested amount.
		err := apperr.Conflict("zero-settled order after-sale must not carry a refund amount")
		s.releaseRefundTask(ctx, afterSaleNo, owner, err)
		return err
	}
	reply, err := s.orders.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: afterSaleNo, OrderNo: row.orderNo, GroupNo: groupNo,
		AmountCent: 0, Source: zeroAfterSaleSource,
	})
	if err != nil {
		s.releaseRefundTask(ctx, afterSaleNo, owner, err)
		return apperr.Unavailable("zero-amount refund admission", err)
	}
	if reply.GetState() != zeroRefundApplicationDone || reply.GetAmountCent() != 0 {
		// AC-68: S19 must be DONE before AfterSales becomes REFUNDED.
		err := apperr.Failed("zero-amount refund admission is not DONE").
			WithDetails(map[string]any{
				"after_sale_no": afterSaleNo, "state": reply.GetState(), "amount_cent": reply.GetAmountCent(),
			})
		s.releaseRefundTask(ctx, afterSaleNo, owner, err)
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return apperr.Internal("begin zero refund completion", err)
	}
	defer func() { _ = tx.Rollback() }()
	current, err := lockAfterSale(ctx, tx, afterSaleNo)
	if err != nil {
		_ = tx.Rollback()
		s.releaseRefundTask(ctx, afterSaleNo, owner, err)
		return err
	}
	if current.status == statusRefunded {
		// S19 DONE was already projected; only the task may still need closing.
		_ = tx.Rollback()
		return s.completeRefundTask(ctx, afterSaleNo, owner)
	}
	if current.amountCent != 0 || current.effectiveAmountCent != 0 {
		err := apperr.Conflict("zero-settled order after-sale must not carry a refund amount")
		_ = tx.Rollback()
		s.releaseRefundTask(ctx, afterSaleNo, owner, err)
		return err
	}
	if current.status == statusApproved && current.source == sourceSystem {
		if _, err := tx.ExecContext(ctx, `UPDATE after_sales SET status = ?, version = version + 1 WHERE id = ?`,
			statusRefunding, current.id); err != nil {
			return apperr.Internal("system zero refund to REFUNDING", err)
		}
		current.status = statusRefunding
	}
	if current.status != statusRefunding {
		return apperr.Conflict(fmt.Sprintf("after sale is %s, not REFUNDING", current.status))
	}
	if err := s.clearPendingForAfterSaleTx(ctx, tx, afterSaleNo); err != nil {
		return err
	}
	// Quantity accounting matches the positive path; the amount is zero and no
	// line-refunded event is emitted for the zero short-circuit.
	for _, item := range items {
		if item.GetLineNo() <= 0 || item.GetQuantity() <= 0 {
			continue
		}
		if err := s.recordLineRefundTx(ctx, tx, current.orderNo, lineRefund{
			LineNo: item.GetLineNo(), SkuID: item.GetSkuId(), Quantity: int64(item.GetQuantity()), AmountCent: 0,
		}); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE after_sales
		SET status = ?, effective_amount_cent = 0, refund_no = '', refunded_cent = 0, amount_state = ?, version = version + 1
		WHERE id = ?`, statusRefunded, amountConsumed, current.id); err != nil {
		return apperr.Internal("mark zero after sale refunded", err)
	}
	res, err := tx.ExecContext(ctx, `UPDATE after_sale_refund_tasks
		SET status = ?, last_error = '', lease_owner = '', lease_until = NULL, updated_at = NOW(3)
		WHERE after_sale_no = ? AND lease_owner = ?`, taskDone, afterSaleNo, owner)
	if err != nil {
		return apperr.Internal("complete zero refund task", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return apperr.Conflict("refund task lease was lost before zero completion")
	}
	if err := appendLog(ctx, tx, current.id, "REFUND", current.status, statusRefunded, "zero-amount refund completed"); err != nil {
		return err
	}
	payload, _ := json.Marshal(events.AfterSaleCompleted{
		AfterSaleNo: afterSaleNo, OrderNo: current.orderNo, GroupNo: groupNo, UserID: current.userID,
		Type: current.typ, Status: statusRefunded, AmountCent: current.amountCent,
		EffectiveAmountCent: 0, AmountState: amountConsumed,
		RestockState: current.restockState, Reason: current.reason,
	})
	if err := outbox.Append(ctx, tx, outbox.Event{
		Topic: topicAfterSaleCompleted, AggregateType: aggregateAfterSale,
		AggregateID: afterSaleNo, EventType: "aftersale.completed", Payload: payload,
	}); err != nil {
		return apperr.Internal("append zero aftersale.completed", err)
	}
	return tx.Commit()
}

func (s *Service) claimRefundTask(ctx context.Context, afterSaleNo string) (string, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", false, apperr.Internal("begin refund claim", err)
	}
	defer func() { _ = tx.Rollback() }()
	var status, currentOwner string
	var attempts int
	var lease sql.NullTime
	if err := tx.QueryRowContext(ctx, `SELECT status, attempts, lease_owner, lease_until
		FROM after_sale_refund_tasks WHERE after_sale_no = ? FOR UPDATE`, afterSaleNo).
		Scan(&status, &attempts, &currentOwner, &lease); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, apperr.Internal("load refund task", err)
	}
	if status == taskDone || status == taskFailed {
		return "", false, nil
	}
	if status == taskProcessing && lease.Valid && lease.Time.After(time.Now()) {
		return "", false, nil
	}
	owner := id.Event()
	if _, err := tx.ExecContext(ctx, `UPDATE after_sale_refund_tasks
		SET status = ?, attempts = attempts + 1, lease_owner = ?, lease_until = DATE_ADD(NOW(3), INTERVAL 1 MINUTE), updated_at = NOW(3)
		WHERE after_sale_no = ?`, taskProcessing, owner, afterSaleNo); err != nil {
		return "", false, apperr.Internal("claim refund task", err)
	}
	if err := tx.Commit(); err != nil {
		return "", false, apperr.Internal("commit refund claim", err)
	}
	return owner, true, nil
}

func (s *Service) releaseRefundTask(ctx context.Context, afterSaleNo, owner string, cause error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer func() { _ = tx.Rollback() }()
	var attempts int
	var status, currentOwner string
	if err := tx.QueryRowContext(ctx, `SELECT attempts, status, lease_owner FROM after_sale_refund_tasks
		WHERE after_sale_no = ? FOR UPDATE`, afterSaleNo).Scan(&attempts, &status, &currentOwner); err != nil {
		return
	}
	if status != taskProcessing || currentOwner != owner {
		return
	}
	next := taskPending
	permanent := attempts >= maxTaskAttempts
	if permanent {
		next = taskFailed
	}
	if _, err := tx.ExecContext(ctx, `UPDATE after_sale_refund_tasks
		SET status = ?, last_error = ?, lease_owner = '', lease_until = NULL, updated_at = NOW(3)
		WHERE after_sale_no = ?`, next, cutRunes(cause.Error(), 512), afterSaleNo); err != nil {
		return
	}
	if permanent {
		if err := s.releasePendingForAfterSaleTx(ctx, tx, afterSaleNo); err != nil {
			return
		}
		var orderNo, amountState string
		var effective int64
		if err := tx.QueryRowContext(ctx, `SELECT order_no, effective_amount_cent, amount_state
			FROM after_sales WHERE after_sale_no = ? FOR UPDATE`, afterSaleNo).
			Scan(&orderNo, &effective, &amountState); err != nil {
			return
		}
		if amountState != amountReleased && effective > 0 {
			if _, err := tx.ExecContext(ctx, `UPDATE after_sale_amount_ledger
				SET consumed_cent = GREATEST(consumed_cent - ?, 0), version = version + 1, updated_at = NOW(3)
				WHERE order_no = ?`, effective, orderNo); err != nil {
				return
			}
			if _, err := tx.ExecContext(ctx, `UPDATE after_sales
				SET amount_state = ?, version = version + 1 WHERE after_sale_no = ?`,
				amountReleased, afterSaleNo); err != nil {
				return
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return
	}
	if permanent {
		// Alertable terminal overflow: the task stays FAILED_PERMANENT and the
		// after-sale is never marked REFUNDED (no faked completion).
		metrics.AfterSaleRefundRecovery.WithLabelValues("failed_permanent").Inc()
	}
}

func (s *Service) completeRefundTask(ctx context.Context, afterSaleNo, owner string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return apperr.Internal("begin complete refund task", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE after_sale_refund_tasks
		SET status = ?, last_error = '', lease_owner = '', lease_until = NULL, updated_at = NOW(3)
		WHERE after_sale_no = ? AND lease_owner = ?`, taskDone, afterSaleNo, owner); err != nil {
		return apperr.Internal("complete refund task", err)
	}
	return tx.Commit()
}

// ensureRefundTaskTx creates the task or re-arms a permanently failed one so an
// explicit Refund retry can recover without a status scanner.
func ensureRefundTaskTx(ctx context.Context, tx *sql.Tx, afterSaleNo string) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO after_sale_refund_tasks
		(after_sale_no, status, attempts, last_error, created_at, updated_at)
		VALUES (?, ?, 0, '', NOW(3), NOW(3))
		ON DUPLICATE KEY UPDATE
		  attempts = IF(status = 'FAILED_PERMANENT', 0, attempts),
		  last_error = IF(status = 'FAILED_PERMANENT', '', last_error),
		  status = IF(status = 'FAILED_PERMANENT', 'PENDING', status)`,
		afterSaleNo, taskPending); err != nil {
		return apperr.Internal("ensure refund task", err)
	}
	return nil
}

// observeTaskBacklog feeds the restock/refund backlog gauges on every worker
// tick. It is deliberately read-only and failure-tolerant: metrics must never
// stop durable task processing.
func observeTaskBacklog(ctx context.Context, database *sql.DB) {
	var restock, refund int64
	var restockOldest, refundOldest float64
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*),
		COALESCE(TIMESTAMPDIFF(SECOND, MIN(created_at), NOW(3)), 0)
		FROM after_sale_restock_tasks WHERE status IN ('PENDING', 'PROCESSING')`).
		Scan(&restock, &restockOldest); err == nil {
		metrics.AfterSaleRestockBacklog.Set(float64(restock))
		metrics.AfterSaleRestockOldestSeconds.Set(restockOldest)
	}
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*),
		COALESCE(TIMESTAMPDIFF(SECOND, MIN(created_at), NOW(3)), 0)
		FROM after_sale_refund_tasks WHERE status IN ('PENDING', 'PROCESSING')`).
		Scan(&refund, &refundOldest); err == nil {
		metrics.AfterSaleRefundBacklog.Set(float64(refund))
		metrics.AfterSaleRefundOldestSeconds.Set(refundOldest)
	}
}

func groupFreightShare(order *pb.Order, groupNo string) int64 {
	for _, group := range order.GetFulfillmentGroups() {
		if group.GetGroupNo() == groupNo {
			return group.GetFreightShareCent()
		}
	}
	return 0
}

func (s *Service) clearPendingLineTx(ctx context.Context, tx *sql.Tx, orderNo string, lineNo int32, quantity int64) error {
	if lineNo <= 0 || quantity <= 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE after_sale_line_ledger
		SET pending_quantity = GREATEST(pending_quantity - ?, 0),
		    pending_amount_cent = GREATEST(pending_amount_cent - 0, 0),
		    version = version + 1, updated_at = NOW(3)
		WHERE order_no = ? AND line_no = ?`, quantity, orderNo, lineNo); err != nil {
		return apperr.Internal("clear pending line quantity", err)
	}
	return nil
}
