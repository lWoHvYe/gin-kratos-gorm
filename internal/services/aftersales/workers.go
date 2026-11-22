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
	"gin-kratos-gorm/internal/platform/id"
	"gin-kratos-gorm/internal/platform/metrics"
	"gin-kratos-gorm/internal/platform/outbox"
)

// taskLease bounds a worker's claim. A crashed worker's task becomes claimable
// after the lease expires; no status scanner over business states is involved.
const taskLease = time.Minute

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
	if row.status != statusRefunding {
		err := apperr.Conflict(fmt.Sprintf("after sale is %s, not REFUNDING", row.status))
		s.releaseRefundTask(ctx, afterSaleNo, owner, err)
		return err
	}
	if row.typ == typeReturnRefund && row.restockState != restockDone {
		err := apperr.Failed("return-refund stock must be restocked before refunding")
		s.releaseRefundTask(ctx, afterSaleNo, owner, err)
		return err
	}
	if s.payments == nil {
		err := apperr.Unavailable("payment client is not configured", nil)
		s.releaseRefundTask(ctx, afterSaleNo, owner, err)
		return err
	}
	if row.effectiveAmountCent <= 0 {
		err := apperr.Failed("after sale has no positive effective amount")
		s.releaseRefundTask(ctx, afterSaleNo, owner, err)
		return err
	}
	reply, err := s.payments.RefundOrder(ctx, &pb.RefundOrderRequest{
		OrderNo: row.orderNo, AmountCent: row.effectiveAmountCent,
		Reason: "after-sale " + afterSaleNo, IdempotencyKey: afterSaleNo,
	})
	if err != nil {
		s.releaseRefundTask(ctx, afterSaleNo, owner, err)
		return apperr.Unavailable("payment refund failed; retry is safe", err)
	}
	refundNo := ""
	if reply != nil {
		refundNo = reply.GetRefundNo()
	}
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
		if _, err := tx.ExecContext(ctx, `UPDATE after_sale_refund_tasks
			SET status = ?, lease_owner = '', lease_until = NULL, updated_at = NOW(3)
			WHERE after_sale_no = ?`, taskDone, afterSaleNo); err != nil {
			return apperr.Internal("complete refund task", err)
		}
		return tx.Commit()
	}
	if current.status != statusRefunding {
		return apperr.Conflict(fmt.Sprintf("after sale is %s, not REFUNDING", current.status))
	}
	if _, err := tx.ExecContext(ctx, `UPDATE after_sales
		SET status = ?, refund_no = ?, refunded_cent = ?, amount_state = ?, version = version + 1
		WHERE id = ?`, statusRefunded, refundNo, current.effectiveAmountCent, amountConsumed, current.id); err != nil {
		return apperr.Internal("mark after sale refunded", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE after_sale_refund_tasks
		SET status = ?, last_error = '', lease_owner = '', lease_until = NULL, updated_at = NOW(3)
		WHERE after_sale_no = ?`, taskDone, afterSaleNo); err != nil {
		return apperr.Internal("complete refund task", err)
	}
	if err := appendLog(ctx, tx, current.id, "REFUND", statusRefunding, statusRefunded, "payment refund completed"); err != nil {
		return err
	}
	payload, _ := json.Marshal(afterSaleEvent{
		AfterSaleNo: afterSaleNo, OrderNo: current.orderNo, UserID: current.userID,
		Type: current.typ, Status: statusRefunded, AmountCent: current.amountCent,
		EffectiveAmountCent: current.effectiveAmountCent, AmountState: amountConsumed,
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
	if attempts >= maxTaskAttempts {
		next = taskFailed
	}
	if _, err := tx.ExecContext(ctx, `UPDATE after_sale_refund_tasks
		SET status = ?, last_error = ?, lease_owner = '', lease_until = NULL, updated_at = NOW(3)
		WHERE after_sale_no = ?`, next, cutRunes(cause.Error(), 512), afterSaleNo); err != nil {
		return
	}
	_ = tx.Commit()
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
