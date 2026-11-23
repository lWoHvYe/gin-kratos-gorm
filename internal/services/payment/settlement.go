package payment

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"gin-kratos-gorm/internal/platform/id"
)

const (
	settlementPending    = "PENDING"
	settlementProcessing = "PROCESSING"
	settlementDone       = "DONE"
	settlementFailed     = "FAILED_PERMANENT"
	settlementMaxAttempt = 10
)

type settlementTask struct {
	PaymentNo string
	OrderNo   string
	Attempts  int
	GID       string
}

// ensureSettlementTask is called in the same transaction as the SUCCEEDED
// intent/captured write. It never resets a completed task.
func (r *repo) ensureSettlementTask(ctx context.Context, tx *sql.Tx, paymentNo, orderNo string, now time.Time) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO payment_settlement_tasks
		(payment_no, order_no, state, attempts, version, created_at, updated_at)
		VALUES (?, ?, ?, 0, 1, ?, ?)
		ON DUPLICATE KEY UPDATE order_no = VALUES(order_no)`,
		paymentNo, orderNo, settlementPending, now, now)
	return err
}

func (r *repo) settlementTaskByPayment(ctx context.Context, q querier, paymentNo string) (*settlementTask, error) {
	var task settlementTask
	var state string
	err := q.QueryRowContext(ctx, `SELECT payment_no, order_no, state, attempts, dtm_gid
		FROM payment_settlement_tasks WHERE payment_no = ?`, paymentNo).
		Scan(&task.PaymentNo, &task.OrderNo, &state, &task.Attempts, &task.GID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// claimSettlementTask leases one pending task. The caller owns it until
// finish/release; a crashed owner is recovered after lease_until.
func claimSettlementTask(ctx context.Context, database *sql.DB, owner string, lease time.Duration) (*settlementTask, error) {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var task settlementTask
	err = tx.QueryRowContext(ctx, `SELECT payment_no, order_no, attempts, dtm_gid
		FROM payment_settlement_tasks
		WHERE state = ? AND (lease_until IS NULL OR lease_until < NOW(3))
		ORDER BY created_at ASC LIMIT 1 FOR UPDATE SKIP LOCKED`, settlementPending).
		Scan(&task.PaymentNo, &task.OrderNo, &task.Attempts, &task.GID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE payment_settlement_tasks
		SET state = ?, lease_owner = ?, lease_until = DATE_ADD(NOW(3), INTERVAL ? SECOND),
		    version = version + 1, updated_at = NOW(3)
		WHERE payment_no = ? AND state = ?`,
		settlementProcessing, owner, int(lease.Seconds()), task.PaymentNo, settlementPending); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &task, nil
}

func finishSettlementTask(ctx context.Context, database *sql.DB, paymentNo, state, errMsg string) error {
	_, err := database.ExecContext(ctx, `UPDATE payment_settlement_tasks
		SET state = ?, lease_owner = '', lease_until = NULL, last_error = ?, version = version + 1, updated_at = NOW(3)
		WHERE payment_no = ?`, state, errMsg, paymentNo)
	return err
}

func releaseSettlementTask(ctx context.Context, database *sql.DB, paymentNo, cause string) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var attempts int
	if err := tx.QueryRowContext(ctx, `SELECT attempts FROM payment_settlement_tasks WHERE payment_no = ? FOR UPDATE`, paymentNo).
		Scan(&attempts); err != nil {
		return err
	}
	next := settlementPending
	if attempts+1 >= settlementMaxAttempt {
		next = settlementFailed
	}
	_, err = tx.ExecContext(ctx, `UPDATE payment_settlement_tasks
		SET state = ?, attempts = attempts + 1, lease_owner = '', lease_until = NULL,
		    last_error = ?, version = version + 1, updated_at = NOW(3)
		WHERE payment_no = ?`, next, truncateReason(cause), paymentNo)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// RunSettlementWorker drives captured payments to settlement. The worker is
// opt-in via PAYMENT_SETTLEMENT_WORKER_ENABLED so the existing synchronous path
// can remain the fallback during rollout.
func RunSettlementWorker(ctx context.Context, database *sql.DB, svc *Service, logger *slog.Logger, interval time.Duration) {
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := runSettlementOnce(ctx, database, svc, logger); err != nil && logger != nil {
				logger.Error("settlement worker", "error", err)
			}
		}
	}
}

func runSettlementOnce(ctx context.Context, database *sql.DB, svc *Service, logger *slog.Logger) error {
	task, err := claimSettlementTask(ctx, database, "settlement:"+id.Event(), time.Minute)
	if err != nil || task == nil {
		return err
	}
	in, err := newRepo(database).intentByNo(ctx, database, task.PaymentNo, false)
	if err != nil {
		_ = releaseSettlementTask(ctx, database, task.PaymentNo, err.Error())
		return err
	}
	if in.Status != statusSucceeded {
		// The callback may not have committed the captured fact yet.
		return releaseSettlementTask(ctx, database, task.PaymentNo, "intent is not SUCCEEDED")
	}
	if err := svc.markPaidTCC(ctx, in); err != nil {
		_ = releaseSettlementTask(ctx, database, task.PaymentNo, err.Error())
		return err
	}
	if err := finishSettlementTask(ctx, database, task.PaymentNo, settlementDone, ""); err != nil {
		if logger != nil {
			logger.Error("mark settlement task done", "payment_no", task.PaymentNo, "error", err)
		}
	}
	return nil
}
