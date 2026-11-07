package payment

import (
	"context"
	"testing"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"

	"github.com/go-sql-driver/mysql"
)

func TestSignature(t *testing.T) {
	got := Signature("test-secret", "PAY123", "EVT456", 1999, "SUCCEEDED")
	const want = "86b0a43a57a2f976b50d6c2994b7b2887ed9d25b521953dee5bfa20c4fcae5e7"
	if got != want {
		t.Fatalf("Signature() = %s, want %s", got, want)
	}
	if got := Signature("other-secret", "PAY123", "EVT456", 1999, "SUCCEEDED"); got == want {
		t.Fatal("signature must depend on the secret")
	}
	if got := Signature("test-secret", "PAY123", "EVT456", 2000, "SUCCEEDED"); got == want {
		t.Fatal("signature must depend on the amount")
	}
	if got := Signature("test-secret", "PAY123", "EVT456", 1999, "FAILED"); got == want {
		t.Fatal("signature must depend on the status")
	}
}

func TestVerifySignature(t *testing.T) {
	open := New(nil, nil, "", nil)
	if err := open.verifySignature("PAY1", "EVT1", 100, "SUCCEEDED", ""); err != nil {
		t.Fatalf("signature must be optional without a secret: %v", err)
	}

	secure := New(nil, nil, "test-secret", nil)
	sig := Signature("test-secret", "PAY1", "EVT1", 100, "SUCCEEDED")
	if err := secure.verifySignature("PAY1", "EVT1", 100, "SUCCEEDED", sig); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	if err := secure.verifySignature("PAY1", "EVT1", 100, "SUCCEEDED", "  "+sig+"  "); err != nil {
		t.Fatalf("signature should tolerate surrounding whitespace: %v", err)
	}
	cases := []struct {
		name string
		sig  string
	}{
		{name: "missing", sig: ""},
		{name: "wrong", sig: Signature("wrong-secret", "PAY1", "EVT1", 100, "SUCCEEDED")},
		{name: "amount changed", sig: Signature("test-secret", "PAY1", "EVT1", 200, "SUCCEEDED")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := secure.verifySignature("PAY1", "EVT1", 100, "SUCCEEDED", tc.sig)
			if !apperr.IsCode(err, apperr.CodeUnauthenticated) {
				t.Fatalf("verifySignature() error = %v, want UNAUTHENTICATED", err)
			}
		})
	}
}

func TestNormalizeStatus(t *testing.T) {
	tests := []struct {
		raw     string
		want    string
		wantErr bool
	}{
		{raw: "SUCCEEDED", want: statusSucceeded},
		{raw: " success ", want: statusSucceeded},
		{raw: "PAID", want: statusSucceeded},
		{raw: "failed", want: statusFailed},
		{raw: "CLOSED", want: statusFailed},
		{raw: "CANCELED", want: statusFailed},
		{raw: "", wantErr: true},
		{raw: "PENDING", wantErr: true},
		{raw: "REFUNDED", wantErr: true},
	}
	for _, tc := range tests {
		got, err := normalizeStatus(tc.raw)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("normalizeStatus(%q) = %q, want error", tc.raw, got)
			}
			continue
		}
		if err != nil {
			t.Fatalf("normalizeStatus(%q) error = %v", tc.raw, err)
		}
		if got != tc.want {
			t.Fatalf("normalizeStatus(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestAdvance(t *testing.T) {
	tests := []struct {
		current  string
		callback string
		next     string
		changed  bool
		ok       bool
	}{
		{statusPending, statusSucceeded, statusSucceeded, true, true},
		{statusPending, statusFailed, statusFailed, true, true},
		{statusFailed, statusSucceeded, statusSucceeded, true, true},
		{statusFailed, statusFailed, statusFailed, false, true},
		{statusSucceeded, statusSucceeded, statusSucceeded, false, true},
		{statusSucceeded, statusFailed, statusSucceeded, false, true},
		{statusRefunded, statusSucceeded, statusRefunded, false, true},
		{statusRefunded, statusFailed, statusRefunded, false, true},
		{"UNKNOWN", statusSucceeded, "", false, false},
		{statusPending, "UNKNOWN", "", false, false},
	}
	for _, tc := range tests {
		next, changed, ok := advance(tc.current, tc.callback)
		if next != tc.next || changed != tc.changed || ok != tc.ok {
			t.Fatalf("advance(%q, %q) = (%q, %v, %v), want (%q, %v, %v)",
				tc.current, tc.callback, next, changed, ok, tc.next, tc.changed, tc.ok)
		}
	}
}

func TestPayURL(t *testing.T) {
	if got, want := payURL("PAY123"), "mock://pay/PAY123"; got != want {
		t.Fatalf("payURL() = %q, want %q", got, want)
	}
}

func TestIntentToPB(t *testing.T) {
	in := &intent{
		PaymentNo:  "PAY123",
		OrderNo:    "ORD123",
		UserID:     7,
		AmountCent: 1999,
		Channel:    "mock",
		Status:     statusPending,
		PayURL:     "mock://pay/PAY123",
		ExpiresAt:  time.Unix(1_800_000_000, 0),
	}
	got := in.toPB()
	if got.GetPaymentNo() != "PAY123" || got.GetOrderNo() != "ORD123" || got.GetAmountCent() != 1999 ||
		got.GetChannel() != "mock" || got.GetStatus() != statusPending ||
		got.GetPayUrl() != "mock://pay/PAY123" || got.GetExpiresAtUnix() != 1_800_000_000 {
		t.Fatalf("toPB() = %+v", got)
	}
}

func TestPaymentEvent(t *testing.T) {
	in := &intent{PaymentNo: "PAY1", OrderNo: "ORD1", AmountCent: 100, Channel: "mock"}

	ok := paymentEvent(in, statusSucceeded)
	if ok.Topic != topicPaymentSucceeded || ok.EventType != eventSucceeded {
		t.Fatalf("succeeded event = %+v", ok)
	}
	payload, isMap := ok.Payload.(map[string]any)
	if !isMap {
		t.Fatalf("payload type = %T", ok.Payload)
	}
	if payload["status"] != statusSucceeded || payload["payment_no"] != "PAY1" || payload["amount_cent"] != int64(100) {
		t.Fatalf("payload = %+v", payload)
	}

	bad := paymentEvent(in, statusFailed)
	if bad.Topic != topicPaymentFailed || bad.EventType != eventFailed {
		t.Fatalf("failed event = %+v", bad)
	}
}

func TestIsDuplicateKey(t *testing.T) {
	if !isDuplicateKey(&mysql.MySQLError{Number: 1062}) {
		t.Fatal("MySQL 1062 must be treated as a duplicate key")
	}
	if isDuplicateKey(&mysql.MySQLError{Number: 1213}) {
		t.Fatal("deadlock must not be treated as a duplicate key")
	}
	if isDuplicateKey(nil) {
		t.Fatal("nil must not be treated as a duplicate key")
	}
}

func TestValidationRejectsBeforeDatabase(t *testing.T) {
	s := New(nil, nil, "", nil)
	ctx := context.Background()

	if _, err := s.CreateIntent(ctx, &pb.CreateIntentRequest{OrderNo: "", AmountCent: 100}); !apperr.IsCode(err, apperr.CodeInvalidArgument) {
		t.Fatalf("CreateIntent without order_no: %v", err)
	}
	if _, err := s.CreateIntent(ctx, &pb.CreateIntentRequest{OrderNo: "ORD1", AmountCent: 0}); !apperr.IsCode(err, apperr.CodeInvalidArgument) {
		t.Fatalf("CreateIntent with zero amount: %v", err)
	}
	if _, err := s.CreateIntent(ctx, &pb.CreateIntentRequest{OrderNo: "ORD1", AmountCent: -1}); !apperr.IsCode(err, apperr.CodeInvalidArgument) {
		t.Fatalf("CreateIntent with negative amount: %v", err)
	}
	if _, err := s.GetIntent(ctx, &pb.GetIntentRequest{PaymentNo: ""}); !apperr.IsCode(err, apperr.CodeInvalidArgument) {
		t.Fatalf("GetIntent without payment_no: %v", err)
	}
	if _, err := s.Callback(ctx, &pb.CallbackRequest{}); !apperr.IsCode(err, apperr.CodeInvalidArgument) {
		t.Fatalf("Callback without ids: %v", err)
	}
	if _, err := s.Callback(ctx, &pb.CallbackRequest{
		PaymentNo: "PAY1", Channel: "mock", EventId: "EVT1", AmountCent: 100, Status: "weird",
	}); !apperr.IsCode(err, apperr.CodeInvalidArgument) {
		t.Fatalf("Callback with unsupported status: %v", err)
	}
	if _, err := s.Refund(ctx, &pb.RefundRequest{PaymentNo: "PAY1", AmountCent: 0}); !apperr.IsCode(err, apperr.CodeInvalidArgument) {
		t.Fatalf("Refund with zero amount: %v", err)
	}
}

func TestTCCConfiguration(t *testing.T) {
	t.Setenv("PAYMENT_TCC_ENABLED", "false")
	if tccEnabled() {
		t.Fatal("PAYMENT_TCC_ENABLED=false must disable TCC")
	}
	t.Setenv("PAYMENT_TCC_ENABLED", "true")
	if !tccEnabled() {
		t.Fatal("PAYMENT_TCC_ENABLED=true must enable TCC")
	}
	t.Setenv("ORDER_TCC_URL", "order:9006")
	t.Setenv("INVENTORY_TCC_URL", "inventory:9003")
	t.Setenv("PRICING_TCC_URL", "pricing:9005")
	o, i, p := tccURLs()
	if o != "order:9006" || i != "inventory:9003" || p != "pricing:9005" {
		t.Fatalf("unexpected tcc urls: %s %s %s", o, i, p)
	}
}
