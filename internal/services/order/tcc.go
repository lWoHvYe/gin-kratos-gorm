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
	if record.RefundState == refundStateRefunding || record.RefundState == refundStateFull {
		return nil, apperr.Failed("order refund is in progress")
	}
	switch record.Status {
	case statusPaid:
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
	if record.RefundState == refundStateRefunding || record.RefundState == refundStateFull {
		return nil, apperr.Failed("order refund is in progress")
	}
	paymentNo := strings.TrimSpace(req.GetPaymentNo())
	if record.Status == statusPaid {
		if paymentNo != "" && record.SettlementPaymentNo == paymentNo {
			return &pb.Empty{}, nil
		}
		return nil, apperr.Conflict("order was already settled by another payment")
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
	operator := "payment"
	if paymentNo != "" {
		operator = truncate("payment:"+paymentNo, 64)
	}
	if err := s.checkOrderReady(ctx, orderNo, items); err != nil {
		return nil, err
	}
	settled := false
	err = db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		// The TCC order branch uses the same freeze/CAS primitive as the direct
		// path. It must not mark PAID unless every reservation is confirmed;
		// checkOrderReady above is the synchronous gate for that.
		affected, err := freezeAllocationTx(ctx, tx, orderNo, paid, paymentNo)
		if err != nil {
			return err
		}
		if affected == 0 {
			return nil
		}
		settled = true
		if err := insertStatusLog(ctx, tx, orderNo, statusPendingPayment, statusPaid, "payment "+paymentNo, operator); err != nil {
			return err
		}
		if err := setFulfillmentGroupStatus(ctx, tx, orderNo, statusAllocated); err != nil {
			return err
		}
		if err := appendAllocationFrozenEvent(ctx, tx, orderNo, record.AllocationHash, record.PackageCount, record.WarehouseCount); err != nil {
			return err
		}
		if err := appendOrderSettledEvent(ctx, tx, record, paymentNo); err != nil {
			return err
		}
		if err := ensurePricingConfirmationTask(ctx, tx, record); err != nil {
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
	if !settled {
		latest, err := findOrder(ctx, s.db, orderNo)
		if err != nil {
			return nil, apperr.Internal("reload order after tcc confirm", err)
		}
		if latest == nil {
			return nil, apperr.NotFound("order not found")
		}
		if latest.Status == statusPaid && latest.SettlementPaymentNo == paymentNo {
			return &pb.Empty{}, nil
		}
		if latest.Status == statusCancelled {
			return nil, apperr.Failed("order was cancelled before payment confirmation")
		}
		return nil, apperr.Conflict(fmt.Sprintf("order cannot be paid in status %s", latest.Status))
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
	paymentNo := strings.TrimSpace(req.GetPaymentNo())
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
	if record.RefundState == refundStateRefunding || record.RefundState == refundStateFull || record.Status == statusCancelled {
		return &pb.Empty{}, nil
	}
	from := record.Status
	if from != statusPendingPayment && from != statusPaid {
		return &pb.Empty{}, nil
	}

	// Compensation refunds what was actually captured, never what is due.
	paid := req.GetPaidCent()
	if paid <= 0 {
		paid = record.PaidCent
	}
	if paid > record.PaidCent {
		return nil, apperr.Invalid("paid_cent exceeds captured amount")
	}
	switch tccCancelDecision(from, paid) {
	case tccCancelLocal:
		// Nothing was captured: cancel locally so order.cancelled releases the
		// inventory hold. Parking an unpaid order in REFUNDING made it terminal
		// and the refund consumer dropped zero-amount events.
		return s.cancelPendingAfterTCC(ctx, record, req.GetPaymentNo())
	case tccCancelKeepPaid:
		// Fully discounted order: there is nothing to refund. Keep it paid
		// instead of moving it to a terminal REFUNDING state.
		s.log.Warn("payment tcc cancelled a zero-amount paid order", "order_no", orderNo)
		return &pb.Empty{}, nil
	}
	err = db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		// Refund is orthogonal to the lifecycle. If the order was still pending
		// when money was captured, record PAID (without an order.paid event so
		// fulfillment does not auto-create a shipment) and REFUNDING; otherwise
		// keep the fulfilled lifecycle and only change refund_state.
		nextStatus := from
		if from == statusPendingPayment && paid > 0 {
			res, err := tx.ExecContext(ctx, `UPDATE orders
				SET status = ?, paid_cent = ?, refund_state = ?, version = version + 1, updated_at = ?
				WHERE order_no = ? AND status = ? AND refund_state <> ?`,
				statusPaid, paid, refundStateRefunding, time.Now(), orderNo, statusPendingPayment, refundStateFull)
			if err != nil {
				return apperr.Internal("mark captured order paid for refund", err)
			}
			if n, _ := res.RowsAffected(); n == 0 {
				return nil
			}
			nextStatus = statusPaid
		} else {
			res, err := tx.ExecContext(ctx, `UPDATE orders
				SET refund_state = ?, version = version + 1, updated_at = ?
				WHERE order_no = ? AND refund_state <> ?`,
				refundStateRefunding, time.Now(), orderNo, refundStateFull)
			if err != nil {
				return apperr.Internal("mark order refunding", err)
			}
			if n, _ := res.RowsAffected(); n == 0 {
				return nil
			}
		}
		if err := insertStatusLog(ctx, tx, orderNo, from, nextStatus, "payment tcc cancelled", "system"); err != nil {
			return err
		}
		if nextStatus == statusPaid {
			if err := setFulfillmentGroupStatus(ctx, tx, orderNo, statusAllocated); err != nil {
				return err
			}
		}
		return outbox.Append(ctx, tx, outbox.Event{
			Topic: topicOrderRefundRequested, AggregateType: "order", AggregateID: orderNo, EventType: "order.refund_requested",
			Payload: map[string]any{"order_no": orderNo, "payment_no": req.GetPaymentNo(), "reason": "payment confirm tcc cancelled", "refund_state": refundStateRefunding, "status": nextStatus, "paid_cent": paid, "created_at": time.Now().Unix()},
		})
	})
	if err != nil {
		return nil, apperr.Internal("tcc cancel paid", err)
	}
	// TCC compensation must also close unshipped groups. The full admission
	// fan-out is still pending, but shipping must not be allowed to proceed
	// while the captured money is being refunded.
	groups, gerr := loadFulfillmentGroups(ctx, s.db, orderNo)
	if gerr != nil {
		return nil, apperr.Internal("load groups for tcc cancel", gerr)
	}
	groupNos := make([]string, 0, len(groups))
	for _, group := range groups {
		groupNos = append(groupNos, group.GroupNo)
	}
	if len(groupNos) == 0 {
		groupNos = []string{"G1"}
	}
	for _, groupNo := range groupNos {
		if _, berr := s.BlockShipmentForRefund(ctx, &pb.ShipmentPermitRequest{
			OrderNo: orderNo, AfterSaleNo: "tcc:" + paymentNo, FulfillmentGroupNo: groupNo,
		}); berr != nil {
			return nil, apperr.Unavailable("block shipping for tcc cancel", berr)
		}
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
	if record.RefundState == refundStateRefunding {
		return &pb.TccQueryReply{Prepared: true}, nil
	}
	switch record.Status {
	case statusPendingPayment, statusPaid:
		return &pb.TccQueryReply{Prepared: true}, nil
	default:
		return &pb.TccQueryReply{Prepared: false}, nil
	}
}

// cancelPendingAfterTCC closes an unpaid order after the payment-confirm TCC
// aborts. The durable order.cancelled event drives inventory and pricing
// compensation, mirroring the normal cancel path.
func (s *Service) cancelPendingAfterTCC(ctx context.Context, record *orderRecord, paymentNo string) (*pb.Empty, error) {
	reason := "payment confirm tcc cancelled"
	if paymentNo != "" {
		reason = truncate(reason+" "+paymentNo, 255)
	}
	items, err := loadOrderItems(ctx, s.db, record.OrderNo)
	if err != nil {
		return nil, apperr.Internal("load order items for tcc cancel", err)
	}
	err = db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		affected, err := updateOrderStatus(ctx, tx, record.OrderNo, statusPendingPayment, statusCancelled, record.PaidCent)
		if err != nil {
			return err
		}
		if affected == 0 {
			return nil
		}
		if err := insertStatusLog(ctx, tx, record.OrderNo, statusPendingPayment, statusCancelled, reason, "system"); err != nil {
			return err
		}
		return outbox.Append(ctx, tx, outbox.Event{
			Topic:         topicOrderCancelled,
			AggregateType: "order",
			AggregateID:   record.OrderNo,
			EventType:     "order.cancelled",
			Payload: map[string]any{
				"order_no":     record.OrderNo,
				"user_id":      record.UserID,
				"quote_id":     record.QuoteID,
				"reason":       reason,
				"operator":     "system",
				"status":       statusCancelled,
				"cancelled_at": time.Now().Unix(),
				"items":        orderItemsPayload(items),
			},
		})
	})
	if err != nil {
		return nil, apperr.Internal("cancel unpaid order after tcc", err)
	}
	return &pb.Empty{}, nil
}

type tccCancelAction int

const (
	// tccCancelNone leaves a terminal order untouched.
	tccCancelNone tccCancelAction = iota
	// tccCancelLocal moves an unpaid order to CANCELLED.
	tccCancelLocal
	// tccCancelRefund moves a captured order to REFUNDING.
	tccCancelRefund
	// tccCancelKeepPaid leaves a zero-amount paid order alone.
	tccCancelKeepPaid
)

// tccCancelDecision maps (status, captured amount) to the compensation path.
// A PENDING order with no capture must be cancelled, not parked in REFUNDING.
func tccCancelDecision(status string, paid int64) tccCancelAction {
	switch status {
	case statusPendingPayment:
		if paid <= 0 {
			return tccCancelLocal
		}
		return tccCancelRefund
	case statusPaid:
		if paid <= 0 {
			return tccCancelKeepPaid
		}
		return tccCancelRefund
	default:
		return tccCancelNone
	}
}
