package fulfillment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Settlement consumer modes (ADR-0007 Rev 2.3 §3.2). The config package owns
// the env parsing; Fulfillment re-declares the accepted values so the startup
// gate never depends on an unvalidated string.
const (
	SettlementModePaid    = "paid"
	SettlementModeShadow  = "shadow"
	SettlementModeSettled = "settled"

	approvalStatusApproved = "APPROVED"
)

// SettlementSwitchConfig is the startup view of the settlement switch. It is
// deliberately separate from config.Config so the verification rules can be
// tested without loading the whole process environment.
type SettlementSwitchConfig struct {
	Mode         string
	ApprovalID   string
	ReleaseRange string
}

// SettlementApproval is the approval row accepted at startup. For paid mode it
// is the zero value (paid needs no row).
type SettlementApproval struct {
	ApprovalID   string
	TargetMode   string
	ReleaseRange string
	ExpiresAt    sql.NullTime
}

// SettlementSwitchError carries the operator-facing rejection context required
// by ADR-0007 Rev 2.3 §3.2: mode, approval id and reason. Callers must treat it
// as fatal (non-zero exit); there is no paid fallback.
type SettlementSwitchError struct {
	Mode       string
	ApprovalID string
	Reason     string
	Err        error
}

func (e *SettlementSwitchError) Error() string {
	base := fmt.Sprintf("settlement switch rejected: mode=%q approval_id=%q reason=%s", e.Mode, e.ApprovalID, e.Reason)
	if e.Err != nil {
		return base + ": " + e.Err.Error()
	}
	return base
}

func (e *SettlementSwitchError) Unwrap() error { return e.Err }

func switchError(mode, approvalID, reason string, err error) error {
	return &SettlementSwitchError{Mode: mode, ApprovalID: approvalID, Reason: reason, Err: err}
}

// VerifySettlementSwitch performs the read-only startup check against
// settlement_switch_approvals before any consumer/worker is registered.
//
//   - paid: SETTLE_RELEASE_APPROVAL_ID must be empty (N5); no DB read.
//   - shadow: approval id must be empty; at least one APPROVED shadow row that
//     is unexpired (expires_at IS NULL OR expiry > DB NOW(3)) and whose
//     release_range equals SETTLE_RELEASE_RANGE.
//   - settled: approval id non-empty; the row must be target_mode='settled',
//     status='APPROVED', expires_at NOT NULL and unexpired, and its
//     release_range must equal SETTLE_RELEASE_RANGE.
//
// Any failure returns an error; the caller must exit non-zero and must not
// fall back to paid. The function never writes/updates approval rows and has
// no runtime hot-switch behaviour. Expiry is evaluated by the database clock
// (NOW(3)) so a skewed application host cannot extend an approval.
func VerifySettlementSwitch(ctx context.Context, db *sql.DB, cfg SettlementSwitchConfig) (SettlementApproval, error) {
	mode := strings.TrimSpace(cfg.Mode)
	approvalID := strings.TrimSpace(cfg.ApprovalID)
	releaseRange := strings.TrimSpace(cfg.ReleaseRange)

	switch mode {
	case SettlementModePaid:
		if approvalID != "" {
			return SettlementApproval{}, switchError(mode, approvalID, "N5: SETTLE_RELEASE_APPROVAL_ID must be empty when the requested mode is paid", nil)
		}
		return SettlementApproval{}, nil
	case SettlementModeShadow:
		if approvalID != "" {
			return SettlementApproval{}, switchError(mode, approvalID, "N5: SETTLE_RELEASE_APPROVAL_ID must be empty when the requested mode is shadow", nil)
		}
		if releaseRange == "" {
			return SettlementApproval{}, switchError(mode, approvalID, "SETTLE_RELEASE_RANGE is required for shadow (empty range is never a wildcard)", nil)
		}
		if db == nil {
			return SettlementApproval{}, switchError(mode, approvalID, "database is required to read settlement_switch_approvals", nil)
		}
		return verifyShadowApproval(ctx, db, releaseRange)
	case SettlementModeSettled:
		if approvalID == "" {
			return SettlementApproval{}, switchError(mode, approvalID, "SETTLE_RELEASE_APPROVAL_ID is required for settled", nil)
		}
		if releaseRange == "" {
			return SettlementApproval{}, switchError(mode, approvalID, "SETTLE_RELEASE_RANGE is required for settled (empty range is never a wildcard)", nil)
		}
		if db == nil {
			return SettlementApproval{}, switchError(mode, approvalID, "database is required to read settlement_switch_approvals", nil)
		}
		return verifySettledApproval(ctx, db, approvalID, releaseRange)
	default:
		return SettlementApproval{}, switchError(mode, approvalID, fmt.Sprintf("unknown settlement consumer mode %q (want paid, shadow or settled)", mode), nil)
	}
}

func verifyShadowApproval(ctx context.Context, db *sql.DB, releaseRange string) (SettlementApproval, error) {
	// Read every shadow row (any status) so the rejection names the actual
	// condition instead of collapsing to "missing". Multiple rows are allowed;
	// one fully valid APPROVED/unexpired/range-matching row is sufficient.
	rows, err := db.QueryContext(ctx, `
		SELECT approval_id, target_mode, status, release_range, expires_at,
		       (expires_at IS NULL OR expires_at > NOW(3))
		FROM settlement_switch_approvals
		WHERE target_mode = 'shadow'
		ORDER BY approved_at DESC, approval_id ASC`)
	if err != nil {
		return SettlementApproval{}, switchError(SettlementModeShadow, "", "cannot read settlement_switch_approvals", err)
	}
	defer rows.Close()

	var reasons []string
	for rows.Next() {
		var (
			approvalID, targetMode, status, rowRange string
			expiresAt                                sql.NullTime
			unexpired                                bool
		)
		if err := rows.Scan(&approvalID, &targetMode, &status, &rowRange, &expiresAt, &unexpired); err != nil {
			return SettlementApproval{}, switchError(SettlementModeShadow, "", "cannot scan settlement_switch_approvals", err)
		}
		switch {
		case status != approvalStatusApproved:
			reasons = append(reasons, fmt.Sprintf("%s: status=%s", approvalID, status))
		case !unexpired:
			reasons = append(reasons, fmt.Sprintf("%s: expired at %s", approvalID, expiresAt.Time.Format("2006-01-02T15:04:05Z07:00")))
		case rowRange != releaseRange:
			reasons = append(reasons, fmt.Sprintf("%s: release_range=%q does not match deployment range %q", approvalID, rowRange, releaseRange))
		default:
			return SettlementApproval{
				ApprovalID:   approvalID,
				TargetMode:   targetMode,
				ReleaseRange: rowRange,
				ExpiresAt:    expiresAt,
			}, nil
		}
	}
	if err := rows.Err(); err != nil {
		return SettlementApproval{}, switchError(SettlementModeShadow, "", "cannot read settlement_switch_approvals", err)
	}
	reason := "no APPROVED, unexpired shadow approval row with a matching release range"
	if len(reasons) > 0 {
		reason += " (" + strings.Join(reasons, "; ") + ")"
	}
	return SettlementApproval{}, switchError(SettlementModeShadow, "", reason, nil)
}

func verifySettledApproval(ctx context.Context, db *sql.DB, approvalID, releaseRange string) (SettlementApproval, error) {
	var (
		rowID, targetMode, status, rowRange string
		expiresAt                           sql.NullTime
		unexpired                           bool
	)
	err := db.QueryRowContext(ctx, `
		SELECT approval_id, target_mode, status, release_range, expires_at,
		       (expires_at IS NOT NULL AND expires_at > NOW(3))
		FROM settlement_switch_approvals
		WHERE approval_id = ?`, approvalID).
		Scan(&rowID, &targetMode, &status, &rowRange, &expiresAt, &unexpired)
	if errors.Is(err, sql.ErrNoRows) {
		return SettlementApproval{}, switchError(SettlementModeSettled, approvalID, "approval row not found", nil)
	}
	if err != nil {
		return SettlementApproval{}, switchError(SettlementModeSettled, approvalID, "cannot read settlement_switch_approvals", err)
	}

	var reasons []string
	if targetMode != SettlementModeSettled {
		reasons = append(reasons, fmt.Sprintf("target_mode=%q (want settled)", targetMode))
	}
	if status != approvalStatusApproved {
		reasons = append(reasons, fmt.Sprintf("status=%q (want APPROVED)", status))
	}
	if !expiresAt.Valid {
		reasons = append(reasons, "expires_at is NULL (settled requires a finite approval expiry)")
	} else if !unexpired {
		reasons = append(reasons, fmt.Sprintf("expired at %s", expiresAt.Time.Format("2006-01-02T15:04:05Z07:00")))
	}
	if rowRange != releaseRange {
		reasons = append(reasons, fmt.Sprintf("release_range=%q does not match deployment range %q", rowRange, releaseRange))
	}
	if len(reasons) > 0 {
		return SettlementApproval{}, switchError(SettlementModeSettled, approvalID, strings.Join(reasons, "; "), nil)
	}
	return SettlementApproval{
		ApprovalID:   rowID,
		TargetMode:   targetMode,
		ReleaseRange: rowRange,
		ExpiresAt:    expiresAt,
	}, nil
}
