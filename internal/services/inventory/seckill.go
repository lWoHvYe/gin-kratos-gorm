package inventory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/db"
	"gin-kratos-gorm/internal/platform/id"
)

const (
	seckillStatusAllocated = "ALLOCATED"
	seckillStatusReleased  = "RELEASED"

	seckillLedgerAllocate = "ALLOCATE"
	seckillLedgerRelease  = "RELEASE"
)

type seckillStock struct {
	activityID  int64
	skuID       int64
	warehouseID int64
	total       int64
	available   int64
	held        int64
	sold        int64
	status      string
	version     int64
}

func (s *seckillStock) toPB() *pb.SeckillStock {
	return &pb.SeckillStock{
		ActivityId: s.activityID, SkuId: s.skuID, WarehouseId: s.warehouseID,
		Total: s.total, Available: s.available, Held: s.held, Sold: s.sold,
		Status: s.status, Version: s.version,
	}
}

// AllocateSeckillStock carves quantity units out of normal inventory into a
// dedicated activity bucket. It is idempotent by activity_id: a replay with the
// same sku/warehouse/quantity returns the existing bucket; any other replay is
// a conflict.
func (s *Service) AllocateSeckillStock(ctx context.Context, req *pb.AllocateSeckillStockRequest) (*pb.SeckillStock, error) {
	if req.GetActivityId() <= 0 {
		return nil, apperr.Invalid("activity_id is required")
	}
	if req.GetSkuId() <= 0 {
		return nil, apperr.Invalid("sku_id is required")
	}
	if req.GetQuantity() <= 0 {
		return nil, apperr.Invalid("quantity must be positive")
	}
	warehouseID := normalizeWarehouse(req.GetWarehouseId())

	existing, err := s.getSeckillStock(ctx, s.db, req.GetActivityId(), false)
	switch {
	case err == nil:
		if existing.skuID != req.GetSkuId() || existing.warehouseID != warehouseID || existing.total != req.GetQuantity() {
			return nil, apperr.Conflict("seckill stock already allocated with different parameters")
		}
		return existing.toPB(), nil
	case apperr.IsCode(err, apperr.CodeNotFound):
		// First allocation.
	default:
		return nil, err
	}

	var created *seckillStock
	err = db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE inventory_stocks
			SET reserved = reserved + ?, version = version + 1
			WHERE sku_id = ? AND warehouse_id = ? AND on_hand - reserved >= ?`,
			req.GetQuantity(), req.GetSkuId(), warehouseID, req.GetQuantity())
		if err != nil {
			return apperr.Internal("allocate seckill stock", err)
		}
		rows, err := res.RowsAffected()
		if err != nil {
			return apperr.Internal("allocate seckill rows", err)
		}
		if rows == 0 {
			return apperr.Conflict("insufficient inventory for seckill allocation")
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO seckill_stocks
			(activity_id, sku_id, warehouse_id, total, available, held, sold, status, version)
			VALUES (?, ?, ?, ?, ?, 0, 0, ?, 1)`,
			req.GetActivityId(), req.GetSkuId(), warehouseID, req.GetQuantity(), req.GetQuantity(), seckillStatusAllocated); err != nil {
			if isDuplicateEntry(err) {
				return errSeckillStockExists
			}
			return apperr.Internal("insert seckill stock", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO seckill_stock_ledger
			(activity_id, delta, balance_after, reason, ref_no) VALUES (?, ?, ?, ?, ?)`,
			req.GetActivityId(), req.GetQuantity(), req.GetQuantity(), seckillLedgerAllocate, fmt.Sprintf("activity:%d", req.GetActivityId())); err != nil {
			return apperr.Internal("insert seckill ledger", err)
		}
		created, err = s.getSeckillStock(ctx, tx, req.GetActivityId(), false)
		return err
	})
	if errors.Is(err, errSeckillStockExists) {
		winner, readErr := s.getSeckillStock(ctx, s.db, req.GetActivityId(), false)
		if readErr != nil {
			return nil, readErr
		}
		return winner.toPB(), nil
	}
	if err != nil {
		return nil, err
	}
	return created.toPB(), nil
}

// ReleaseSeckillStock returns the unsold (available) part of a bucket to normal
// inventory. Held/sold quantities are never silently returned. It is idempotent.
func (s *Service) ReleaseSeckillStock(ctx context.Context, req *pb.ReleaseSeckillStockRequest) (*pb.SeckillStock, error) {
	if req.GetActivityId() <= 0 {
		return nil, apperr.Invalid("activity_id is required")
	}
	var released *seckillStock
	err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		current, err := s.getSeckillStock(ctx, tx, req.GetActivityId(), true)
		if err != nil {
			return err
		}
		if current.status == seckillStatusReleased {
			released = current
			return nil
		}
		if current.available < 0 {
			return apperr.Internal("invalid seckill stock", fmt.Errorf("available=%d", current.available))
		}
		if current.available > 0 {
			res, err := tx.ExecContext(ctx, `UPDATE inventory_stocks
				SET reserved = reserved - ?, version = version + 1
				WHERE sku_id = ? AND warehouse_id = ? AND reserved >= ?`,
				current.available, current.skuID, current.warehouseID, current.available)
			if err != nil {
				return apperr.Internal("release seckill stock", err)
			}
			rows, err := res.RowsAffected()
			if err != nil {
				return apperr.Internal("release seckill rows", err)
			}
			if rows == 0 {
				return apperr.Conflict("seckill release would make inventory negative")
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE seckill_stocks
			SET available = 0, status = ?, version = version + 1 WHERE activity_id = ?`,
			seckillStatusReleased, current.activityID); err != nil {
			return apperr.Internal("mark seckill released", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO seckill_stock_ledger
			(activity_id, delta, balance_after, reason, ref_no) VALUES (?, ?, 0, ?, ?)`,
			current.activityID, -current.available, seckillLedgerRelease, fmt.Sprintf("activity:%d", current.activityID)); err != nil {
			return apperr.Internal("insert seckill release ledger", err)
		}
		released, err = s.getSeckillStock(ctx, tx, current.activityID, false)
		return err
	})
	if err != nil {
		return nil, err
	}
	return released.toPB(), nil
}

func (s *Service) GetSeckillStock(ctx context.Context, req *pb.GetSeckillStockRequest) (*pb.SeckillStock, error) {
	if req.GetActivityId() <= 0 {
		return nil, apperr.Invalid("activity_id is required")
	}
	st, err := s.getSeckillStock(ctx, s.db, req.GetActivityId(), false)
	if err != nil {
		return nil, err
	}
	return st.toPB(), nil
}

func (s *Service) getSeckillStock(ctx context.Context, q rowQuerier, activityID int64, forUpdate bool) (*seckillStock, error) {
	query := `SELECT activity_id, sku_id, warehouse_id, total, available, held, sold, status, version
		FROM seckill_stocks WHERE activity_id = ?`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var st seckillStock
	err := q.QueryRowContext(ctx, query, activityID).Scan(&st.activityID, &st.skuID, &st.warehouseID,
		&st.total, &st.available, &st.held, &st.sold, &st.status, &st.version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound(fmt.Sprintf("seckill stock not found for activity %d", activityID))
	}
	if err != nil {
		return nil, apperr.Internal("query seckill stock", err)
	}
	return &st, nil
}

var errSeckillStockExists = errors.New("inventory: seckill stock already exists")

const (
	seckillLedgerHold        = "HOLD"
	seckillLedgerReleaseHold = "RELEASE_HOLD"
)

// HoldSeckillStock moves one unit from available to held for a qualification.
// The ref_no (token hash) makes the operation idempotent. This is the durable
// side of S2 admission; Redis is only the fast gate.
func (s *Service) HoldSeckillStock(ctx context.Context, req *pb.HoldSeckillStockRequest) (*pb.SeckillStock, error) {
	if req.GetActivityId() <= 0 || req.GetQuantity() <= 0 || strings.TrimSpace(req.GetRefNo()) == "" {
		return nil, apperr.Invalid("activity_id, quantity and ref_no are required")
	}
	var out *seckillStock
	err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		exists, err := s.seckillLedgerExists(ctx, tx, req.GetActivityId(), seckillLedgerHold, req.GetRefNo())
		if err != nil {
			return err
		}
		if exists {
			out, err = s.getSeckillStock(ctx, tx, req.GetActivityId(), false)
			return err
		}
		current, err := s.getSeckillStock(ctx, tx, req.GetActivityId(), true)
		if err != nil {
			return err
		}
		if current.status != seckillStatusAllocated {
			return apperr.Conflict("seckill stock bucket is not open")
		}
		if current.available < req.GetQuantity() {
			return apperr.Conflict("seckill stock is exhausted")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE seckill_stocks
			SET available = available - ?, held = held + ?, version = version + 1
			WHERE activity_id = ? AND available >= ?`,
			req.GetQuantity(), req.GetQuantity(), req.GetActivityId(), req.GetQuantity()); err != nil {
			return apperr.Internal("hold seckill stock", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO seckill_stock_ledger
			(activity_id, delta, balance_after, reason, ref_no) VALUES (?, ?, ?, ?, ?)`,
			req.GetActivityId(), -req.GetQuantity(), current.available-req.GetQuantity(), seckillLedgerHold, req.GetRefNo()); err != nil {
			return apperr.Internal("insert seckill hold ledger", err)
		}
		out, err = s.getSeckillStock(ctx, tx, req.GetActivityId(), false)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out.toPB(), nil
}

// ReleaseSeckillHold returns a held qualification to available. It is
// idempotent by ref_no and is used by qualification expiry/compensation.
func (s *Service) ReleaseSeckillHold(ctx context.Context, req *pb.ReleaseSeckillHoldRequest) (*pb.SeckillStock, error) {
	if req.GetActivityId() <= 0 || req.GetQuantity() <= 0 || strings.TrimSpace(req.GetRefNo()) == "" {
		return nil, apperr.Invalid("activity_id, quantity and ref_no are required")
	}
	var out *seckillStock
	err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		released, err := s.seckillLedgerExists(ctx, tx, req.GetActivityId(), seckillLedgerReleaseHold, req.GetRefNo())
		if err != nil {
			return err
		}
		if released {
			out, err = s.getSeckillStock(ctx, tx, req.GetActivityId(), false)
			return err
		}
		current, err := s.getSeckillStock(ctx, tx, req.GetActivityId(), true)
		if err != nil {
			return err
		}
		if current.held < req.GetQuantity() {
			return apperr.Conflict("seckill held stock is insufficient")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE seckill_stocks
			SET available = available + ?, held = held - ?, version = version + 1
			WHERE activity_id = ? AND held >= ?`,
			req.GetQuantity(), req.GetQuantity(), req.GetActivityId(), req.GetQuantity()); err != nil {
			return apperr.Internal("release seckill hold", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO seckill_stock_ledger
			(activity_id, delta, balance_after, reason, ref_no) VALUES (?, ?, ?, ?, ?)`,
			req.GetActivityId(), req.GetQuantity(), current.available+req.GetQuantity(), seckillLedgerReleaseHold, req.GetRefNo()); err != nil {
			return apperr.Internal("insert seckill release-hold ledger", err)
		}
		out, err = s.getSeckillStock(ctx, tx, req.GetActivityId(), false)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out.toPB(), nil
}

func (s *Service) seckillLedgerExists(ctx context.Context, q rowQuerier, activityID int64, reason, refNo string) (bool, error) {
	var one int
	err := q.QueryRowContext(ctx, `SELECT 1 FROM seckill_stock_ledger
		WHERE activity_id=? AND reason=? AND ref_no=? LIMIT 1`, activityID, reason, refNo).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, apperr.Internal("query seckill ledger", err)
	}
	return true, nil
}

type seckillOrderStock struct {
	activityID int64
	orderNo    string
	skuID      int64
	quantity   int64
	refNo      string
	status     string
}

func (s *Service) seckillOrderMapping(ctx context.Context, q rowQuerier, orderNo string, skuID int64, forUpdate bool) (*seckillOrderStock, error) {
	query := `SELECT activity_id, order_no, sku_id, quantity, ref_no, status
		FROM seckill_order_stock WHERE order_no=? AND sku_id=?`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var m seckillOrderStock
	err := q.QueryRowContext(ctx, query, orderNo, skuID).Scan(&m.activityID, &m.orderNo, &m.skuID, &m.quantity, &m.refNo, &m.status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, apperr.Internal("query seckill order stock", err)
	}
	return &m, nil
}

// confirmSeckillMapping commits a held seckill unit when its order is paid.
func (s *Service) confirmSeckillMapping(ctx context.Context, tx *sql.Tx, r *reservation) error {
	m, err := s.seckillOrderMapping(ctx, tx, r.orderNo, r.skuID, true)
	if err != nil || m == nil {
		return err
	}
	if m.status == statusConfirmed {
		return nil
	}
	if m.status != statusHeld {
		return apperr.Conflict("seckill order stock is not held")
	}
	st, err := s.getSeckillStock(ctx, tx, m.activityID, true)
	if err != nil {
		return err
	}
	if st.held < m.quantity {
		return apperr.Conflict("seckill held stock is insufficient")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE seckill_stocks
		SET held = held - ?, sold = sold + ?, version = version + 1
		WHERE activity_id = ? AND held >= ?`,
		m.quantity, m.quantity, m.activityID, m.quantity); err != nil {
		return apperr.Internal("confirm seckill order stock", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE seckill_order_stock SET status=? WHERE order_no=? AND sku_id=?`,
		statusConfirmed, m.orderNo, m.skuID); err != nil {
		return apperr.Internal("confirm seckill order mapping", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO seckill_stock_ledger
		(activity_id, delta, balance_after, reason, ref_no) VALUES (?, ?, ?, ?, ?)`,
		m.activityID, -m.quantity, st.available, seckillLedgerSell, m.orderNo); err != nil {
		return apperr.Internal("insert seckill sell ledger", err)
	}
	return nil
}

// releaseSeckillMapping returns an order-held unit to the activity bucket. It is
// called by the normal reservation Release path and reports whether the
// reservation is a seckill reservation (so inventory_stocks.reserved must not
// be decremented a second time).
func (s *Service) releaseSeckillMapping(ctx context.Context, tx *sql.Tx, r *reservation) (bool, error) {
	m, err := s.seckillOrderMapping(ctx, tx, r.orderNo, r.skuID, true)
	if err != nil || m == nil {
		return false, err
	}
	switch m.status {
	case statusReleased:
		return true, nil
	case statusConfirmed:
		return true, apperr.Conflict("seckill order stock is already confirmed")
	case statusHeld:
	default:
		return true, apperr.Conflict("unexpected seckill order stock status")
	}
	st, err := s.getSeckillStock(ctx, tx, m.activityID, true)
	if err != nil {
		return true, err
	}
	if st.held < m.quantity {
		return true, apperr.Conflict("seckill held stock is insufficient")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE seckill_stocks
		SET held = held - ?, available = available + ?, version = version + 1
		WHERE activity_id = ? AND held >= ?`,
		m.quantity, m.quantity, m.activityID, m.quantity); err != nil {
		return true, apperr.Internal("release seckill order stock", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE seckill_order_stock SET status=? WHERE order_no=? AND sku_id=?`,
		statusReleased, m.orderNo, m.skuID); err != nil {
		return true, apperr.Internal("release seckill order mapping", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO seckill_stock_ledger
		(activity_id, delta, balance_after, reason, ref_no) VALUES (?, ?, ?, ?, ?)`,
		m.activityID, m.quantity, st.available+m.quantity, seckillLedgerReleaseOrder, m.orderNo); err != nil {
		return true, apperr.Internal("insert seckill order-release ledger", err)
	}
	return true, nil
}

// CreateSeckillReservation converts an existing qualification hold into an
// order reservation without incrementing inventory_stocks.reserved again. The
// inventory reservation row is still the object the payment path confirms.
func (s *Service) CreateSeckillReservation(ctx context.Context, req *pb.CreateSeckillReservationRequest) (*pb.ReserveReply, error) {
	if req.GetActivityId() <= 0 || strings.TrimSpace(req.GetOrderNo()) == "" || req.GetSkuId() <= 0 ||
		req.GetQuantity() <= 0 || strings.TrimSpace(req.GetRefNo()) == "" {
		return nil, apperr.Invalid("activity_id, order_no, sku_id, quantity and ref_no are required")
	}
	existing, err := s.reservationByOrderSku(ctx, s.db, req.GetOrderNo(), req.GetSkuId(), false)
	if err == nil {
		return reservationReply(existing)
	}
	if !apperr.IsCode(err, apperr.CodeNotFound) {
		return nil, err
	}
	ttl := defaultReservationTTL
	if req.GetTtlSeconds() > 0 {
		ttl = time.Duration(req.GetTtlSeconds()) * time.Second
		if ttl > maxReservationTTL {
			ttl = maxReservationTTL
		}
	}
	var created *reservation
	err = db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		m, err := s.seckillOrderMapping(ctx, tx, req.GetOrderNo(), req.GetSkuId(), true)
		if err != nil {
			return err
		}
		if m != nil && m.status == statusHeld {
			// Already converted.
			r, rerr := s.reservationByOrderSku(ctx, tx, req.GetOrderNo(), req.GetSkuId(), false)
			created = r
			return rerr
		}
		st, err := s.getSeckillStock(ctx, tx, req.GetActivityId(), true)
		if err != nil {
			return err
		}
		if st.held < req.GetQuantity() {
			return apperr.Conflict("seckill held stock is insufficient")
		}
		held, err := s.seckillLedgerExists(ctx, tx, req.GetActivityId(), seckillLedgerHold, req.GetRefNo())
		if err != nil {
			return err
		}
		if !held {
			return apperr.Conflict("seckill qualification hold is missing")
		}
		now := time.Now()
		r := &reservation{
			reservationNo: id.Reservation(),
			orderNo:       req.GetOrderNo(),
			skuID:         req.GetSkuId(),
			warehouseID:   st.warehouseID,
			quantity:      req.GetQuantity(),
			status:        statusHeld,
			expiresAt:     now.Add(ttl),
			createdAt:     now,
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO stock_reservations
			(reservation_no, order_no, sku_id, warehouse_id, quantity, status, expires_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			r.reservationNo, r.orderNo, r.skuID, r.warehouseID, r.quantity, r.status, r.expiresAt, now, now); err != nil {
			return apperr.Internal("insert seckill reservation", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO seckill_order_stock
			(activity_id, order_no, sku_id, quantity, ref_no, status) VALUES (?, ?, ?, ?, ?, ?)`,
			req.GetActivityId(), r.orderNo, r.skuID, r.quantity, req.GetRefNo(), statusHeld); err != nil {
			return apperr.Internal("insert seckill order mapping", err)
		}
		created = r
		return nil
	})
	if err != nil {
		return nil, err
	}
	if created == nil {
		return nil, apperr.Internal("seckill reservation was not created", nil)
	}
	return reservationReply(created)
}

// ReleaseSeckillReservation is the failure path used when order creation fails
// after the inventory reservation was created. It keeps the global allocation
// intact and returns the unit to the seckill bucket.
func (s *Service) ReleaseSeckillReservation(ctx context.Context, req *pb.ReleaseSeckillReservationRequest) (*pb.Empty, error) {
	if strings.TrimSpace(req.GetOrderNo()) == "" || req.GetSkuId() <= 0 {
		return nil, apperr.Invalid("order_no and sku_id are required")
	}
	err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		r, err := s.reservationByOrderSku(ctx, tx, req.GetOrderNo(), req.GetSkuId(), true)
		if err != nil {
			return err
		}
		if r.status == statusReleased {
			return nil
		}
		if r.status != statusHeld {
			return apperr.Conflict("seckill reservation is not held")
		}
		seckill, err := s.releaseSeckillMapping(ctx, tx, r)
		if err != nil {
			return err
		}
		if !seckill {
			return apperr.Conflict("reservation is not a seckill reservation")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE stock_reservations SET status=?, updated_at=? WHERE id=? AND status=?`,
			statusReleased, time.Now(), r.id, statusHeld); err != nil {
			return apperr.Internal("release seckill reservation", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

const (
	seckillLedgerSell         = "SELL"
	seckillLedgerReleaseOrder = "RELEASE_ORDER"
)
