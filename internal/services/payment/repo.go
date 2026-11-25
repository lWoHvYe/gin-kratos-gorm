package payment

import (
	"context"
	"database/sql"
	"errors"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"

	"github.com/go-sql-driver/mysql"
)

// errNotUpdated is returned when a write that must touch exactly one row
// touched none. It normally means the row disappeared between the locking
// read and the update.
var errNotUpdated = errors.New("payment: row was not updated")

// querier is implemented by both *sql.DB (reads) and *sql.Tx (writes), so the
// repository helpers can run outside or inside a transaction.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// intent is one payment_intents row.
type intent struct {
	PaymentNo     string
	OrderNo       string
	UserID        int64
	AmountCent    int64
	CapturedCent  int64
	RefundedCent  int64
	Channel       string
	PaymentMethod string
	Status        string
	PayURL        string
	ExpiresAt     time.Time
}

func (in *intent) toPB() *pb.Intent {
	return &pb.Intent{
		PaymentNo:     in.PaymentNo,
		OrderNo:       in.OrderNo,
		AmountCent:    in.AmountCent,
		CapturedCent:  in.CapturedCent,
		RefundedCent:  in.RefundedCent,
		Status:        in.Status,
		Channel:       in.Channel,
		PaymentMethod: in.PaymentMethod,
		PayUrl:        in.PayURL,
		ExpiresAtUnix: in.ExpiresAt.Unix(),
	}
}

// callback is one payment_callbacks row.
type callback struct {
	Channel     string
	EventID     string
	PaymentNo   string
	AmountCent  int64
	Status      string
	ProcessedAt time.Time
}

// refund is one refunds row. IdempotencyKey is the caller's refund_id
// (C-04 §2.8): Payment never introduces a second refund identity column.
// BatchID is the atomic RefundBatch that wrote the row, or "" for a legacy
// single Refund.
type refund struct {
	RefundNo       string
	PaymentNo      string
	OrderNo        string
	AmountCent     int64
	Reason         string
	Status         string
	Channel        string
	IdempotencyKey string
	BatchID        string
	CreatedAt      time.Time
}

// toPB exposes a persisted refund fact. RefundID is the stored
// idempotency_key: callers must key lazy reconciliation on refund_id, never on
// refund_no (C-04 §1.2).
func (r *refund) toPB() *pb.Refund {
	return &pb.Refund{
		RefundNo:      r.RefundNo,
		PaymentNo:     r.PaymentNo,
		OrderNo:       r.OrderNo,
		AmountCent:    r.AmountCent,
		Reason:        r.Reason,
		Status:        r.Status,
		CreatedAtUnix: r.CreatedAt.Unix(),
		RefundId:      r.IdempotencyKey,
	}
}

// refundBatchRecord is one payment_refund_batches row: the persistent
// idempotency ledger for RefundBatch. ItemResults is the JSON array of
// refundBatchItemResult (refund_id/refund_no/status/amount_cent) returned to
// the caller, so a replay returns the same reply.
type refundBatchRecord struct {
	PaymentNo        string
	BatchID          string
	ItemsFingerprint string
	Status           string
	RefundedCent     int64
	Reason           string
	ItemResults      []byte
	CreatedAt        time.Time
}

type repo struct {
	db *sql.DB
}

func newRepo(database *sql.DB) *repo {
	return &repo{db: database}
}

const intentColumns = `payment_no, order_no, user_id, amount_cent, captured_cent, refunded_cent, channel, payment_method, status, pay_url, expires_at`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanIntent(row rowScanner) (*intent, error) {
	var in intent
	if err := row.Scan(&in.PaymentNo, &in.OrderNo, &in.UserID, &in.AmountCent,
		&in.CapturedCent, &in.RefundedCent, &in.Channel, &in.PaymentMethod,
		&in.Status, &in.PayURL, &in.ExpiresAt); err != nil {
		return nil, err
	}
	return &in, nil
}

// intentByNo loads a payment intent by its payment number. When forUpdate is
// set the row is locked for the surrounding transaction.
func (r *repo) intentByNo(ctx context.Context, q querier, paymentNo string, forUpdate bool) (*intent, error) {
	query := `SELECT ` + intentColumns + ` FROM payment_intents WHERE payment_no = ?`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	return scanIntent(q.QueryRowContext(ctx, query, paymentNo))
}

// intentByKey loads the intent identified by the CreateIntent idempotency key
// (order_no, amount_cent, channel). It returns sql.ErrNoRows when no intent
// exists yet.
func (r *repo) intentByKey(ctx context.Context, q querier, orderNo string, amountCent int64, channel string) (*intent, error) {
	return scanIntent(q.QueryRowContext(ctx, `SELECT `+intentColumns+` FROM payment_intents
		WHERE order_no = ? AND amount_cent = ? AND channel = ?`, orderNo, amountCent, channel))
}

func (r *repo) insertIntent(ctx context.Context, q querier, in *intent, now time.Time) error {
	_, err := q.ExecContext(ctx, `INSERT INTO payment_intents
		(payment_no, order_no, user_id, amount_cent, captured_cent, refunded_cent, channel, payment_method, status, pay_url, expires_at, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`,
		in.PaymentNo, in.OrderNo, in.UserID, in.AmountCent, in.CapturedCent, in.RefundedCent, in.Channel, in.PaymentMethod, in.Status, in.PayURL, in.ExpiresAt, now, now)
	return err
}

func (r *repo) updateIntentStatus(ctx context.Context, q querier, paymentNo, status string, now time.Time) error {
	res, err := q.ExecContext(ctx, `UPDATE payment_intents
		SET status = ?, captured_cent = CASE WHEN ? = 'SUCCEEDED' THEN amount_cent ELSE captured_cent END,
		    payment_method = CASE WHEN ? = 'SUCCEEDED' AND payment_method = '' THEN channel ELSE payment_method END,
		    version = version + 1, updated_at = ?
		WHERE payment_no = ?`, status, status, status, now, paymentNo)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return errNotUpdated
	}
	return nil
}

// insertCallback records a channel event. The (channel, event_id) unique key
// makes callbacks idempotent: a duplicate delivery reports duplicate=true
// instead of failing the transaction.
func (r *repo) insertCallback(ctx context.Context, q querier, cb *callback) (duplicate bool, err error) {
	_, err = q.ExecContext(ctx, `INSERT INTO payment_callbacks
		(channel, event_id, payment_no, amount_cent, status, processed_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		cb.Channel, cb.EventID, cb.PaymentNo, cb.AmountCent, cb.Status, cb.ProcessedAt, cb.ProcessedAt, cb.ProcessedAt)
	if err != nil {
		if isDuplicateKey(err) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

// callbackPaymentNo returns the payment recorded for a channel event.
func (r *repo) callbackPaymentNo(ctx context.Context, q querier, channel, eventID string) (string, error) {
	var paymentNo string
	err := q.QueryRowContext(ctx, `SELECT payment_no FROM payment_callbacks
		WHERE channel = ? AND event_id = ?`, channel, eventID).Scan(&paymentNo)
	return paymentNo, err
}

const refundColumns = `refund_no, payment_no, order_no, amount_cent, reason, status, channel, COALESCE(idempotency_key, ''), COALESCE(batch_id, ''), created_at`

func (r *repo) refundByKey(ctx context.Context, q querier, paymentNo, idempotencyKey string) (*refund, error) {
	var ref refund
	err := q.QueryRowContext(ctx, `SELECT `+refundColumns+` FROM refunds
		WHERE payment_no = ? AND idempotency_key = ?`, paymentNo, idempotencyKey).
		Scan(&ref.RefundNo, &ref.PaymentNo, &ref.OrderNo, &ref.AmountCent, &ref.Reason, &ref.Status, &ref.Channel, &ref.IdempotencyKey, &ref.BatchID, &ref.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &ref, nil
}

func (r *repo) insertRefund(ctx context.Context, q querier, ref *refund, now time.Time) error {
	_, err := q.ExecContext(ctx, `INSERT INTO refunds
		(refund_no, payment_no, order_no, amount_cent, reason, status, channel, idempotency_key, batch_id, completed_at, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`,
		ref.RefundNo, ref.PaymentNo, ref.OrderNo, ref.AmountCent, ref.Reason, ref.Status, ref.Channel, ref.IdempotencyKey, ref.BatchID, now, now, now)
	return err
}

const refundBatchColumns = `payment_no, batch_id, items_fingerprint, status, refunded_cent, reason, item_results, created_at`

// refundBatchByKey loads the persistent batch ledger row. When forUpdate is
// set the row is locked; RefundBatch also locks the intent row first, so the
// two locks always follow the same order.
func (r *repo) refundBatchByKey(ctx context.Context, q querier, paymentNo, batchID string, forUpdate bool) (*refundBatchRecord, error) {
	query := `SELECT ` + refundBatchColumns + ` FROM payment_refund_batches
		WHERE payment_no = ? AND batch_id = ?`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var rec refundBatchRecord
	err := q.QueryRowContext(ctx, query, paymentNo, batchID).
		Scan(&rec.PaymentNo, &rec.BatchID, &rec.ItemsFingerprint, &rec.Status,
			&rec.RefundedCent, &rec.Reason, &rec.ItemResults, &rec.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func (r *repo) insertRefundBatch(ctx context.Context, q querier, rec *refundBatchRecord, now time.Time) error {
	_, err := q.ExecContext(ctx, `INSERT INTO payment_refund_batches
		(payment_no, batch_id, items_fingerprint, status, refunded_cent, reason, item_results, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.PaymentNo, rec.BatchID, rec.ItemsFingerprint, rec.Status, rec.RefundedCent,
		rec.Reason, string(rec.ItemResults), now, now)
	return err
}

func (r *repo) updateIntentRefund(ctx context.Context, q querier, paymentNo string, refundedCent int64, status string, now time.Time) error {
	res, err := q.ExecContext(ctx, `UPDATE payment_intents
		SET refunded_cent = ?, status = ?, version = version + 1, updated_at = ?
		WHERE payment_no = ?`, refundedCent, status, now, paymentNo)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return errNotUpdated
	}
	return nil
}

// isDuplicateKey reports whether err is a MySQL unique-key violation.
func isDuplicateKey(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

// intentByOrder returns the latest settled intent for an order. Aftersales
// refunds are order-keyed and do not need the payment number.
func (r *repo) intentByOrder(ctx context.Context, q querier, orderNo string) (*intent, error) {
	rows, err := r.intentsByOrder(ctx, q, orderNo)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	return rows[len(rows)-1], nil
}

// allIntentsByOrder returns every intent for an order, regardless of status.
// It is the read-only source for Order's historical effective-payment
// resolution; it never mutates state.
func (r *repo) allIntentsByOrder(ctx context.Context, q querier, orderNo string) ([]*intent, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+intentColumns+` FROM payment_intents
		WHERE order_no = ? ORDER BY id ASC`, orderNo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*intent, 0, 2)
	for rows.Next() {
		in, err := scanIntent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, in)
	}
	return out, rows.Err()
}

// intentsByOrder returns every settled intent for an order in deterministic
// creation order. A single order may have multiple captured intents (retries or
// channel fallbacks); refund allocation and captured facts must aggregate them.
func (r *repo) intentsByOrder(ctx context.Context, q querier, orderNo string) ([]*intent, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+intentColumns+` FROM payment_intents
		WHERE order_no = ? AND status IN (?, ?, ?) ORDER BY id ASC`,
		orderNo, statusSucceeded, statusPartiallyRefunded, statusRefunded)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*intent, 0, 2)
	for rows.Next() {
		in, err := scanIntent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, in)
	}
	return out, rows.Err()
}
