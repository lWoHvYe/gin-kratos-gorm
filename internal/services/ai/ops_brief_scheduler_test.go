package ai

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestPostOpsBriefWebhookSignsRequest(t *testing.T) {
	const secret = "0123456789abcdef"
	var request *http.Request
	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request = r
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	payload := []byte(`{"summary":"过去 1 天","total_orders":3}`)
	if err := postOpsBriefWebhook(context.Background(), server.Client(), server.URL, secret, payload); err != nil {
		t.Fatal(err)
	}
	if request == nil {
		t.Fatal("webhook was not called")
	}
	if got := request.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q", got)
	}
	if got := request.Header.Get(webhookEventHeader); got != "ops.brief" {
		t.Fatalf("event = %q", got)
	}
	if got := request.Header.Get(webhookDeliveryHeader); len(got) < 8 {
		t.Fatalf("delivery id = %q", got)
	}
	timestamp := request.Header.Get(webhookTimestampHeader)
	unix, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || time.Since(time.Unix(unix, 0)) > time.Minute {
		t.Fatalf("timestamp = %q err=%v", timestamp, err)
	}
	// Recompute the signature independently so the receiver contract is pinned.
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if got := request.Header.Get(webhookSignatureHeader); got != want {
		t.Fatalf("signature = %q, want %q", got, want)
	}
}

func TestPostOpsBriefWebhookFailsClosedWithoutSecret(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	if err := postOpsBriefWebhook(context.Background(), server.Client(), server.URL, "", []byte(`{}`)); err == nil {
		t.Fatal("unsigned webhook was accepted")
	}
	if err := postOpsBriefWebhook(context.Background(), server.Client(), server.URL, "short", []byte(`{}`)); err == nil {
		t.Fatal("short secret was accepted")
	}
	if hits != 0 {
		t.Fatalf("webhook was called %d times without a valid secret", hits)
	}
}

func TestPostOpsBriefWebhookRejectsNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	if err := postOpsBriefWebhook(context.Background(), server.Client(), server.URL,
		"0123456789abcdef", []byte(`{}`)); err == nil {
		t.Fatal("500 response was treated as delivered")
	}
}

func TestWebhookSignatureBindsTimestampAndBody(t *testing.T) {
	const secret = "0123456789abcdef"
	base := webhookSignature(secret, "1800000000", []byte(`{"a":1}`))
	if base == webhookSignature(secret, "1800000001", []byte(`{"a":1}`)) {
		t.Fatal("signature does not bind the timestamp")
	}
	if base == webhookSignature(secret, "1800000000", []byte(`{"a":2}`)) {
		t.Fatal("signature does not bind the body")
	}
	if base == webhookSignature("fedcba9876543210", "1800000000", []byte(`{"a":1}`)) {
		t.Fatal("signature does not bind the secret")
	}
}
