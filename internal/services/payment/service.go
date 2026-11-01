// Package payment implements the mocked payment service.
//
// There is no real payment SDK: an intent is a row plus a mock:// pay URL and
// the channel reports the outcome through the Callback RPC. All money is moved
// as integer cents, never floats.
package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/db"
	"gin-kratos-gorm/internal/platform/dtm"
	"gin-kratos-gorm/internal/platform/id"
	"gin-kratos-gorm/internal/platform/outbox"
)

const (
	statusPending   = "PENDING"
	statusSucceeded = "SUCCEEDED"
	statusFailed    = "FAILED"
	statusRefunded  = "REFUNDED"

	defaultChannel = "mock"

	// intentTTL is how long a mock pay URL stays valid.
	intentTTL = 30 * time.Minute

	topicPaymentSucceeded = "shop.payment.succeeded.v1"
	topicPaymentFailed    = "shop.payment.failed.v1"
	topicPaymentRefunded  = "shop.payment.refunded.v1"

	eventSucceeded = "payment.succeeded"
	eventFailed    = "payment.failed"
	eventRefunded  = "payment.refunded"

	aggregatePayment = "payment"

	// orderCallTimeout bounds the MarkPaid call made after a success callback.
	orderCallTimeout = 3 * time.Second
)

var _ pb.PaymentServiceServer = (*Service)(nil)

// Service implements pb.PaymentServiceServer on top of database/sql.
type Service struct {
	pb.UnimplementedPaymentServiceServer

	db     *sql.DB
	orders pb.OrderServiceClient
	secret string
	log    *slog.Logger
	now    func() time.Time
	tcc    *tccConfig
}

// New builds the payment service. orders may be nil, in which case MarkPaid is
// skipped with a warning (useful for standalone unit tests). When secret is
// empty callback signatures are not verified, which is the mock default.
func New(database *sql.DB, orders pb.OrderServiceClient, secret string, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{db: database, orders: orders, secret: secret, log: logger, now: time.Now}
}

// CreateIntent opens a payment intent for an order. It is idempotent by
// (order_no, amount_cent, channel): a retry returns the intent created by the
// first call instead of opening a second one.
func (s *Service) CreateIntent(ctx context.Context, req *pb.CreateIntentRequest) (*pb.Intent, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	if req.GetAmountCent() <= 0 {
		return nil, apperr.Invalid("amount_cent must be greater than 0")
	}
	channel := strings.TrimSpace(req.GetChannel())
	if channel == "" {
		channel = defaultChannel
	}

	r := newRepo(s.db)
	existing, err := r.intentByKey(ctx, s.db, orderNo, req.GetAmountCent(), channel)
	switch {
	case err == nil:
		return existing.toPB(), nil
	case !errors.Is(err, sql.ErrNoRows):
		return nil, apperr.Internal("query payment intent", err)
	}

	now := s.now()
	paymentNo := id.Payment()
	in := &intent{
		PaymentNo:  paymentNo,
		OrderNo:    orderNo,
		UserID:     req.GetUserId(),
		AmountCent: req.GetAmountCent(),
		Channel:    channel,
		Status:     statusPending,
		PayURL:     payURL(paymentNo),
		ExpiresAt:  now.Add(intentTTL),
	}

	err = db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		return r.insertIntent(ctx, tx, in, now)
	})
	if err != nil {
		if isDuplicateKey(err) {
			// Lost a concurrent create race: return the winning intent.
			winner, qerr := r.intentByKey(ctx, s.db, orderNo, in.AmountCent, channel)
			if qerr != nil {
				return nil, apperr.Internal("load payment intent after conflict", qerr)
			}
			return winner.toPB(), nil
		}
		return nil, apperr.Internal("create payment intent", err)
	}
	s.log.Info("payment intent created", "payment_no", in.PaymentNo, "order_no", in.OrderNo,
		"amount_cent", in.AmountCent, "channel", in.Channel)
	return in.toPB(), nil
}

// GetIntent returns a payment intent by payment number.
func (s *Service) GetIntent(ctx context.Context, req *pb.GetIntentRequest) (*pb.Intent, error) {
	paymentNo := strings.TrimSpace(req.GetPaymentNo())
	if paymentNo == "" {
		return nil, apperr.Invalid("payment_no is required")
	}
	in, err := newRepo(s.db).intentByNo(ctx, s.db, paymentNo, false)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("payment intent not found")
	}
	if err != nil {
		return nil, apperr.Internal("get payment intent", err)
	}
	return in.toPB(), nil
}

// Callback applies a channel event to an intent. A (channel, event_id) pair is
// processed at most once; replaying it is a successful no-op that still
// reconciles the order, so a callback retried after a MarkPaid failure is able
// to finish the job.
func (s *Service) Callback(ctx context.Context, req *pb.CallbackRequest) (*pb.CallbackReply, error) {
	paymentNo := strings.TrimSpace(req.GetPaymentNo())
	channel := strings.TrimSpace(req.GetChannel())
	eventID := strings.TrimSpace(req.GetEventId())
	rawStatus := strings.TrimSpace(req.GetStatus())
	if paymentNo == "" || channel == "" || eventID == "" {
		return nil, apperr.Invalid("payment_no, channel and event_id are required")
	}
	if req.GetAmountCent() <= 0 {
		return nil, apperr.Invalid("amount_cent must be greater than 0")
	}
	if err := s.verifySignature(paymentNo, eventID, req.GetAmountCent(), rawStatus, req.GetSignature()); err != nil {
		return nil, err
	}
	cbStatus, err := normalizeStatus(rawStatus)
	if err != nil {
		return nil, err
	}

	now := s.now()
	r := newRepo(s.db)
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		duplicate, err := r.insertCallback(ctx, tx, &callback{
			Channel:     channel,
			EventID:     eventID,
			PaymentNo:   paymentNo,
			AmountCent:  req.GetAmountCent(),
			Status:      cbStatus,
			ProcessedAt: now,
		})
		if err != nil {
			return err
		}
		if duplicate {
			// A replay must not silently apply to a different payment.
			stored, err := r.callbackPaymentNo(ctx, tx, channel, eventID)
			if err != nil {
				return err
			}
			if stored != paymentNo {
				return apperr.Conflict("event_id was already used for another payment")
			}
			return nil
		}

		in, err := r.intentByNo(ctx, tx, paymentNo, true)
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound("payment intent not found")
		}
		if err != nil {
			return err
		}
		if in.AmountCent != req.GetAmountCent() {
			return apperr.Failed(fmt.Sprintf("callback amount %d does not match intent amount %d",
				req.GetAmountCent(), in.AmountCent))
		}
		next, changed, ok := advance(in.Status, cbStatus)
		if !ok {
			return apperr.Failed(fmt.Sprintf("payment intent in status %s cannot accept a %s callback", in.Status, cbStatus))
		}
		if !changed {
			return nil
		}
		if err := r.updateIntentStatus(ctx, tx, in.PaymentNo, next, now); err != nil {
			return err
		}
		in.Status = next
		return outbox.Append(ctx, tx, paymentEvent(in, next))
	})
	if err != nil {
		return nil, encodeError("process payment callback", err)
	}

	// Reload so that replayed events also re-run the order reconciliation.
	in, err := r.intentByNo(ctx, s.db, paymentNo, false)
	if err != nil {
		return nil, encodeError("reload payment intent", err)
	}
	if cbStatus == statusSucceeded && in.Status == statusSucceeded {
		if err := s.markPaidTCC(ctx, in); err != nil {
			return nil, err
		}
	}
	return &pb.CallbackReply{Ok: true}, nil
}

// Refund refunds a successful payment. It is idempotent by
// (payment_no, amount_cent, reason) and emits shop.payment.refunded.v1.
func (s *Service) Refund(ctx context.Context, req *pb.RefundRequest) (*pb.Empty, error) {
	paymentNo := strings.TrimSpace(req.GetPaymentNo())
	if paymentNo == "" {
		return nil, apperr.Invalid("payment_no is required")
	}
	if req.GetAmountCent() <= 0 {
		return nil, apperr.Invalid("amount_cent must be greater than 0")
	}
	amount := req.GetAmountCent()
	reason := strings.TrimSpace(req.GetReason())

	now := s.now()
	r := newRepo(s.db)
	var refundNo string
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := r.refundByKey(ctx, tx, paymentNo, amount, reason); err == nil {
			return nil // already refunded with this exact key
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		in, err := r.intentByNo(ctx, tx, paymentNo, true)
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound("payment intent not found")
		}
		if err != nil {
			return err
		}
		if in.Status != statusSucceeded {
			return apperr.Failed(fmt.Sprintf("payment intent is %s; only SUCCEEDED payments can be refunded", in.Status))
		}
		if amount > in.AmountCent {
			return apperr.Invalid("refund amount %d exceeds payment amount %d", amount, in.AmountCent)
		}

		ref := &refund{
			RefundNo:   id.New("RFD"),
			PaymentNo:  in.PaymentNo,
			OrderNo:    in.OrderNo,
			AmountCent: amount,
			Reason:     reason,
			Status:     statusRefunded,
			Channel:    in.Channel,
		}
		if err := r.insertRefund(ctx, tx, ref, now); err != nil {
			if isDuplicateKey(err) {
				return nil // a concurrent identical refund won the race
			}
			return err
		}
		if err := r.updateIntentStatus(ctx, tx, in.PaymentNo, statusRefunded, now); err != nil {
			return err
		}
		refundNo = ref.RefundNo
		return outbox.Append(ctx, tx, outbox.Event{
			Topic:         topicPaymentRefunded,
			AggregateType: aggregatePayment,
			AggregateID:   in.PaymentNo,
			EventType:     eventRefunded,
			Payload: map[string]any{
				"refund_no":   ref.RefundNo,
				"payment_no":  in.PaymentNo,
				"order_no":    in.OrderNo,
				"user_id":     in.UserID,
				"amount_cent": amount,
				"reason":      reason,
				"channel":     in.Channel,
				"status":      statusRefunded,
			},
		})
	})
	if err != nil {
		return nil, encodeError("refund payment", err)
	}
	if refundNo != "" {
		s.log.Info("payment refunded", "payment_no", paymentNo, "refund_no", refundNo, "amount_cent", amount)
	}
	return &pb.Empty{}, nil
}

// inTx runs fn inside a DTM branch transaction when the RPC arrives as a DTM
// branch, otherwise inside a local transaction. Channel callbacks are ordinary
// top-level calls and take the local path.
func (s *Service) inTx(ctx context.Context, fn func(*sql.Tx) error) error {
	if dtm.InBranch(ctx) {
		return dtm.Branch(ctx, s.db, fn)
	}
	return db.WithTx(ctx, s.db, fn)
}

// markPaid tells the order service that the payment succeeded. The call is
// made after the intent transaction commits: if it fails the callback returns
// an error and the channel retry re-runs it from the SUCCEEDED intent.
// verifySignature checks the optional callback HMAC. Verification is skipped
// when no secret is configured, which is the mock default.
func (s *Service) verifySignature(paymentNo, eventID string, amountCent int64, rawStatus, signature string) error {
	if strings.TrimSpace(s.secret) == "" {
		return nil
	}
	expected := Signature(s.secret, paymentNo, eventID, amountCent, rawStatus)
	provided := strings.ToLower(strings.TrimSpace(signature))
	if provided == "" || !hmac.Equal([]byte(provided), []byte(expected)) {
		return apperr.Unauthenticated("invalid payment callback signature")
	}
	return nil
}

// Signature is the callback signature expected when PAYMENT_SECRET is set:
// hex(HMAC-SHA256(secret, "payment_no|event_id|amount_cent|status")). The
// status participating in the signature is the raw value sent by the channel,
// before normalization.
func Signature(secret, paymentNo, eventID string, amountCent int64, status string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(paymentNo + "|" + eventID + "|" + strconv.FormatInt(amountCent, 10) + "|" + status))
	return hex.EncodeToString(mac.Sum(nil))
}

// normalizeStatus maps channel vocabulary onto the intent status set.
func normalizeStatus(raw string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case statusSucceeded, "SUCCESS", "PAID":
		return statusSucceeded, nil
	case statusFailed, "CLOSED", "CANCELLED", "CANCELED":
		return statusFailed, nil
	default:
		return "", apperr.Invalid("unsupported callback status %q", raw)
	}
}

// advance applies a normalized channel status to the current intent status.
// It returns the next status, whether the intent changed, and whether the
// transition is valid. Late successes are honoured; late failures never
// downgrade an already paid (or refunded) intent.
func advance(current, cbStatus string) (next string, changed, ok bool) {
	switch cbStatus {
	case statusSucceeded:
		switch current {
		case statusPending, statusFailed:
			return statusSucceeded, true, true
		case statusSucceeded, statusRefunded:
			return current, false, true
		}
	case statusFailed:
		switch current {
		case statusPending:
			return statusFailed, true, true
		case statusFailed, statusSucceeded, statusRefunded:
			return current, false, true
		}
	}
	return "", false, false
}

// paymentEvent builds the outbox event for a settled intent.
func paymentEvent(in *intent, status string) outbox.Event {
	topic, eventType := topicPaymentSucceeded, eventSucceeded
	if status == statusFailed {
		topic, eventType = topicPaymentFailed, eventFailed
	}
	return outbox.Event{
		Topic:         topic,
		AggregateType: aggregatePayment,
		AggregateID:   in.PaymentNo,
		EventType:     eventType,
		Payload: map[string]any{
			"payment_no":  in.PaymentNo,
			"order_no":    in.OrderNo,
			"user_id":     in.UserID,
			"amount_cent": in.AmountCent,
			"channel":     in.Channel,
			"status":      status,
		},
	}
}

// encodeError keeps domain errors intact and wraps the rest as INTERNAL.
func encodeError(msg string, err error) error {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae
	}
	return apperr.Internal(msg, err)
}

func payURL(paymentNo string) string {
	return "mock://pay/" + paymentNo
}
