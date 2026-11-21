package order

import (
	"context"
	"database/sql"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

const defaultGroupNo = "G1"

// fulfillmentGroup is the phase-1 shadow of the allocation snapshot. One
// existing/new order has exactly one G1 group; cross-warehouse split orders
// will add more groups in a later, separately reviewed phase.
type fulfillmentGroup struct {
	GroupNo          string
	OrderNo          string
	Seq              int
	WarehouseID      int64
	Status           string
	ReservationState string
	PermitState      string
	FreightCent      int64
}

func insertFulfillmentGroups(ctx context.Context, tx *sql.Tx, orderNo string, groups []*pb.OrderFulfillmentGroup, fallbackFreight int64) error {
	if len(groups) == 0 {
		groups = []*pb.OrderFulfillmentGroup{{
			GroupNo: defaultGroupNo, WarehouseId: defaultWarehouseID, FreightCent: fallbackFreight,
		}}
	}
	seenGroup := make(map[string]struct{}, len(groups))
	seenWarehouse := make(map[int64]struct{}, len(groups))
	for i, group := range groups {
		groupNo := group.GetGroupNo()
		if groupNo == "" {
			groupNo = defaultGroupNo
		}
		warehouseID := group.GetWarehouseId()
		if warehouseID <= 0 {
			warehouseID = defaultWarehouseID
		}
		if _, exists := seenGroup[groupNo]; exists {
			return apperr.Internal("duplicate fulfillment group", nil)
		}
		if _, exists := seenWarehouse[warehouseID]; exists {
			return apperr.Internal("duplicate fulfillment warehouse", nil)
		}
		seenGroup[groupNo] = struct{}{}
		seenWarehouse[warehouseID] = struct{}{}
		if _, err := tx.ExecContext(ctx, `INSERT INTO order_fulfillment_groups
			(group_no, order_no, seq, warehouse_id, status, reservation_state, freight_cent, version)
			VALUES (?, ?, ?, ?, 'PENDING', 'HELD', ?, 1)`,
			groupNo, orderNo, i+1, warehouseID, group.GetFreightCent()); err != nil {
			return err
		}
	}
	return nil
}

// setFulfillmentGroupStatus keeps the phase-1 group shadow aligned with the
// order lifecycle. It deliberately does not participate in any guard yet.
func setFulfillmentGroupStatus(ctx context.Context, tx *sql.Tx, orderNo, status string) error {
	reservation := "HELD"
	if status == "CANCELLED" {
		reservation = "RELEASED"
	}
	_, err := tx.ExecContext(ctx, `UPDATE order_fulfillment_groups
		SET status = ?, reservation_state = ?, version = version + 1, updated_at = NOW(3)
		WHERE order_no = ?`, status, reservation, orderNo)
	return err
}

func loadFulfillmentGroupsTx(ctx context.Context, tx *sql.Tx, orderNo string) ([]*fulfillmentGroup, error) {
	return loadFulfillmentGroups(ctx, tx, orderNo)
}

func loadFulfillmentGroups(ctx context.Context, q querier, orderNo string) ([]*fulfillmentGroup, error) {
	rows, err := q.QueryContext(ctx, `SELECT g.group_no, g.order_no, g.seq, g.warehouse_id, g.status,
		g.reservation_state, COALESCE(p.state, 'NONE'), g.freight_cent
		FROM order_fulfillment_groups g
		LEFT JOIN order_shipping_permits p
		  ON p.order_no = g.order_no AND p.fulfillment_group_no = g.group_no
		WHERE g.order_no = ? ORDER BY g.seq`, orderNo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := make([]*fulfillmentGroup, 0, 1)
	for rows.Next() {
		var g fulfillmentGroup
		if err := rows.Scan(&g.GroupNo, &g.OrderNo, &g.Seq, &g.WarehouseID, &g.Status,
			&g.ReservationState, &g.PermitState, &g.FreightCent); err != nil {
			return nil, err
		}
		groups = append(groups, &g)
	}
	return groups, rows.Err()
}
