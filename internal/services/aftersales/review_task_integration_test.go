package aftersales

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

// AC-69 / AC-72 / T2 (ADR-0007 Amd1 AC-50 Addendum 0.6 §8.5): AfterSales only
// mirrors the Order review decision on its own task row. It must detect the
// decision from typed details (never the message), write the canonical
// `REVIEW_REQUIRED reason=<CODE> ticket=RVW:<refund_id>` text, keep the task
// PENDING, and never promote a REVIEW task to FAILED_PERMANENT at the retry
// limit. Non-REVIEW failures keep the existing 8-attempt FAILED_PERMANENT path.

func reviewBeforeAdmit() *apperr.Error {
	return apperr.Failed("REFUND_ADMISSION_CONFLICT: consumed permit without shipment; manual review required").
		WithDetails(map[string]any{
			"result":       refundReviewRequired,
			"reason":       refundReviewReasonNoShipment,
			"ticket":       "RVW:AS-REVIEW",
			"manual_queue": "AC-27-NOWB-01",
		})
}

func TestReviewTaskMirror(t *testing.T) {
	db := afterSaleTestDB(t)
	svc, order, _, _ := newAfterSaleTestService(t, db, "PAID")
	as := createAfterSale(t, svc, typeRefundOnly, 600)
	approveAfterSale(t, svc, as.GetAfterSaleNo(), 600)
	// Production path: the Order client returns a gRPC status with ErrorInfo.
	order.blockErr = apperr.GRPC(reviewBeforeAdmit())

	if _, err := svc.Refund(context.Background(), &pb.RefundAfterSaleRequest{AfterSaleNo: as.GetAfterSaleNo()}); err == nil {
		t.Fatal("review refund: expected an error")
	}
	var status, lastError string
	var attempts int
	if err := db.QueryRow(`SELECT status, attempts, last_error FROM after_sale_refund_tasks WHERE after_sale_no = ?`,
		as.GetAfterSaleNo()).Scan(&status, &attempts, &lastError); err != nil {
		t.Fatal(err)
	}
	if status != taskPending {
		t.Fatalf("review task status = %s, want PENDING", status)
	}
	want := refundReviewCanonical(as.GetAfterSaleNo(), refundReviewReasonNoShipment, refundTaskLastErrorWidth)
	if lastError != want {
		t.Fatalf("last_error = %q, want canonical %q", lastError, want)
	}
	// O23: the single parser accepts the written canonical text.
	reason, ticket, truncated, ok := parseRefundReviewLastError(lastError)
	if !ok || reason != refundReviewReasonNoShipment || ticket != "RVW:"+as.GetAfterSaleNo() || truncated {
		t.Fatalf("parse(%q) = %q/%q/%v/%v", lastError, reason, ticket, truncated, ok)
	}
}

func TestReviewTaskRetryStaysPending(t *testing.T) {
	db := afterSaleTestDB(t)
	svc, order, _, _ := newAfterSaleTestService(t, db, "PAID")
	as := createAfterSale(t, svc, typeRefundOnly, 600)
	approveAfterSale(t, svc, as.GetAfterSaleNo(), 600)
	order.blockErr = reviewBeforeAdmit()
	// The first Refund call creates the durable task (and already runs once).
	_, _ = svc.Refund(context.Background(), &pb.RefundAfterSaleRequest{AfterSaleNo: as.GetAfterSaleNo()})
	// Drive the worker to the retry limit. Every attempt re-reads the same
	// persisted review; the task must stay PENDING and attempts must reach 8.
	for attempt := 2; attempt <= maxTaskAttempts; attempt++ {
		_ = svc.processRefundTask(context.Background(), as.GetAfterSaleNo())
	}
	var status, lastError string
	var attempts int
	if err := db.QueryRow(`SELECT status, attempts, last_error FROM after_sale_refund_tasks WHERE after_sale_no = ?`,
		as.GetAfterSaleNo()).Scan(&status, &attempts, &lastError); err != nil {
		t.Fatal(err)
	}
	if status != taskPending || attempts != maxTaskAttempts {
		t.Fatalf("review task = %s/%d, want PENDING/%d", status, attempts, maxTaskAttempts)
	}
	if lastError != refundReviewCanonical(as.GetAfterSaleNo(), refundReviewReasonNoShipment, refundTaskLastErrorWidth) {
		t.Fatalf("review task last_error = %q", lastError)
	}
	if got := countAfterSaleRows(t, db, `SELECT COUNT(*) FROM after_sales WHERE after_sale_no = ? AND status = ?`,
		as.GetAfterSaleNo(), statusRefunded); got != 0 {
		t.Fatal("review task was marked REFUNDED")
	}
	// The scanner must not auto-claim the task beyond the retry limit.
	before := countAfterSaleRows(t, db, `SELECT COUNT(*) FROM after_sales WHERE after_sale_no = ?`, as.GetAfterSaleNo())
	runRefundBatch(context.Background(), db, svc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	var afterAttempts int
	if err := db.QueryRow(`SELECT attempts FROM after_sale_refund_tasks WHERE after_sale_no = ?`, as.GetAfterSaleNo()).
		Scan(&afterAttempts); err != nil {
		t.Fatal(err)
	}
	if afterAttempts != maxTaskAttempts {
		t.Fatalf("scanner re-claimed the review task: attempts %d -> %d", maxTaskAttempts, afterAttempts)
	}
	if before != 1 {
		t.Fatalf("after-sale rows changed: %d", before)
	}
}

func TestReviewTaskNonReviewKeepsPermanentFailure(t *testing.T) {
	db := afterSaleTestDB(t)
	svc, order, _, _ := newAfterSaleTestService(t, db, "PAID")
	as := createAfterSale(t, svc, typeRefundOnly, 600)
	approveAfterSale(t, svc, as.GetAfterSaleNo(), 600)
	order.blockErr = apperr.Unavailable("order temporarily unavailable", errors.New("dial timeout"))
	_, _ = svc.Refund(context.Background(), &pb.RefundAfterSaleRequest{AfterSaleNo: as.GetAfterSaleNo()})
	for attempt := 2; attempt <= maxTaskAttempts; attempt++ {
		_ = svc.processRefundTask(context.Background(), as.GetAfterSaleNo())
	}
	var status, lastError string
	var attempts int
	if err := db.QueryRow(`SELECT status, attempts, last_error FROM after_sale_refund_tasks WHERE after_sale_no = ?`,
		as.GetAfterSaleNo()).Scan(&status, &attempts, &lastError); err != nil {
		t.Fatal(err)
	}
	if status != taskFailed || attempts != maxTaskAttempts {
		t.Fatalf("non-review task = %s/%d, want FAILED_PERMANENT/%d", status, attempts, maxTaskAttempts)
	}
	if strings.HasPrefix(lastError, "REVIEW_REQUIRED") {
		t.Fatalf("non-review task got a review mirror: %q", lastError)
	}
}

func TestReviewCanonicalParseAndTruncate(t *testing.T) {
	// O23: exact canonical text, deterministic truncation and rejection rules.
	longID := strings.Repeat("r", 600)
	text := refundReviewCanonical(longID, refundReviewReasonNoShipment, refundTaskLastErrorWidth)
	if len([]rune(text)) > refundTaskLastErrorWidth {
		t.Fatalf("truncated text length = %d, want <= %d", len([]rune(text)), refundTaskLastErrorWidth)
	}
	if !strings.HasSuffix(text, " truncated=1") {
		t.Fatalf("truncated text = %q, want trailing ' truncated=1'", text)
	}
	if !strings.HasPrefix(text, "REVIEW_REQUIRED reason="+refundReviewReasonNoShipment+" ticket=RVW:") {
		t.Fatalf("truncated text lost the prefix: %q", text)
	}
	reason, ticket, truncated, ok := parseRefundReviewLastError(text)
	if !ok || reason != refundReviewReasonNoShipment || !truncated || !strings.HasPrefix(ticket, "RVW:") {
		t.Fatalf("parse(truncated) = %q/%q/%v/%v", reason, ticket, truncated, ok)
	}
	for _, invalid := range []string{
		"",
		"REVIEW_REQUIRED reason=CONSUMED_WITHOUT_SHIPMENT ticket=RVW:", // empty ticket
		"REVIEW_REQUIRED reason=CONSUMED_WITHOUT_SHIPMENT ticket=RVW:X arm=1",
		"REVIEW_REQUIRED reason=UNKNOWN ticket=RVW:X",
		"REVIEW_REQUIRED reason=CONSUMED_WITHOUT_SHIPMENT ticket=RVW:X ", // trailing space
		"REVIEW_REQUIRED: reason=CONSUMED_WITHOUT_SHIPMENT ticket=RVW:X",
	} {
		if _, _, _, ok := parseRefundReviewLastError(invalid); ok {
			t.Fatalf("parser accepted invalid text %q", invalid)
		}
	}
}

func countAfterSaleRows(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var count int
	if err := db.QueryRow(query, args...).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
