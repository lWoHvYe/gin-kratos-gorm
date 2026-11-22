package aftersales

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

type lineQuery interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type lineRefund struct {
	LineNo        int32
	SkuID         int64
	Quantity      int64
	LinePaidCent  int64
	UnitPaidCent  int64
	RefundedQty   int64
	RefundedCent  int64
	AmountCent    int64
	RemainingQty  int64
	RemainingCent int64
}

func isNoRows(err error) bool { return errors.Is(err, sql.ErrNoRows) }

// computeLineRefunds is the single refund calculator (ADR 0005 §6.4). It locks
// the ledger rows when requested so a remote Payment call can be made while the
// line quota is serialized. Callers must not trust any caller-supplied amount.
func (s *Service) computeLineRefunds(ctx context.Context, q lineQuery, order *pb.Order, items []*pb.AfterSaleItem, typ string, lock bool) ([]lineRefund, int64, int64, string, error) {
	byLine := make(map[int32]*pb.OrderItem, len(order.GetItems()))
	for _, item := range order.GetItems() {
		byLine[item.GetLineNo()] = item
	}
	refunds := make([]lineRefund, 0, len(items))
	var goods int64
	for _, item := range items {
		line := byLine[item.GetLineNo()]
		if line == nil || line.GetQuantity() <= 0 {
			return nil, 0, 0, "", apperr.Invalid("line %d is not part of order %s", item.GetLineNo(), order.GetOrderNo())
		}
		if item.GetQuantity() <= 0 {
			return nil, 0, 0, "", apperr.Invalid("refund quantity must be positive")
		}
		var refundedQty, refundedCent int64
		query := `SELECT refunded_quantity, refunded_amount_cent FROM after_sale_line_ledger
			WHERE order_no = ? AND line_no = ?`
		if lock {
			query += ` FOR UPDATE`
		}
		err := q.QueryRowContext(ctx, query, order.GetOrderNo(), item.GetLineNo()).Scan(&refundedQty, &refundedCent)
		if errors.Is(err, sql.ErrNoRows) {
			if lock {
				if err := s.execLineLedgerInsert(ctx, q, order.GetOrderNo(), item.GetLineNo()); err != nil {
					return nil, 0, 0, "", err
				}
				if err := q.QueryRowContext(ctx, query, order.GetOrderNo(), item.GetLineNo()).Scan(&refundedQty, &refundedCent); err != nil {
					return nil, 0, 0, "", apperr.Internal("lock new line refund ledger", err)
				}
			}
		} else if err != nil {
			return nil, 0, 0, "", apperr.Internal("load line refund ledger", err)
		}
		linePaid := line.GetLinePaidCent()
		lineQty := int64(line.GetQuantity())
		remainingQty := lineQty - refundedQty
		remainingCent := linePaid - refundedCent
		if int64(item.GetQuantity()) > remainingQty {
			return nil, 0, 0, "", apperr.Invalid("refund quantity exceeds the remaining line quantity").
				WithDetails(map[string]any{"line_no": item.GetLineNo(), "remaining_quantity": remainingQty})
		}
		unit := linePaid / lineQty
		amount := unit * int64(item.GetQuantity())
		if refundedQty+int64(item.GetQuantity()) == lineQty {
			amount += linePaid - unit*lineQty
		}
		if amount > remainingCent {
			amount = remainingCent
		}
		if amount < 0 {
			return nil, 0, 0, "", apperr.Failed("line refund amount is negative")
		}
		refunds = append(refunds, lineRefund{
			LineNo: item.GetLineNo(), SkuID: item.GetSkuId(), Quantity: int64(item.GetQuantity()),
			LinePaidCent: linePaid, UnitPaidCent: unit, RefundedQty: refundedQty,
			RefundedCent: refundedCent, AmountCent: amount, RemainingQty: remainingQty,
			RemainingCent: remainingCent,
		})
		goods += amount
	}
	freight := int64(0)
	rule := "LINE_REFUND"
	if typ == typeRefundOnly && orderUnshipped(order) {
		var already int64
		if err := q.QueryRowContext(ctx, `SELECT COALESCE(SUM(b.freight_cent), 0)
			FROM after_sale_refund_breakdowns b JOIN after_sales a ON a.after_sale_no = b.after_sale_no
			WHERE a.order_no = ? AND a.status = ?`, order.GetOrderNo(), statusRefunded).Scan(&already); err != nil {
			return nil, 0, 0, "", apperr.Internal("load refunded freight", err)
		}
		freight = order.GetFreightCent() - already
		if freight < 0 {
			freight = 0
		}
		rule = "UNSHIPPED_CANCEL_FULL_FREIGHT"
	}
	return refunds, goods, freight, rule, nil
}

func orderUnshipped(order *pb.Order) bool {
	for _, group := range order.GetFulfillmentGroups() {
		if group.GetStatus() == "SHIPPED" || group.GetStatus() == "RECEIVED" {
			return false
		}
	}
	return true
}

func (s *Service) execLineLedgerInsert(ctx context.Context, q lineQuery, orderNo string, lineNo int32) error {
	exec, ok := q.(interface {
		ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	})
	if !ok {
		return apperr.Internal("line ledger requires an executor", nil)
	}
	_, err := exec.ExecContext(ctx, `INSERT IGNORE INTO after_sale_line_ledger
		(order_no, line_no, refunded_quantity, refunded_amount_cent) VALUES (?, ?, 0, 0)`, orderNo, lineNo)
	if err != nil {
		return apperr.Internal("create line refund ledger", err)
	}
	return nil
}

// PreviewRefundQuote answers "how much can be refunded for this line/quantity"
// without writing anything. The caller can only choose quantity.
func (s *Service) PreviewRefundQuote(ctx context.Context, req *pb.PreviewRefundQuoteRequest) (*pb.RefundQuote, error) {
	orderNo := req.GetOrderNo()
	if orderNo == "" || req.GetLineNo() <= 0 || req.GetQuantity() <= 0 || req.GetUserId() <= 0 {
		return nil, apperr.Invalid("order_no, line_no, quantity and user_id are required")
	}
	order, err := s.loadOrder(ctx, orderNo, req.GetUserId())
	if err != nil {
		return nil, err
	}
	typ := strings.TrimSpace(req.GetType())
	if typ == "" {
		typ = typeReturnRefund
	}
	refunds, goods, freight, rule, err := s.computeLineRefunds(ctx, s.db, order,
		[]*pb.AfterSaleItem{{SkuId: 0, Quantity: int32(req.GetQuantity()), LineNo: req.GetLineNo()}}, typ, false)
	if err != nil {
		return nil, err
	}
	if len(refunds) != 1 {
		return nil, apperr.Internal("refund preview did not produce one line", nil)
	}
	line := refunds[0]
	return &pb.RefundQuote{
		OrderNo: orderNo, LineNo: line.LineNo, LinePaidCent: line.LinePaidCent,
		UnitPaidCent: line.UnitPaidCent, RequestedQuantity: line.Quantity,
		RemainingQuantity: line.RemainingQty, RemainingAmountCent: line.RemainingCent,
		ComputedAmountCent: line.AmountCent, GoodsCent: goods, FreightCent: freight,
		FreightRuleId: rule,
	}, nil
}

// ListLineRefunds is the read model for the BFF order detail. Order never stores
// refunded quantity/amount (INV 铁律 4).
func (s *Service) ListLineRefunds(ctx context.Context, req *pb.ListLineRefundsRequest) (*pb.LineRefundList, error) {
	orderNo := req.GetOrderNo()
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	if !req.GetAdmin() && req.GetUserId() <= 0 {
		return nil, apperr.Unauthenticated("user_id is required")
	}
	var userID int64
	if req.GetUserId() > 0 {
		userID = req.GetUserId()
	}
	if s.orders == nil {
		return nil, apperr.Unavailable("order client is not configured", nil)
	}
	order, err := s.orders.GetOrder(ctx, &pb.GetOrderRequest{OrderNo: orderNo, UserId: userID, Admin: req.GetAdmin()})
	if err != nil {
		return nil, apperr.Unavailable("load order for line refunds", err)
	}
	ledger := make(map[int32]*pb.LineRefund)
	rows, err := s.db.QueryContext(ctx, `SELECT order_no, line_no, refunded_quantity, refunded_amount_cent, version
		FROM after_sale_line_ledger WHERE order_no = ?`, orderNo)
	if err != nil {
		return nil, apperr.Internal("list line refund ledger", err)
	}
	for rows.Next() {
		var refund pb.LineRefund
		if err := rows.Scan(&refund.OrderNo, &refund.LineNo, &refund.RefundedQuantity,
			&refund.RefundedAmountCent, &refund.Version); err != nil {
			rows.Close()
			return nil, apperr.Internal("scan line refund ledger", err)
		}
		ledger[refund.LineNo] = &refund
	}
	if err := rows.Close(); err != nil {
		return nil, apperr.Internal("close line refund ledger rows", err)
	}
	reply := &pb.LineRefundList{}
	for _, item := range order.GetItems() {
		refund := ledger[item.GetLineNo()]
		if refund == nil {
			refund = &pb.LineRefund{OrderNo: orderNo, LineNo: item.GetLineNo()}
		}
		refund.OrderedQuantity = int64(item.GetQuantity())
		refund.LinePaidCent = item.GetLinePaidCent()
		refund.RemainingQuantity = int64(item.GetQuantity()) - refund.GetRefundedQuantity()
		refund.RemainingAmountCent = item.GetLinePaidCent() - refund.GetRefundedAmountCent()
		reply.Lines = append(reply.Lines, refund)
	}
	return reply, nil
}

func (s *Service) recordLineRefundTx(ctx context.Context, tx *sql.Tx, orderNo string, refund lineRefund) error {
	if refund.LineNo <= 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO after_sale_line_ledger
		(order_no, line_no, refunded_quantity, refunded_amount_cent)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		  refunded_quantity = refunded_quantity + VALUES(refunded_quantity),
		  refunded_amount_cent = refunded_amount_cent + VALUES(refunded_amount_cent),
		  version = version + 1, updated_at = NOW(3)`,
		orderNo, refund.LineNo, refund.Quantity, refund.AmountCent); err != nil {
		return apperr.Internal("update line refund ledger", err)
	}
	return nil
}

func (s *Service) recordRefundBreakdownTx(ctx context.Context, tx *sql.Tx, afterSaleNo string, goods, freight int64, rule string) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO after_sale_refund_breakdowns
		(after_sale_no, goods_cent, freight_cent, total_cent, rule_id)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE goods_cent = VALUES(goods_cent), freight_cent = VALUES(freight_cent),
			total_cent = VALUES(total_cent), rule_id = VALUES(rule_id)`,
		afterSaleNo, goods, freight, goods+freight, rule); err != nil {
		return apperr.Internal("record refund breakdown", err)
	}
	return nil
}

func toProtoItems(items []afterSaleItem) []*pb.AfterSaleItem {
	out := make([]*pb.AfterSaleItem, 0, len(items))
	for _, item := range items {
		out = append(out, &pb.AfterSaleItem{
			SkuId: item.SkuID, Quantity: item.Quantity, WarehouseId: item.WarehouseID,
			GroupNo: item.GroupNo, LineNo: item.LineNo, LinePaidCent: item.LinePaidCent,
			UnitPaidCent: item.UnitPaidCent,
		})
	}
	return out
}

// AuthorizeCrossWarehouseRestock records the explicit manual exception for
// returning stock to a warehouse other than the line's origin (ADR 0005 §9.4).
func (s *Service) AuthorizeCrossWarehouseRestock(ctx context.Context, req *pb.AuthorizeCrossWarehouseRestockRequest) (*pb.Empty, error) {
	afterSaleNo := req.GetAfterSaleNo()
	if afterSaleNo == "" || req.GetSkuId() <= 0 || req.GetToWarehouseId() <= 0 ||
		strings.TrimSpace(req.GetOperator()) == "" || strings.TrimSpace(req.GetReason()) == "" {
		return nil, apperr.Invalid("after_sale_no, sku_id, to_warehouse_id, operator and reason are required")
	}
	row, err := s.getAfterSaleRow(ctx, afterSaleNo)
	if err != nil {
		return nil, err
	}
	order, err := s.loadOrder(ctx, row.orderNo, row.userID)
	if err != nil {
		return nil, err
	}
	origin := int64(0)
	for _, item := range order.GetItems() {
		if item.GetSkuId() == req.GetSkuId() {
			if origin != 0 && origin != item.GetWarehouseId() {
				return nil, apperr.Invalid("sku has multiple origin warehouses; authorize by line is required")
			}
			origin = item.GetWarehouseId()
		}
	}
	if origin == 0 {
		return nil, apperr.NotFound("sku is not part of the order")
	}
	if req.GetFromWarehouseId() != 0 && req.GetFromWarehouseId() != origin {
		return nil, apperr.Invalid("from_warehouse_id does not match the line origin")
	}
	if req.GetToWarehouseId() == origin {
		return nil, apperr.Invalid("cross-warehouse authorization is only for a different warehouse")
	}
	authNo := restockAuthNo(afterSaleNo, req.GetSkuId())
	if _, err := s.db.ExecContext(ctx, `INSERT INTO restock_exception_authorizations
		(after_sale_no, sku_id, from_warehouse_id, to_warehouse_id, operator, reason, auth_no, expires_at, used_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, DATE_ADD(NOW(3), INTERVAL 24 HOUR), NULL)
		ON DUPLICATE KEY UPDATE operator = VALUES(operator), reason = VALUES(reason),
			auth_no = VALUES(auth_no), expires_at = VALUES(expires_at), used_at = NULL`,
		afterSaleNo, req.GetSkuId(), origin, req.GetToWarehouseId(),
		cutRunes(req.GetOperator(), 64), cutRunes(req.GetReason(), 255), authNo); err != nil {
		return nil, apperr.Internal("authorize cross-warehouse restock", err)
	}
	return &pb.Empty{}, nil
}
