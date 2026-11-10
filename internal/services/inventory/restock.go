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
	"gin-kratos-gorm/internal/platform/id"
)

// ListStocks returns inventory rows for the admin stock page.
func (s *Service) ListStocks(ctx context.Context, req *pb.ListStocksRequest) (*pb.StockList, error) {
	page, size := req.GetPage(), req.GetPageSize()
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 200 {
		size = 50
	}
	where := "WHERE 1 = 1"
	args := []any{}
	if req.GetSkuId() > 0 {
		where += " AND sku_id = ?"
		args = append(args, req.GetSkuId())
	}
	if req.GetWarehouseId() > 0 {
		where += " AND warehouse_id = ?"
		args = append(args, req.GetWarehouseId())
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM inventory_stocks "+where, args...).Scan(&total); err != nil {
		return nil, apperr.Internal("count stocks", err)
	}
	args = append(args, size, (page-1)*size)
	rows, err := s.db.QueryContext(ctx, `SELECT sku_id, warehouse_id, on_hand, reserved FROM inventory_stocks `+where+` ORDER BY sku_id, warehouse_id LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, apperr.Internal("list stocks", err)
	}
	defer rows.Close()
	reply := &pb.StockList{Total: total}
	for rows.Next() {
		var st stock
		if err := rows.Scan(&st.skuID, &st.warehouseID, &st.onHand, &st.reserved); err != nil {
			return nil, apperr.Internal("scan stock", err)
		}
		reply.Stocks = append(reply.Stocks, &pb.Stock{SkuId: st.skuID, WarehouseId: st.warehouseID, OnHand: st.onHand, Reserved: st.reserved, Available: st.available()})
	}
	return reply, rows.Err()
}

// Restock returns sold goods to stock after a return has been received. A
// damaged return is recorded in the ledger but never becomes sellable stock.
// The restock key is deterministic per after-sale, making re-delivery safe.
func (s *Service) Restock(ctx context.Context, req *pb.RestockRequest) (*pb.Stock, error) {
	skuID, quantity := req.GetSkuId(), req.GetQuantity()
	warehouseID := normalizeWarehouse(req.GetWarehouseId())
	restockNo := strings.TrimSpace(req.GetRestockNo())
	if restockNo == "" {
		restockNo = strings.TrimSpace(req.GetOrderNo())
	}
	if skuID <= 0 || quantity <= 0 || restockNo == "" {
		return nil, apperr.Invalid("restock_no, sku_id and positive quantity are required")
	}
	condition := strings.ToUpper(strings.TrimSpace(req.GetCondition()))
	if condition == "" {
		condition = "SELLABLE"
	}
	if condition != "SELLABLE" && condition != "DAMAGED" {
		return nil, apperr.Invalid("condition must be SELLABLE or DAMAGED")
	}
	returnOrder := "RET:" + truncateString(restockNo, 56)
	var result *stock
	err := s.write(ctx, func(tx *sql.Tx) error {
		_, err := scanReservation(tx.QueryRowContext(ctx, `SELECT id, reservation_no, order_no, sku_id, warehouse_id, quantity, status, expires_at, created_at
			FROM stock_reservations WHERE order_no = ? AND sku_id = ?`, returnOrder, skuID))
		if err == nil {
			result, err = s.stock(ctx, tx, skuID, warehouseID)
			return err
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return apperr.Internal("query restock", err)
		}
		if condition == "SELLABLE" {
			res, err := tx.ExecContext(ctx, `UPDATE inventory_stocks SET on_hand = on_hand + ?, version = version + 1
				WHERE sku_id = ? AND warehouse_id = ?`, quantity, skuID, warehouseID)
			if err != nil {
				return apperr.Internal("restock inventory", err)
			}
			if n, _ := res.RowsAffected(); n == 0 {
				return apperr.NotFound(fmt.Sprintf("stock row not found for sku %d", skuID))
			}
		}
		now := time.Now()
		r := &reservation{
			reservationNo: id.Reservation(), orderNo: returnOrder, skuID: skuID,
			warehouseID: warehouseID, quantity: quantity,
			status: "RESTOCKED", expiresAt: now, createdAt: now,
		}
		if condition == "DAMAGED" {
			r.status = "DAMAGED"
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO stock_reservations
			(reservation_no, order_no, sku_id, warehouse_id, quantity, status, expires_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			r.reservationNo, r.orderNo, r.skuID, r.warehouseID, r.quantity, r.status, r.expiresAt, now, now); err != nil {
			return apperr.Internal("insert restock record", err)
		}
		result, err = s.stock(ctx, tx, skuID, warehouseID)
		if err != nil {
			return err
		}
		entry := entryRestock
		if condition == "DAMAGED" {
			entry = entryDamaged
		}
		if err := insertLedger(ctx, tx, r, entry, result); err != nil {
			return err
		}
		if condition == "SELLABLE" {
			return appendStockChanged(ctx, tx, result)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &pb.Stock{SkuId: result.skuID, WarehouseId: result.warehouseID, OnHand: result.onHand, Reserved: result.reserved, Available: result.available()}, nil
}

func truncateString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
