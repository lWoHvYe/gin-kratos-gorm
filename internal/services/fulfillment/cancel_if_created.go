package fulfillment

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

// CancelIfCreated is the refund-admission probe. It only cancels a CREATED
// shipment; a shipment that already left the warehouse is reported back to the
// caller and is never rolled back.
//
// R-2 (ADR-0007 Amd1 Addendum 2 §3.2.5): the S6 warehouse-exit fact is
// `exception_state != NONE && shipped_at` (a carrier exception after pickup)
// in addition to `status SHIPPED/RECEIVED`. Such a shipment must answer
// ALREADY_SHIPPED instead of being cancelled or looping on TRANSIENT_ERROR;
// an exception without `shipped_at` is ambiguous, is never cancelled and
// answers TRANSIENT_ERROR for a retry/manual reconciliation.
func (s *Service) CancelIfCreated(ctx context.Context, req *pb.CancelIfCreatedRequest) (*pb.CancelIfCreatedReply, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	groupNo := strings.TrimSpace(req.GetGroupNo())
	if groupNo == "" {
		groupNo = "G1"
	}
	shipmentNo := strings.TrimSpace(req.GetShipmentNo())

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin cancel if created", err)
	}
	defer func() { _ = tx.Rollback() }()

	var status, exceptionState string
	var shippedAtUnix int64
	query := `SELECT status, exception_state, CAST(COALESCE(UNIX_TIMESTAMP(shipped_at), 0) AS SIGNED)
		FROM shipments WHERE order_no = ? AND fulfillment_group_no = ?`
	args := []any{orderNo, groupNo}
	if shipmentNo != "" {
		query += ` AND shipment_no = ?`
		args = append(args, shipmentNo)
	}
	query += ` FOR UPDATE`
	err = tx.QueryRowContext(ctx, query, args...).Scan(&status, &exceptionState, &shippedAtUnix)
	if errors.Is(err, sql.ErrNoRows) {
		// No parcel exists: nothing to cancel. This is the "no shipment"
		// branch of refund admission and is safe for the caller.
		return &pb.CancelIfCreatedReply{Status: statusCancelled}, nil
	}
	if err != nil {
		return nil, apperr.Internal("load shipment for cancel", err)
	}
	// R-2: the warehouse-exit fact wins over the optimistic `CREATED` status.
	// A pickup exception after departure means the goods are gone.
	if exceptionState != "" && exceptionState != "NONE" {
		if shippedAtUnix > 0 {
			return &pb.CancelIfCreatedReply{Status: "ALREADY_SHIPPED"}, nil
		}
		if s.log != nil {
			s.log.Warn("shipment exception without shipped_at; refusing to cancel",
				"order_no", orderNo, "group_no", groupNo,
				"shipment_no", shipmentNo, "exception_state", exceptionState)
		}
		return &pb.CancelIfCreatedReply{Status: "TRANSIENT_ERROR"}, nil
	}
	switch status {
	case statusCreated:
		if _, err := tx.ExecContext(ctx, `UPDATE shipments
			SET status = ?, updated_at = NOW(3) WHERE order_no = ? AND fulfillment_group_no = ? AND status = ?`,
			statusCancelled, orderNo, groupNo, statusCreated); err != nil {
			return nil, apperr.Internal("cancel created shipment", err)
		}
		if err := tx.Commit(); err != nil {
			return nil, apperr.Internal("commit cancel created shipment", err)
		}
		return &pb.CancelIfCreatedReply{Status: statusCancelled}, nil
	case statusCancelled:
		return &pb.CancelIfCreatedReply{Status: statusCancelled}, nil
	case statusShipped:
		return &pb.CancelIfCreatedReply{Status: "ALREADY_SHIPPED"}, nil
	case statusReceived:
		return &pb.CancelIfCreatedReply{Status: "RECEIVED"}, nil
	default:
		return &pb.CancelIfCreatedReply{Status: "TRANSIENT_ERROR"}, nil
	}
}
