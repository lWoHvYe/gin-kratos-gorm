package inventory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/id"
	"gin-kratos-gorm/internal/platform/outbox"

	"github.com/go-sql-driver/mysql"
)

const (
	statusHeld      = "HELD"
	statusConfirmed = "CONFIRMED"
	statusReleased  = "RELEASED"

	entryReserve = "RESERVE"
	entryConfirm = "CONFIRM"
	entryRelease = "RELEASE"
	entryRestock = "RESTOCK"
	entryDamaged = "DAMAGED"

	topicStockChanged = "shop.inventory.stock.changed.v1"

	topicReserved  = "shop.inventory.reserved.v1"
	topicConfirmed = "shop.inventory.confirmed.v1"
	topicReleased  = "shop.inventory.released.v1"

	eventReserved  = "inventory.reserved"
	eventConfirmed = "inventory.confirmed"
	eventReleased  = "inventory.released"
)

// errReservationExists signals that a concurrent request already created the
// reservation for (order_no, sku_id). The transaction is rolled back and the
// caller re-reads the winning row.
var errReservationExists = errors.New("inventory: reservation already exists")

type rowQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type stock struct {
	skuID       int64
	warehouseID int64
	onHand      int64
	reserved    int64
}

func (s *stock) available() int64 { return s.onHand - s.reserved }

type reservation struct {
	id            int64
	reservationNo string
	orderNo       string
	skuID         int64
	warehouseID   int64
	quantity      int64
	status        string
	expiresAt     time.Time
	createdAt     time.Time
}

func scanReservation(row *sql.Row) (*reservation, error) {
	var r reservation
	if err := row.Scan(&r.id, &r.reservationNo, &r.orderNo, &r.skuID, &r.warehouseID,
		&r.quantity, &r.status, &r.expiresAt, &r.createdAt); err != nil {
		return nil, err
	}
	return &r, nil
}

// stock loads the inventory row for (sku_id, warehouse_id). A missing row is a
// NOT_FOUND: inventory rows are provisioned out of band, this service never
// invents stock.
func (s *Service) stock(ctx context.Context, q rowQuerier, skuID, warehouseID int64) (*stock, error) {
	var st stock
	err := q.QueryRowContext(ctx, `SELECT sku_id, warehouse_id, on_hand, reserved
		FROM inventory_stocks WHERE sku_id = ? AND warehouse_id = ?`, skuID, warehouseID).
		Scan(&st.skuID, &st.warehouseID, &st.onHand, &st.reserved)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound(fmt.Sprintf("stock not found for sku %d in warehouse %d", skuID, warehouseID))
	}
	if err != nil {
		return nil, apperr.Internal("query stock", err)
	}
	return &st, nil
}

// reservationByOrderSku resolves the unique reservation of an order line. When
// forUpdate is set the row is locked for the remainder of the transaction so
// status transitions serialize.
func (s *Service) reservationByOrderSku(ctx context.Context, q rowQuerier, orderNo string, skuID int64, forUpdate bool) (*reservation, error) {
	query := `SELECT id, reservation_no, order_no, sku_id, warehouse_id, quantity, status, expires_at, created_at
		FROM stock_reservations WHERE order_no = ? AND sku_id = ?`
	if forUpdate {
		query += " FOR UPDATE"
	}
	r, err := scanReservation(q.QueryRowContext(ctx, query, orderNo, skuID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound(fmt.Sprintf("reservation not found for order %s sku %d", orderNo, skuID))
	}
	if err != nil {
		return nil, apperr.Internal("query reservation", err)
	}
	return r, nil
}

// reserveTx performs the conditional stock decrement, inserts the HELD
// reservation, writes the RESERVE ledger row and appends the outbox event. All
// statements share the caller's *sql.Tx.
func (s *Service) reserveTx(ctx context.Context, tx *sql.Tx, in reserveInput) (*reservation, error) {
	res, err := tx.ExecContext(ctx, `UPDATE inventory_stocks
		SET reserved = reserved + ?, version = version + 1
		WHERE sku_id = ? AND warehouse_id = ? AND on_hand - reserved >= ?`,
		in.quantity, in.skuID, in.warehouseID, in.quantity)
	if err != nil {
		return nil, apperr.Internal("reserve stock", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, apperr.Internal("reserve stock rows", err)
	}
	if affected == 0 {
		st, err := s.stock(ctx, tx, in.skuID, in.warehouseID)
		if err != nil {
			return nil, err
		}
		return nil, apperr.Failed(fmt.Sprintf("insufficient stock: available %d", st.available()))
	}
	now := time.Now()
	r := &reservation{
		reservationNo: id.Reservation(),
		orderNo:       in.orderNo,
		skuID:         in.skuID,
		warehouseID:   in.warehouseID,
		quantity:      in.quantity,
		status:        statusHeld,
		expiresAt:     now.Add(in.ttl),
		createdAt:     now,
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO stock_reservations
		(reservation_no, order_no, sku_id, warehouse_id, quantity, status, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.reservationNo, r.orderNo, r.skuID, r.warehouseID, r.quantity, r.status, r.expiresAt, now, now); err != nil {
		if isDuplicateEntry(err) {
			return nil, errReservationExists
		}
		return nil, apperr.Internal("insert reservation", err)
	}
	st, err := s.stock(ctx, tx, r.skuID, r.warehouseID)
	if err != nil {
		return nil, err
	}
	if err := insertLedger(ctx, tx, r, entryReserve, st); err != nil {
		return nil, err
	}
	if err := appendEvent(ctx, tx, r, topicReserved, eventReserved); err != nil {
		return nil, err
	}
	if err := appendStockChanged(ctx, tx, st); err != nil {
		return nil, err
	}
	return r, nil
}

// confirmTx converts a HELD reservation into a CONFIRMED one and consumes the
// reserved quantity. Re-confirming an already CONFIRMED reservation is a no-op.
func (s *Service) confirmTx(ctx context.Context, tx *sql.Tx, in reserveInput) (*reservation, error) {
	r, err := s.reservationByOrderSku(ctx, tx, in.orderNo, in.skuID, true)
	if err != nil {
		return nil, err
	}
	if in.quantity > 0 && in.quantity != r.quantity {
		return nil, apperr.Invalid("quantity %d does not match reservation %s quantity %d", in.quantity, r.reservationNo, r.quantity)
	}
	switch r.status {
	case statusConfirmed:
		return r, nil
	case statusReleased:
		return nil, apperr.Conflict(fmt.Sprintf("reservation %s is already released", r.reservationNo))
	case statusHeld:
	default:
		return nil, apperr.Internal("unexpected reservation status", fmt.Errorf("status %q", r.status))
	}

	res, err := tx.ExecContext(ctx, `UPDATE inventory_stocks
		SET on_hand = on_hand - ?, reserved = reserved - ?, version = version + 1
		WHERE sku_id = ? AND warehouse_id = ? AND on_hand >= ? AND reserved >= ?`,
		r.quantity, r.quantity, r.skuID, r.warehouseID, r.quantity, r.quantity)
	if err != nil {
		return nil, apperr.Internal("confirm stock", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, apperr.Internal("confirm stock rows", err)
	}
	if affected == 0 {
		return nil, apperr.Failed(fmt.Sprintf("stock is inconsistent for reservation %s", r.reservationNo))
	}
	res, err = tx.ExecContext(ctx, `UPDATE stock_reservations SET status = ?, updated_at = ?
		WHERE id = ? AND status = ?`, statusConfirmed, time.Now(), r.id, statusHeld)
	if err != nil {
		return nil, apperr.Internal("confirm reservation", err)
	}
	affected, err = res.RowsAffected()
	if err != nil {
		return nil, apperr.Internal("confirm reservation rows", err)
	}
	if affected == 0 {
		return nil, apperr.Conflict(fmt.Sprintf("reservation %s changed concurrently", r.reservationNo))
	}
	if err := s.confirmSeckillMapping(ctx, tx, r); err != nil {
		return nil, err
	}
	st, err := s.stock(ctx, tx, r.skuID, r.warehouseID)
	if err != nil {
		return nil, err
	}
	r.status = statusConfirmed
	if err := insertLedger(ctx, tx, r, entryConfirm, st); err != nil {
		return nil, err
	}
	if err := appendEvent(ctx, tx, r, topicConfirmed, eventConfirmed); err != nil {
		return nil, err
	}
	if err := appendStockChanged(ctx, tx, st); err != nil {
		return nil, err
	}
	return r, nil
}

// releaseTx gives a HELD reservation back to availability. Releasing an already
// RELEASED reservation is a no-op; a CONFIRMED reservation must go through the
// restock flow instead.
func (s *Service) releaseTx(ctx context.Context, tx *sql.Tx, in reserveInput) (*reservation, error) {
	r, err := s.reservationByOrderSku(ctx, tx, in.orderNo, in.skuID, true)
	if err != nil {
		return nil, err
	}
	if in.quantity > 0 && in.quantity != r.quantity {
		return nil, apperr.Invalid("quantity %d does not match reservation %s quantity %d", in.quantity, r.reservationNo, r.quantity)
	}
	switch r.status {
	case statusReleased:
		return r, nil
	case statusConfirmed:
		return nil, apperr.Conflict(fmt.Sprintf("reservation %s is confirmed; use the restock flow", r.reservationNo))
	case statusHeld:
	default:
		return nil, apperr.Internal("unexpected reservation status", fmt.Errorf("status %q", r.status))
	}

	seckill, err := s.releaseSeckillMapping(ctx, tx, r)
	if err != nil {
		return nil, err
	}
	if !seckill {
		res, err := tx.ExecContext(ctx, `UPDATE inventory_stocks
			SET reserved = reserved - ?, version = version + 1
			WHERE sku_id = ? AND warehouse_id = ? AND reserved >= ?`,
			r.quantity, r.skuID, r.warehouseID, r.quantity)
		if err != nil {
			return nil, apperr.Internal("release stock", err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return nil, apperr.Internal("release stock rows", err)
		}
		if affected == 0 {
			return nil, apperr.Failed(fmt.Sprintf("stock is inconsistent for reservation %s", r.reservationNo))
		}
	}
	res, err := tx.ExecContext(ctx, `UPDATE stock_reservations SET status = ?, updated_at = ?
		WHERE id = ? AND status = ?`, statusReleased, time.Now(), r.id, statusHeld)
	if err != nil {
		return nil, apperr.Internal("release reservation", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, apperr.Internal("release reservation rows", err)
	}
	if affected == 0 {
		return nil, apperr.Conflict(fmt.Sprintf("reservation %s changed concurrently", r.reservationNo))
	}
	st, err := s.stock(ctx, tx, r.skuID, r.warehouseID)
	if err != nil {
		return nil, err
	}
	r.status = statusReleased
	if err := insertLedger(ctx, tx, r, entryRelease, st); err != nil {
		return nil, err
	}
	if err := appendEvent(ctx, tx, r, topicReleased, eventReleased); err != nil {
		return nil, err
	}
	if err := appendStockChanged(ctx, tx, st); err != nil {
		return nil, err
	}
	return r, nil
}

func insertLedger(ctx context.Context, tx *sql.Tx, r *reservation, entryType string, st *stock) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO inventory_ledger
		(reservation_no, order_no, sku_id, warehouse_id, quantity, entry_type, on_hand_after, reserved_after)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		r.reservationNo, r.orderNo, r.skuID, r.warehouseID, r.quantity, entryType, st.onHand, st.reserved)
	if err != nil {
		return apperr.Internal("insert ledger", err)
	}
	return nil
}

type inventoryEvent struct {
	ReservationNo  string `json:"reservation_no"`
	OrderNo        string `json:"order_no"`
	SkuID          int64  `json:"sku_id"`
	WarehouseID    int64  `json:"warehouse_id"`
	Quantity       int64  `json:"quantity"`
	Status         string `json:"status"`
	ExpiresAtUnix  int64  `json:"expires_at_unix,omitempty"`
	OccurredAtUnix int64  `json:"occurred_at_unix"`
}

func appendEvent(ctx context.Context, tx *sql.Tx, r *reservation, topic, eventType string) error {
	payload := inventoryEvent{
		ReservationNo:  r.reservationNo,
		OrderNo:        r.orderNo,
		SkuID:          r.skuID,
		WarehouseID:    r.warehouseID,
		Quantity:       r.quantity,
		Status:         r.status,
		ExpiresAtUnix:  r.expiresAt.Unix(),
		OccurredAtUnix: time.Now().Unix(),
	}
	err := outbox.Append(ctx, tx, outbox.Event{
		ID:            eventID(eventType, r.reservationNo),
		Topic:         topic,
		AggregateType: "stock_reservation",
		AggregateID:   r.reservationNo,
		EventType:     eventType,
		Payload:       payload,
	})
	if err != nil {
		return apperr.Internal("append outbox", err)
	}
	return nil
}

// eventID is deterministic per (event type, reservation) so the outbox can
// de-duplicate a replayed branch. outbox_events.event_id is VARCHAR(64).
func eventID(eventType, reservationNo string) string {
	v := "inv-" + eventType + "-" + reservationNo
	if len(v) > 64 {
		v = v[:64]
	}
	return v
}

func isDuplicateEntry(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

// appendStockChanged projects the authoritative inventory counters to the
// catalog read model. It is deliberately separate from the reservation domain
// event so consumers can subscribe to exactly one stock projection stream.
func appendStockChanged(ctx context.Context, tx *sql.Tx, st *stock) error {
	if st == nil {
		return nil
	}
	return outbox.Append(ctx, tx, outbox.Event{
		Topic:         topicStockChanged,
		AggregateType: "inventory_stock",
		AggregateID:   fmt.Sprintf("%d:%d", st.skuID, st.warehouseID),
		EventType:     "inventory.stock_changed",
		Payload: map[string]any{
			"sku_id":       st.skuID,
			"warehouse_id": st.warehouseID,
			"on_hand":      st.onHand,
			"reserved":     st.reserved,
			"available":    st.available(),
			"changed_at":   time.Now().Unix(),
		},
	})
}
