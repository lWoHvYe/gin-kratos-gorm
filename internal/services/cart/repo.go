package cart

import (
	"context"
	"database/sql"
	"errors"

	pb "gin-kratos-gorm/api/gen/mall/v1"
)

// querier is implemented by both *sql.DB (reads) and *sql.Tx (writes), so the
// query helpers can run either outside or inside a transaction.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type repo struct {
	db *sql.DB
}

func newRepo(database *sql.DB) *repo {
	return &repo{db: database}
}

// ensureCart returns the id of the user's cart, creating an ACTIVE cart when
// the user does not have one yet. carts.user_id is unique, so a concurrent
// first insert resolves to the same row.
func (r *repo) ensureCart(ctx context.Context, q querier, userID int64) (int64, error) {
	res, err := q.ExecContext(ctx, `INSERT INTO carts (user_id, status) VALUES (?, 'ACTIVE')
		ON DUPLICATE KEY UPDATE id = LAST_INSERT_ID(id)`, userID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// cartID looks up the user's cart. It returns sql.ErrNoRows when the user has
// never added an item.
func (r *repo) cartID(ctx context.Context, q querier, userID int64) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `SELECT id FROM carts WHERE user_id = ?`, userID).Scan(&id)
	return id, err
}

func (r *repo) items(ctx context.Context, q querier, cartID int64) ([]*pb.CartItem, error) {
	rows, err := q.QueryContext(ctx, `SELECT sku_id, name, price_cent, quantity, selected
		FROM cart_items WHERE cart_id = ? ORDER BY id`, cartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*pb.CartItem
	for rows.Next() {
		var (
			item     pb.CartItem
			selected bool
		)
		if err := rows.Scan(&item.SkuId, &item.Name, &item.PriceCent, &item.Quantity, &selected); err != nil {
			return nil, err
		}
		item.Selected = selected
		items = append(items, &item)
	}
	return items, rows.Err()
}

// itemExists reports whether the cart already holds the given sku.
func (r *repo) itemExists(ctx context.Context, q querier, cartID, skuID int64) (bool, error) {
	var id int64
	err := q.QueryRowContext(ctx, `SELECT id FROM cart_items WHERE cart_id = ? AND sku_id = ?`, cartID, skuID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// upsertItem inserts the line or merges into the existing (cart_id, sku_id)
// row: quantity is accumulated and name/price are refreshed from catalog.
func (r *repo) upsertItem(ctx context.Context, q querier, cartID int64, item *pb.CartItem) error {
	_, err := q.ExecContext(ctx, `INSERT INTO cart_items (cart_id, sku_id, name, price_cent, quantity, selected)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			name = VALUES(name),
			price_cent = VALUES(price_cent),
			quantity = quantity + VALUES(quantity),
			selected = VALUES(selected)`,
		cartID, item.GetSkuId(), item.GetName(), item.GetPriceCent(), item.GetQuantity(), item.GetSelected())
	return err
}

func (r *repo) updateItem(ctx context.Context, q querier, cartID, skuID int64, quantity int32, selected bool) error {
	_, err := q.ExecContext(ctx, `UPDATE cart_items SET quantity = ?, selected = ? WHERE cart_id = ? AND sku_id = ?`,
		quantity, selected, cartID, skuID)
	return err
}

// deleteItem removes a line and reports how many rows were deleted (0 when the
// sku was not in the cart).
func (r *repo) deleteItem(ctx context.Context, q querier, cartID, skuID int64) (int64, error) {
	res, err := q.ExecContext(ctx, `DELETE FROM cart_items WHERE cart_id = ? AND sku_id = ?`, cartID, skuID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
