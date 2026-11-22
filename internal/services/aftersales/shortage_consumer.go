package aftersales

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/consumer"
	"gin-kratos-gorm/internal/platform/id"
	"gin-kratos-gorm/internal/platform/mq"
	"gin-kratos-gorm/internal/platform/outbox"

	"github.com/nats-io/nats.go"
)

const (
	sourceSystem        = "SYSTEM"
	sourceAdminRepair   = "ADMIN_REPAIR"
	shortageFreightRule = "SHORTAGE_MERCHANT_FREIGHT_SHARE"
)

type shortageResolvedEvent struct {
	OrderNo    string `json:"order_no"`
	GroupNo    string `json:"group_no"`
	Status     string `json:"status"`
	Resolution string `json:"resolution"`
	ReasonCode string `json:"reason_code"`
	ResolvedAt int64  `json:"resolved_at"`
	Items      []struct {
		LineNo   int32 `json:"line_no"`
		SkuID    int64 `json:"sku_id"`
		Quantity int32 `json:"quantity"`
	} `json:"items"`
}

// RunShortageConsumer turns the Order shortage resolution into a durable
// AfterSales system refund. The handler only creates the intent + task; the
// existing refund worker performs the Payment call.
func RunShortageConsumer(ctx context.Context, client *mq.Client, svc *Service, logger *slog.Logger) {
	if client == nil || client.JS == nil {
		return
	}
	err := consumer.Subscribe(ctx, client, logger, consumer.Config{
		Subject: "shop.order.shortage.resolved.v1", Durable: "aftersales-shortage-refund",
		Queue: "aftersales-shortage-refund", MaxDeliver: 10,
	}, func(hctx context.Context, msg *nats.Msg) error {
		var event shortageResolvedEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			return consumer.Permanent(err)
		}
		if strings.TrimSpace(event.OrderNo) == "" || strings.TrimSpace(event.GroupNo) == "" {
			return consumer.Permanent(fmt.Errorf("shortage event missing order_no/group_no"))
		}
		if event.Resolution != "REFUND" || event.Status != shortageResolved {
			return nil
		}
		if err := svc.createSystemRefund(hctx, event); err != nil {
			if apperr.IsCode(err, apperr.CodeFailedPrecondition) || apperr.IsCode(err, apperr.CodeInvalidArgument) {
				return consumer.Permanent(err)
			}
			return err
		}
		return nil
	})
	if err != nil {
		logger.Error("subscribe shortage.resolved", "error", err)
	}
}

const shortageResolved = "RESOLVED_REFUND"

func (s *Service) createSystemRefund(ctx context.Context, event shortageResolvedEvent) error {
	if s.orders == nil {
		return apperr.Unavailable("order client is not configured for system refund", nil)
	}
	order, err := s.orders.GetOrder(ctx, &pb.GetOrderRequest{OrderNo: event.OrderNo, Admin: true})
	if err != nil {
		return apperr.Unavailable("load order for shortage refund", err)
	}
	if order.GetOrderNo() == "" {
		return apperr.NotFound("order not found")
	}
	groupStatus := ""
	var groupShare int64
	for _, group := range order.GetFulfillmentGroups() {
		if group.GetGroupNo() == event.GroupNo {
			groupStatus = group.GetStatus()
			groupShare = group.GetFreightShareCent()
			break
		}
	}
	if groupStatus == "" {
		return apperr.NotFound("fulfillment group not found")
	}
	if groupStatus == "SHIPPED" || groupStatus == "RECEIVED" {
		// A shipped group cannot be shortage-refunded; the order should never have
		// reported it, so this is a permanent event error, not a retry loop.
		return apperr.Failed("cannot shortage-refund a shipped group")
	}
	items := systemRefundItems(order, event)
	if len(items) == 0 {
		return nil
	}
	key := fmt.Sprintf("SYS-SHORTAGE:%s:%s", event.OrderNo, event.GroupNo)
	capturedCent, capturedSource, err := s.capturedForOrder(ctx, order)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return apperr.Internal("begin system refund", err)
	}
	defer func() { _ = tx.Rollback() }()
	var existing string
	err = tx.QueryRowContext(ctx, `SELECT after_sale_no FROM after_sales
		WHERE user_id = ? AND client_request_id = ? FOR UPDATE`, order.GetUserId(), key).Scan(&existing)
	if err == nil {
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return apperr.Internal("load system refund idempotency", err)
	}
	refunds, goodsCent, _, _, err := s.computeLineRefunds(ctx, tx, order, items, typeRefundOnly, true)
	if err != nil {
		return err
	}
	freightCent, err := s.systemFreightRefund(ctx, tx, order, event.GroupNo, groupShare)
	if err != nil {
		return err
	}
	effective := goodsCent + freightCent
	if effective <= 0 {
		return nil
	}
	ledger, err := s.ensureAmountLedgerTx(ctx, tx, event.OrderNo, capturedCent, capturedSource)
	if err != nil {
		return err
	}
	if err := reserveAmountTx(ctx, tx, ledger, effective); err != nil {
		return err
	}
	afterSaleNo := id.AfterSale()
	reasonCode := strings.TrimSpace(event.ReasonCode)
	if reasonCode == "" {
		reasonCode = "SHORTAGE_DEFAULT"
	}
	res, err := tx.ExecContext(ctx, "INSERT INTO after_sales (after_sale_no, order_no, user_id, `type`, status, amount_cent, effective_amount_cent, amount_state, restock_state, reason, client_request_id, source, responsibility_party, responsibility_source, responsibility_evidence_ref, responsibility_reason_code, responsibility_determined_by, responsibility_determined_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(3))",
		afterSaleNo, event.OrderNo, order.GetUserId(), typeRefundOnly, statusApproved,
		effective, effective, amountActive, restockNone,
		"shortage "+event.GroupNo, key, sourceSystem,
		"MERCHANT_WAREHOUSE", "SYSTEM", "system:"+reasonCode, reasonCode, "system")
	if err != nil {
		return apperr.Internal("create system after sale", err)
	}
	afterSaleID, err := res.LastInsertId()
	if err != nil {
		return apperr.Internal("system after sale id", err)
	}
	if err := appendAmountVersionTx(ctx, tx, afterSaleID, 1, effective, "CREATE_SYSTEM"); err != nil {
		return err
	}
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `INSERT INTO after_sale_items
			(after_sale_id, sku_id, quantity, warehouse_id, group_no, line_no,
			 line_paid_cent_snapshot, unit_paid_cent_snapshot)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			afterSaleID, item.GetSkuId(), item.GetQuantity(), item.GetWarehouseId(), item.GetGroupNo(),
			item.GetLineNo(), item.GetLinePaidCent(), item.GetUnitPaidCent()); err != nil {
			return apperr.Internal("create system after sale item", err)
		}
		if err := s.reservePendingLineTx(ctx, tx, event.OrderNo, item.GetLineNo(), int64(item.GetQuantity())); err != nil {
			return err
		}
	}
	if err := ensureRefundTaskTx(ctx, tx, afterSaleNo); err != nil {
		return err
	}
	if err := appendLog(ctx, tx, afterSaleID, "CREATE_SYSTEM", "", statusApproved, reasonCode); err != nil {
		return err
	}
	payload, _ := json.Marshal(afterSaleEvent{
		AfterSaleNo: afterSaleNo, OrderNo: event.OrderNo, UserID: order.GetUserId(),
		Type: typeRefundOnly, Status: statusApproved, AmountCent: effective,
		EffectiveAmountCent: effective, AmountState: amountActive, RestockState: restockNone,
		Reason: "shortage " + event.GroupNo, GroupNo: event.GroupNo,
	})
	if err := outbox.Append(ctx, tx, outbox.Event{
		Topic: topicAfterSaleCreated, AggregateType: aggregateAfterSale,
		AggregateID: afterSaleNo, EventType: "aftersale.created", Payload: payload,
	}); err != nil {
		return apperr.Internal("append system after sale created", err)
	}
	_ = refunds
	return tx.Commit()
}

func systemRefundItems(order *pb.Order, event shortageResolvedEvent) []*pb.AfterSaleItem {
	wanted := make(map[int32]bool, len(event.Items))
	for _, item := range event.Items {
		if item.LineNo > 0 {
			wanted[item.LineNo] = true
		}
	}
	items := make([]*pb.AfterSaleItem, 0, len(order.GetItems()))
	for _, line := range order.GetItems() {
		groupNo := line.GetGroupNo()
		if groupNo == "" {
			groupNo = "G1"
		}
		if groupNo != event.GroupNo {
			continue
		}
		if len(wanted) > 0 && !wanted[line.GetLineNo()] {
			continue
		}
		items = append(items, &pb.AfterSaleItem{
			SkuId: line.GetSkuId(), Quantity: line.GetQuantity(), WarehouseId: line.GetWarehouseId(),
			GroupNo: groupNo, LineNo: line.GetLineNo(), LinePaidCent: line.GetLinePaidCent(),
			UnitPaidCent: line.GetUnitPaidCent(),
		})
	}
	return items
}

func (s *Service) systemFreightRefund(ctx context.Context, tx *sql.Tx, order *pb.Order, groupNo string, groupShare int64) (int64, error) {
	if groupShare <= 0 {
		return 0, nil
	}
	var already int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(b.freight_cent), 0)
		FROM after_sale_refund_breakdowns b JOIN after_sales a ON a.after_sale_no = b.after_sale_no
		WHERE a.order_no = ? AND a.status = ?`, order.GetOrderNo(), statusRefunded).Scan(&already); err != nil {
		return 0, apperr.Internal("load refunded freight", err)
	}
	remaining := order.GetFreightCent() - already
	if groupShare > remaining {
		groupShare = remaining
	}
	if groupShare < 0 {
		groupShare = 0
	}
	_ = groupNo
	return groupShare, nil
}

func (s *Service) reservePendingLineTx(ctx context.Context, tx *sql.Tx, orderNo string, lineNo int32, quantity int64) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO after_sale_line_ledger
		(order_no, line_no, refunded_quantity, refunded_amount_cent, pending_quantity, pending_amount_cent)
		VALUES (?, ?, 0, 0, ?, 0)
		ON DUPLICATE KEY UPDATE pending_quantity = pending_quantity + VALUES(pending_quantity),
			version = version + 1, updated_at = NOW(3)`, orderNo, lineNo, quantity); err != nil {
		return apperr.Internal("reserve pending line quantity", err)
	}
	return nil
}
