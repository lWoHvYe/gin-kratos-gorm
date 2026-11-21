package aftersales

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// amountLedger is the per-order lifetime refund budget. It lives in the
// aftersales database so concurrent applications serialize locally; captured
// comes from Payment (or the order snapshot only for legacy data).
type amountLedger struct {
	orderNo  string
	captured int64
	consumed int64
	source   string
	version  int64
}

func (s *Service) loadOrder(ctx context.Context, orderNo string, userID int64) (*pb.Order, error) {
	if s.orders == nil {
		return nil, apperr.Unavailable("order client is not configured", nil)
	}
	order, err := s.orders.GetOrder(ctx, &pb.GetOrderRequest{OrderNo: orderNo, UserId: userID})
	if err != nil {
		return nil, apperr.Unavailable("load order", err)
	}
	if order.GetOrderNo() == "" || order.GetUserId() != userID {
		return nil, apperr.NotFound("order not found")
	}
	return order, nil
}

// capturedForOrder prefers Payment captured_cent. The order paid_cent snapshot
// is only a legacy fallback when Payment has no settled intent.
func (s *Service) capturedForOrder(ctx context.Context, order *pb.Order) (int64, string, error) {
	if s.payments != nil && order.GetOrderNo() != "" {
		intent, err := s.payments.GetPaymentByOrder(ctx, &pb.GetPaymentByOrderRequest{OrderNo: order.GetOrderNo()})
		if err == nil {
			if intent.GetCapturedCent() > 0 {
				return intent.GetCapturedCent(), "payment", nil
			}
		} else if !apperr.IsCode(err, apperr.CodeNotFound) && status.Code(err) != codes.NotFound {
			return 0, "", apperr.Unavailable("load payment facts for after-sale", err)
		}
	}
	if order.GetPaidCent() > 0 {
		return order.GetPaidCent(), "order_snapshot", nil
	}
	return 0, "", apperr.Failed("order has no captured payment to refund")
}

func (s *Service) ensureAmountLedgerTx(ctx context.Context, tx *sql.Tx, orderNo string, captured int64, source string) (*amountLedger, error) {
	if captured <= 0 {
		return nil, apperr.Failed("captured payment amount is required for after-sale")
	}
	source = strings.TrimSpace(source)
	if source == "" {
		source = "payment"
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO after_sale_amount_ledger
		(order_no, captured_cent, consumed_cent, source, version, created_at, updated_at)
		VALUES (?, ?, 0, ?, 1, NOW(3), NOW(3))
		ON DUPLICATE KEY UPDATE order_no = order_no`, orderNo, captured, source); err != nil {
		return nil, apperr.Internal("ensure amount ledger", err)
	}
	var ledger amountLedger
	if err := tx.QueryRowContext(ctx, `SELECT order_no, captured_cent, consumed_cent, source, version
		FROM after_sale_amount_ledger WHERE order_no = ? FOR UPDATE`, orderNo).
		Scan(&ledger.orderNo, &ledger.captured, &ledger.consumed, &ledger.source, &ledger.version); err != nil {
		return nil, apperr.Internal("lock amount ledger", err)
	}
	if ledger.consumed < 0 {
		return nil, apperr.Conflict("after-sale amount ledger has a negative consumed amount")
	}
	if ledger.captured != captured || ledger.source == "uninitialized" {
		if captured < ledger.consumed {
			return nil, apperr.Conflict(fmt.Sprintf(
				"captured payment %d is below already consumed after-sale amount %d", captured, ledger.consumed))
		}
		if _, err := tx.ExecContext(ctx, `UPDATE after_sale_amount_ledger
			SET captured_cent = ?, source = ?, version = version + 1, updated_at = NOW(3)
			WHERE order_no = ?`, captured, source, orderNo); err != nil {
			return nil, apperr.Internal("refresh amount ledger capture", err)
		}
		ledger.captured = captured
		ledger.source = source
		ledger.version++
	}
	return &ledger, nil
}

func reserveAmountTx(ctx context.Context, tx *sql.Tx, ledger *amountLedger, amountCent int64) error {
	if amountCent <= 0 {
		return apperr.Invalid("effective amount must be positive")
	}
	if ledger.consumed+amountCent > ledger.captured {
		return apperr.Invalid("after-sale amount %d exceeds remaining refundable amount %d",
			amountCent, ledger.captured-ledger.consumed)
	}
	res, err := tx.ExecContext(ctx, `UPDATE after_sale_amount_ledger
		SET consumed_cent = consumed_cent + ?, version = version + 1, updated_at = NOW(3)
		WHERE order_no = ? AND consumed_cent + ? <= captured_cent`,
		amountCent, ledger.orderNo, amountCent)
	if err != nil {
		return apperr.Internal("reserve after-sale amount", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return apperr.Conflict("after-sale amount ledger changed concurrently")
	}
	ledger.consumed += amountCent
	ledger.version++
	return nil
}

func releaseAmountTx(ctx context.Context, tx *sql.Tx, ledger *amountLedger, amountCent int64) error {
	if amountCent <= 0 {
		return nil
	}
	res, err := tx.ExecContext(ctx, `UPDATE after_sale_amount_ledger
		SET consumed_cent = consumed_cent - ?, version = version + 1, updated_at = NOW(3)
		WHERE order_no = ? AND consumed_cent >= ?`, amountCent, ledger.orderNo, amountCent)
	if err != nil {
		return apperr.Internal("release after-sale amount", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return apperr.Conflict("after-sale amount ledger has insufficient consumed amount")
	}
	ledger.consumed -= amountCent
	ledger.version++
	return nil
}

func appendAmountVersionTx(ctx context.Context, tx *sql.Tx, afterSaleID, afterSaleVersion, amountCent int64, reason string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO after_sale_amount_versions
		(after_sale_id, version, effective_amount_cent, reason)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE effective_amount_cent = effective_amount_cent`,
		afterSaleID, afterSaleVersion, amountCent, cutRunes(reason, 64))
	if err != nil {
		return apperr.Internal("append amount version", err)
	}
	return nil
}

func (s *Service) getAfterSaleRow(ctx context.Context, afterSaleNo string) (*afterSaleRow, error) {
	var row afterSaleRow
	err := s.db.QueryRowContext(ctx, `SELECT id, order_no, user_id, `+"`type`"+`, status, amount_cent,
		effective_amount_cent, amount_state, restock_state, version, reason, return_carrier,
		return_tracking_no, refund_no, refunded_cent
		FROM after_sales WHERE after_sale_no = ?`, afterSaleNo).
		Scan(&row.id, &row.orderNo, &row.userID, &row.typ, &row.status, &row.amountCent,
			&row.effectiveAmountCent, &row.amountState, &row.restockState, &row.version, &row.reason,
			&row.returnCarrier, &row.returnTracking, &row.refundNo, &row.refundedCent)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("after sale not found")
	}
	if err != nil {
		return nil, apperr.Internal("load after sale row", err)
	}
	return &row, nil
}
