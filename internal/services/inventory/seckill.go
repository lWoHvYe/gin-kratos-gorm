package inventory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/db"
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
