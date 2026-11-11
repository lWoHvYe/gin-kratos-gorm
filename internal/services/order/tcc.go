package order

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/db"
	"gin-kratos-gorm/internal/platform/outbox"
)

const topicOrderRefundRequested = "shop.order.refund_requested.v1"

// TccTryMarkPaid validates the order for the payment-confirm TCC. It is a
// read-only probe: no status changes happen until Confirm.
func (s *Service) TccTryMarkPaid(ctx context.Context, req *pb.TccPaymentRequest) (*pb.Empty, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	record, err := findOrder(ctx, s.db, orderNo)
	if err != nil {
		return nil, apperr.Internal("load order", err)
	}
	if record == nil {
		return nil, apperr.NotFound("order not found")
	}
	switch record.Status {
	case statusPaid, statusRefunding:
		return &pb.Empty{}, nil
	case statusPendingPayment:
		paid := req.GetPaidCent()
		if paid <= 0 {
			paid = record.PayCent
		}
		if paid > record.PayCent {
			return nil, apperr.Invalid("paid_cent exceeds pay_cent")
		}
		return &pb.Empty{}, nil
	default:
		return nil, apperr.Failed(fmt.Sprintf("order cannot be paid in status %s", record.Status))
	}
}

// TccConfirmMarkPaid commits the order side of the payment-confirm TCC. Stock
// and coupon confirmation are separate TCC branches.
func (s *Service) TccConfirmMarkPaid(ctx context.Context, req *pb.TccPaymentRequest) (*pb.Empty, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	record, err := findOrder(ctx, s.db, orderNo)
	if err != nil {
		return nil, apperr.Internal("load order", err)
	}
	if record == nil {
		return nil, apperr.NotFound("order not found")
	}
	if record.Status == statusPaid || record.Status == statusRefunding {
		return &pb.Empty{}, nil
	}
	if record.Status != statusPendingPayment {
		return nil, apperr.Failed(fmt.Sprintf("order cannot be paid in status %s", record.Status))
	}
	paid := req.GetPaidCent()
	if paid <= 0 {
		paid = record.PayCent
	}
	if paid > record.PayCent {
		return nil, apperr.Invalid("paid_cent exceeds pay_cent")
	}
	items, err := loadOrderItems(ctx, s.db, orderNo)
	if err != nil {
		return nil, apperr.Internal("load order items", err)
	}
	paymentNo := strings.TrimSpace(req.GetPaymentNo())
	operator := "payment"
	if paymentNo != "" {
		operator = truncate("payment:"+paymentNo, 64)
	}
	err = db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		affected, err := updateOrderStatus(ctx, tx, orderNo, statusPendingPayment, statusPaid, paid)
		if err != nil {
			return err
		}
		if affected == 0 {
			return nil
		}
		if err := insertStatusLog(ctx, tx, orderNo, statusPendingPayment, statusPaid, "payment "+paymentNo, operator); err != nil {
			return err
		}
		return outbox.Append(ctx, tx, outbox.Event{
			Topic: topicOrderPaid, AggregateType: "order", AggregateID: orderNo, EventType: "order.paid",
			Payload: map[string]any{
				"order_no": orderNo, "user_id": record.UserID, "payment_no": paymentNo,
				"paid_cent": paid, "pay_cent": record.PayCent, "status": statusPaid,
				"paid_at": time.Now().Unix(), "items": orderItemsPayload(items),
			},
		})
	})
	if err != nil {
		return nil, apperr.Internal("tcc confirm paid", err)
	}
	if record = recordWithStatus(record, statusPaid); record != nil {
		s.afterPaid(ctx, record)
	}
	return &pb.Empty{}, nil
}

func recordWithStatus(record *orderRecord, status string) *orderRecord {
	if record == nil {
		return nil
	}
	copy := *record
	copy.Status = status
	return &copy
}

// TccCancelMarkPaid is the compensation side. The payment has already been
// captured, so a paid order moves to REFUNDING and an outbox event asks the
// payment/aftersales flow to refund; a still-pending order is left for the
// caller's refund decision.
func (s *Service) TccCancelMarkPaid(ctx context.Context, req *pb.TccPaymentRequest) (*pb.Empty, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	record, err := findOrder(ctx, s.db, orderNo)
	if err != nil {
		return nil, apperr.Internal("load order", err)
	}
	if record == nil {
		return &pb.Empty{}, nil
	} // nothing was created
	if record.Status == statusRefunding || record.Status == statusCancelled {
		return &pb.Empty{}, nil
	}
	from := record.Status
	if from != statusPendingPayment && from != statusPaid {
		return &pb.Empty{}, nil
	}
	paid := req.GetPaidCent()
	if paid <= 0 {
		paid = record.PaidCent
	}
	err = db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		affected, err := updateOrderStatus(ctx, tx, orderNo, from, statusRefunding, paid)
		if err != nil {
			return err
		}
		if affected == 0 {
			return nil
		}
		if err := insertStatusLog(ctx, tx, orderNo, from, statusRefunding, "payment tcc cancelled", "system"); err != nil {
			return err
		}
		return outbox.Append(ctx, tx, outbox.Event{
			Topic: topicOrderRefundRequested, AggregateType: "order", AggregateID: orderNo, EventType: "order.refund_requested",
			Payload: map[string]any{"order_no": orderNo, "payment_no": req.GetPaymentNo(), "reason": "payment confirm tcc cancelled", "status": statusRefunding, "paid_cent": paid, "created_at": time.Now().Unix()},
		})
	})
	if err != nil {
		return nil, apperr.Internal("tcc cancel paid", err)
	}
	return &pb.Empty{}, nil
}

func (s *Service) TccQueryMarkPaid(ctx context.Context, req *pb.TccPaymentRequest) (*pb.TccQueryReply, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	record, err := findOrder(ctx, s.db, orderNo)
	if err != nil {
		return nil, apperr.Internal("load order", err)
	}
	if record == nil {
		return &pb.TccQueryReply{Prepared: false}, nil
	}
	switch record.Status {
	case statusPendingPayment, statusPaid, statusRefunding:
		return &pb.TccQueryReply{Prepared: true}, nil
	default:
		return &pb.TccQueryReply{Prepared: false}, nil
	}
}
