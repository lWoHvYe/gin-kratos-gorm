package order

import (
	"context"
	"strings"
	"testing"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"

	"google.golang.org/grpc/metadata"
)

func TestOrderPayCent(t *testing.T) {
	cases := []struct {
		name     string
		total    int64
		discount int64
		freight  int64
		want     int64
	}{
		{name: "plain", total: 10_000, discount: 0, freight: 0, want: 10_000},
		{name: "with discount", total: 10_000, discount: 1_500, freight: 0, want: 8_500},
		{name: "with freight", total: 10_000, discount: 0, freight: 800, want: 10_800},
		{name: "discount and freight", total: 10_000, discount: 2_000, freight: 800, want: 8_800},
		{name: "fully discounted", total: 10_000, discount: 10_000, freight: 0, want: 0},
		{name: "clamped to zero", total: 1_000, discount: 5_000, freight: 0, want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := orderPayCent(tc.total, tc.discount, tc.freight); got != tc.want {
				t.Fatalf("orderPayCent(%d, %d, %d) = %d, want %d", tc.total, tc.discount, tc.freight, got, tc.want)
			}
		})
	}
}

func TestQuoteAmounts(t *testing.T) {
	quote := &pb.Quote{
		QuoteId:      "q-1",
		DiscountCent: 1_000,
		FreightCent:  600,
		Lines: []*pb.QuoteLine{
			{SkuId: 1, PriceCent: 2_000, Quantity: 2},
			{SkuId: 2, PriceCent: 3_500, Quantity: 1},
		},
	}
	total, discount, freight, pay := quoteAmounts(quote)
	if total != 7_500 {
		t.Fatalf("total = %d, want 7500 (sum of lines)", total)
	}
	if discount != 1_000 || freight != 600 {
		t.Fatalf("discount/freight = %d/%d, want 1000/600", discount, freight)
	}
	if want := int64(7_500 - 1_000 + 600); pay != want {
		t.Fatalf("pay = %d, want %d", pay, want)
	}

	// Pricing supplied amounts win over the local computation.
	explicit := &pb.Quote{TotalCent: 9_000, DiscountCent: 500, FreightCent: 0, PayCent: 8_500}
	total, _, _, pay = quoteAmounts(explicit)
	if total != 9_000 || pay != 8_500 {
		t.Fatalf("total/pay = %d/%d, want 9000/8500", total, pay)
	}

	// A discount larger than the total cannot make the payable amount negative.
	clamped := &pb.Quote{DiscountCent: 5_000, Lines: []*pb.QuoteLine{{SkuId: 1, PriceCent: 1_000, Quantity: 1}}}
	total, discount, _, pay = quoteAmounts(clamped)
	if total != 1_000 || discount != 1_000 || pay != 0 {
		t.Fatalf("total/discount/pay = %d/%d/%d, want 1000/1000/0", total, discount, pay)
	}
}

func TestQuoteExpireAt(t *testing.T) {
	future := time.Now().Add(time.Hour).Truncate(time.Second)
	got := quoteExpireAt(&pb.Quote{ExpiresAtUnix: future.Unix()})
	if !got.Equal(future) {
		t.Fatalf("expire at = %v, want %v", got, future)
	}

	past := time.Now().Add(-time.Hour)
	got = quoteExpireAt(&pb.Quote{ExpiresAtUnix: past.Unix()})
	if got.Before(time.Now().Add(orderExpireWindow / 2)) {
		t.Fatalf("expired quote should fall back to the order window, got %v", got)
	}
}

func TestNormalizeCreateOrder(t *testing.T) {
	valid := func() *pb.CreateOrderRequest {
		return &pb.CreateOrderRequest{
			UserId:          7,
			QuoteId:         " quote-1 ",
			AddressId:       3,
			ClientRequestId: " client-1 ",
			ReceiverName:    " Alice ",
			ReceiverPhone:   " 13800000000 ",
			ReceiverAddress: " 1 Infinite Loop ",
		}
	}

	in, err := normalizeCreateOrder(valid())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if in.QuoteID != "quote-1" || in.ClientRequestID != "client-1" || in.ReceiverName != "Alice" {
		t.Fatalf("fields were not trimmed: %+v", in)
	}
	if in.UserID != 7 || in.AddressID != 3 {
		t.Fatalf("ids were not copied: %+v", in)
	}

	if _, err := normalizeCreateOrder(&pb.CreateOrderRequest{}); err == nil {
		t.Fatal("empty request should be rejected")
	}

	missingReceiver := valid()
	missingReceiver.ReceiverPhone = " "
	if _, err := normalizeCreateOrder(missingReceiver); err == nil {
		t.Fatal("missing receiver phone should be rejected")
	}

	tooLong := valid()
	tooLong.ClientRequestId = strings.Repeat("a", maxClientRequestIDBytes+1)
	if _, err := normalizeCreateOrder(tooLong); err == nil {
		t.Fatal("oversized client_request_id should be rejected")
	}

	nonASCII := valid()
	nonASCII.ClientRequestId = "order-\u00e9"
	if _, err := normalizeCreateOrder(nonASCII); err == nil {
		t.Fatal("non ASCII client_request_id should be rejected")
	}

	generated, err := normalizeCreateOrder(func() *pb.CreateOrderRequest {
		r := valid()
		r.ClientRequestId = ""
		return r
	}())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(generated.ClientRequestID, "auto-") {
		t.Fatalf("client_request_id = %q, want an auto generated key", generated.ClientRequestID)
	}
	if len(generated.ClientRequestID) > maxClientRequestIDBytes {
		t.Fatalf("generated client_request_id is too long: %d", len(generated.ClientRequestID))
	}
}

func TestBuildCreateOrderInner(t *testing.T) {
	in := createOrderInput{
		UserID:          42,
		QuoteID:         "quote-9",
		AddressID:       11,
		ClientRequestID: "client-9",
		ReceiverName:    "Bob",
		ReceiverPhone:   "13900000000",
		ReceiverAddress: "Somewhere 1",
	}
	quote := &pb.Quote{
		QuoteId:      "quote-9",
		TotalCent:    5_000,
		DiscountCent: 500,
		FreightCent:  200,
		PayCent:      4_700,
		Lines: []*pb.QuoteLine{
			{SkuId: 10, Name: "shirt", PriceCent: 2_500, Quantity: 2, DiscountCent: 500},
		},
	}
	expireAt := time.Now().Add(10 * time.Minute).Truncate(time.Second)
	inner := buildCreateOrderInner("ORD1", in, quote, expireAt)

	if inner.GetOrderNo() != "ORD1" || inner.GetUserId() != 42 || inner.GetQuoteId() != "quote-9" {
		t.Fatalf("unexpected identity fields: %+v", inner)
	}
	if inner.GetAddressId() != 11 || inner.GetReceiverName() != "Bob" {
		t.Fatalf("unexpected receiver fields: %+v", inner)
	}
	if inner.GetTotalCent() != 5_000 || inner.GetDiscountCent() != 500 ||
		inner.GetFreightCent() != 200 || inner.GetPayCent() != 4_700 {
		t.Fatalf("unexpected amounts: %+v", inner)
	}
	if inner.GetExpireAtUnix() != expireAt.Unix() {
		t.Fatalf("expire_at_unix = %d, want %d", inner.GetExpireAtUnix(), expireAt.Unix())
	}
	if len(inner.GetItems()) != 1 {
		t.Fatalf("items = %d, want 1", len(inner.GetItems()))
	}
	item := inner.GetItems()[0]
	if item.GetSkuId() != 10 || item.GetName() != "shirt" || item.GetPriceCent() != 2_500 ||
		item.GetQuantity() != 2 || item.GetDiscountCent() != 500 {
		t.Fatalf("unexpected item: %+v", item)
	}
}

func TestValidateCreateOrderInner(t *testing.T) {
	valid := func() *pb.CreateOrderInnerRequest {
		return &pb.CreateOrderInnerRequest{
			OrderNo:         "ORD1",
			UserId:          1,
			QuoteId:         "quote-1",
			TotalCent:       100,
			PayCent:         100,
			ReceiverName:    "Alice",
			ReceiverPhone:   "13800000000",
			ReceiverAddress: "Somewhere 1",
			Items:           []*pb.OrderItem{{SkuId: 1, Name: "x", PriceCent: 100, Quantity: 1}},
		}
	}
	if err := validateCreateOrderInner(valid()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bad := []struct {
		name   string
		mutate func(*pb.CreateOrderInnerRequest)
	}{
		{name: "no order_no", mutate: func(r *pb.CreateOrderInnerRequest) { r.OrderNo = "" }},
		{name: "no user", mutate: func(r *pb.CreateOrderInnerRequest) { r.UserId = 0 }},
		{name: "no quote", mutate: func(r *pb.CreateOrderInnerRequest) { r.QuoteId = "" }},
		{name: "no receiver", mutate: func(r *pb.CreateOrderInnerRequest) { r.ReceiverAddress = "" }},
		{name: "no items", mutate: func(r *pb.CreateOrderInnerRequest) { r.Items = nil }},
		{name: "bad quantity", mutate: func(r *pb.CreateOrderInnerRequest) { r.Items[0].Quantity = 0 }},
		{name: "bad sku", mutate: func(r *pb.CreateOrderInnerRequest) { r.Items[0].SkuId = 0 }},
		{name: "negative amount", mutate: func(r *pb.CreateOrderInnerRequest) { r.PayCent = -1 }},
		{name: "negative item amount", mutate: func(r *pb.CreateOrderInnerRequest) { r.Items[0].PriceCent = -1 }},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			req := valid()
			tc.mutate(req)
			if err := validateCreateOrderInner(req); err == nil {
				t.Fatalf("%s should have been rejected", tc.name)
			}
		})
	}
}

func TestCreateOrderGID(t *testing.T) {
	if got := createOrderGID("ORD42"); got != "order-create:ORD42" {
		t.Fatalf("gid = %q, want order-create:ORD42", got)
	}
}

func TestKnownStatuses(t *testing.T) {
	for _, status := range []string{statusPendingPayment, statusPaid, statusCancelled, statusShipped, statusCompleted, statusClosed} {
		if _, ok := knownStatuses[status]; !ok {
			t.Fatalf("status %q is missing from knownStatuses", status)
		}
	}
	if _, ok := knownStatuses["PENDING"]; ok {
		t.Fatal("unknown status should not be part of knownStatuses")
	}
}

func TestClientRequestIDFromContext(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(clientRequestIDHeader, "client-42"))
	if got := clientRequestIDFromContext(ctx, "fallback"); got != "client-42" {
		t.Fatalf("client request id = %q, want client-42", got)
	}
	if got := clientRequestIDFromContext(context.Background(), "ORD-fallback"); got != "ORD-fallback" {
		t.Fatalf("fallback = %q, want ORD-fallback", got)
	}
	long := strings.Repeat("x", maxClientRequestIDBytes+10)
	ctx = metadata.NewIncomingContext(context.Background(), metadata.Pairs(clientRequestIDHeader, long))
	if got := clientRequestIDFromContext(ctx, "fallback"); len(got) != maxClientRequestIDBytes {
		t.Fatalf("truncated length = %d, want %d", len(got), maxClientRequestIDBytes)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("abcdef", 3); got != "abc" {
		t.Fatalf("truncate = %q, want abc", got)
	}
	if got := truncate("abc", 10); got != "abc" {
		t.Fatalf("truncate = %q, want abc", got)
	}
	if got := truncate("abc", 0); got != "abc" {
		t.Fatalf("truncate with max<=0 = %q, want abc", got)
	}
	// Multibyte runes are cut on a boundary, never in the middle.
	if got := truncate("订单取消", 7); got != "订单" {
		t.Fatalf("truncate = %q, want 订单", got)
	}
}

func TestOrderItemsPayload(t *testing.T) {
	items := []*pb.OrderItem{{SkuId: 1, Name: "x", PriceCent: 100, Quantity: 2, DiscountCent: 10}}
	payload := orderItemsPayload(items)
	if len(payload) != 1 {
		t.Fatalf("payload length = %d, want 1", len(payload))
	}
	if payload[0]["sku_id"] != int64(1) || payload[0]["quantity"] != int32(2) {
		t.Fatalf("unexpected payload: %+v", payload[0])
	}
}

func TestPaidAmountValidatesBeforeSideEffects(t *testing.T) {
	record := &orderRecord{PayCent: 8_800}
	if got, err := paidAmount(record, 0); err != nil || got != 8_800 {
		t.Fatalf("default amount = %d, %v", got, err)
	}
	if got, err := paidAmount(record, 8_800); err != nil || got != 8_800 {
		t.Fatalf("exact amount = %d, %v", got, err)
	}
	if _, err := paidAmount(record, 8_801); err == nil || !apperr.IsCode(err, apperr.CodeInvalidArgument) {
		t.Fatalf("overpay err = %v", err)
	}
}

func TestTccCancelDecision(t *testing.T) {
	cases := []struct {
		name   string
		status string
		paid   int64
		want   tccCancelAction
	}{
		{"unpaid pending cancels locally", statusPendingPayment, 0, tccCancelLocal},
		{"captured pending is refunded", statusPendingPayment, 100, tccCancelRefund},
		{"paid order is refunded", statusPaid, 100, tccCancelRefund},
		{"zero-amount paid stays paid", statusPaid, 0, tccCancelKeepPaid},
		{"refunding is terminal", statusRefunding, 100, tccCancelNone},
		{"cancelled is terminal", statusCancelled, 0, tccCancelNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tccCancelDecision(tc.status, tc.paid); got != tc.want {
				t.Fatalf("tccCancelDecision(%q, %d) = %v, want %v", tc.status, tc.paid, got, tc.want)
			}
		})
	}
}
