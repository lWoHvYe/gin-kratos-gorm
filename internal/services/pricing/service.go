package pricing

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"math/bits"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/db"
	"gin-kratos-gorm/internal/platform/dtm"
	"gin-kratos-gorm/internal/platform/id"
	"gin-kratos-gorm/internal/platform/outbox"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// quoteTTL is how long a price quote stays valid.
	quoteTTL = 15 * time.Minute
	// freeFreightThresholdCent is the order total above which shipping is free.
	freeFreightThresholdCent int64 = 9900
	// flatFreightCent is charged below the free shipping threshold.
	flatFreightCent int64 = 1000
	// maxQuantity caps a single quote line.
	maxQuantity int32 = 999
	// catalogTimeout bounds one catalog lookup.
	catalogTimeout = 3 * time.Second
)

const (
	topicCouponLocked   = "shop.pricing.coupon.locked.v1"
	topicCouponUnlocked = "shop.pricing.coupon.unlocked.v1"
	topicCouponUsed     = "shop.pricing.coupon.used.v1"
)

// Service implements pb.PricingServiceServer on top of database/sql.
type Service struct {
	pb.UnimplementedPricingServiceServer

	db      *sql.DB
	catalog pb.CatalogServiceClient
	log     *slog.Logger
	now     func() time.Time
}

func New(database *sql.DB, catalog pb.CatalogServiceClient, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{db: database, catalog: catalog, log: logger, now: time.Now}
}

// CreateQuote prices the requested items with the catalog service and stores
// the quote (and its lines) for quoteTTL.
func (s *Service) CreateQuote(ctx context.Context, req *pb.QuoteRequest) (*pb.Quote, error) {
	userID := req.GetUserId()
	if userID <= 0 {
		return nil, apperr.Invalid("user_id is required")
	}
	items, err := normalizeItems(req.GetItems())
	if err != nil {
		return nil, err
	}

	now := s.now()
	lines := make([]quoteLine, 0, len(items))
	var totalCent int64
	for _, item := range items {
		sku, err := s.getSku(ctx, item.GetSkuId())
		if err != nil {
			return nil, err
		}
		if st := sku.GetStatus(); st != "" && st != "ENABLED" && st != "ON_SALE" {
			return nil, apperr.Failed(fmt.Sprintf("sku %d is not on sale", item.GetSkuId()))
		}
		amount, err := lineAmount(sku.GetPriceCent(), item.GetQuantity())
		if err != nil {
			return nil, err
		}
		if amount > math.MaxInt64-totalCent {
			return nil, apperr.Failed("quote amount is too large")
		}
		totalCent += amount
		lines = append(lines, quoteLine{
			SkuID:     item.GetSkuId(),
			Name:      skuName(sku),
			PriceCent: sku.GetPriceCent(),
			Quantity:  item.GetQuantity(),
		})
	}

	var (
		discountCent int64
		couponID     string
	)
	if ref := strings.TrimSpace(req.GetCouponId()); ref != "" {
		c, err := resolveCoupon(ctx, s.db, ref, userID)
		if err != nil {
			return nil, err
		}
		if err := validateCoupon(c, userID, totalCent, now); err != nil {
			return nil, err
		}
		discountCent = c.AmountCent
		if discountCent > totalCent {
			discountCent = totalCent
		}
		couponID = c.Code
		allocateDiscount(lines, totalCent, discountCent)
	}

	q := &quote{
		QuoteID:      id.New("QTE"),
		UserID:       userID,
		TotalCent:    totalCent,
		DiscountCent: discountCent,
		FreightCent:  freightFor(totalCent),
		CouponID:     couponID,
		ExpiresAt:    now.Add(quoteTTL),
		Lines:        lines,
	}
	q.PayCent = payFor(totalCent, discountCent, q.FreightCent)

	if err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		if err := insertQuote(ctx, tx, q); err != nil {
			return err
		}
		return insertQuoteLines(ctx, tx, q)
	}); err != nil {
		return nil, err
	}
	s.log.Info("quote created", "quote_id", q.QuoteID, "user_id", userID, "total_cent", q.TotalCent, "pay_cent", q.PayCent)
	return q.toProto(), nil
}

// GetQuote returns a stored quote. Missing quotes are NotFound and expired
// quotes are FailedPrecondition.
func (s *Service) GetQuote(ctx context.Context, req *pb.GetQuoteRequest) (*pb.Quote, error) {
	quoteID := strings.TrimSpace(req.GetQuoteId())
	if quoteID == "" {
		return nil, apperr.Invalid("quote_id is required")
	}
	q, err := loadQuote(ctx, s.db, quoteID)
	if err != nil {
		return nil, err
	}
	if !q.ExpiresAt.After(s.now()) {
		return nil, apperr.Failed("quote expired")
	}
	lines, err := loadQuoteLines(ctx, s.db, quoteID)
	if err != nil {
		return nil, err
	}
	q.Lines = lines
	return q.toProto(), nil
}

// LockCoupon locks the coupon attached to the quote. It is idempotent: calling
// it again with the same quote (the DTM retry/compensate payload) is a no-op.
func (s *Service) LockCoupon(ctx context.Context, req *pb.LockCouponRequest) (*pb.Empty, error) {
	target, err := s.couponTarget(ctx, req)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return &pb.Empty{}, nil
	}
	if err := s.inTx(ctx, func(tx *sql.Tx) error {
		changed, err := lockCoupon(ctx, tx, target.coupon.ID, target.orderNo)
		if err != nil || !changed {
			return err
		}
		return outbox.Append(ctx, tx, outbox.Event{
			Topic:         topicCouponLocked,
			AggregateType: "coupon",
			AggregateID:   fmt.Sprintf("%d", target.coupon.ID),
			EventType:     "coupon.locked",
			Payload:       target.eventPayload(),
		})
	}); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

// UnlockCoupon releases a locked coupon back to AVAILABLE. Unlocking an
// already available coupon is a no-op.
func (s *Service) UnlockCoupon(ctx context.Context, req *pb.LockCouponRequest) (*pb.Empty, error) {
	target, err := s.couponTarget(ctx, req)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return &pb.Empty{}, nil
	}
	if err := s.inTx(ctx, func(tx *sql.Tx) error {
		changed, err := unlockCoupon(ctx, tx, target.coupon.ID, target.orderNo)
		if err != nil || !changed {
			return err
		}
		return outbox.Append(ctx, tx, outbox.Event{
			Topic:         topicCouponUnlocked,
			AggregateType: "coupon",
			AggregateID:   fmt.Sprintf("%d", target.coupon.ID),
			EventType:     "coupon.unlocked",
			Payload:       target.eventPayload(),
		})
	}); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

// ConsumeCoupon marks a locked coupon USED. Consuming an already used coupon
// for the same order is a no-op.
func (s *Service) ConsumeCoupon(ctx context.Context, req *pb.LockCouponRequest) (*pb.Empty, error) {
	target, err := s.couponTarget(ctx, req)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return &pb.Empty{}, nil
	}
	if err := s.inTx(ctx, func(tx *sql.Tx) error {
		changed, err := consumeCoupon(ctx, tx, target.coupon.ID, target.orderNo)
		if err != nil || !changed {
			return err
		}
		return outbox.Append(ctx, tx, outbox.Event{
			Topic:         topicCouponUsed,
			AggregateType: "coupon",
			AggregateID:   fmt.Sprintf("%d", target.coupon.ID),
			EventType:     "coupon.used",
			Payload:       target.eventPayload(),
		})
	}); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

// couponTarget resolves which coupon a branch call refers to and the
// deterministic order reference used for idempotency (the quote id).
type couponTarget struct {
	coupon  *coupon
	orderNo string
	userID  int64
}

func (t *couponTarget) eventPayload() map[string]any {
	return map[string]any{
		"quote_id":    t.orderNo,
		"coupon_id":   t.coupon.ID,
		"code":        t.coupon.Code,
		"user_id":     t.userID,
		"amount_cent": t.coupon.AmountCent,
	}
}

func (s *Service) couponTarget(ctx context.Context, req *pb.LockCouponRequest) (*couponTarget, error) {
	quoteID := strings.TrimSpace(req.GetQuoteId())
	if quoteID == "" {
		return nil, apperr.Invalid("quote_id is required")
	}
	userID := req.GetUserId()
	quoteCouponID := ""
	q, err := loadQuote(ctx, s.db, quoteID)
	switch {
	case err == nil:
		if userID <= 0 {
			userID = q.UserID
		} else if q.UserID > 0 && q.UserID != userID {
			return nil, apperr.Forbidden("quote does not belong to user")
		}
		quoteCouponID = strings.TrimSpace(q.CouponID)
	case apperr.IsCode(err, apperr.CodeNotFound):
		// Compensation may arrive after the quote is gone; fall back to the
		// explicit coupon reference in the payload.
		if strings.TrimSpace(req.GetCouponId()) == "" {
			return nil, err
		}
	default:
		return nil, err
	}

	ref := quoteCouponID
	if ref == "" {
		ref = strings.TrimSpace(req.GetCouponId())
	}
	if ref == "" {
		return nil, nil
	}
	c, err := resolveCoupon(ctx, s.db, ref, userID)
	if err != nil {
		return nil, err
	}
	if c.UserID != 0 && userID > 0 && c.UserID != userID {
		return nil, apperr.Forbidden("coupon does not belong to user")
	}
	return &couponTarget{coupon: c, orderNo: quoteID, userID: userID}, nil
}

// inTx runs fn inside a DTM branch transaction when the call is a branch call,
// otherwise inside a local transaction.
func (s *Service) inTx(ctx context.Context, fn func(*sql.Tx) error) error {
	if dtm.InBranch(ctx) {
		return dtm.Branch(ctx, s.db, fn)
	}
	return db.WithTx(ctx, s.db, fn)
}

func (s *Service) getSku(ctx context.Context, skuID int64) (*pb.Sku, error) {
	if s.catalog == nil {
		return nil, apperr.Unavailable("catalog client is not configured", nil)
	}
	callCtx, cancel := context.WithTimeout(ctx, catalogTimeout)
	defer cancel()
	sku, err := s.catalog.GetSku(callCtx, &pb.GetSkuRequest{SkuId: skuID})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			return nil, apperr.NotFound(fmt.Sprintf("sku %d not found", skuID))
		case codes.Unavailable, codes.DeadlineExceeded:
			return nil, apperr.Unavailable(fmt.Sprintf("catalog unavailable for sku %d", skuID), err)
		default:
			return nil, apperr.Internal(fmt.Sprintf("get sku %d", skuID), err)
		}
	}
	if sku == nil {
		return nil, apperr.NotFound(fmt.Sprintf("sku %d not found", skuID))
	}
	if sku.GetPriceCent() <= 0 {
		return nil, apperr.Failed(fmt.Sprintf("sku %d has no valid price", skuID))
	}
	return sku, nil
}

func normalizeItems(items []*pb.QuoteItem) ([]*pb.QuoteItem, error) {
	if len(items) == 0 {
		return nil, apperr.Invalid("at least one item is required")
	}
	out := make([]*pb.QuoteItem, 0, len(items))
	index := make(map[int64]int, len(items))
	for _, item := range items {
		if item == nil {
			return nil, apperr.Invalid("quote item is required")
		}
		skuID, qty := item.GetSkuId(), item.GetQuantity()
		if skuID <= 0 {
			return nil, apperr.Invalid("sku_id must be positive")
		}
		if qty <= 0 {
			return nil, apperr.Invalid("quantity must be positive for sku %d", skuID)
		}
		if qty > maxQuantity {
			return nil, apperr.Invalid("quantity must be at most %d for sku %d", maxQuantity, skuID)
		}
		if i, ok := index[skuID]; ok {
			merged := out[i].GetQuantity() + qty
			if merged > maxQuantity {
				return nil, apperr.Invalid("quantity must be at most %d for sku %d", maxQuantity, skuID)
			}
			out[i].Quantity = merged
			continue
		}
		index[skuID] = len(out)
		out = append(out, &pb.QuoteItem{SkuId: skuID, Quantity: qty})
	}
	return out, nil
}

func lineAmount(priceCent int64, quantity int32) (int64, error) {
	if priceCent <= 0 {
		return 0, apperr.Failed("sku price is invalid")
	}
	if quantity <= 0 {
		return 0, apperr.Invalid("quantity must be positive")
	}
	if priceCent > math.MaxInt64/int64(quantity) {
		return 0, apperr.Failed("quote amount is too large")
	}
	return priceCent * int64(quantity), nil
}

// freightFor charges flat freight below the free shipping threshold.
func freightFor(totalCent int64) int64 {
	if totalCent >= freeFreightThresholdCent {
		return 0
	}
	return flatFreightCent
}

// payFor is the final amount payable: goods - discount + freight.
func payFor(totalCent, discountCent, freightCent int64) int64 {
	if discountCent < 0 {
		discountCent = 0
	}
	if discountCent > totalCent {
		discountCent = totalCent
	}
	return totalCent - discountCent + freightCent
}

// allocateDiscount spreads the quote-level discount over the lines
// proportionally to their amounts using cumulative rounding, so the line
// discounts always add up to discountCent exactly.
func allocateDiscount(lines []quoteLine, totalCent, discountCent int64) {
	if discountCent <= 0 || totalCent <= 0 || len(lines) == 0 {
		return
	}
	var (
		cumulative int64
		allocated  int64
	)
	for i := range lines {
		cumulative += lines[i].PriceCent * int64(lines[i].Quantity)
		target := mulDivFloor(cumulative, discountCent, totalCent)
		lines[i].DiscountCent = target - allocated
		allocated = target
	}
	// Guard against rounding drift on malformed input.
	if allocated != discountCent && len(lines) > 0 {
		lines[len(lines)-1].DiscountCent += discountCent - allocated
	}
}

// mulDivFloor returns x*y/z without overflowing int64. x and y must be
// non-negative and z must be positive; y is expected to be at most z.
func mulDivFloor(x, y, z int64) int64 {
	if x <= 0 || y <= 0 || z <= 0 {
		return 0
	}
	hi, lo := bits.Mul64(uint64(x), uint64(y))
	if hi >= uint64(z) {
		// Fall back to a float estimate; this is unreachable for valid quotes
		// because x, y <= z < 2^63 keeps the high word below z.
		return int64(float64(x) * float64(y) / float64(z))
	}
	q, _ := bits.Div64(hi, lo, uint64(z))
	return int64(q)
}

func skuName(sku *pb.Sku) string {
	if name := strings.TrimSpace(sku.GetSkuCode()); name != "" {
		return name
	}
	return fmt.Sprintf("SKU %d", sku.GetId())
}

func validateCoupon(c *coupon, userID, totalCent int64, now time.Time) error {
	if c == nil {
		return apperr.NotFound("coupon not found")
	}
	if c.AmountCent <= 0 {
		return apperr.Failed("coupon amount is invalid")
	}
	if c.Status != couponAvailable {
		return apperr.Failed("coupon is not available")
	}
	if c.UserID != 0 && c.UserID != userID {
		return apperr.Forbidden("coupon does not belong to user")
	}
	if !c.ExpireAt.After(now) {
		return apperr.Failed("coupon expired")
	}
	if totalCent < c.MinAmountCent {
		return apperr.Failed("coupon minimum amount not met")
	}
	return nil
}

func (q *quote) toProto() *pb.Quote {
	out := &pb.Quote{
		QuoteId:       q.QuoteID,
		TotalCent:     q.TotalCent,
		DiscountCent:  q.DiscountCent,
		FreightCent:   q.FreightCent,
		PayCent:       q.PayCent,
		CouponId:      q.CouponID,
		ExpiresAtUnix: q.ExpiresAt.Unix(),
	}
	out.Lines = make([]*pb.QuoteLine, 0, len(q.Lines))
	for _, line := range q.Lines {
		out.Lines = append(out.Lines, &pb.QuoteLine{
			SkuId:        line.SkuID,
			Name:         line.Name,
			PriceCent:    line.PriceCent,
			Quantity:     line.Quantity,
			DiscountCent: line.DiscountCent,
		})
	}
	return out
}
