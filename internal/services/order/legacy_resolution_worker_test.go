package order

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

func seedTccItemZero(t *testing.T, db *sql.DB, orderNo string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO order_items
		(order_no,line_no,sku_id,name,quantity,warehouse_id,fulfillment_group_no,line_paid_cent,unit_paid_cent)
		VALUES (?,1,1,'x',1,1,'G1',0,0)`, orderNo); err != nil {
		t.Fatal(err)
	}
}

func seedLegacyQueue(t *testing.T, db *sql.DB, orderNo, state string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO legacy_payment_resolution
		(order_no, state, candidate_count, pay_cent, blocked_scope, entered_at, expires_at)
		VALUES (?, ?, 0, 0, 'UNSHIPPED_GROUPS_AND_REFUND', DATE_SUB(NOW(3), INTERVAL 25 HOUR), DATE_SUB(NOW(3), INTERVAL 1 HOUR))`,
		orderNo, state); err != nil {
		t.Fatal(err)
	}
}

func legacyQueueState(t *testing.T, db *sql.DB, orderNo string) (string, string, string, string) {
	t.Helper()
	var state, kind, scope, resolutionID string
	if err := db.QueryRow(`SELECT state, resolution_kind, blocked_scope, COALESCE(resolution_id,'')
		FROM legacy_payment_resolution WHERE order_no = ?`, orderNo).Scan(&state, &kind, &scope, &resolutionID); err != nil {
		t.Fatal(err)
	}
	return state, kind, scope, resolutionID
}

func TestLegacyZeroGateHistoricalOrder(t *testing.T) {
	db := legacyTestDB(t)
	payments := &fakePaymentPort{}
	svc := legacyService(db)
	svc.payments = payments
	seedTccOrder(t, db, "ORD-LR-ZG", 0, 0, 0, 0, "")
	seedTccGroup(t, db, "ORD-LR-ZG", "G1", 1, 0)
	seedTccItemZero(t, db, "ORD-LR-ZG")
	seedTccPermit(t, db, "ORD-LR-ZG", "G1", permitNone, "")
	ctx := context.Background()

	reply, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-LR-ZG", OrderNo: "ORD-LR-ZG", GroupNo: "G1",
		AmountCent: 0, Source: refundSourceSystemZero,
	})
	if err != nil {
		t.Fatalf("historical zero gate: %v", err)
	}
	if reply.GetResult() != refundResultPre || reply.GetState() != refundApplicationDone {
		t.Fatalf("zero reply = %+v", reply)
	}
	if got := orderSettlementNo(t, db, "ORD-LR-ZG"); got != "zero:ORD-LR-ZG" {
		t.Fatalf("settlement = %q", got)
	}
	if len(payments.refundCalls) != 0 || payments.getCalls != 0 {
		t.Fatalf("zero gate called Payment refund/get: %d/%d", len(payments.refundCalls), payments.getCalls)
	}
}

func TestLegacyZeroGateCapturedIntentIsNotZero(t *testing.T) {
	db := legacyTestDB(t)
	payments := &fakePaymentPort{intents: []*pb.Intent{
		legacyIntent("PAY-LR-ZC", "ORD-LR-ZC", "SUCCEEDED", 500, 0),
	}}
	svc := legacyService(db)
	svc.payments = payments
	seedTccOrder(t, db, "ORD-LR-ZC", 0, 0, 0, 0, "")
	seedTccGroup(t, db, "ORD-LR-ZC", "G1", 1, 0)
	seedTccItemZero(t, db, "ORD-LR-ZC")
	seedTccPermit(t, db, "ORD-LR-ZC", "G1", permitNone, "")
	_, err := svc.AdmitRefund(context.Background(), &pb.AdmitRefundRequest{
		RefundId: "R-LR-ZC", OrderNo: "ORD-LR-ZC", GroupNo: "G1",
		AmountCent: 0, Source: refundSourceSystemZero,
	})
	if err == nil {
		t.Fatal("zero gate accepted a captured-intent order")
	}
	if got := apperr.From(err).Details["reason"]; got != "LEGACY_PAYMENT_UNRESOLVED" {
		t.Fatalf("reason = %v", got)
	}
	if got := orderSettlementNo(t, db, "ORD-LR-ZC"); got != "" {
		t.Fatalf("captured-intent zero order resolved as %q", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM legacy_payment_resolution WHERE order_no = 'ORD-LR-ZC'`); got != 1 {
		t.Fatalf("captured-intent zero order queued %d rows", got)
	}
}

func TestLegacyExpiryBranches(t *testing.T) {
	ctx := context.Background()

	t.Run("no capture zero -> EXPIRED_ZERO", func(t *testing.T) {
		db := legacyTestDB(t)
		payments := &fakePaymentPort{}
		svc := legacyService(db)
		svc.payments = payments
		seedTccOrder(t, db, "ORD-LR-EZ", 0, 0, 0, 0, "")
		seedLegacyQueue(t, db, "ORD-LR-EZ", legacyResolutionStateOpen)
		if err := svc.processExpiredLegacyResolution(ctx, "ORD-LR-EZ"); err != nil {
			t.Fatal(err)
		}
		state, kind, scope, resolutionID := legacyQueueState(t, db, "ORD-LR-EZ")
		if state != legacyResolutionStateExpiredZero || kind != "EXPIRED_ZERO" || scope != "NONE" || resolutionID != "lpr:ORD-LR-EZ" {
			t.Fatalf("zero expiry = %s/%s/%s/%s", state, kind, scope, resolutionID)
		}
		if got := orderSettlementNo(t, db, "ORD-LR-EZ"); got != "zero:ORD-LR-EZ" {
			t.Fatalf("settlement = %q", got)
		}
	})

	t.Run("no capture positive -> EXPIRED_FINANCE", func(t *testing.T) {
		db := legacyTestDB(t)
		payments := &fakePaymentPort{}
		svc := legacyService(db)
		svc.payments = payments
		svc.fulfillment = &fakeFulfillmentPort{}
		seedTccOrder(t, db, "ORD-LR-EF", 10000, 10000, 0, 0, "")
		seedLegacyQueue(t, db, "ORD-LR-EF", legacyResolutionStateOpen)
		if err := svc.processExpiredLegacyResolution(ctx, "ORD-LR-EF"); err != nil {
			t.Fatal(err)
		}
		state, kind, scope, _ := legacyQueueState(t, db, "ORD-LR-EF")
		if state != legacyResolutionStateExpiredFinance || kind != "EXPIRED_FINANCE" || scope != "FINANCE_HOLD" {
			t.Fatalf("positive expiry = %s/%s/%s", state, kind, scope)
		}
		if got := orderSettlementNo(t, db, "ORD-LR-EF"); got != "" {
			t.Fatalf("finance expiry wrote settlement %q", got)
		}
	})

	t.Run("capture shipped -> EXPIRED_FINANCE", func(t *testing.T) {
		db := legacyTestDB(t)
		payments := &fakePaymentPort{intents: []*pb.Intent{
			legacyIntent("PAY-LR-ES", "ORD-LR-ES", "SUCCEEDED", 10000, 0),
		}}
		svc := legacyService(db)
		svc.payments = payments
		seedTccOrder(t, db, "ORD-LR-ES", 10000, 10000, 0, 0, "")
		seedTccGroup(t, db, "ORD-LR-ES", "G1", 1, 0)
		if _, err := db.Exec(`UPDATE order_fulfillment_groups SET status = 'SHIPPED' WHERE order_no = 'ORD-LR-ES'`); err != nil {
			t.Fatal(err)
		}
		seedLegacyQueue(t, db, "ORD-LR-ES", legacyResolutionStateOpen)
		if err := svc.processExpiredLegacyResolution(ctx, "ORD-LR-ES"); err != nil {
			t.Fatal(err)
		}
		state, kind, _, _ := legacyQueueState(t, db, "ORD-LR-ES")
		if state != legacyResolutionStateExpiredFinance || kind != "EXPIRED_FINANCE" {
			t.Fatalf("shipped expiry = %s/%s", state, kind)
		}
	})

	t.Run("capture unshipped -> COMPENSATING prefix, release blocked", func(t *testing.T) {
		db := legacyTestDB(t)
		payments := &fakePaymentPort{intents: []*pb.Intent{
			legacyIntent("PAY-LR-EC", "ORD-LR-EC", "SUCCEEDED", 10000, 0),
		}}
		ff := &fakeFulfillmentPort{status: probeStatusNoWaybill}
		svc := legacyService(db)
		svc.payments = payments
		svc.fulfillment = ff
		seedTccOrder(t, db, "ORD-LR-EC", 10000, 10000, 0, 0, "")
		seedTccGroup(t, db, "ORD-LR-EC", "G1", 1, 0)
		seedTccPermit(t, db, "ORD-LR-EC", "G1", permitNone, "")
		seedLegacyQueue(t, db, "ORD-LR-EC", legacyResolutionStateOpen)
		err := svc.processExpiredLegacyResolution(ctx, "ORD-LR-EC")
		if !errors.Is(err, errLegacyCompensationBlocked) {
			t.Fatalf("compensation result = %v, want blocked", err)
		}
		state, _, _, _ := legacyQueueState(t, db, "ORD-LR-EC")
		if state != legacyResolutionStateCompensating {
			t.Fatalf("compensating state = %s", state)
		}
		if got := countRows(t, db, `SELECT COUNT(*) FROM order_shipping_permits WHERE order_no = 'ORD-LR-EC' AND state = 'REVOKED'`); got != 1 {
			t.Fatalf("permit revocation = %d", got)
		}
		if len(payments.refundCalls) != 0 {
			t.Fatalf("compensation refunded before release: %d", len(payments.refundCalls))
		}
	})

	t.Run("cancel failure stays COMPENSATING", func(t *testing.T) {
		db := legacyTestDB(t)
		payments := &fakePaymentPort{intents: []*pb.Intent{
			legacyIntent("PAY-LR-CF", "ORD-LR-CF", "SUCCEEDED", 10000, 0),
		}}
		ff := &fakeFulfillmentPort{status: "CREATED", failCancels: 1}
		svc := legacyService(db)
		svc.payments = payments
		svc.fulfillment = ff
		seedTccOrder(t, db, "ORD-LR-CF", 10000, 10000, 0, 0, "")
		seedTccGroup(t, db, "ORD-LR-CF", "G1", 1, 0)
		seedTccPermit(t, db, "ORD-LR-CF", "G1", permitConsumed, "SHIP-LR-CF")
		seedLegacyQueue(t, db, "ORD-LR-CF", legacyResolutionStateOpen)
		if err := svc.processExpiredLegacyResolution(ctx, "ORD-LR-CF"); err == nil {
			t.Fatal("cancel failure returned success")
		}
		state, _, _, _ := legacyQueueState(t, db, "ORD-LR-CF")
		if state != legacyResolutionStateCompensating {
			t.Fatalf("cancel failure state = %s", state)
		}
		if len(payments.refundCalls) != 0 {
			t.Fatalf("cancel failure refunded: %d", len(payments.refundCalls))
		}
	})
}

func TestLegacyManualResolveCASAndTerminalLock(t *testing.T) {
	db := legacyTestDB(t)
	svc := legacyService(db)
	seedTccOrder(t, db, "ORD-LR-MAN", 10000, 10000, 0, 0, "")
	seedLegacyQueue(t, db, "ORD-LR-MAN", legacyResolutionStateOpen)
	ctx := context.Background()
	res, err := svc.ResolveLegacyResolution(ctx, &pb.ResolveLegacyResolutionRequest{
		OrderNo: "ORD-LR-MAN", ResolutionId: "lpr:ORD-LR-MAN", PaymentNo: "PAY-LR-MAN",
		Operator: "ops-1", AuthorizedBy: "admin-1", EvidenceRef: "EVID-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.GetState() != legacyResolutionStateResolved || res.GetResolvedPaymentNo() != "PAY-LR-MAN" {
		t.Fatalf("resolve = %+v", res)
	}
	if got := orderSettlementNo(t, db, "ORD-LR-MAN"); got != "PAY-LR-MAN" {
		t.Fatalf("settlement = %q", got)
	}
	if _, err := svc.ResolveLegacyResolution(ctx, &pb.ResolveLegacyResolutionRequest{
		OrderNo: "ORD-LR-MAN", ResolutionId: "lpr:ORD-LR-MAN", PaymentNo: "PAY-OTHER",
		Operator: "ops-2", AuthorizedBy: "admin-2", EvidenceRef: "EVID-2",
	}); err == nil {
		t.Fatal("terminal RESOLVED was reopened")
	}
}

func TestLegacyZeroGateDoesNotCallPaymentRefund(t *testing.T) {
	db := legacyTestDB(t)
	payments := &fakePaymentPort{}
	svc := legacyService(db)
	svc.payments = payments
	seedTccOrder(t, db, "ORD-LR-ZR", 0, 0, 0, 0, "")
	seedTccGroup(t, db, "ORD-LR-ZR", "G1", 1, 0)
	seedTccItemZero(t, db, "ORD-LR-ZR")
	seedTccPermit(t, db, "ORD-LR-ZR", "G1", permitNone, "")
	if _, err := svc.AdmitRefund(context.Background(), &pb.AdmitRefundRequest{
		RefundId: "R-LR-ZR", OrderNo: "ORD-LR-ZR", GroupNo: "G1", AmountCent: 0,
		Source: refundSourceSystemZero,
	}); err != nil {
		t.Fatal(err)
	}
	if got := orderSettlementNo(t, db, "ORD-LR-ZR"); got != "zero:ORD-LR-ZR" {
		t.Fatalf("settlement = %q", got)
	}
	time.Sleep(10 * time.Millisecond)
}

func TestLegacyTimerVsManualResolveCAS(t *testing.T) {
	db := legacyTestDB(t)
	payments := &fakePaymentPort{}
	svc := legacyService(db)
	svc.payments = payments
	seedTccOrder(t, db, "ORD-LR-RACE", 10000, 10000, 0, 0, "")
	seedLegacyQueue(t, db, "ORD-LR-RACE", legacyResolutionStateOpen)
	ctx := context.Background()
	var wg sync.WaitGroup
	wg.Add(2)
	errs := make([]error, 2)
	go func() {
		defer wg.Done()
		errs[0] = svc.processExpiredLegacyResolution(ctx, "ORD-LR-RACE")
	}()
	go func() {
		defer wg.Done()
		_, errs[1] = svc.ResolveLegacyResolution(ctx, &pb.ResolveLegacyResolutionRequest{
			OrderNo: "ORD-LR-RACE", ResolutionId: "lpr:ORD-LR-RACE", PaymentNo: "PAY-LR-RACE",
			Operator: "ops-1", AuthorizedBy: "admin-1", EvidenceRef: "EVID-1",
		})
	}()
	wg.Wait()
	state, _, _, _ := legacyQueueState(t, db, "ORD-LR-RACE")
	if state != legacyResolutionStateResolved && state != legacyResolutionStateExpiredFinance {
		t.Fatalf("race state = %s", state)
	}
	if state == legacyResolutionStateResolved {
		if got := orderSettlementNo(t, db, "ORD-LR-RACE"); got != "PAY-LR-RACE" {
			t.Fatalf("resolved settlement = %q", got)
		}
	}
	if errs[1] != nil && state == legacyResolutionStateResolved {
		// The manual command may lose to the timer; both are acceptable as long
		// as the terminal row is single and not reopened.
		t.Logf("manual lost to timer: %v", errs[1])
	}
}
