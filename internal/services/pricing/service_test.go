package pricing

import (
	"math"
	"testing"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

func TestFreightFor(t *testing.T) {
	cases := []struct {
		name  string
		total int64
		want  int64
	}{
		{"empty", 0, flatFreightCent},
		{"just below threshold", 9899, flatFreightCent},
		{"at threshold", freeFreightThresholdCent, 0},
		{"above threshold", 20000, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := freightFor(tc.total); got != tc.want {
				t.Fatalf("freightFor(%d) = %d, want %d", tc.total, got, tc.want)
			}
		})
	}
}

func TestPayFor(t *testing.T) {
	cases := []struct {
		name                     string
		total, discount, freight int64
		want                     int64
	}{
		{"no coupon free freight", 9900, 0, 0, 9900},
		{"no coupon paid freight", 5000, 0, 1000, 6000},
		{"coupon", 10000, 1000, 0, 9000},
		{"coupon exceeds total", 1000, 5000, 1000, 1000},
		{"negative discount", 1000, -500, 1000, 2000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := payFor(tc.total, tc.discount, tc.freight); got != tc.want {
				t.Fatalf("payFor(%d, %d, %d) = %d, want %d", tc.total, tc.discount, tc.freight, got, tc.want)
			}
		})
	}
}

func TestAllocateDiscount(t *testing.T) {
	lines := []quoteLine{
		{SkuID: 1, PriceCent: 1000, Quantity: 1},
		{SkuID: 2, PriceCent: 2000, Quantity: 1},
		{SkuID: 3, PriceCent: 3000, Quantity: 1},
	}
	allocateDiscount(lines, 6000, 1000)

	want := []int64{166, 334, 500}
	var sum int64
	for i, line := range lines {
		if line.DiscountCent != want[i] {
			t.Fatalf("line %d discount = %d, want %d", i, line.DiscountCent, want[i])
		}
		sum += line.DiscountCent
	}
	if sum != 1000 {
		t.Fatalf("discount sum = %d, want 1000", sum)
	}
}

func TestAllocateDiscountNoCoupon(t *testing.T) {
	lines := []quoteLine{{SkuID: 1, PriceCent: 1000, Quantity: 2}}
	allocateDiscount(lines, 2000, 0)
	if lines[0].DiscountCent != 0 {
		t.Fatalf("discount = %d, want 0", lines[0].DiscountCent)
	}
}

func TestMulDivFloor(t *testing.T) {
	cases := []struct{ x, y, z, want int64 }{
		{10, 5, 10, 5},
		{0, 5, 10, 0},
		{1, 1, 3, 0},
		{2, 1, 3, 0},
		{math.MaxInt64, math.MaxInt64, math.MaxInt64, math.MaxInt64},
	}
	for _, tc := range cases {
		if got := mulDivFloor(tc.x, tc.y, tc.z); got != tc.want {
			t.Fatalf("mulDivFloor(%d, %d, %d) = %d, want %d", tc.x, tc.y, tc.z, got, tc.want)
		}
	}
}

func TestNormalizeItems(t *testing.T) {
	items, err := normalizeItems([]*pb.QuoteItem{
		{SkuId: 7, Quantity: 1},
		{SkuId: 7, Quantity: 2},
		{SkuId: 9, Quantity: 3},
	})
	if err != nil {
		t.Fatalf("normalizeItems() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].GetSkuId() != 7 || items[0].GetQuantity() != 3 {
		t.Fatalf("merged item = %+v, want sku 7 qty 3", items[0])
	}
	if items[1].GetSkuId() != 9 || items[1].GetQuantity() != 3 {
		t.Fatalf("second item = %+v, want sku 9 qty 3", items[1])
	}

	bad := [][]*pb.QuoteItem{
		nil,
		{{SkuId: 0, Quantity: 1}},
		{{SkuId: 1, Quantity: 0}},
		{{SkuId: 1, Quantity: -1}},
		{{SkuId: 1, Quantity: maxQuantity + 1}},
		{{SkuId: 1, Quantity: 2}, {SkuId: 1, Quantity: maxQuantity}},
	}
	for i, input := range bad {
		if _, err := normalizeItems(input); !apperr.IsCode(err, apperr.CodeInvalidArgument) {
			t.Fatalf("case %d: error = %v, want invalid argument", i, err)
		}
	}
}

func TestLineAmount(t *testing.T) {
	if got, err := lineAmount(1234, 3); err != nil || got != 3702 {
		t.Fatalf("lineAmount(1234, 3) = %d, %v; want 3702, nil", got, err)
	}
	if _, err := lineAmount(0, 1); err == nil {
		t.Fatal("expected error for zero price")
	}
	if _, err := lineAmount(1, 0); err == nil {
		t.Fatal("expected error for zero quantity")
	}
	if _, err := lineAmount(math.MaxInt64, 2); err == nil {
		t.Fatal("expected overflow error")
	}
}

func TestValidateCoupon(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	base := coupon{
		ID: 1, Code: "WELCOME10", AmountCent: 1000, MinAmountCent: 5000,
		Status: couponAvailable, ExpireAt: now.Add(time.Hour),
	}
	cases := []struct {
		name     string
		mutate   func(*coupon)
		userID   int64
		total    int64
		wantCode apperr.Code
	}{
		{"valid", func(*coupon) {}, 42, 5000, ""},
		{"public valid", func(*coupon) {}, 42, 6000, ""},
		{"expired", func(c *coupon) { c.ExpireAt = now }, 42, 6000, apperr.CodeFailedPrecondition},
		{"min not met", func(*coupon) {}, 42, 4999, apperr.CodeFailedPrecondition},
		{"locked", func(c *coupon) { c.Status = couponLocked }, 42, 6000, apperr.CodeFailedPrecondition},
		{"used", func(c *coupon) { c.Status = couponUsed }, 42, 6000, apperr.CodeFailedPrecondition},
		{"other user", func(c *coupon) { c.UserID = 7 }, 42, 6000, apperr.CodePermissionDenied},
		{"zero amount", func(c *coupon) { c.AmountCent = 0 }, 42, 6000, apperr.CodeFailedPrecondition},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := base
			tc.mutate(&c)
			err := validateCoupon(&c, tc.userID, tc.total, now)
			if tc.wantCode == "" {
				if err != nil {
					t.Fatalf("validateCoupon() error = %v, want nil", err)
				}
				return
			}
			if !apperr.IsCode(err, tc.wantCode) {
				t.Fatalf("validateCoupon() error = %v, want code %s", err, tc.wantCode)
			}
		})
	}
}

func TestQuoteToProto(t *testing.T) {
	q := &quote{
		QuoteID: "QTE1", UserID: 1, TotalCent: 6000, DiscountCent: 1000,
		FreightCent: 0, PayCent: 5000, CouponID: "WELCOME10",
		ExpiresAt: time.Unix(1700000000, 0),
		Lines:     []quoteLine{{SkuID: 3, Name: "SKU-A", PriceCent: 3000, Quantity: 2, DiscountCent: 1000}},
	}
	out := q.toProto()
	if out.GetQuoteId() != "QTE1" || out.GetPayCent() != 5000 || out.GetExpiresAtUnix() != 1700000000 {
		t.Fatalf("unexpected quote: %+v", out)
	}
	if len(out.GetLines()) != 1 {
		t.Fatalf("len(lines) = %d, want 1", len(out.GetLines()))
	}
	line := out.GetLines()[0]
	if line.GetSkuId() != 3 || line.GetName() != "SKU-A" || line.GetDiscountCent() != 1000 {
		t.Fatalf("unexpected line: %+v", line)
	}
}
