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
// captured, so a paid order must be refunded per fulfillment group: the Σ
// precheck runs before any state change, then one short Tx1 freezes REFUNDING,
// the refund batch, the group applications and the permit decision
// (SPEC-0002 v1.8 AC-51/AC-65, ADR-0007 Amd1 §4.1.2, Plan T1.2/T1.3).
//
// Tx1 creates durable state only: AdmitRefund, CancelIfCreated, Payment and
// order.refund_requested belong to later tasks. A rerun on a REFUNDING order
// resumes the existing batch and fills only what is missing.
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
	if record.RefundState == refundStateFull || record.Status == statusCancelled {
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

	// AC-65: the authoritative Payment snapshot and the Σ equation come before
	// any write. A mismatch returns the computed numbers and changes no table,
	// event or state. A REFUNDING order is a resume of the existing batch: its
	// own commitments are excluded inside Tx1, so the precheck here only takes
	// the snapshot.
	var snapshot *tccPaymentSnapshot
	if record.RefundState == refundStateRefunding {
		snapshot, err = s.readTccPaymentSnapshot(ctx, record, paymentNo)
	} else {
		var pre *tccPrecheck
		pre, snapshot, err = s.precheckTccSumWithSnapshot(ctx, orderNo, paymentNo)
		if err != nil {
			// A concurrent TCC cancel may have frozen the same compensation
			// between this call's initial read and the precheck. That freeze
			// already validated Σ, so converge on it instead of returning a
			// spurious mismatch (concurrent TCC cancels must be idempotent).
			if converged, cerr := s.resumeTccCancelledOrder(ctx, orderNo, paymentNo, paid); converged {
				if cerr != nil {
					return nil, cerr
				}
				return &pb.Empty{}, nil
			}
			return nil, err
		}
		if pre.NoCaptured {
			// AC-65: no captured money is a cancel/zero-converge branch, not a
			// per-group refund. T1.6 owns that branch; until then fail closed
			// instead of reporting a refund that never happens.
			return nil, apperr.Failed("TCC compensation has no captured amount").
				WithDetails(map[string]any{
					"order_no": orderNo, "payment_no": paymentNo,
					"pay_cent": pre.PayCent, "captured_cent": pre.CapturedCent,
				})
		}
	}
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, apperr.Internal("tcc compensation has no payment snapshot", nil)
	}
	if err := s.applyTccCompensation(ctx, orderNo, snapshot, paid); err != nil {
		return nil, err
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
