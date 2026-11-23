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

	var status string
	query := `SELECT status FROM shipments WHERE order_no = ? AND fulfillment_group_no = ?`
	args := []any{orderNo, groupNo}
	if shipmentNo != "" {
		query += ` AND shipment_no = ?`
		args = append(args, shipmentNo)
	}
	query += ` FOR UPDATE`
	err = tx.QueryRowContext(ctx, query, args...).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		// No parcel exists: nothing to cancel. This is the "no shipment"
		// branch of refund admission and is safe for the caller.
		return &pb.CancelIfCreatedReply{Status: statusCancelled}, nil
	}
	if err != nil {
		return nil, apperr.Internal("load shipment for cancel", err)
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
