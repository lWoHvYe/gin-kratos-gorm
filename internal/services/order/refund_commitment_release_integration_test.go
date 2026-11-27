package order

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"testing"
	"time"
)

func seedReleaseCommitment(t *testing.T, db *sql.DB, orderNo, groupNo, refundID string, amount int64, state string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO order_refund_commitments
		(order_no, group_no, refund_id, amount_cent, state, version)
		VALUES (?, ?, ?, ?, ?, 1)`, orderNo, groupNo, refundID, amount, state); err != nil {
		t.Fatal(err)
	}
}

func releaseRequest(refundID string, amount int64) RefundCommitmentReleaseRequest {
	now := time.Now().UTC()
	return RefundCommitmentReleaseRequest{
		ReleaseID: "REL-" + refundID, RefundID: refundID, OrderNo: "ORD-REL", GroupNo: "G1",
		AmountCent: amount, AuthorizationID: "AUTH-" + refundID,
		Operator: "ops", AuthorizedBy: "lead", ReasonCode: "MANUAL_ABANDON",
		EvidenceRef: "OPS-TICKET-1", AuthorizedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour),
	}
}

func TestReleaseCommitmentSuccessAndIdempotent(t *testing.T) {
	db := orderTestDB(t)
	svc := legacyService(db)
	seedReleaseOrder(t, db, "ORD-REL", 1000, 1000, "PAY-REL", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 1000, groupNo: "G1"},
	}, permitRevoked)
	seedReleaseGroup(t, db, "ORD-REL", "G1", 1, "SHIPPED", refundStateNone)
	seedReleaseCommitment(t, db, "ORD-REL", "G1", "R-REL", 1000, commitmentReserved)

	req := releaseRequest("R-REL", 1000)
	res, err := svc.ReleaseRefundCommitment(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.State != commitmentReleased || res.Version != 2 || res.AlreadyReleased {
		t.Fatalf("release result = %+v", res)
	}
	var state string
	var version int64
	if err := db.QueryRow(`SELECT state, version FROM order_refund_commitments WHERE refund_id = 'R-REL'`).
		Scan(&state, &version); err != nil {
		t.Fatal(err)
	}
	if state != commitmentReleased || version != 2 {
		t.Fatalf("commitment = %s/%d", state, version)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_commitment_releases WHERE refund_id = 'R-REL'`); got != 1 {
		t.Fatalf("release rows = %d", got)
	}

	again, err := svc.ReleaseRefundCommitment(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !again.AlreadyReleased || again.Version != 2 {
		t.Fatalf("idempotent result = %+v", again)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_commitment_releases WHERE refund_id = 'R-REL'`); got != 1 {
		t.Fatalf("idempotent release rows = %d", got)
	}
	changed := req
	changed.ExpiresAt = req.ExpiresAt.Add(time.Hour)
	if _, err := svc.ReleaseRefundCommitment(context.Background(), changed); err == nil {
		t.Fatal("release_id with different authorization was accepted")
	}
	changed = req
	changed.ReviewedAt = time.Now().UTC()
	if _, err := svc.ReleaseRefundCommitment(context.Background(), changed); err == nil {
		t.Fatal("release_id with a different reviewed_at was accepted")
	}
	// AC-62/INV-45: only quota is released; the order, permit and group stay.
	if got := orderStatus(t, db, "ORD-REL"); got != statusPaid {
		t.Fatalf("order status = %q, want PAID", got)
	}
	if got := orderSettlementNo(t, db, "ORD-REL"); got != "PAY-REL" {
		t.Fatalf("settlement = %q", got)
	}
	assertPermitState(t, db, "ORD-REL", "G1", permitRevoked)
	var groupStatus string
	if err := db.QueryRow(`SELECT status FROM order_fulfillment_groups WHERE order_no = 'ORD-REL' AND group_no = 'G1'`).
		Scan(&groupStatus); err != nil {
		t.Fatal(err)
	}
	if groupStatus != "SHIPPED" {
		t.Fatalf("group status = %q, want SHIPPED", groupStatus)
	}
	var refunded int64
	if err := db.QueryRow(`SELECT refunded_cent FROM orders WHERE order_no = 'ORD-REL'`).Scan(&refunded); err != nil {
		t.Fatal(err)
	}
	if refunded != 0 {
		t.Fatalf("order refunded_cent = %d, want 0", refunded)
	}
}

func TestReleaseCommitmentRejectsSettled(t *testing.T) {
	db := orderTestDB(t)
	svc := legacyService(db)
	seedReleaseCommitment(t, db, "ORD-REL", "G1", "R-SETTLED", 1000, commitmentSettled)
	if _, err := svc.ReleaseRefundCommitment(context.Background(), releaseRequest("R-SETTLED", 1000)); err == nil ||
		!strings.Contains(err.Error(), "REFUND_COMMITMENT_CONFLICT") {
		t.Fatalf("settled release error = %v", err)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_commitment_releases WHERE refund_id = 'R-SETTLED'`); got != 0 {
		t.Fatalf("settled release rows = %d", got)
	}
}

func TestReleaseCommitmentSecondReview(t *testing.T) {
	db := orderTestDB(t)
	svc := legacyService(db)
	seedReleaseCommitment(t, db, "ORD-REL", "G1", "R-BIG", 100000, commitmentReserved)

	req := releaseRequest("R-BIG", 100000)
	if _, err := svc.ReleaseRefundCommitment(context.Background(), req); err == nil ||
		!strings.Contains(err.Error(), "REFUND_RELEASE_NOT_AUTHORIZED") {
		t.Fatalf("missing reviewer error = %v", err)
	}
	req.ReviewedBy = req.AuthorizedBy
	if _, err := svc.ReleaseRefundCommitment(context.Background(), req); err == nil {
		t.Fatal("same-person review was accepted")
	}
	req.ReviewedBy = "director"
	req.ReviewedAt = time.Now().UTC().Add(-30 * time.Minute)
	if _, err := svc.ReleaseRefundCommitment(context.Background(), req); err != nil {
		t.Fatal(err)
	}
}

func TestReleaseCommitmentExpiredAndMismatch(t *testing.T) {
	db := orderTestDB(t)
	svc := legacyService(db)
	seedReleaseCommitment(t, db, "ORD-REL", "G1", "R-BAD", 1000, commitmentReserved)

	req := releaseRequest("R-BAD", 1000)
	req.ExpiresAt = time.Now().UTC().Add(-time.Minute)
	if _, err := svc.ReleaseRefundCommitment(context.Background(), req); err == nil ||
		!strings.Contains(err.Error(), "REFUND_RELEASE_NOT_AUTHORIZED") {
		t.Fatalf("expired release error = %v", err)
	}

	req = releaseRequest("R-BAD", 1000)
	req.AmountCent = 999
	if _, err := svc.ReleaseRefundCommitment(context.Background(), req); err == nil ||
		!strings.Contains(err.Error(), "REFUND_COMMITMENT_CONFLICT") {
		t.Fatalf("amount mismatch error = %v", err)
	}
	req = releaseRequest("R-BAD", 1000)
	req.OrderNo = "ORD-OTHER"
	if _, err := svc.ReleaseRefundCommitment(context.Background(), req); err == nil {
		t.Fatal("order mismatch was accepted")
	}
	req = releaseRequest("R-BAD", 1000)
	req.GroupNo = "G2"
	if _, err := svc.ReleaseRefundCommitment(context.Background(), req); err == nil {
		t.Fatal("group mismatch was accepted")
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_commitment_releases WHERE refund_id = 'R-BAD'`); got != 0 {
		t.Fatalf("mismatch release rows = %d", got)
	}
}

func TestReleaseCommitmentTrigger(t *testing.T) {
	db := orderTestDB(t)
	svc := legacyService(db)
	seedReleaseCommitment(t, db, "ORD-REL", "G1", "R-TRIGGER", 1000, commitmentReserved)

	req := releaseRequest("R-TRIGGER", 1000)
	req.ReasonCode = "BUSINESS_REJECT"
	if _, err := svc.ReleaseRefundCommitment(context.Background(), req); err == nil {
		t.Fatal("BUSINESS_REJECT without an application was accepted")
	}
	if _, err := db.Exec(`INSERT INTO refund_applications (refund_id, source, order_no, state)
		VALUES ('R-TRIGGER','aftersale','ORD-REL','PENDING')`); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReleaseRefundCommitment(context.Background(), req); err == nil {
		t.Fatal("PENDING application trigger was accepted")
	}
	if _, err := db.Exec(`UPDATE refund_applications SET state = 'REJECTED' WHERE refund_id = 'R-TRIGGER'`); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReleaseRefundCommitment(context.Background(), req); err != nil {
		t.Fatal(err)
	}
}

func TestReleaseCommitmentDryRunWritesNothing(t *testing.T) {
	db := orderTestDB(t)
	svc := legacyService(db)
	seedReleaseCommitment(t, db, "ORD-REL", "G1", "R-DRY", 1000, commitmentReserved)
	req := releaseRequest("R-DRY", 1000)
	req.DryRun = true
	res, err := svc.ReleaseRefundCommitment(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.State != commitmentReserved || res.AlreadyReleased {
		t.Fatalf("dry-run result = %+v", res)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_commitment_releases WHERE refund_id = 'R-DRY'`); got != 0 {
		t.Fatalf("dry-run release rows = %d", got)
	}
	var state string
	if err := db.QueryRow(`SELECT state FROM order_refund_commitments WHERE refund_id = 'R-DRY'`).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != commitmentReserved {
		t.Fatalf("dry-run state = %s", state)
	}
}

func TestReleaseCommitmentConcurrentOnlyOne(t *testing.T) {
	db := orderTestDB(t)
	svc := legacyService(db)
	seedReleaseCommitment(t, db, "ORD-REL", "G1", "R-CONC", 1000, commitmentReserved)
	reqA := releaseRequest("R-CONC", 1000)
	reqB := releaseRequest("R-CONC", 1000)
	reqA.ReleaseID = "REL-CONC-A"
	reqA.AuthorizationID = "AUTH-CONC-A"
	reqB.ReleaseID = "REL-CONC-B"
	reqB.AuthorizationID = "AUTH-CONC-B"

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, req := range []RefundCommitmentReleaseRequest{reqA, reqB} {
		wg.Add(1)
		go func(index int, request RefundCommitmentReleaseRequest) {
			defer wg.Done()
			_, errs[index] = svc.ReleaseRefundCommitment(context.Background(), request)
		}(i, req)
	}
	wg.Wait()
	successes := 0
	for _, err := range errs {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent successes = %d, errs = %v", successes, errs)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_commitment_releases WHERE refund_id = 'R-CONC'`); got != 1 {
		t.Fatalf("concurrent release rows = %d", got)
	}
}

func TestReleaseCommitmentTriggerVariants(t *testing.T) {
	db := orderTestDB(t)
	svc := legacyService(db)
	seedReleaseCommitment(t, db, "ORD-REL", "G1", "R-WITHDRAW", 1000, commitmentReserved)
	if _, err := db.Exec(`INSERT INTO refund_applications (refund_id, source, order_no, state)
		VALUES ('R-WITHDRAW','aftersale','ORD-REL','CANCELLED')`); err != nil {
		t.Fatal(err)
	}
	req := releaseRequest("R-WITHDRAW", 1000)
	req.ReasonCode = "USER_WITHDRAW"
	if _, err := svc.ReleaseRefundCommitment(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	seedReleaseCommitment(t, db, "ORD-REL", "G1", "R-FAILED", 1000, commitmentReserved)
	failed := releaseRequest("R-FAILED", 1000)
	failed.ReasonCode = "FAILED_PERMANENT"
	if _, err := svc.ReleaseRefundCommitment(context.Background(), failed); err != nil {
		t.Fatal(err)
	}
}

func TestReleaseCommitmentAuthorizationReuse(t *testing.T) {
	db := orderTestDB(t)
	svc := legacyService(db)
	seedReleaseCommitment(t, db, "ORD-REL", "G1", "R-AUTH1", 1000, commitmentReserved)
	seedReleaseCommitment(t, db, "ORD-REL", "G1", "R-AUTH2", 1000, commitmentReserved)
	first := releaseRequest("R-AUTH1", 1000)
	first.AuthorizationID = "AUTH-SHARED"
	if _, err := svc.ReleaseRefundCommitment(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	second := releaseRequest("R-AUTH2", 1000)
	second.AuthorizationID = "AUTH-SHARED"
	if _, err := svc.ReleaseRefundCommitment(context.Background(), second); err == nil {
		t.Fatal("authorization_id reuse was accepted")
	}
}
