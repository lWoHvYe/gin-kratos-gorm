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
	PaymentNo  string
	OrderNo    string
	UserID     int64
	AmountCent int64
	Channel    string
	Status     string
	PayURL     string
	ExpiresAt  time.Time
}

func (in *intent) toPB() *pb.Intent {
	return &pb.Intent{
		PaymentNo:     in.PaymentNo,
		OrderNo:       in.OrderNo,
		AmountCent:    in.AmountCent,
		Status:        in.Status,
		Channel:       in.Channel,
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

// refund is one refunds row.
type refund struct {
	RefundNo   string
	PaymentNo  string
	OrderNo    string
	AmountCent int64
	Reason     string
	Status     string
	Channel    string
}

type repo struct {
	db *sql.DB
}

func newRepo(database *sql.DB) *repo {
	return &repo{db: database}
}

const intentColumns = `payment_no, order_no, user_id, amount_cent, channel, status, pay_url, expires_at`

func scanIntent(row *sql.Row) (*intent, error) {
	var in intent
	if err := row.Scan(&in.PaymentNo, &in.OrderNo, &in.UserID, &in.AmountCent,
		&in.Channel, &in.Status, &in.PayURL, &in.ExpiresAt); err != nil {
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
		(payment_no, order_no, user_id, amount_cent, channel, status, pay_url, expires_at, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`,
		in.PaymentNo, in.OrderNo, in.UserID, in.AmountCent, in.Channel, in.Status, in.PayURL, in.ExpiresAt, now, now)
	return err
}

func (r *repo) updateIntentStatus(ctx context.Context, q querier, paymentNo, status string, now time.Time) error {
	res, err := q.ExecContext(ctx, `UPDATE payment_intents
		SET status = ?, version = version + 1, updated_at = ?
		WHERE payment_no = ?`, status, now, paymentNo)
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

const refundColumns = `refund_no, payment_no, order_no, amount_cent, reason, status, channel`

// refundByKey loads the refund identified by the Refund idempotency key
// (payment_no, amount_cent, reason).
func (r *repo) refundByKey(ctx context.Context, q querier, paymentNo string, amountCent int64, reason string) (*refund, error) {
	var ref refund
	err := q.QueryRowContext(ctx, `SELECT `+refundColumns+` FROM refunds
		WHERE payment_no = ? AND amount_cent = ? AND reason = ?`, paymentNo, amountCent, reason).
		Scan(&ref.RefundNo, &ref.PaymentNo, &ref.OrderNo, &ref.AmountCent, &ref.Reason, &ref.Status, &ref.Channel)
	if err != nil {
		return nil, err
	}
	return &ref, nil
}

func (r *repo) insertRefund(ctx context.Context, q querier, ref *refund, now time.Time) error {
	_, err := q.ExecContext(ctx, `INSERT INTO refunds
		(refund_no, payment_no, order_no, amount_cent, reason, status, channel, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`,
		ref.RefundNo, ref.PaymentNo, ref.OrderNo, ref.AmountCent, ref.Reason, ref.Status, ref.Channel, now, now)
	return err
}

// isDuplicateKey reports whether err is a MySQL unique-key violation.
func isDuplicateKey(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}
