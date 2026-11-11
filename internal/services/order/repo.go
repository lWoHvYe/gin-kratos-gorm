package order

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"

	"github.com/go-sql-driver/mysql"
)

// Order lifecycle states.
const (
	statusPendingPayment    = "PENDING_PAYMENT"
	statusPaid              = "PAID"
	statusAllocated         = "ALLOCATED"
	statusCancelled         = "CANCELLED"
	statusShipped           = "SHIPPED"
	statusCompleted         = "COMPLETED"
	statusRefunding         = "REFUNDING"
	statusPartiallyRefunded = "PARTIALLY_REFUNDED"
	statusRefunded          = "REFUNDED"
	statusClosed            = "CLOSED"
)

// knownStatuses is the set of statuses accepted by ListOrders filters.
var knownStatuses = map[string]struct{}{
	statusPendingPayment:    {},
	statusPaid:              {},
	statusAllocated:         {},
	statusCancelled:         {},
	statusShipped:           {},
	statusCompleted:         {},
	statusRefunding:         {},
	statusPartiallyRefunded: {},
	statusRefunded:          {},
	statusClosed:            {},
}

// errDuplicateOrder reports a unique-key conflict on orders. It is translated
// into the already existing order by the caller so that concurrent retries stay
// idempotent.
var errDuplicateOrder = errors.New("order already exists")

type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// orderRecord mirrors one orders row.
type orderRecord struct {
	OrderNo               string
	UserID                int64
	Status                string
	TotalCent             int64
	DiscountCent          int64
	PromotionDiscountCent int64
	AppliedPromotionsJSON string
	FreightCent           int64
	PayCent               int64
	PaidCent              int64
	RefundedCent          int64
	RefundStatus          string
	ReceiverName          string
	ReceiverPhone         string
	ReceiverAddress       string
	QuoteID               string
	ClientRequestID       string
	CreatedAt             time.Time
	ExpireAt              time.Time
}

func (o *orderRecord) toProto(items []*pb.OrderItem) *pb.Order {
	return &pb.Order{
		OrderNo:               o.OrderNo,
		UserId:                o.UserID,
		Status:                o.Status,
		TotalCent:             o.TotalCent,
		DiscountCent:          o.DiscountCent,
		PromotionDiscountCent: o.PromotionDiscountCent,
		AppliedPromotionsJson: o.AppliedPromotionsJSON,
		FreightCent:           o.FreightCent,
		PayCent:               o.PayCent,
		PaidCent:              o.PaidCent,
		RefundedCent:          o.RefundedCent,
		RefundStatus:          o.RefundStatus,
		ReceiverName:          o.ReceiverName,
		ReceiverPhone:         o.ReceiverPhone,
		ReceiverAddress:       o.ReceiverAddress,
		Items:                 items,
		QuoteId:               o.QuoteID,
		CreatedAtUnix:         o.CreatedAt.Unix(),
		ExpireAtUnix:          o.ExpireAt.Unix(),
	}
}

const orderColumns = `order_no, user_id, status, total_cent, discount_cent, promotion_discount_cent, applied_promotions_json,
	freight_cent, pay_cent, paid_cent, refunded_cent, refund_status, receiver_name, receiver_phone, receiver_address, quote_id, client_request_id, created_at, expire_at`

func scanOrder(row *sql.Row) (*orderRecord, error) {
	var o orderRecord
	err := row.Scan(&o.OrderNo, &o.UserID, &o.Status, &o.TotalCent, &o.DiscountCent, &o.PromotionDiscountCent,
		&o.AppliedPromotionsJSON, &o.FreightCent, &o.PayCent, &o.PaidCent, &o.RefundedCent, &o.RefundStatus,
		&o.ReceiverName, &o.ReceiverPhone, &o.ReceiverAddress, &o.QuoteID,
		&o.ClientRequestID, &o.CreatedAt, &o.ExpireAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// findOrder returns nil when the order does not exist.
func findOrder(ctx context.Context, q querier, orderNo string) (*orderRecord, error) {
	return scanOrder(q.QueryRowContext(ctx, `SELECT `+orderColumns+` FROM orders WHERE order_no = ?`, orderNo))
}

// findOrderByClientRequest returns nil when no order matches the idempotency key.
func findOrderByClientRequest(ctx context.Context, q querier, userID int64, clientRequestID string) (*orderRecord, error) {
	return scanOrder(q.QueryRowContext(ctx, `SELECT `+orderColumns+`
		FROM orders WHERE user_id = ? AND client_request_id = ? ORDER BY id LIMIT 1`, userID, clientRequestID))
}

func insertOrder(ctx context.Context, tx *sql.Tx, o *orderRecord, now time.Time) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO orders
		(order_no, user_id, status, total_cent, discount_cent, promotion_discount_cent, applied_promotions_json,
		 freight_cent, pay_cent, paid_cent, refunded_cent, refund_status,
		 receiver_name, receiver_phone, receiver_address, quote_id, client_request_id,
		 expire_at, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`,
		o.OrderNo, o.UserID, o.Status, o.TotalCent, o.DiscountCent, o.PromotionDiscountCent, o.AppliedPromotionsJSON,
		o.FreightCent, o.PayCent, o.PaidCent, o.RefundedCent, o.RefundStatus,
		o.ReceiverName, o.ReceiverPhone, o.ReceiverAddress, o.QuoteID, o.ClientRequestID,
		o.ExpireAt, now, now)
	return err
}

func insertOrderItems(ctx context.Context, tx *sql.Tx, orderNo string, items []*pb.OrderItem) error {
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `INSERT INTO order_items
			(order_no, sku_id, name, price_cent, quantity, discount_cent)
			VALUES (?, ?, ?, ?, ?, ?)`,
			orderNo, item.GetSkuId(), item.GetName(), item.GetPriceCent(), item.GetQuantity(), item.GetDiscountCent()); err != nil {
			return err
		}
	}
	return nil
}

// loadOrderItems loads the items of a single order.
func loadOrderItems(ctx context.Context, q querier, orderNo string) ([]*pb.OrderItem, error) {
	rows, err := q.QueryContext(ctx, `SELECT sku_id, name, price_cent, quantity, discount_cent
		FROM order_items WHERE order_no = ? ORDER BY id`, orderNo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]*pb.OrderItem, 0, 4)
	for rows.Next() {
		var item pb.OrderItem
		if err := rows.Scan(&item.SkuId, &item.Name, &item.PriceCent, &item.Quantity, &item.DiscountCent); err != nil {
			return nil, err
		}
		items = append(items, &item)
	}
	return items, rows.Err()
}

// loadOrderItemsByOrder loads the items of several orders grouped by order_no.
func loadOrderItemsByOrder(ctx context.Context, q querier, orderNos []string) (map[string][]*pb.OrderItem, error) {
	grouped := make(map[string][]*pb.OrderItem, len(orderNos))
	if len(orderNos) == 0 {
		return grouped, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(orderNos)), ",")
	args := make([]any, 0, len(orderNos))
	for _, orderNo := range orderNos {
		args = append(args, orderNo)
	}
	rows, err := q.QueryContext(ctx, `SELECT order_no, sku_id, name, price_cent, quantity, discount_cent
		FROM order_items WHERE order_no IN (`+placeholders+`) ORDER BY id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			orderNo string
			item    pb.OrderItem
		)
		if err := rows.Scan(&orderNo, &item.SkuId, &item.Name, &item.PriceCent, &item.Quantity, &item.DiscountCent); err != nil {
			return nil, err
		}
		grouped[orderNo] = append(grouped[orderNo], &item)
	}
	return grouped, rows.Err()
}

func insertStatusLog(ctx context.Context, tx *sql.Tx, orderNo, from, to, reason, operator string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO order_status_logs (order_no, from_status, to_status, reason, operator)
		VALUES (?, ?, ?, ?, ?)`, orderNo, from, to, truncate(reason, 255), truncate(operator, 64))
	return err
}

// updateOrderStatus applies a guarded status transition and bumps the version.
// It returns the number of affected rows; 0 means the order was not in
// from-status anymore (a concurrent request won).
func updateOrderStatus(ctx context.Context, tx *sql.Tx, orderNo, from, to string, paidCent int64) (int64, error) {
	res, err := tx.ExecContext(ctx, `UPDATE orders
		SET status = ?, paid_cent = ?, version = version + 1, updated_at = ?
		WHERE order_no = ? AND status = ?`, to, paidCent, time.Now(), orderNo, from)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// listOrders returns one page of orders plus the total count.
func listOrders(ctx context.Context, q querier, where string, args []any, orderBy string, limit, offset int32) ([]*orderRecord, error) {
	query := `SELECT ` + orderColumns + ` FROM orders ` + where + " ORDER BY " + orderBy +
		` LIMIT ? OFFSET ?`
	rows, err := q.QueryContext(ctx, query, append(args, limit, offset)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []*orderRecord
	for rows.Next() {
		var o orderRecord
		if err := rows.Scan(&o.OrderNo, &o.UserID, &o.Status, &o.TotalCent, &o.DiscountCent, &o.PromotionDiscountCent,
			&o.AppliedPromotionsJSON, &o.FreightCent, &o.PayCent, &o.PaidCent, &o.RefundedCent, &o.RefundStatus,
			&o.ReceiverName, &o.ReceiverPhone, &o.ReceiverAddress, &o.QuoteID,
			&o.ClientRequestID, &o.CreatedAt, &o.ExpireAt); err != nil {
			return nil, err
		}
		records = append(records, &o)
	}
	return records, rows.Err()
}

func countOrders(ctx context.Context, q querier, where string, args []any) (int64, error) {
	var total int64
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders `+where, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func isDuplicateKey(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}
