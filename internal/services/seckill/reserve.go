package seckill

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/metrics"

	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
)

const (
	qualificationTTL = 5 * time.Minute

	reserveAccepted  = int64(0)
	reserveSoldOut   = int64(1)
	reserveExisting  = int64(2)
	reserveNotWarm   = int64(-1)
	reserveNotActive = int64(-2)
)

var (
	// reserveScript is the complete admission decision. All keys share the
	// {activity_id} hash tag, so it is Redis Cluster compatible.
	reserveScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then
  return -1
end
local status = redis.call('HGET', KEYS[1], 'status')
if status ~= 'ACTIVE' then
  return -2
end
if redis.call('HGET', KEYS[3], ARGV[1]) then
  return 2
end
local stock = tonumber(redis.call('GET', KEYS[2]) or '-1')
if stock < 0 then
  return -1
end
if stock <= 0 then
  return 1
end
redis.call('DECR', KEYS[2])
redis.call('HSET', KEYS[3], ARGV[1], ARGV[2])
redis.call('HSET', KEYS[4], ARGV[2], 'ISSUED')
redis.call('EXPIRE', KEYS[3], ARGV[3])
redis.call('EXPIRE', KEYS[4], ARGV[3])
return 0
`)
	// compensateScript only releases a qualification created by the same
	// token. A replay cannot decrement another request's stock.
	compensateScript = redis.NewScript(`
local current = redis.call('HGET', KEYS[1], ARGV[1])
if current == ARGV[2] then
  redis.call('HDEL', KEYS[1], ARGV[1])
  redis.call('HDEL', KEYS[2], ARGV[2])
  if redis.call('EXISTS', KEYS[4]) == 1 then
    redis.call('INCR', KEYS[3])
  end
  return 1
end
return 0
`)
)

type qualification struct {
	activityID    int64
	userID        int64
	addressID     int64
	receiverName  string
	receiverPhone string
	receiverAddr  string
	tokenHash     string
	status        string
	orderNo       string
	paymentNo     string
	expiresAt     time.Time
	publishedAt   sql.NullTime
}

func (q *qualification) toPB(rawToken string) *pb.SeckillQualification {
	return &pb.SeckillQualification{
		Token: rawToken, ActivityId: q.activityID, UserId: q.userID,
		Status: q.status, OrderNo: q.orderNo, PaymentNo: q.paymentNo, ExpiresAtUnix: q.expiresAt.Unix(),
		AddressId: q.addressID, ReceiverName: q.receiverName,
		ReceiverPhone: q.receiverPhone, ReceiverAddress: q.receiverAddr,
	}
}

func (s *Service) tokenFor(a *activity, userID int64) string {
	mac := hmac.New(sha256.New, s.tokenSecret)
	_, _ = fmt.Fprintf(mac, "%d:%d:%d", a.id, userID, a.version)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Reserve performs Redis admission and persists the qualification. The Redis
// script is the only place that decrements stock; a DB failure compensates the
// exact token, never another request's reservation.
func (s *Service) Reserve(ctx context.Context, req *pb.ReserveSeckillRequest) (*pb.SeckillQualification, error) {
	if req.GetActivityId() <= 0 || req.GetUserId() <= 0 {
		return nil, apperr.Invalid("activity_id and user_id are required")
	}
	a, err := s.getActivity(ctx, req.GetActivityId(), false)
	if err != nil {
		return nil, err
	}
	now := s.now()
	if a.status != statusActive || now.Before(a.startAt) || !now.Before(a.endAt) {
		return nil, apperr.Failed("seckill activity is not active")
	}
	if s.cache == nil {
		return nil, apperr.Unavailable("seckill redis is not configured", fmt.Errorf("redis client is nil"))
	}
	limiter := redis_rate.NewLimiter(s.cache)
	rate, err := limiter.Allow(ctx, fmt.Sprintf("seckill:rl:user:%d", req.GetUserId()), redis_rate.PerMinute(10))
	if err != nil {
		return nil, apperr.Unavailable("seckill rate limiter unavailable", err)
	}
	if rate.Allowed == 0 {
		metrics.SeckillAdmissions.WithLabelValues("rate_limited").Inc()
		return nil, apperr.RateLimited("too many seckill attempts")
	}

	rawToken := s.tokenFor(a, req.GetUserId())
	tokenHash := hashToken(rawToken)
	// Check the durable qualification before touching Redis. This prevents a
	// retry after the Redis user-key TTL from consuming a second unit while the
	// DB unique row still prevents a second purchase.
	if existing, lookupErr := s.getQualification(ctx, tokenHash, req.GetUserId()); lookupErr == nil {
		return existing.toPB(rawToken), nil
	} else if !apperr.IsCode(lookupErr, apperr.CodeNotFound) {
		return nil, lookupErr
	}
	if until, ok := s.soldOut.Load(a.id); ok {
		if t, ok := until.(time.Time); ok && now.Before(t) {
			metrics.SeckillAdmissions.WithLabelValues("sold_out_cached").Inc()
			return nil, apperr.Conflict("seckill is sold out")
		}
		s.soldOut.Delete(a.id)
	}
	expiresAt := now.Add(qualificationTTL)
	if expiresAt.After(a.endAt) {
		expiresAt = a.endAt
	}
	ttl := int64(time.Until(expiresAt).Seconds())
	if ttl <= 0 {
		return nil, apperr.Failed("seckill qualification window has elapsed")
	}
	keys := []string{
		fmt.Sprintf("seckill:{%d}:meta", a.id),
		fmt.Sprintf("seckill:{%d}:stock", a.id),
		fmt.Sprintf("seckill:{%d}:users", a.id),
		fmt.Sprintf("seckill:{%d}:tokens", a.id),
	}
	code, err := reserveScript.Run(ctx, s.cache, keys, req.GetUserId(), tokenHash, ttl).Int64()
	if err != nil {
		return nil, apperr.Unavailable("seckill admission unavailable", err)
	}
	switch code {
	case reserveAccepted:
		metrics.SeckillAdmissions.WithLabelValues("accepted").Inc()
		if err := s.holdQualification(ctx, a.id, tokenHash); err != nil {
			s.compensate(ctx, a.id, req.GetUserId(), tokenHash)
			return nil, err
		}
		q, err := s.saveQualification(ctx, a.id, req.GetUserId(), tokenHash, expiresAt, req)
		if err != nil {
			s.releaseQualification(ctx, a.id, tokenHash)
			s.compensate(ctx, a.id, req.GetUserId(), tokenHash)
			return nil, err
		}
		if !q.publishedAt.Valid {
			if err := s.publishQualification(ctx, q, a); err != nil {
				// The qualification is durable and the republisher will retry.
				s.log.Warn("publish seckill command deferred", "activity_id", a.id, "error", err)
			}
		}
		return q.toPB(rawToken), nil
	case reserveExisting:
		metrics.SeckillAdmissions.WithLabelValues("existing").Inc()
		q, err := s.getQualification(ctx, tokenHash, req.GetUserId())
		if err == nil {
			// Older S2 rows may predate the durable hold; ensuring it is
			// idempotent by the token hash.
			if err := s.holdQualification(ctx, a.id, tokenHash); err != nil {
				return nil, err
			}
			if !q.publishedAt.Valid {
				if err := s.publishQualification(ctx, q, a); err != nil {
					s.log.Warn("republish seckill command deferred", "activity_id", a.id, "error", err)
				}
			}
			return q.toPB(rawToken), nil
		}
		if !apperr.IsCode(err, apperr.CodeNotFound) {
			return nil, err
		}
		// Redis admitted the user but the DB write was lost. Persist the
		// deterministic token now; compensation is unsafe until we know the
		// insert outcome.
		if err := s.holdQualification(ctx, a.id, tokenHash); err != nil {
			return nil, err
		}
		q, err = s.saveQualification(ctx, a.id, req.GetUserId(), tokenHash, expiresAt, req)
		if err != nil {
			s.releaseQualification(ctx, a.id, tokenHash)
			return nil, err
		}
		return q.toPB(rawToken), nil
	case reserveSoldOut:
		metrics.SeckillAdmissions.WithLabelValues("sold_out").Inc()
		s.soldOut.Store(a.id, now.Add(time.Second))
		return nil, apperr.Conflict("seckill is sold out")
	case reserveNotWarm:
		metrics.SeckillAdmissions.WithLabelValues("not_warm").Inc()
		return nil, apperr.Unavailable("seckill stock is not warmed up", fmt.Errorf("missing activity keys"))
	case reserveNotActive:
		metrics.SeckillAdmissions.WithLabelValues("not_active").Inc()
		return nil, apperr.Failed("seckill activity is not active")
	default:
		return nil, apperr.Internal("unexpected seckill admission result", fmt.Errorf("code %d", code))
	}
}

func (s *Service) GetQualification(ctx context.Context, req *pb.GetSeckillQualificationRequest) (*pb.SeckillQualification, error) {
	if req.GetUserId() <= 0 || req.GetToken() == "" {
		return nil, apperr.Invalid("token and user_id are required")
	}
	q, err := s.getQualification(ctx, hashToken(req.GetToken()), req.GetUserId())
	if err != nil {
		return nil, err
	}
	return q.toPB(req.GetToken()), nil
}

func (s *Service) saveQualification(ctx context.Context, activityID, userID int64, tokenHash string, expiresAt time.Time, req *pb.ReserveSeckillRequest) (*qualification, error) {
	res, err := s.db.ExecContext(ctx, `INSERT IGNORE INTO seckill_qualifications
		(activity_id, user_id, address_id, receiver_name, receiver_phone, receiver_address, token_hash, status, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'ISSUED', ?)`,
		activityID, userID, req.GetAddressId(), req.GetReceiverName(), req.GetReceiverPhone(),
		req.GetReceiverAddress(), tokenHash, expiresAt)
	if err != nil {
		return nil, apperr.Internal("save seckill qualification", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		// Unique (activity,user) or token: return the existing row.
		return s.getQualification(ctx, tokenHash, userID)
	}
	return s.getQualification(ctx, tokenHash, userID)
}

func (s *Service) getQualification(ctx context.Context, tokenHash string, userID int64) (*qualification, error) {
	var q qualification
	err := s.db.QueryRowContext(ctx, `SELECT activity_id, user_id, address_id, receiver_name, receiver_phone,
		receiver_address, token_hash, status, order_no, payment_no, expires_at, published_at
		FROM seckill_qualifications WHERE token_hash=? AND user_id=?`, tokenHash, userID).
		Scan(&q.activityID, &q.userID, &q.addressID, &q.receiverName, &q.receiverPhone,
			&q.receiverAddr, &q.tokenHash, &q.status, &q.orderNo, &q.paymentNo, &q.expiresAt, &q.publishedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("seckill qualification not found")
	}
	if err != nil {
		return nil, apperr.Internal("query seckill qualification", err)
	}
	if q.status == "ISSUED" && !q.expiresAt.After(s.now()) {
		q.status = "EXPIRED"
	}
	return &q, nil
}

func (s *Service) holdQualification(ctx context.Context, activityID int64, tokenHash string) error {
	_, err := s.inventory.HoldSeckillStock(ctx, &pb.HoldSeckillStockRequest{
		ActivityId: activityID, Quantity: 1, RefNo: tokenHash,
	})
	return err
}

func (s *Service) releaseQualification(ctx context.Context, activityID int64, tokenHash string) error {
	_, err := s.inventory.ReleaseSeckillHold(ctx, &pb.ReleaseSeckillHoldRequest{
		ActivityId: activityID, Quantity: 1, RefNo: tokenHash,
	})
	if err != nil {
		s.log.Warn("release seckill stock hold", "activity_id", activityID, "error", err)
	}
	return err
}

func (s *Service) compensate(ctx context.Context, activityID, userID int64, tokenHash string) {
	keys := []string{
		fmt.Sprintf("seckill:{%d}:users", activityID),
		fmt.Sprintf("seckill:{%d}:tokens", activityID),
		fmt.Sprintf("seckill:{%d}:stock", activityID),
		fmt.Sprintf("seckill:{%d}:meta", activityID),
	}
	if _, err := compensateScript.Run(ctx, s.cache, keys, userID, tokenHash).Result(); err != nil {
		s.log.Error("compensate seckill qualification", "activity_id", activityID, "user_id", userID, "error", err)
	}
}
