package pricing

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gin-kratos-gorm/internal/platform/apperr"
)

const (
	couponAvailable = "AVAILABLE"
	couponLocked    = "LOCKED"
	couponUsed      = "USED"
)

// coupon is one row of the coupons table.
type coupon struct {
	ID            int64
	Code          string
	UserID        int64
	AmountCent    int64
	MinAmountCent int64
	Status        string
	ExpireAt      time.Time
	OrderNo       string
}

// quoteLine is one priced line of a quote.
type quoteLine struct {
	SkuID        int64
	Name         string
	PriceCent    int64
	Quantity     int32
	DiscountCent int64
}

// quote is a priced basket with a fixed lifetime.
type quote struct {
	QuoteID      string
	UserID       int64
	TotalCent    int64
	DiscountCent int64
	FreightCent  int64
	PayCent      int64
	CouponID     string
	ExpiresAt    time.Time
	Lines        []quoteLine
}

// querier is satisfied by both *sql.DB and *sql.Tx.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

const couponColumns = `id, code, user_id, amount_cent, min_amount_cent, status, expire_at, order_no`

func scanCoupon(scan func(dest ...any) error) (*coupon, error) {
	var c coupon
	if err := scan(&c.ID, &c.Code, &c.UserID, &c.AmountCent, &c.MinAmountCent, &c.Status, &c.ExpireAt, &c.OrderNo); err != nil {
		return nil, err
	}
	return &c, nil
}

func couponByCode(ctx context.Context, q querier, code string, userID int64) (*coupon, error) {
	row := q.QueryRowContext(ctx, `SELECT `+couponColumns+` FROM coupons
		WHERE code = ? AND (user_id = 0 OR user_id = ?)
		ORDER BY user_id = 0, id LIMIT 1`, code, userID)
	return scanCoupon(row.Scan)
}

func couponByID(ctx context.Context, q querier, id, userID int64) (*coupon, error) {
	row := q.QueryRowContext(ctx, `SELECT `+couponColumns+` FROM coupons
		WHERE id = ? AND (user_id = 0 OR user_id = ?)
		ORDER BY user_id = 0, id LIMIT 1`, id, userID)
	return scanCoupon(row.Scan)
}

// resolveCoupon accepts either a coupon code or a numeric coupon id. An empty
// reference resolves to no coupon.
func resolveCoupon(ctx context.Context, q querier, ref string, userID int64) (*coupon, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, nil
	}
	c, err := couponByCode(ctx, q, ref, userID)
	if err == nil {
		return c, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.Internal("query coupon by code", err)
	}
	if id, perr := strconv.ParseInt(ref, 10, 64); perr == nil && id > 0 {
		c, err = couponByID(ctx, q, id, userID)
		if err == nil {
			return c, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.Internal("query coupon by id", err)
		}
	}
	return nil, apperr.NotFound(fmt.Sprintf("coupon %q not found", ref))
}

func getCouponForUpdate(ctx context.Context, q querier, id int64) (*coupon, error) {
	row := q.QueryRowContext(ctx, `SELECT `+couponColumns+` FROM coupons WHERE id = ? FOR UPDATE`, id)
	c, err := scanCoupon(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("coupon not found")
	}
	if err != nil {
		return nil, apperr.Internal("load coupon", err)
	}
	return c, nil
}

func insertQuote(ctx context.Context, tx *sql.Tx, q *quote) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO quotes
		(quote_id, user_id, total_cent, discount_cent, freight_cent, pay_cent, coupon_id, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		q.QuoteID, q.UserID, q.TotalCent, q.DiscountCent, q.FreightCent, q.PayCent, q.CouponID, q.ExpiresAt)
	if err != nil {
		return apperr.Internal("insert quote", err)
	}
	return nil
}

func insertQuoteLines(ctx context.Context, tx *sql.Tx, q *quote) error {
	for _, line := range q.Lines {
		if _, err := tx.ExecContext(ctx, `INSERT INTO quote_lines
			(quote_id, sku_id, name, price_cent, quantity, discount_cent)
			VALUES (?, ?, ?, ?, ?, ?)`,
			q.QuoteID, line.SkuID, line.Name, line.PriceCent, line.Quantity, line.DiscountCent); err != nil {
			return apperr.Internal("insert quote line", err)
		}
	}
	return nil
}

func loadQuote(ctx context.Context, q querier, quoteID string) (*quote, error) {
	var (
		out       quote
		couponID  string
		expiresAt time.Time
	)
	err := q.QueryRowContext(ctx, `SELECT quote_id, user_id, total_cent, discount_cent, freight_cent, pay_cent, coupon_id, expires_at
		FROM quotes WHERE quote_id = ?`, quoteID).
		Scan(&out.QuoteID, &out.UserID, &out.TotalCent, &out.DiscountCent, &out.FreightCent, &out.PayCent, &couponID, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("quote not found")
	}
	if err != nil {
		return nil, apperr.Internal("load quote", err)
	}
	out.CouponID = couponID
	out.ExpiresAt = expiresAt
	return &out, nil
}

func loadQuoteLines(ctx context.Context, q querier, quoteID string) ([]quoteLine, error) {
	rows, err := q.QueryContext(ctx, `SELECT sku_id, name, price_cent, quantity, discount_cent
		FROM quote_lines WHERE quote_id = ? ORDER BY sku_id`, quoteID)
	if err != nil {
		return nil, apperr.Internal("load quote lines", err)
	}
	defer rows.Close()
	var lines []quoteLine
	for rows.Next() {
		var line quoteLine
		if err := rows.Scan(&line.SkuID, &line.Name, &line.PriceCent, &line.Quantity, &line.DiscountCent); err != nil {
			return nil, apperr.Internal("scan quote line", err)
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal("iterate quote lines", err)
	}
	return lines, nil
}

// lockCoupon moves AVAILABLE -> LOCKED for orderNo. Re-locking the same order
// is a no-op so DTM retries are safe.
func lockCoupon(ctx context.Context, tx *sql.Tx, couponID int64, orderNo string) (bool, error) {
	res, err := tx.ExecContext(ctx, `UPDATE coupons SET status = ?, order_no = ?, version = version + 1
		WHERE id = ? AND status = ?`, couponLocked, orderNo, couponID, couponAvailable)
	if err != nil {
		return false, apperr.Internal("lock coupon", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, apperr.Internal("update coupon", err)
	}
	if n > 0 {
		return true, nil
	}
	cur, err := getCouponForUpdate(ctx, tx, couponID)
	if err != nil {
		return false, err
	}
	switch cur.Status {
	case couponLocked:
		if cur.OrderNo == orderNo {
			return false, nil
		}
		return false, apperr.Conflict("coupon is locked by another order")
	case couponUsed:
		return false, apperr.Failed("coupon is already used")
	default:
		return false, apperr.Failed("coupon is not available")
	}
}

// unlockCoupon moves LOCKED -> AVAILABLE. Unlocking an already available
// coupon is a no-op so DTM compensation retries are safe.
func unlockCoupon(ctx context.Context, tx *sql.Tx, couponID int64, orderNo string) (bool, error) {
	res, err := tx.ExecContext(ctx, `UPDATE coupons SET status = ?, order_no = '', version = version + 1
		WHERE id = ? AND status = ? AND order_no = ?`, couponAvailable, couponID, couponLocked, orderNo)
	if err != nil {
		return false, apperr.Internal("unlock coupon", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, apperr.Internal("update coupon", err)
	}
	if n > 0 {
		return true, nil
	}
	cur, err := getCouponForUpdate(ctx, tx, couponID)
	if err != nil {
		return false, err
	}
	switch cur.Status {
	case couponAvailable:
		return false, nil
	case couponUsed:
		return false, apperr.Failed("coupon is already used")
	case couponLocked:
		if cur.OrderNo == orderNo {
			return false, nil
		}
		return false, apperr.Conflict("coupon is locked by another order")
	default:
		return false, apperr.Failed(fmt.Sprintf("coupon cannot be unlocked from status %s", cur.Status))
	}
}

// consumeCoupon moves LOCKED -> USED. Consuming an already used coupon is a
// no-op so retries are safe.
func consumeCoupon(ctx context.Context, tx *sql.Tx, couponID int64, orderNo string) (bool, error) {
	res, err := tx.ExecContext(ctx, `UPDATE coupons SET status = ?, version = version + 1
		WHERE id = ? AND status = ? AND order_no = ?`, couponUsed, couponID, couponLocked, orderNo)
	if err != nil {
		return false, apperr.Internal("consume coupon", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, apperr.Internal("update coupon", err)
	}
	if n > 0 {
		return true, nil
	}
	cur, err := getCouponForUpdate(ctx, tx, couponID)
	if err != nil {
		return false, err
	}
	switch cur.Status {
	case couponUsed:
		if cur.OrderNo == orderNo {
			return false, nil
		}
		return false, apperr.Conflict("coupon is used by another order")
	case couponAvailable:
		return false, apperr.Failed("coupon is not locked")
	case couponLocked:
		if cur.OrderNo == orderNo {
			return false, nil
		}
		return false, apperr.Conflict("coupon is locked by another order")
	default:
		return false, apperr.Failed(fmt.Sprintf("coupon cannot be consumed from status %s", cur.Status))
	}
}
