package order

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/db"
)

// This file implements Tx1 of the TCC compensation (SPEC-0002 v1.8 AC-51/AC-65,
// ADR-0007 Amendment 1 §4.1.2 step 3, Plan T1.2) and its idempotent resume
// (Plan T1.3).
//
// Tx1 is deliberately short and local: the order row lock, the Σ recheck with
// the snapshot taken outside the transaction, orders.refund_state=REFUNDING
// (CAS), refund_batches(state=FROZEN), one refund_application per active group,
// the permit decisions and, for CONSUMED groups only, the durable remote
// admission plus the quota commitment and its line composition.
//
// Not in this task (and intentionally not called here): AdmitRefund, Payment,
// Fulfillment.CancelIfCreated, order.refund_requested. Those belong to T1.4+;
// leaving them out keeps Tx1 free of remote calls and lets the durable batch be
// resumed after a crash.

const (
	refundBatchSourceTCC = "tcc"
	refundBatchFrozen    = "FROZEN"
	refundBatchDone      = "DONE"
	refundBatchFailedPre = "PRECHECK_FAILED"

	refundApplicationSourceTCC = "tcc"
	refundApplicationPending   = "PENDING"
	refundApplicationDone      = "DONE"

	commitmentReserved = "RESERVED"
	commitmentSettled  = "SETTLED"
)

// tccRefundBatch mirrors one refund_batches row for one TCC compensation.
type tccRefundBatch struct {
	BatchID               string
	OrderNo               string
	PaymentNo             string
	AttemptSeq            int
	State                 string
	CapturedSnapshotCent  int64
	RefundedSnapshotCent  int64
	SumGroupCapCent       int64
	RemainingCapturedCent int64
	Version               int64
}

// tccBatchID / tccGroupRefundID are the deterministic identities of ADR §6.1:
// the batch is `tcc:<payment_no>:<attempt_seq>` (new batch only when a failed
// attempt has to be superseded) and the group refund is
// `tcc:<payment_no>:<group_no>` (stable across resumes, so a crash can never
// create a second refund identity).
func tccBatchID(paymentNo string, attemptSeq int) string {
	return fmt.Sprintf("tcc:%s:%d", paymentNo, attemptSeq)
}

func tccGroupRefundID(paymentNo, groupNo string) string {
	return "tcc:" + paymentNo + ":" + groupNo
}

func tccCommitmentPrefix(paymentNo string) string {
	return "tcc:" + paymentNo + ":"
}

func tccPermitAfterSaleNo(paymentNo string) string {
	return truncate("tcc:"+paymentNo, 64)
}

// applyTccCompensation runs the single Tx1 for a TCC refund. The Payment
// snapshot was captured outside the transaction; the Σ equation is rechecked
// inside the transaction with that same snapshot (ADR §4.1.2 step 3).
func (s *Service) applyTccCompensation(ctx context.Context, orderNo string, snapshot *tccPaymentSnapshot, paid int64) error {
	if snapshot == nil {
		return apperr.Internal("TCC compensation requires a payment snapshot", nil)
	}
	return db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		return s.freezeTccCompensationTx(ctx, tx, orderNo, snapshot, paid)
	})
}

// freezeTccCompensationTx is Tx1: lock, recheck, write REFUNDING/batch/
// applications/permits, or resume a batch that a previous run already froze.
func (s *Service) freezeTccCompensationTx(ctx context.Context, tx *sql.Tx, orderNo string, snapshot *tccPaymentSnapshot, paid int64) error {
	locked, err := findOrderForUpdate(ctx, tx, orderNo)
	if err != nil {
		return apperr.Internal("lock order for tcc compensation", err)
	}
	if locked == nil {
		return apperr.NotFound("order not found")
	}
	if locked.RefundState == refundStateFull || locked.Status == statusCancelled {
		return nil
	}
	if locked.Status != statusPendingPayment && locked.Status != statusPaid {
		return nil
	}

	batch, completed, err := findTccBatchForResume(ctx, tx, orderNo, snapshot.PaymentNo)
	if err != nil {
		return err
	}
	if completed {
		return nil
	}

	if batch == nil {
		return s.createTccCompensationTx(ctx, tx, locked, snapshot, paid)
	}
	return s.resumeTccCompensationTx(ctx, tx, locked, snapshot, batch)
}

// createTccCompensationTx is the fresh Tx1 path: strict Σ recheck (no own
// commitments exist yet), REFUNDING CAS, batch, applications and permits.
func (s *Service) createTccCompensationTx(ctx context.Context, tx *sql.Tx, locked *orderRecord, snapshot *tccPaymentSnapshot, paid int64) error {
	rechecked, err := s.recheckTccSumLocked(ctx, tx, locked, snapshot)
	if err != nil {
		return err
	}
	if rechecked.NoCaptured {
		// AC-65: no captured money is a cancel/zero-converge branch, never a
		// per-group refund. T1.6 owns that branch; until it lands this path is
		// fail-closed instead of silently skipping the Compensation.
		return apperr.Failed("TCC compensation has no captured amount").
			WithDetails(map[string]any{
				"order_no": locked.OrderNo, "payment_no": snapshot.PaymentNo,
				"pay_cent": rechecked.PayCent, "captured_cent": rechecked.CapturedCent,
			})
	}
	if err := markOrderRefundingTx(ctx, tx, locked, paid); err != nil {
		return err
	}
	batch, err := insertTccBatchTx(ctx, tx, locked.OrderNo, snapshot, rechecked)
	if err != nil {
		return err
	}
	return s.writeTccBatchGroupsTx(ctx, tx, batch, snapshot, rechecked.Groups, "")
}

// resumeTccCompensationTx is Tx1 for a rerun: the batch already froze the
// compensation, so only missing groups are filled. The Σ recheck ignores this
// compensation's own commitments (they are the quota being resumed) and must
// still match the batch snapshot; otherwise it stops before writing anything.
func (s *Service) resumeTccCompensationTx(ctx context.Context, tx *sql.Tx, locked *orderRecord, snapshot *tccPaymentSnapshot, batch *tccRefundBatch) error {
	if locked.RefundState != refundStateRefunding {
		// A batch only exists after Tx1 wrote REFUNDING in the same
		// transaction. Seeing it without REFUNDING is manual damage.
		return apperr.Failed("refund batch exists but the order is not REFUNDING; manual reconciliation required").
			WithDetails(map[string]any{
				"order_no": locked.OrderNo, "batch_id": batch.BatchID,
				"refund_state": locked.RefundState, "batch_state": batch.State,
			})
	}
	rechecked, err := s.recheckTccSumTxExcluding(ctx, tx, locked.OrderNo, snapshot, tccCommitmentPrefix(snapshot.PaymentNo))
	if err != nil {
		return err
	}
	if rechecked.NoCaptured {
		return apperr.Failed("TCC compensation resume has no captured amount").
			WithDetails(map[string]any{
				"order_no": locked.OrderNo, "batch_id": batch.BatchID,
				"payment_no": snapshot.PaymentNo, "captured_cent": rechecked.CapturedCent,
			})
	}
	if rechecked.SumGroupCapCent != batch.SumGroupCapCent || rechecked.RemainingCapturedCent != batch.RemainingCapturedCent {
		// AC-65: the authoritative captured/refunded facts changed since the
		// batch froze. Do not extend it by guessing; stop for manual review
		// (T1.4 owns MANUAL_HOLD for the paying phase).
		return apperr.Failed("TCC compensation snapshot changed since the batch was frozen").
			WithDetails(map[string]any{
				"order_no": locked.OrderNo, "batch_id": batch.BatchID, "payment_no": snapshot.PaymentNo,
				"batch_sum_group_cap_cent":      batch.SumGroupCapCent,
				"recomputed_sum_group_cap_cent": rechecked.SumGroupCapCent,
				"batch_remaining_captured_cent": batch.RemainingCapturedCent,
				"recomputed_remaining_captured": rechecked.RemainingCapturedCent,
				"recomputed_active_group_count": rechecked.ActiveGroupCount,
				"recomputed_refunded_captured":  rechecked.RefundedCent,
			})
	}
	return s.writeTccBatchGroupsTx(ctx, tx, batch, snapshot, rechecked.Groups, tccCommitmentPrefix(snapshot.PaymentNo))
}

// resumeTccCancelledOrder converges a concurrent TCC cancel: if another caller
// already froze the compensation (REFUNDING) between this call's initial read
// and its precheck, resume that batch instead of returning a spurious Σ
// mismatch. It reports converged=false when the order is not REFUNDING, so the
// caller's original precheck error stands.
func (s *Service) resumeTccCancelledOrder(ctx context.Context, orderNo, paymentNo string, paid int64) (bool, error) {
	latest, err := findOrder(ctx, s.db, orderNo)
	if err != nil {
		return false, err
	}
	if latest == nil || latest.RefundState != refundStateRefunding {
		return false, nil
	}
	snapshot, err := s.readTccPaymentSnapshot(ctx, latest, paymentNo)
	if err != nil {
		return true, err
	}
	return true, s.applyTccCompensation(ctx, orderNo, snapshot, paid)
}

// markOrderRefundingTx is the refund-state CAS of ADR §4.1.2 step 3. A
// PENDING_PAYMENT order whose money was captured is first moved to PAID without
// an order.paid event (T1.6 narrows this branch later); an already REFUNDING
// order is left untouched so a resume never rewrites lifecycle history.
func markOrderRefundingTx(ctx context.Context, tx *sql.Tx, locked *orderRecord, paid int64) error {
	if locked.RefundState == refundStateRefunding {
		return nil
	}
	now := time.Now()
	if locked.Status == statusPendingPayment && paid > 0 {
		res, err := tx.ExecContext(ctx, `UPDATE orders
			SET status = ?, paid_cent = ?, refund_state = ?, version = version + 1, updated_at = ?
			WHERE order_no = ? AND status = ? AND refund_state <> ?`,
			statusPaid, paid, refundStateRefunding, now, locked.OrderNo, statusPendingPayment, refundStateFull)
		if err != nil {
			return apperr.Internal("mark captured order paid for tcc refund", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return apperr.Conflict("order could not be moved to REFUNDING for tcc compensation").
				WithDetails(map[string]any{"order_no": locked.OrderNo, "status": locked.Status, "refund_state": locked.RefundState})
		}
		if err := insertStatusLog(ctx, tx, locked.OrderNo, statusPendingPayment, statusPaid, "payment tcc cancelled", "system"); err != nil {
			return err
		}
		return setFulfillmentGroupStatus(ctx, tx, locked.OrderNo, statusAllocated)
	}
	res, err := tx.ExecContext(ctx, `UPDATE orders
		SET refund_state = ?, version = version + 1, updated_at = ?
		WHERE order_no = ? AND refund_state <> ?`,
		refundStateRefunding, now, locked.OrderNo, refundStateFull)
	if err != nil {
		return apperr.Internal("mark order refunding for tcc compensation", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.Conflict("order could not be moved to REFUNDING for tcc compensation").
			WithDetails(map[string]any{"order_no": locked.OrderNo, "refund_state": locked.RefundState})
	}
	return nil
}

// findTccBatchForResume returns the batch a rerun must continue (T1.3) and
// whether a previous attempt already completed. Multiple resumable batches for
// one (order, payment) violate ADR §6.1 and fail closed.
func findTccBatchForResume(ctx context.Context, tx *sql.Tx, orderNo, paymentNo string) (*tccRefundBatch, bool, error) {
	rows, err := tx.QueryContext(ctx, `SELECT batch_id, order_no, payment_no, attempt_seq, state,
		captured_snapshot_cent, refunded_snapshot_cent, sum_group_cap_cent, remaining_captured_cent, version
		FROM refund_batches
		WHERE source = ? AND order_no = ? AND payment_no = ?
		ORDER BY attempt_seq DESC FOR UPDATE`, refundBatchSourceTCC, orderNo, paymentNo)
	if err != nil {
		return nil, false, apperr.Internal("load tcc refund batches", err)
	}
	var resumable []*tccRefundBatch
	completed := false
	for rows.Next() {
		batch := &tccRefundBatch{}
		if err := rows.Scan(&batch.BatchID, &batch.OrderNo, &batch.PaymentNo, &batch.AttemptSeq,
			&batch.State, &batch.CapturedSnapshotCent, &batch.RefundedSnapshotCent,
			&batch.SumGroupCapCent, &batch.RemainingCapturedCent, &batch.Version); err != nil {
			_ = rows.Close()
			return nil, false, apperr.Internal("scan tcc refund batch", err)
		}
		switch batch.State {
		case refundBatchFrozen, "ADMITTING", "ADMITTED", "PAYING", "MANUAL_HOLD", "FAILED_PERMANENT":
			resumable = append(resumable, batch)
		case refundBatchDone:
			completed = true
		case refundBatchFailedPre:
			// Audit only, no applications (ADR §6.1): a later attempt may be
			// created, but a precheck-failed row is never resumed.
		default:
			_ = rows.Close()
			return nil, false, apperr.Failed("unknown tcc refund batch state").
				WithDetails(map[string]any{"batch_id": batch.BatchID, "state": batch.State})
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, false, apperr.Internal("iterate tcc refund batches", err)
	}
	if err := rows.Close(); err != nil {
		return nil, false, apperr.Internal("close tcc refund batches", err)
	}
	if len(resumable) > 1 {
		return nil, false, apperr.Failed("multiple active TCC refund batches; manual reconciliation required").
			WithDetails(map[string]any{
				"order_no": orderNo, "payment_no": paymentNo,
				"active_batches": len(resumable), "latest_batch": resumable[0].BatchID,
			})
	}
	if len(resumable) == 1 {
		return resumable[0], false, nil
	}
	return nil, completed, nil
}

// insertTccBatchTx creates the FROZEN batch. The audit numbers are the Tx1
// calculation: a new attempt only exists after the Σ equation held.
func insertTccBatchTx(ctx context.Context, tx *sql.Tx, orderNo string, snapshot *tccPaymentSnapshot, pre *tccPrecheck) (*tccRefundBatch, error) {
	var maxSeq int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(attempt_seq),0) FROM refund_batches
		WHERE source = ? AND order_no = ? AND payment_no = ?`,
		refundBatchSourceTCC, orderNo, snapshot.PaymentNo).Scan(&maxSeq); err != nil {
		return nil, apperr.Internal("allocate tcc refund batch attempt", err)
	}
	batch := &tccRefundBatch{
		BatchID:               tccBatchID(snapshot.PaymentNo, maxSeq+1),
		OrderNo:               orderNo,
		PaymentNo:             snapshot.PaymentNo,
		AttemptSeq:            maxSeq + 1,
		State:                 refundBatchFrozen,
		CapturedSnapshotCent:  pre.CapturedCent,
		RefundedSnapshotCent:  pre.RefundedCent,
		SumGroupCapCent:       pre.SumGroupCapCent,
		RemainingCapturedCent: pre.RemainingCapturedCent,
		Version:               1,
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO refund_batches
		(batch_id, source, order_no, payment_no, attempt_seq, state, captured_snapshot_cent,
		 refunded_snapshot_cent, sum_group_cap_cent, remaining_captured_cent, attempts, version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 1)`,
		batch.BatchID, refundBatchSourceTCC, orderNo, snapshot.PaymentNo, batch.AttemptSeq, refundBatchFrozen,
		batch.CapturedSnapshotCent, batch.RefundedSnapshotCent, batch.SumGroupCapCent, batch.RemainingCapturedCent); err != nil {
		return nil, apperr.Internal("insert tcc refund batch", err)
	}
	return batch, nil
}

// writeTccBatchGroupsTx materializes the per-group ledger for the batch. Every
// write is idempotent, so a resume fills only what a crash left missing:
//
//	NONE  permit        -> CAS NONE->REVOKED, application(PRE_SHIP_REVOKED)
//	CONSUMED or CREATED -> application(PENDING_REMOTE) + admission(PENDING_REMOTE)
//	                       + RESERVED commitment with line composition
//	shipped / received  -> application(POST_SHIP_ALLOWED), permit untouched
//
// CONSUMED never writes REVOKED (ADR §4.1.3).
func (s *Service) writeTccBatchGroupsTx(ctx context.Context, tx *sql.Tx, batch *tccRefundBatch, snapshot *tccPaymentSnapshot, groups []tccGroupCap, excludeRefundPrefix string) error {
	composition, err := loadTccGroupComposition(ctx, tx, batch.OrderNo, excludeRefundPrefix)
	if err != nil {
		return err
	}
	linesByGroup, err := loadTccLinesByGroup(ctx, tx, batch.OrderNo)
	if err != nil {
		return err
	}
	for _, group := range groups {
		if !group.Active {
			continue
		}
		comp := composition[group.GroupNo]
		if comp == nil {
			comp = &tccGroupComposition{}
		}
		goodsCap := group.PaidCent - comp.CommittedGoods
		freightCap := group.FreightCent - comp.CommittedFreight
		if goodsCap < 0 || freightCap < 0 || goodsCap+freightCap != group.CapCent {
			return apperr.Failed("tcc group composition does not match the prechecked cap").
				WithDetails(map[string]any{
					"order_no": batch.OrderNo, "group_no": group.GroupNo, "batch_id": batch.BatchID,
					"goods_cap_cent": goodsCap, "freight_cap_cent": freightCap,
					"group_cap_cent":       group.CapCent,
					"committed_goods_cent": comp.CommittedGoods, "committed_freight_cent": comp.CommittedFreight,
				})
		}

		refundID := tccGroupRefundID(snapshot.PaymentNo, group.GroupNo)
		admissionResult := refundResultPre
		consumed := false
		if isShippedGroupStatus(group.GroupStatus) {
			admissionResult = refundResultPost
		} else {
			result, remote, err := decideTccPermitTx(ctx, tx, batch.OrderNo, group.GroupNo, snapshot.PaymentNo)
			if err != nil {
				return err
			}
			admissionResult, consumed = result, remote
		}

		if _, err := tx.ExecContext(ctx, `UPDATE order_fulfillment_groups
			SET refund_state = ?, version = version + 1, updated_at = NOW(3)
			WHERE order_no = ? AND group_no = ? AND refund_state <> ?`,
			refundStateRefunding, batch.OrderNo, group.GroupNo, groupRefundStateRefunded); err != nil {
			return apperr.Internal("mark group refunding for tcc compensation", err)
		}

		done, err := ensureTccApplicationTx(ctx, tx, batch, refundID, group, goodsCap, freightCap, admissionResult)
		if err != nil {
			return err
		}
		if done {
			// The group's refund application already completed; never re-open it.
			continue
		}
		if !consumed {
			continue
		}
		if err := ensureTccRemoteAdmissionTx(ctx, tx, batch.OrderNo, group.GroupNo, refundID, group.CapCent); err != nil {
			return err
		}
		if err := ensureTccCommitmentTx(ctx, tx, batch.OrderNo, group.GroupNo, refundID,
			group.CapCent, goodsCap, freightCap, comp, linesByGroup[group.GroupNo]); err != nil {
			return err
		}
	}
	return nil
}

// decideTccPermitTx reads the group permit and applies the AC-50/A-65 permit
// branch. It returns the durable admission result and whether the group takes
// the PENDING_REMOTE (CONSUMED) two-phase path. The remote CancelIfCreated call
// is deliberately NOT made here (T1.4 owns it); Tx1 only persists the decision.
func decideTccPermitTx(ctx context.Context, tx *sql.Tx, orderNo, groupNo, paymentNo string) (string, bool, error) {
	permit, err := ensurePermitTx(ctx, tx, orderNo, groupNo)
	if err != nil {
		return "", false, err
	}
	switch permit.state {
	case permitNone:
		res, err := tx.ExecContext(ctx, `UPDATE order_shipping_permits
			SET state = ?, after_sale_no = ?, version = version + 1, updated_at = NOW(3)
			WHERE order_no = ? AND fulfillment_group_no = ? AND state = ?`,
			permitRevoked, tccPermitAfterSaleNo(paymentNo), orderNo, groupNo, permitNone)
		if err != nil {
			return "", false, apperr.Internal("revoke shipping permit for tcc refund", err)
		}
		if n, _ := res.RowsAffected(); n == 1 {
			return refundResultPre, false, nil
		}
		// Lost the CAS (a concurrent Ship reached the permit). Converge by
		// re-reading, exactly like AC-50: a consumed permit takes the remote
		// path, a revoked one is idempotent.
		permit, err = ensurePermitTx(ctx, tx, orderNo, groupNo)
		if err != nil {
			return "", false, err
		}
		switch permit.state {
		case permitRevoked:
			return refundResultPre, false, nil
		case permitConsumed:
			return refundPending, true, nil
		default:
			return "", false, apperr.Failed("shipping permit state is unknown for tcc refund").
				WithDetails(map[string]any{"order_no": orderNo, "group_no": groupNo, "permit_state": permit.state})
		}
	case permitConsumed:
		return refundPending, true, nil
	case permitRevoked:
		return refundResultPre, false, nil
	default:
		// D-29: GRANTED and any other value are not legal persisted states.
		return "", false, apperr.Failed("shipping permit state is not a legal persisted state").
			WithDetails(map[string]any{"order_no": orderNo, "group_no": groupNo, "permit_state": permit.state})
	}
}

func isShippedGroupStatus(status string) bool {
	return status == "SHIPPED" || status == "RECEIVED"
}

// ensureTccApplicationTx inserts the durable per-group refund intent and
// verifies an existing one matches the batch (idempotent resume). It reports
// done=true when the application already completed (DONE), so the group is
// skipped and never refunded twice (AC-65).
func ensureTccApplicationTx(ctx context.Context, tx *sql.Tx, batch *tccRefundBatch, refundID string, group tccGroupCap, goodsCap, freightCap int64, admissionResult string) (bool, error) {
	if _, err := tx.ExecContext(ctx, `INSERT INTO refund_applications
		(refund_id, batch_id, source, order_no, group_no, non_effective_capture,
		 amount_cent, goods_cent, freight_cent, state, admission_result, version)
		VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?, ?, ?, 1)
		ON DUPLICATE KEY UPDATE refund_id = refund_id`,
		refundID, batch.BatchID, refundApplicationSourceTCC, batch.OrderNo, group.GroupNo,
		group.CapCent, goodsCap, freightCap, refundApplicationPending, admissionResult); err != nil {
		return false, apperr.Internal("insert tcc refund application", err)
	}
	var (
		existingBatch, existingGroup, existingState, existingResult string
		amount, goods, freight                                      int64
		nonEffective                                                int
	)
	if err := tx.QueryRowContext(ctx, `SELECT batch_id, group_no, amount_cent, goods_cent, freight_cent,
		state, admission_result, non_effective_capture
		FROM refund_applications WHERE refund_id = ? FOR UPDATE`, refundID).
		Scan(&existingBatch, &existingGroup, &amount, &goods, &freight,
			&existingState, &existingResult, &nonEffective); err != nil {
		return false, apperr.Internal("load tcc refund application", err)
	}
	if existingBatch != batch.BatchID || existingGroup != group.GroupNo ||
		amount != group.CapCent || goods != goodsCap || freight != freightCap || nonEffective != 0 {
		return false, apperr.Failed("tcc refund application conflicts with the prechecked group").
			WithDetails(map[string]any{
				"order_no": batch.OrderNo, "group_no": group.GroupNo, "refund_id": refundID,
				"batch_id": existingBatch, "amount_cent": amount, "goods_cent": goods, "freight_cent": freight,
				"want_amount_cent": group.CapCent, "want_goods_cent": goodsCap, "want_freight_cent": freightCap,
			})
	}
	switch existingState {
	case refundApplicationDone:
		return true, nil
	case refundApplicationPending:
	default:
		return false, apperr.Failed("tcc refund application is not resumable in its current state").
			WithDetails(map[string]any{
				"order_no": batch.OrderNo, "group_no": group.GroupNo,
				"refund_id": refundID, "state": existingState,
			})
	}
	// admission_result is monotonic: a later resume may observe the resolved
	// PRE/POST result written by the admission phase; it must never go back.
	if existingResult != "" && existingResult != admissionResult && existingResult != refundResultPre && existingResult != refundResultPost {
		return false, apperr.Failed("tcc refund application admission result conflicts with the permit decision").
			WithDetails(map[string]any{
				"order_no": batch.OrderNo, "group_no": group.GroupNo, "refund_id": refundID,
				"admission_result": existingResult, "want_admission_result": admissionResult,
			})
	}
	return false, nil
}

// ensureTccRemoteAdmissionTx persists the PENDING_REMOTE admission before any
// remote cancel/payment is attempted (ADR §4.1.2 step 3). An already resolved
// admission is left untouched and is never overwritten (T1.4 owns Tx2).
func ensureTccRemoteAdmissionTx(ctx context.Context, tx *sql.Tx, orderNo, groupNo, refundID string, amount int64) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO order_refund_admissions
		(refund_id, order_no, group_no, result, state, amount_cent, version)
		VALUES (?, ?, ?, ?, ?, ?, 1)
		ON DUPLICATE KEY UPDATE refund_id = refund_id`,
		refundID, orderNo, groupNo, refundPending, refundPending, amount); err != nil {
		return apperr.Internal("insert tcc refund admission", err)
	}
	var existingOrder, existingGroup, result, state string
	var existingAmount int64
	if err := tx.QueryRowContext(ctx, `SELECT order_no, group_no, result, state, amount_cent
		FROM order_refund_admissions WHERE refund_id = ? FOR UPDATE`, refundID).
		Scan(&existingOrder, &existingGroup, &result, &state, &existingAmount); err != nil {
		return apperr.Internal("load tcc refund admission", err)
	}
	if existingOrder != orderNo || existingGroup != groupNo || existingAmount != amount {
		return apperr.Failed("tcc refund admission conflicts with the batch").
			WithDetails(map[string]any{
				"order_no": orderNo, "group_no": groupNo, "refund_id": refundID,
				"admission_amount_cent": existingAmount, "want_amount_cent": amount,
			})
	}
	switch result {
	case refundPending, refundResultPre, refundResultPost:
		return nil
	default:
		return apperr.Failed("tcc refund admission result is not a legal value").
			WithDetails(map[string]any{"refund_id": refundID, "result": result, "state": state})
	}
}

// ensureTccCommitmentTx reserves the group's quota before the remote admission
// (AC-64/INV-24): header composition plus one line per order line and the
// freight line. It is idempotent and verifies that a resumed row still equals
// the prechecked composition.
func ensureTccCommitmentTx(ctx context.Context, tx *sql.Tx, orderNo, groupNo, refundID string, amount, goodsCap, freightCap int64, comp *tccGroupComposition, lines []tccOrderLine) error {
	expected, err := buildTccCommitmentLines(comp, lines, goodsCap, freightCap)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO order_refund_commitments
		(order_no, group_no, refund_id, amount_cent, goods_cent, freight_cent,
		 composition_known, composition_source, state, version)
		VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?, 1)
		ON DUPLICATE KEY UPDATE refund_id = refund_id`,
		orderNo, groupNo, refundID, amount, goodsCap, freightCap,
		refundApplicationSourceTCC, commitmentReserved); err != nil {
		return apperr.Internal("insert tcc refund commitment", err)
	}
	var (
		existingAmount, existingGoods, existingFreight int64
		compositionKnown                               int
		state, source                                  string
	)
	if err := tx.QueryRowContext(ctx, `SELECT amount_cent, goods_cent, freight_cent, composition_known, state, composition_source
		FROM order_refund_commitments WHERE refund_id = ? FOR UPDATE`, refundID).
		Scan(&existingAmount, &existingGoods, &existingFreight, &compositionKnown, &state, &source); err != nil {
		return apperr.Internal("load tcc refund commitment", err)
	}
	if existingAmount != amount || existingGoods != goodsCap || existingFreight != freightCap || compositionKnown != 1 {
		return apperr.Failed("tcc refund commitment conflicts with the prechecked composition").
			WithDetails(map[string]any{
				"order_no": orderNo, "group_no": groupNo, "refund_id": refundID,
				"amount_cent": existingAmount, "goods_cent": existingGoods, "freight_cent": existingFreight,
				"composition_known": compositionKnown,
				"want_amount_cent":  amount, "want_goods_cent": goodsCap, "want_freight_cent": freightCap,
			})
	}
	if state != commitmentReserved && state != commitmentSettled {
		return apperr.Failed("tcc refund commitment is not resumable in its current state").
			WithDetails(map[string]any{"order_no": orderNo, "group_no": groupNo, "refund_id": refundID, "state": state})
	}
	for _, line := range expected {
		if _, err := tx.ExecContext(ctx, `INSERT INTO order_refund_commitment_lines
			(refund_id, line_no, quantity, goods_cent, freight_cent)
			VALUES (?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE line_no = line_no`,
			refundID, line.LineNo, line.Quantity, line.GoodsCent, line.FreightCent); err != nil {
			return apperr.Internal("insert tcc commitment line", err)
		}
	}
	stored, err := loadTccCommitmentLines(ctx, tx, refundID)
	if err != nil {
		return err
	}
	if !equalTccCommitmentLines(expected, stored) {
		return apperr.Failed("tcc commitment line composition does not match the commitment header").
			WithDetails(map[string]any{
				"order_no": orderNo, "group_no": groupNo, "refund_id": refundID,
				"expected_lines": expected, "stored_lines": stored,
			})
	}
	return nil
}

type tccCommitmentLine struct {
	LineNo      int
	Quantity    int64
	GoodsCent   int64
	FreightCent int64
}

// buildTccCommitmentLines derives the new commitment's line composition from
// the order lines minus the lines already attributed to RESERVED/SETTLED
// commitments. A legacy commitment without lines cannot be attributed per line,
// so it fails closed instead of guessing (ADR §4.3).
func buildTccCommitmentLines(comp *tccGroupComposition, lines []tccOrderLine, goodsCap, freightCap int64) ([]tccCommitmentLine, error) {
	if comp == nil {
		comp = &tccGroupComposition{}
	}
	expected := make([]tccCommitmentLine, 0, len(lines)+1)
	var goodsTotal int64
	for _, line := range lines {
		lineNo := int(line.LineNo)
		committedGoods := comp.LineGoods[lineNo]
		committedQuantity := comp.LineQuantity[lineNo]
		goodsRemaining := line.LinePaidCent - committedGoods
		quantityRemaining := int64(line.Quantity) - committedQuantity
		if goodsRemaining < 0 || quantityRemaining < 0 {
			return nil, apperr.Failed("tcc commitment line is already over-committed").
				WithDetails(map[string]any{
					"line_no": lineNo, "line_paid_cent": line.LinePaidCent,
					"committed_goods_cent": committedGoods, "quantity": line.Quantity,
					"committed_quantity": committedQuantity,
				})
		}
		if goodsRemaining == 0 && quantityRemaining == 0 {
			continue
		}
		expected = append(expected, tccCommitmentLine{
			LineNo: lineNo, Quantity: quantityRemaining, GoodsCent: goodsRemaining,
		})
		goodsTotal += goodsRemaining
	}
	if goodsTotal != goodsCap {
		return nil, apperr.Failed("tcc commitment line composition cannot be attributed").
			WithDetails(map[string]any{
				"goods_cap_cent": goodsCap, "attributed_goods_cent": goodsTotal,
			})
	}
	if freightCap > 0 {
		expected = append(expected, tccCommitmentLine{LineNo: 0, FreightCent: freightCap})
	}
	sort.Slice(expected, func(i, j int) bool { return expected[i].LineNo < expected[j].LineNo })
	return expected, nil
}

func loadTccCommitmentLines(ctx context.Context, q querier, refundID string) ([]tccCommitmentLine, error) {
	rows, err := q.QueryContext(ctx, `SELECT line_no, quantity, goods_cent, freight_cent
		FROM order_refund_commitment_lines WHERE refund_id = ? ORDER BY line_no`, refundID)
	if err != nil {
		return nil, apperr.Internal("load tcc commitment lines", err)
	}
	defer rows.Close()
	lines := make([]tccCommitmentLine, 0, 4)
	for rows.Next() {
		var line tccCommitmentLine
		if err := rows.Scan(&line.LineNo, &line.Quantity, &line.GoodsCent, &line.FreightCent); err != nil {
			return nil, apperr.Internal("scan tcc commitment line", err)
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal("iterate tcc commitment lines", err)
	}
	return lines, nil
}

func equalTccCommitmentLines(want, got []tccCommitmentLine) bool {
	if len(want) != len(got) {
		return false
	}
	for i := range want {
		if want[i] != got[i] {
			return false
		}
	}
	return true
}

// tccGroupComposition is the quota already taken by RESERVED/SETTLED
// commitments of one group, with the line attribution of their lines.
type tccGroupComposition struct {
	CommittedAmount  int64
	CommittedGoods   int64
	CommittedFreight int64
	LineGoods        map[int]int64
	LineQuantity     map[int]int64
}

// loadTccGroupComposition reads commitments and their lines. A header whose
// goods+freight does not equal the committed amount (legacy row, or an old
// writer that never filled composition) cannot be attributed safely; the
// caller stops instead of inventing an attribution (ADR §4.3).
func loadTccGroupComposition(ctx context.Context, q querier, orderNo, excludeRefundPrefix string) (map[string]*tccGroupComposition, error) {
	out := make(map[string]*tccGroupComposition, 2)
	rows, err := q.QueryContext(ctx, `SELECT group_no, refund_id, amount_cent, goods_cent, freight_cent
		FROM order_refund_commitments WHERE order_no = ? AND state IN (?, ?)`,
		orderNo, commitmentReserved, commitmentSettled)
	if err != nil {
		return nil, apperr.Internal("load refund commitment composition", err)
	}
	for rows.Next() {
		var groupNo, refundID string
		var amount, goods, freight int64
		if err := rows.Scan(&groupNo, &refundID, &amount, &goods, &freight); err != nil {
			_ = rows.Close()
			return nil, apperr.Internal("scan refund commitment composition", err)
		}
		if excludeRefundPrefix != "" && strings.HasPrefix(strings.TrimSpace(refundID), excludeRefundPrefix) {
			continue
		}
		if amount != goods+freight {
			_ = rows.Close()
			return nil, apperr.Failed("refund commitment composition is incomplete; manual backfill required").
				WithDetails(map[string]any{
					"order_no": orderNo, "refund_id": refundID, "amount_cent": amount,
					"goods_cent": goods, "freight_cent": freight,
				})
		}
		groupNo = strings.TrimSpace(groupNo)
		comp := out[groupNo]
		if comp == nil {
			comp = &tccGroupComposition{LineGoods: map[int]int64{}, LineQuantity: map[int]int64{}}
			out[groupNo] = comp
		}
		comp.CommittedAmount += amount
		comp.CommittedGoods += goods
		comp.CommittedFreight += freight
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, apperr.Internal("iterate refund commitment composition", err)
	}
	if err := rows.Close(); err != nil {
		return nil, apperr.Internal("close refund commitment composition", err)
	}

	type lineKey struct {
		groupNo string
		lineNo  int
	}
	lineGoods := map[lineKey]int64{}
	lineFreight := map[string]int64{}
	lineQuantity := map[lineKey]int64{}
	lineRows, err := q.QueryContext(ctx, `SELECT c.group_no, c.refund_id, l.line_no,
		COALESCE(SUM(l.goods_cent),0), COALESCE(SUM(l.freight_cent),0), COALESCE(SUM(l.quantity),0)
		FROM order_refund_commitment_lines l
		JOIN order_refund_commitments c ON c.refund_id = l.refund_id
		WHERE c.order_no = ? AND c.state IN (?, ?)
		GROUP BY c.group_no, c.refund_id, l.line_no`, orderNo, commitmentReserved, commitmentSettled)
	if err != nil {
		return nil, apperr.Internal("load refund commitment lines", err)
	}
	for lineRows.Next() {
		var groupNo, refundID string
		var lineNo int
		var goods, freight, quantity int64
		if err := lineRows.Scan(&groupNo, &refundID, &lineNo, &goods, &freight, &quantity); err != nil {
			_ = lineRows.Close()
			return nil, apperr.Internal("scan refund commitment lines", err)
		}
		if excludeRefundPrefix != "" && strings.HasPrefix(strings.TrimSpace(refundID), excludeRefundPrefix) {
			continue
		}
		groupNo = strings.TrimSpace(groupNo)
		key := lineKey{groupNo: groupNo, lineNo: lineNo}
		if lineNo > 0 {
			lineGoods[key] += goods
			lineQuantity[key] += quantity
		} else {
			lineFreight[groupNo] += freight
		}
	}
	if err := lineRows.Err(); err != nil {
		_ = lineRows.Close()
		return nil, apperr.Internal("iterate refund commitment lines", err)
	}
	if err := lineRows.Close(); err != nil {
		return nil, apperr.Internal("close refund commitment lines", err)
	}
	for groupNo, comp := range out {
		var goods, freight int64
		for key, value := range lineGoods {
			if key.groupNo == groupNo {
				goods += value
				comp.LineGoods[key.lineNo] = value
			}
		}
		for key, value := range lineQuantity {
			if key.groupNo == groupNo {
				comp.LineQuantity[key.lineNo] = value
			}
		}
		freight = lineFreight[groupNo]
		if goods != comp.CommittedGoods || freight != comp.CommittedFreight {
			return nil, apperr.Failed("refund commitment lines do not match the commitment headers").
				WithDetails(map[string]any{
					"order_no": orderNo, "group_no": groupNo,
					"header_goods_cent": comp.CommittedGoods, "line_goods_cent": goods,
					"header_freight_cent": comp.CommittedFreight, "line_freight_cent": freight,
				})
		}
	}
	return out, nil
}

type tccOrderLine struct {
	LineNo       int32
	Quantity     int32
	LinePaidCent int64
}

// loadTccLinesByGroup groups the order lines, keeping the same empty->G1
// fallback as the Σ precheck.
func loadTccLinesByGroup(ctx context.Context, q querier, orderNo string) (map[string][]tccOrderLine, error) {
	rows, err := q.QueryContext(ctx, `SELECT line_no, quantity, line_paid_cent, fulfillment_group_no
		FROM order_items WHERE order_no = ? ORDER BY line_no`, orderNo)
	if err != nil {
		return nil, apperr.Internal("load order lines for tcc composition", err)
	}
	defer rows.Close()
	out := make(map[string][]tccOrderLine, 2)
	for rows.Next() {
		var line tccOrderLine
		var groupNo string
		if err := rows.Scan(&line.LineNo, &line.Quantity, &line.LinePaidCent, &groupNo); err != nil {
			return nil, apperr.Internal("scan order line for tcc composition", err)
		}
		groupNo = strings.TrimSpace(groupNo)
		if groupNo == "" {
			groupNo = defaultGroupNo
		}
		out[groupNo] = append(out[groupNo], line)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal("iterate order lines for tcc composition", err)
	}
	return out, nil
}
