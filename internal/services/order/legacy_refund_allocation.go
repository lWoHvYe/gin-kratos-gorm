package order

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/db"
)

// allocateLegacyRefundID resolves the AC-60 no-after-sale identity exactly
// once per (order_no, client_request_id). It allocates a sequence only when no
// mapping exists; a retry returns the first refund_id. A concurrent loser
// consumes a sequence but converges on the winner's mapping.
func (s *Service) allocateLegacyRefundID(ctx context.Context, orderNo, requestID string) (string, error) {
	orderNo = strings.TrimSpace(orderNo)
	requestID = strings.TrimSpace(requestID)
	if orderNo == "" {
		return "", apperr.Invalid("order_no is required for legacy refund allocation")
	}
	if requestID == "" {
		return "", apperr.Invalid("client_request_id is required when refund_id is empty").
			WithDetails(map[string]any{"order_no": orderNo})
	}
	var existing string
	err := s.db.QueryRowContext(ctx, `SELECT refund_id FROM legacy_refund_allocations
		WHERE order_no = ? AND request_id = ?`, orderNo, requestID).Scan(&existing)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", apperr.Internal("load legacy refund allocation", err)
	}
	seq, err := s.nextLegacyRefundSeq(ctx, orderNo)
	if err != nil {
		return "", err
	}
	refundID := legacyRefundID(orderNo, seq)
	err = db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		_, ierr := tx.ExecContext(ctx, `INSERT INTO legacy_refund_allocations
			(order_no, request_id, refund_id) VALUES (?, ?, ?)`, orderNo, requestID, refundID)
		return ierr
	})
	if err == nil {
		return refundID, nil
	}
	if isDuplicateKey(err) {
		var winner string
		if qerr := s.db.QueryRowContext(ctx, `SELECT refund_id FROM legacy_refund_allocations
			WHERE order_no = ? AND request_id = ?`, orderNo, requestID).Scan(&winner); qerr == nil {
			return winner, nil
		}
	}
	return "", apperr.Internal("store legacy refund allocation", err)
}
