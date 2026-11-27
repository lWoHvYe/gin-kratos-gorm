package order

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"gin-kratos-gorm/internal/platform/apperr"
)

// commitmentReleased is the terminal AC-62 state. A release only removes the
// commitment's quota; it never touches the permit, order, group or Payment.
const commitmentReleased = "RELEASED"

// releaseReasonCodes is the closed AC-62/D-24 trigger enum. USER_WITHDRAW and
// BUSINESS_REJECT are the pre-payment application termination triggers; the
// other two are explicit human abandonment.
var releaseReasonCodes = map[string]struct{}{
	"USER_WITHDRAW":    {},
	"BUSINESS_REJECT":  {},
	"FAILED_PERMANENT": {},
	"MANUAL_ABANDON":   {},
}

type RefundCommitmentReleaseRequest struct {
	ReleaseID       string
	RefundID        string
	OrderNo         string
	GroupNo         string
	AmountCent      int64
	AuthorizationID string
	Operator        string
	AuthorizedBy    string
	ReviewedBy      string
	ReasonCode      string
	EvidenceRef     string
	AuthorizedAt    time.Time
	ReviewedAt      time.Time
	ExpiresAt       time.Time
	DryRun          bool
}

type RefundCommitmentReleaseResult struct {
	ReleaseID       string
	RefundID        string
	AmountCent      int64
	State           string
	Version         int64
	AlreadyReleased bool
}

type refundCommitmentReleaseRow struct {
	ReleaseID       string
	RefundID        string
	AmountCent      int64
	AuthorizationID string
	Operator        string
	AuthorizedBy    string
	ReviewedBy      string
	ReasonCode      string
	EvidenceRef     string
	ReviewedAt      sql.NullTime
	AuthorizedAt    time.Time
	ExpiresAt       time.Time
}

// ReleaseRefundCommitment is the only RESERVED->RELEASED path. It is manual by
// construction: the caller must supply a complete D-24 authorization, and the
// release row is the audit record. There is no worker, retry or automatic
// trigger in this package.
func (s *Service) ReleaseRefundCommitment(ctx context.Context, req RefundCommitmentReleaseRequest) (RefundCommitmentReleaseResult, error) {
	var result RefundCommitmentReleaseResult
	if s == nil || s.db == nil {
		return result, apperr.Internal("order service db is required for commitment release", nil)
	}
	req.ReleaseID = strings.TrimSpace(req.ReleaseID)
	req.RefundID = strings.TrimSpace(req.RefundID)
	req.OrderNo = strings.TrimSpace(req.OrderNo)
	req.GroupNo = strings.TrimSpace(req.GroupNo)
	req.AuthorizationID = strings.TrimSpace(req.AuthorizationID)
	req.Operator = strings.TrimSpace(req.Operator)
	req.AuthorizedBy = strings.TrimSpace(req.AuthorizedBy)
	req.ReviewedBy = strings.TrimSpace(req.ReviewedBy)
	req.ReasonCode = strings.ToUpper(strings.TrimSpace(req.ReasonCode))
	req.EvidenceRef = strings.TrimSpace(req.EvidenceRef)
	if err := validateReleaseAuthorization(req, time.Now().UTC()); err != nil {
		return result, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, apperr.Internal("begin commitment release", err)
	}
	defer func() { _ = tx.Rollback() }()

	var (
		orderNo, groupNo, state string
		amount                  int64
		version                 int64
	)
	if err := tx.QueryRowContext(ctx, `SELECT order_no, group_no, amount_cent, state, version
		FROM order_refund_commitments WHERE refund_id = ? FOR UPDATE`, req.RefundID).
		Scan(&orderNo, &groupNo, &amount, &state, &version); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return result, releaseConflict("commitment not found", map[string]any{"refund_id": req.RefundID})
		}
		return result, apperr.Internal("load refund commitment for release", err)
	}
	if req.OrderNo != "" && req.OrderNo != orderNo {
		return result, releaseConflict("commitment order does not match", map[string]any{
			"refund_id": req.RefundID, "commitment_order_no": orderNo, "request_order_no": req.OrderNo})
	}
	if req.GroupNo != "" && req.GroupNo != groupNo {
		return result, releaseConflict("commitment group does not match", map[string]any{
			"refund_id": req.RefundID, "commitment_group_no": groupNo, "request_group_no": req.GroupNo})
	}
	if req.AmountCent != amount {
		return result, releaseConflict("release amount does not equal the commitment amount", map[string]any{
			"refund_id": req.RefundID, "commitment_amount_cent": amount, "request_amount_cent": req.AmountCent})
	}

	existing, err := loadRefundCommitmentRelease(ctx, tx, req.ReleaseID)
	if err != nil {
		return result, err
	}
	if existing != nil {
		if !sameReleaseAuthorization(existing, req) {
			return result, releaseConflict("release_id already has a different authorization", map[string]any{"release_id": req.ReleaseID})
		}
		if state != commitmentReleased {
			return result, releaseConflict("release row exists but commitment is not RELEASED", map[string]any{
				"refund_id": req.RefundID, "state": state})
		}
		return RefundCommitmentReleaseResult{
			ReleaseID: req.ReleaseID, RefundID: req.RefundID, AmountCent: amount,
			State: state, Version: version, AlreadyReleased: true,
		}, nil
	}
	byRefund, err := loadRefundCommitmentReleaseByRefund(ctx, tx, req.RefundID)
	if err != nil {
		return result, err
	}
	if byRefund != nil {
		return result, releaseConflict("commitment already has a release", map[string]any{
			"refund_id": req.RefundID, "release_id": byRefund.ReleaseID})
	}
	byAuthorization, err := loadRefundCommitmentReleaseByAuthorization(ctx, tx, req.AuthorizationID)
	if err != nil {
		return result, err
	}
	if byAuthorization != nil {
		return result, releaseConflict("authorization_id is already used", map[string]any{
			"authorization_id": req.AuthorizationID, "release_id": byAuthorization.ReleaseID})
	}
	if state == commitmentSettled {
		return result, releaseConflict("commitment is SETTLED; payment already refunded", map[string]any{
			"refund_id": req.RefundID, "state": state})
	}
	if state != commitmentReserved {
		return result, releaseConflict("commitment is not RESERVED", map[string]any{
			"refund_id": req.RefundID, "state": state})
	}
	if err := validateReleaseTrigger(ctx, tx, req.RefundID, req.ReasonCode); err != nil {
		return result, err
	}
	if req.DryRun {
		return RefundCommitmentReleaseResult{
			ReleaseID: req.ReleaseID, RefundID: req.RefundID, AmountCent: amount,
			State: state, Version: version,
		}, nil
	}

	var reviewedAt any
	if !req.ReviewedAt.IsZero() {
		reviewedAt = req.ReviewedAt.UTC()
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO refund_commitment_releases
		(release_id, refund_id, amount_cent, authorization_id, operator, authorized_by,
		 reviewed_by, reason_code, evidence_ref, authorized_at, reviewed_at, expires_at, version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)`,
		req.ReleaseID, req.RefundID, amount, req.AuthorizationID, req.Operator, req.AuthorizedBy,
		req.ReviewedBy, req.ReasonCode, req.EvidenceRef, req.AuthorizedAt.UTC(), reviewedAt, req.ExpiresAt.UTC()); err != nil {
		return result, apperr.Internal("insert refund commitment release", err)
	}
	res, err := tx.ExecContext(ctx, `UPDATE order_refund_commitments
		SET state = ?, version = version + 1, updated_at = NOW(3)
		WHERE refund_id = ? AND state = ?`, commitmentReleased, req.RefundID, commitmentReserved)
	if err != nil {
		return result, apperr.Internal("release refund commitment", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return result, releaseConflict("commitment state changed during release", map[string]any{"refund_id": req.RefundID})
	}
	if err := tx.Commit(); err != nil {
		return result, apperr.Internal("commit refund release", err)
	}
	if s.log != nil {
		s.log.InfoContext(ctx, "refund commitment released",
			"refund_id", req.RefundID, "release_id", req.ReleaseID,
			"amount_cent", amount, "authorized_by", req.AuthorizedBy,
			"reason_code", req.ReasonCode, "evidence_ref", req.EvidenceRef)
	}
	return RefundCommitmentReleaseResult{
		ReleaseID: req.ReleaseID, RefundID: req.RefundID, AmountCent: amount,
		State: commitmentReleased, Version: version + 1,
	}, nil
}

func validateReleaseAuthorization(req RefundCommitmentReleaseRequest, now time.Time) error {
	for field, value := range map[string]string{
		"release_id": req.ReleaseID, "refund_id": req.RefundID,
		"authorization_id": req.AuthorizationID, "operator": req.Operator,
		"authorized_by": req.AuthorizedBy, "reason_code": req.ReasonCode,
		"evidence_ref": req.EvidenceRef,
	} {
		if value == "" {
			return releaseNotAuthorized("missing "+field, map[string]any{"field": field})
		}
	}
	if req.AmountCent <= 0 {
		return releaseNotAuthorized("amount_cent must be positive", nil)
	}
	if _, ok := releaseReasonCodes[req.ReasonCode]; !ok {
		return releaseNotAuthorized("reason_code is not an AC-62 trigger", map[string]any{"reason_code": req.ReasonCode})
	}
	if req.AuthorizedAt.IsZero() {
		return releaseNotAuthorized("authorized_at is required", nil)
	}
	if req.AuthorizedAt.After(now.Add(time.Minute)) {
		return releaseNotAuthorized("authorized_at is in the future", nil)
	}
	if req.ExpiresAt.IsZero() || !req.ExpiresAt.After(now) {
		return releaseNotAuthorized("authorization has expired", map[string]any{"expires_at": req.ExpiresAt})
	}
	if req.AmountCent >= 100000 {
		if req.ReviewedBy == "" {
			return releaseNotAuthorized("amount >= 100000 requires a second reviewer", nil)
		}
		if req.ReviewedBy == req.AuthorizedBy {
			return releaseNotAuthorized("reviewer must differ from authorized_by", nil)
		}
		if req.ReviewedAt.IsZero() {
			return releaseNotAuthorized("reviewed_at is required for a second review", nil)
		}
		if req.ReviewedAt.Before(req.AuthorizedAt) || req.ReviewedAt.After(now.Add(time.Minute)) {
			return releaseNotAuthorized("reviewed_at is outside the authorization window", nil)
		}
	}
	return nil
}

func validateReleaseTrigger(ctx context.Context, tx *sql.Tx, refundID, reasonCode string) error {
	switch reasonCode {
	case "FAILED_PERMANENT", "MANUAL_ABANDON":
		return nil
	case "USER_WITHDRAW", "BUSINESS_REJECT":
		var state string
		err := tx.QueryRowContext(ctx, `SELECT state FROM refund_applications WHERE refund_id = ?`, refundID).Scan(&state)
		if errors.Is(err, sql.ErrNoRows) {
			return releaseNotAuthorized("pre-payment trigger cannot be verified without a refund application", map[string]any{"refund_id": refundID})
		}
		if err != nil {
			return apperr.Internal("load refund application for release trigger", err)
		}
		if state != "REJECTED" && state != "CANCELLED" {
			return releaseNotAuthorized("refund application is not REJECTED/CANCELLED", map[string]any{
				"refund_id": refundID, "state": state})
		}
		return nil
	default:
		return releaseNotAuthorized("reason_code is not an AC-62 trigger", map[string]any{"reason_code": reasonCode})
	}
}

func loadRefundCommitmentRelease(ctx context.Context, q querier, releaseID string) (*refundCommitmentReleaseRow, error) {
	return loadReleaseRow(ctx, q, `SELECT release_id, refund_id, amount_cent, authorization_id, operator,
		authorized_by, reviewed_by, reason_code, evidence_ref, reviewed_at, authorized_at, expires_at
		FROM refund_commitment_releases WHERE release_id = ? FOR UPDATE`, releaseID)
}

func loadRefundCommitmentReleaseByRefund(ctx context.Context, q querier, refundID string) (*refundCommitmentReleaseRow, error) {
	return loadReleaseRow(ctx, q, `SELECT release_id, refund_id, amount_cent, authorization_id, operator,
		authorized_by, reviewed_by, reason_code, evidence_ref, reviewed_at, authorized_at, expires_at
		FROM refund_commitment_releases WHERE refund_id = ? FOR UPDATE`, refundID)
}

func loadRefundCommitmentReleaseByAuthorization(ctx context.Context, q querier, authorizationID string) (*refundCommitmentReleaseRow, error) {
	return loadReleaseRow(ctx, q, `SELECT release_id, refund_id, amount_cent, authorization_id, operator,
		authorized_by, reviewed_by, reason_code, evidence_ref, reviewed_at, authorized_at, expires_at
		FROM refund_commitment_releases WHERE authorization_id = ? FOR UPDATE`, authorizationID)
}

func loadReleaseRow(ctx context.Context, q querier, query string, key string) (*refundCommitmentReleaseRow, error) {
	var row refundCommitmentReleaseRow
	err := q.QueryRowContext(ctx, query, key).Scan(&row.ReleaseID, &row.RefundID, &row.AmountCent,
		&row.AuthorizationID, &row.Operator, &row.AuthorizedBy, &row.ReviewedBy,
		&row.ReasonCode, &row.EvidenceRef, &row.ReviewedAt, &row.AuthorizedAt, &row.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, apperr.Internal("load refund commitment release", err)
	}
	return &row, nil
}

func sameReleaseAuthorization(existing *refundCommitmentReleaseRow, req RefundCommitmentReleaseRequest) bool {
	return existing.RefundID == req.RefundID &&
		existing.AmountCent == req.AmountCent &&
		existing.AuthorizationID == req.AuthorizationID &&
		existing.Operator == req.Operator &&
		existing.AuthorizedBy == req.AuthorizedBy &&
		existing.ReviewedBy == req.ReviewedBy &&
		existing.ReasonCode == req.ReasonCode &&
		existing.EvidenceRef == req.EvidenceRef &&
		sameReleaseTime(existing.ReviewedAt, req.ReviewedAt) &&
		releaseTimesEqual(existing.AuthorizedAt, req.AuthorizedAt) &&
		releaseTimesEqual(existing.ExpiresAt, req.ExpiresAt)
}

func sameReleaseTime(stored sql.NullTime, requested time.Time) bool {
	if !stored.Valid {
		return requested.IsZero()
	}
	if requested.IsZero() {
		return false
	}
	return releaseTimesEqual(stored.Time, requested)
}

// releaseTimesEqual tolerates MySQL DATETIME(3) rounding: the driver stores
// 13:02:00.937771 as .938, so truncating both sides is not sufficient.
func releaseTimesEqual(a, b time.Time) bool {
	return a.UTC().Sub(b.UTC()).Abs() <= time.Millisecond
}

func releaseNotAuthorized(message string, details map[string]any) error {
	err := apperr.Failed("REFUND_RELEASE_NOT_AUTHORIZED: " + message)
	if details != nil {
		err = err.WithDetails(details)
	}
	return err
}

func releaseConflict(message string, details map[string]any) error {
	err := apperr.Failed("REFUND_COMMITMENT_CONFLICT: " + message)
	if details != nil {
		err = err.WithDetails(details)
	}
	return err
}
