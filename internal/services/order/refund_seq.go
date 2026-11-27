package order

import (
	"context"
	"strconv"
	"strings"

	"gin-kratos-gorm/internal/platform/apperr"
)

// nextLegacyRefundSeq allocates the next `legacy:<order_no>:<seq>` number.
// The caller must already hold the orders row lock so refund creation for one
// order is serialized. The sequence itself is committed in an independent
// short transaction (ADR-0010): a later refund rollback must not reuse the
// number. Gaps are allowed; only committed refund ids are guaranteed unique.
//
// The INSERT ... ON DUPLICATE KEY UPDATE form allocates distinct numbers under
// concurrency because each transaction locks the sequence row; LAST_INSERT_ID
// is connection-scoped and read back before the short transaction commits.
func (s *Service) nextLegacyRefundSeq(ctx context.Context, orderNo string) (int64, error) {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return 0, apperr.Invalid("order_no is required for legacy refund sequence")
	}
	if s == nil || s.db == nil {
		return 0, apperr.Internal("legacy refund sequence requires a service db", nil)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, apperr.Internal("begin legacy refund sequence transaction", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `INSERT INTO order_refund_sequences (order_no, next_seq)
		VALUES (?, LAST_INSERT_ID(1))
		ON DUPLICATE KEY UPDATE next_seq = LAST_INSERT_ID(next_seq + 1),
			version = version + 1`, orderNo); err != nil {
		return 0, apperr.Internal("allocate legacy refund sequence", err)
	}
	var seq int64
	if err := tx.QueryRowContext(ctx, `SELECT LAST_INSERT_ID()`).Scan(&seq); err != nil {
		return 0, apperr.Internal("read legacy refund sequence", err)
	}
	if seq < 1 {
		return 0, apperr.Internal("legacy refund sequence out of range", nil)
	}
	if err := tx.Commit(); err != nil {
		return 0, apperr.Internal("commit legacy refund sequence", err)
	}
	return seq, nil
}

// legacyRefundID builds the SPEC-0002 v1.8 refund identity for a historical
// order without an after-sales number. The sequence is the only variable part;
// callers store it with the refund so retries reuse the same identity.
func legacyRefundID(orderNo string, seq int64) string {
	return "legacy:" + strings.TrimSpace(orderNo) + ":" + strconv.FormatInt(seq, 10)
}
