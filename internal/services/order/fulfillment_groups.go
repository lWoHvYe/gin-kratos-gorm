package order

import (
	"context"
	"database/sql"
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
	FreightCent      int64
}

func insertFulfillmentGroup(ctx context.Context, tx *sql.Tx, orderNo string, freightCent int64) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO order_fulfillment_groups
		(group_no, order_no, seq, warehouse_id, status, reservation_state, freight_cent, version)
		VALUES (?, ?, 1, ?, 'PENDING', 'HELD', ?, 1)`,
		defaultGroupNo, orderNo, defaultWarehouseID, freightCent)
	return err
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
		WHERE order_no = ? AND group_no = ?`, status, reservation, orderNo, defaultGroupNo)
	return err
}

func loadFulfillmentGroups(ctx context.Context, q querier, orderNo string) ([]*fulfillmentGroup, error) {
	rows, err := q.QueryContext(ctx, `SELECT group_no, order_no, seq, warehouse_id, status,
		reservation_state, freight_cent FROM order_fulfillment_groups
		WHERE order_no = ? ORDER BY seq`, orderNo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := make([]*fulfillmentGroup, 0, 1)
	for rows.Next() {
		var g fulfillmentGroup
		if err := rows.Scan(&g.GroupNo, &g.OrderNo, &g.Seq, &g.WarehouseID, &g.Status,
			&g.ReservationState, &g.FreightCent); err != nil {
			return nil, err
		}
		groups = append(groups, &g)
	}
	return groups, rows.Err()
}
