package order

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/events"

	"google.golang.org/grpc"
)

// T0c (ADR-0007 Amd1 AC-50 Addendum 0.6 A8/A12, W5/W13): the Tx2 CAS is
// `state='PENDING_REMOTE' AND result <> 'REVIEW_REQUIRED'`. A REVIEW_REQUIRED
// row is immutable to every automatic path: a late CANCELLED probe or a
// concurrent loser must re-read the persisted review instead of writing PRE.

type ac50RaceProbe struct {
	status   string
	onCancel func(refundID string)
	calls    atomic.Int64
}

func (p *ac50RaceProbe) CancelIfCreated(_ context.Context, in *pb.CancelIfCreatedRequest, _ ...grpc.CallOption) (*pb.CancelIfCreatedReply, error) {
	p.calls.Add(1)
	if p.onCancel != nil {
		p.onCancel(in.GetRefundId())
	}
	return &pb.CancelIfCreatedReply{Status: p.status}, nil
}

func (p *ac50RaceProbe) ListShipments(context.Context, *pb.ListShipmentsRequest, ...grpc.CallOption) (*pb.ShipmentList, error) {
	return &pb.ShipmentList{}, nil
}

func ac50RunConcurrent(t *testing.T, svc *Service, req *pb.AdmitRefundRequest, n int) ([]*pb.AdmitRefundReply, []error) {
	t.Helper()
	replies := make([]*pb.AdmitRefundReply, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			replies[i], errs[i] = svc.AdmitRefund(context.Background(), req)
		}(i)
	}
	wg.Wait()
	return replies, errs
}

// O11/W5: concurrent `<NO_WAYBILL>` admissions converge on exactly one
// REVIEW_REQUIRED row; both callers get the same bounded conflict and no
// release/terminal outbox is written.
func TestAC50ReviewConcurrent(t *testing.T) {
	db := orderTestDB(t)
	ff := &ac50RaceProbe{status: probeStatusNoWaybill}
	svc := ac50Service(db, &fakePaymentPort{captured: 10000}, ff)
	ac50SeedOrder(t, db, "ORD-RC50", 10000)
	ac50SeedPermit(t, db, "ORD-RC50", permitConsumed)
	req := &pb.AdmitRefundRequest{
		RefundId: "R-RC50", OrderNo: "ORD-RC50", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
	}
	_, errs := ac50RunConcurrent(t, svc, req, 2)
	for i, err := range errs {
		if err == nil {
			t.Fatalf("goroutine %d: expected REFUND_ADMISSION_CONFLICT", i)
		}
		ac50RequireReviewConflict(t, err, "R-RC50")
	}
	result, state, _ := ac50Admission(t, db, "R-RC50")
	if result != refundReviewRequired || state != refundPending {
		t.Fatalf("admission = %s/%s, want REVIEW_REQUIRED/PENDING_REMOTE", result, state)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_admissions WHERE refund_id = 'R-RC50'`); got != 1 {
		t.Fatalf("admission rows = %d, want 1", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events
		WHERE aggregate_id = 'ORD-RC50' AND event_type = 'order.refund_admitted'`); got != 1 {
		t.Fatalf("refund_admitted events = %d, want 1 placeholder", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events
		WHERE aggregate_id = 'ORD-RC50' AND event_type = 'order.refund_admitted'
		  AND JSON_UNQUOTE(JSON_EXTRACT(payload, '$.state')) IN (?, ?)`, refundResultPre, refundResultPost); got != 0 {
		t.Fatalf("terminal refund_admitted events = %d, want 0", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events
		WHERE aggregate_id = 'ORD-RC50' AND event_type = ?`, events.RefundReleaseEventType); got != 0 {
		t.Fatalf("inventory release events = %d, want 0", got)
	}
	if ff.calls.Load() < 1 {
		t.Fatal("concurrent review never probed CancelIfCreated")
	}
}

// O11b: concurrent real CANCELLED probes converge on one PRE admission, one
// terminal outbox and at most one release; the loser reads the persisted PRE.
func TestAC50ReviewConcurrentCancelled(t *testing.T) {
	db := orderTestDB(t)
	ff := &ac50RaceProbe{status: "CANCELLED"}
	svc := ac50Service(db, &fakePaymentPort{captured: 10000}, ff)
	ac50SeedOrder(t, db, "ORD-RC51", 10000)
	ac50SeedPermit(t, db, "ORD-RC51", permitConsumed)
	req := &pb.AdmitRefundRequest{
		RefundId: "R-RC51", OrderNo: "ORD-RC51", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
	}
	replies, errs := ac50RunConcurrent(t, svc, req, 2)
	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: %v", i, err)
		}
		if replies[i].GetResult() != refundResultPre {
			t.Fatalf("goroutine %d result = %q, want PRE", i, replies[i].GetResult())
		}
	}
	result, state, _ := ac50Admission(t, db, "R-RC51")
	if result != refundResultPre || state != refundResultPre {
		t.Fatalf("admission = %s/%s, want PRE/PRE", result, state)
	}
	// Tx1 writes one placeholder and Tx2 writes one terminal PRE event.
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events
		WHERE aggregate_id = 'ORD-RC51' AND event_type = 'order.refund_admitted'
		  AND JSON_UNQUOTE(JSON_EXTRACT(payload, '$.state')) = ?`, refundResultPre); got != 1 {
		t.Fatalf("terminal refund_admitted events = %d, want 1", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events
		WHERE aggregate_id = 'ORD-RC51' AND event_type = 'order.refund_admitted'
		  AND JSON_UNQUOTE(JSON_EXTRACT(payload, '$.state')) = ?`, refundPending); got != 1 {
		t.Fatalf("placeholder refund_admitted events = %d, want 1", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events
		WHERE aggregate_id = 'ORD-RC51' AND event_type = ?`, events.RefundReleaseEventType); got != 1 {
		t.Fatalf("inventory release events = %d, want 1", got)
	}
}

// O11c: a Tx2 whose admission row disappeared (0 rows) returns a Conflict and
// writes no outbox/release; it must not panic or treat 0 rows as success.
func TestAC50ReviewTx2Disappeared(t *testing.T) {
	db := orderTestDB(t)
	ac50SeedOrder(t, db, "ORD-RC52", 10000)
	if _, err := db.Exec(`INSERT INTO order_refund_admissions
		(refund_id, order_no, group_no, result, state, amount_cent, version)
		VALUES ('R-RC52','ORD-RC52','G1',?,?,10000,1)`, refundResultPost, refundPending); err != nil {
		t.Fatal(err)
	}
	ff := &ac50RaceProbe{status: "CANCELLED", onCancel: func(refundID string) {
		if _, err := db.Exec(`DELETE FROM order_refund_admissions WHERE refund_id = ?`, refundID); err != nil {
			t.Errorf("delete admission: %v", err)
		}
	}}
	svc := ac50Service(db, &fakePaymentPort{captured: 10000}, ff)
	_, err := svc.AdmitRefund(context.Background(), &pb.AdmitRefundRequest{
		RefundId: "R-RC52", OrderNo: "ORD-RC52", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
	})
	if err == nil {
		t.Fatal("disappeared admission: expected Conflict")
	}
	ae := apperr.From(err)
	if ae == nil || ae.Code != apperr.CodeConflict || !strings.Contains(ae.Message, "admission disappeared") {
		t.Fatalf("disappeared admission error = %v, want CONFLICT admission disappeared", err)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-RC52'`); got != 0 {
		t.Fatalf("0-row Tx2 wrote %d outbox events", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events
		WHERE aggregate_id = 'ORD-RC52' AND event_type = ?`, events.RefundReleaseEventType); got != 0 {
		t.Fatalf("0-row Tx2 wrote %d release events", got)
	}
}

// O11d/W13: a late CANCELLED probe arriving after REVIEW_REQUIRED must not wash
// the review into PRE. The double-predicate CAS matches no row; the read-back
// returns the same conflict and the admission row stays REVIEW/PENDING.
func TestAC50ReviewLateCancelDoesNotWash(t *testing.T) {
	db := orderTestDB(t)
	ac50SeedOrder(t, db, "ORD-RC53", 10000)
	if _, err := db.Exec(`INSERT INTO order_refund_admissions
		(refund_id, order_no, group_no, result, state, amount_cent, version)
		VALUES ('R-RC53','ORD-RC53','G1',?,?,10000,1)`, refundReviewRequired, refundPending); err != nil {
		t.Fatal(err)
	}
	ff := &ac50RaceProbe{status: "CANCELLED"}
	svc := ac50Service(db, &fakePaymentPort{captured: 10000}, ff)
	ctx := context.Background()
	row, err := findRefundAdmission(ctx, db, "R-RC53")
	if err != nil || row == nil {
		t.Fatalf("load review row: %v", err)
	}
	if _, err := svc.finishRemoteAdmission(ctx, row, "aftersale"); err == nil {
		t.Fatal("late CANCELLED: expected the persisted REVIEW conflict")
	} else {
		ac50RequireReviewConflict(t, err, "R-RC53")
	}
	result, state, version := ac50Admission(t, db, "R-RC53")
	if result != refundReviewRequired || state != refundPending || version != 1 {
		t.Fatalf("late probe mutated the review: %s/%s v%d", result, state, version)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-RC53'`); got != 0 {
		t.Fatalf("late probe wrote %d outbox events", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events
		WHERE aggregate_id = 'ORD-RC53' AND event_type = ?`, events.RefundReleaseEventType); got != 0 {
		t.Fatalf("late probe wrote %d release events", got)
	}
}
