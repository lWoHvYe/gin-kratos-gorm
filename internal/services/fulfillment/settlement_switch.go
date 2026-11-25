package fulfillment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"gin-kratos-gorm/internal/platform/config"
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

// Stable reason tokens for the startup rejection log (Addendum-r0.4 §2.1.4).
// The values live in internal/platform/config so the config errors and the
// process log cannot drift; these aliases keep the gate's API local.
const (
	ReasonN5ApprovalIDModeMismatch = config.ReasonN5ApprovalIDModeMismatch
	ReasonModeUnknown              = config.ReasonModeUnknown
	ReasonReleaseRangeMissing      = config.ReasonReleaseRangeMissing
	ReasonApprovalRowMissing       = config.ReasonApprovalRowMissing
	ReasonApprovalNotApproved      = config.ReasonApprovalNotApproved
	ReasonApprovalExpired          = config.ReasonApprovalExpired
	ReasonReleaseRangeMismatch     = config.ReasonReleaseRangeMismatch
	ReasonApprovalDBUnavailable    = config.ReasonApprovalDBUnavailable
	ReasonShadowNotDelivered       = config.ReasonShadowNotDelivered
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
// by Addendum-r0.4 §2.1.4: mode, approval id and a stable reason code, plus a
// human-readable detail. Callers must treat it as fatal (non-zero exit); there
// is no paid fallback.
type SettlementSwitchError struct {
	Mode       string
	ApprovalID string
	Reason     string
	Detail     string
	Err        error
}

func (e *SettlementSwitchError) Error() string {
	base := fmt.Sprintf("settlement switch rejected: mode=%q approval_id=%q reason=%s", e.Mode, e.ApprovalID, e.Reason)
	if e.Detail != "" {
		base += " detail=" + e.Detail
	}
	if e.Err != nil {
		return base + ": " + e.Err.Error()
	}
	return base
}

func (e *SettlementSwitchError) Unwrap() error { return e.Err }

func switchError(mode, approvalID, reason, detail string, err error) error {
	return &SettlementSwitchError{Mode: mode, ApprovalID: approvalID, Reason: reason, Detail: detail, Err: err}
}

// VerifySettlementSwitch performs the read-only startup check against
// settlement_switch_approvals before any consumer/worker is registered.
//
//   - paid: SETTLE_RELEASE_APPROVAL_ID must be empty (N5); no DB read.
//   - shadow: approval id must be empty; at least one APPROVED shadow row that
//     is unexpired (expires_at IS NULL OR expiry > DB NOW(3)) and whose
//     release_range equals SETTLE_RELEASE_RANGE. Passing this gate is still not
//     enough to run shadow: T4.4 is not delivered, so main exits 4.
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
			return SettlementApproval{}, switchError(mode, approvalID, ReasonN5ApprovalIDModeMismatch,
				"SETTLE_RELEASE_APPROVAL_ID must be empty when the requested mode is paid", nil)
		}
		return SettlementApproval{}, nil
	case SettlementModeShadow:
		if approvalID != "" {
			return SettlementApproval{}, switchError(mode, approvalID, ReasonN5ApprovalIDModeMismatch,
				"SETTLE_RELEASE_APPROVAL_ID must be empty when the requested mode is shadow", nil)
		}
		if releaseRange == "" {
			return SettlementApproval{}, switchError(mode, approvalID, ReasonReleaseRangeMissing,
				"SETTLE_RELEASE_RANGE is required for shadow (empty range is never a wildcard)", nil)
		}
		if db == nil {
			return SettlementApproval{}, switchError(mode, approvalID, ReasonApprovalDBUnavailable,
				"database is required to read settlement_switch_approvals", nil)
		}
		return verifyShadowApproval(ctx, db, releaseRange)
	case SettlementModeSettled:
		if approvalID == "" {
			return SettlementApproval{}, switchError(mode, approvalID, ReasonApprovalRowMissing,
				"SETTLE_RELEASE_APPROVAL_ID is required for settled", nil)
		}
		if releaseRange == "" {
			return SettlementApproval{}, switchError(mode, approvalID, ReasonReleaseRangeMissing,
				"SETTLE_RELEASE_RANGE is required for settled (empty range is never a wildcard)", nil)
		}
		if db == nil {
			return SettlementApproval{}, switchError(mode, approvalID, ReasonApprovalDBUnavailable,
				"database is required to read settlement_switch_approvals", nil)
		}
		return verifySettledApproval(ctx, db, approvalID, releaseRange)
	default:
		return SettlementApproval{}, switchError(mode, approvalID, ReasonModeUnknown,
			fmt.Sprintf("unknown settlement consumer mode %q (want paid, shadow or settled)", mode), nil)
	}
}

// approvalRow is one settlement_switch_approvals row plus the DB-clock expiry
// verdict computed by the query.
type approvalRow struct {
	ApprovalID   string
	TargetMode   string
	Status       string
	ReleaseRange string
	ExpiresAt    sql.NullTime
	Unexpired    bool
}

// classifyShadowRow returns a stable reason code for the first failing shadow
// condition, or "" when the row satisfies the shadow gate.
func classifyShadowRow(row approvalRow, releaseRange string) (reason, detail string) {
	switch {
	case row.Status != approvalStatusApproved:
		return ReasonApprovalNotApproved, fmt.Sprintf("%s: status=%s", row.ApprovalID, row.Status)
	case !row.Unexpired:
		return ReasonApprovalExpired, fmt.Sprintf("%s: expired at %s", row.ApprovalID,
			row.ExpiresAt.Time.Format("2006-01-02T15:04:05Z07:00"))
	case row.ReleaseRange != releaseRange:
		return ReasonReleaseRangeMismatch, fmt.Sprintf("%s: release_range=%q does not match deployment range %q",
			row.ApprovalID, row.ReleaseRange, releaseRange)
	default:
		return "", ""
	}
}

// classifySettledRow returns a stable reason code for the first failing settled
// condition, or "" when the row satisfies the settled gate. A row whose
// target_mode is not settled does not approve the requested mode and is
// reported as APPROVAL_NOT_APPROVED with the target_mode in the detail (the
// accepted reason vocabulary has no separate target-mode token).
func classifySettledRow(row approvalRow, releaseRange string) (reason, detail string) {
	switch {
	case row.TargetMode != SettlementModeSettled:
		return ReasonApprovalNotApproved, fmt.Sprintf("target_mode=%q (want settled)", row.TargetMode)
	case row.Status != approvalStatusApproved:
		return ReasonApprovalNotApproved, fmt.Sprintf("status=%q (want APPROVED)", row.Status)
	case !row.ExpiresAt.Valid:
		return ReasonApprovalExpired, "expires_at is NULL (settled requires a finite approval expiry)"
	case !row.Unexpired:
		return ReasonApprovalExpired, fmt.Sprintf("expired at %s",
			row.ExpiresAt.Time.Format("2006-01-02T15:04:05Z07:00"))
	case row.ReleaseRange != releaseRange:
		return ReasonReleaseRangeMismatch, fmt.Sprintf("release_range=%q does not match deployment range %q",
			row.ReleaseRange, releaseRange)
	default:
		return "", ""
	}
}

func verifyShadowApproval(ctx context.Context, db *sql.DB, releaseRange string) (SettlementApproval, error) {
	// Read every shadow row (any status) so the rejection names the actual
	// condition instead of collapsing to "missing". Multiple rows are allowed;
	// one fully valid APPROVED/unexpired/range-matching row is sufficient. The
	// ordering makes the reported reason deterministic.
	rows, err := db.QueryContext(ctx, `
		SELECT approval_id, target_mode, status, release_range, expires_at,
		       (expires_at IS NULL OR expires_at > NOW(3))
		FROM settlement_switch_approvals
		WHERE target_mode = 'shadow'
		ORDER BY approved_at DESC, approval_id ASC`)
	if err != nil {
		return SettlementApproval{}, switchError(SettlementModeShadow, "", ReasonApprovalDBUnavailable,
			"cannot read settlement_switch_approvals", err)
	}
	defer rows.Close()

	var (
		seen      int
		firstCode string
		details   []string
	)
	for rows.Next() {
		seen++
		var row approvalRow
		if err := rows.Scan(&row.ApprovalID, &row.TargetMode, &row.Status, &row.ReleaseRange,
			&row.ExpiresAt, &row.Unexpired); err != nil {
			return SettlementApproval{}, switchError(SettlementModeShadow, "", ReasonApprovalDBUnavailable,
				"cannot scan settlement_switch_approvals", err)
		}
		reason, detail := classifyShadowRow(row, releaseRange)
		if reason == "" {
			return SettlementApproval{
				ApprovalID:   row.ApprovalID,
				TargetMode:   row.TargetMode,
				ReleaseRange: row.ReleaseRange,
				ExpiresAt:    row.ExpiresAt,
			}, nil
		}
		if firstCode == "" {
			firstCode = reason
		}
		details = append(details, detail)
	}
	if err := rows.Err(); err != nil {
		return SettlementApproval{}, switchError(SettlementModeShadow, "", ReasonApprovalDBUnavailable,
			"cannot read settlement_switch_approvals", err)
	}
	if seen == 0 {
		return SettlementApproval{}, switchError(SettlementModeShadow, "", ReasonApprovalRowMissing,
			"no shadow approval row exists", nil)
	}
	return SettlementApproval{}, switchError(SettlementModeShadow, "", firstCode,
		"no APPROVED, unexpired shadow approval row with a matching release range ("+strings.Join(details, "; ")+")", nil)
}

func verifySettledApproval(ctx context.Context, db *sql.DB, approvalID, releaseRange string) (SettlementApproval, error) {
	var row approvalRow
	err := db.QueryRowContext(ctx, `
		SELECT approval_id, target_mode, status, release_range, expires_at,
		       (expires_at IS NOT NULL AND expires_at > NOW(3))
		FROM settlement_switch_approvals
		WHERE approval_id = ?`, approvalID).
		Scan(&row.ApprovalID, &row.TargetMode, &row.Status, &row.ReleaseRange, &row.ExpiresAt, &row.Unexpired)
	if errors.Is(err, sql.ErrNoRows) {
		return SettlementApproval{}, switchError(SettlementModeSettled, approvalID, ReasonApprovalRowMissing,
			"approval row not found", nil)
	}
	if err != nil {
		return SettlementApproval{}, switchError(SettlementModeSettled, approvalID, ReasonApprovalDBUnavailable,
			"cannot read settlement_switch_approvals", err)
	}
	if reason, detail := classifySettledRow(row, releaseRange); reason != "" {
		return SettlementApproval{}, switchError(SettlementModeSettled, approvalID, reason, detail, nil)
	}
	return SettlementApproval{
		ApprovalID:   row.ApprovalID,
		TargetMode:   row.TargetMode,
		ReleaseRange: row.ReleaseRange,
		ExpiresAt:    row.ExpiresAt,
	}, nil
}
