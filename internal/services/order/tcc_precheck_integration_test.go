package order

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"testing"

	"gin-kratos-gorm/internal/platform/apperr"
)

// T1.1 evidence: AC-65 / INV-48 Σ precheck. The helper must be read-only, must
// take the Payment snapshot outside the transaction, and must be reusable by
// Tx1 with the same snapshot.

func tccTestService(t *testing.T, db *sql.DB, payments paymentPort) *Service {
	t.Helper()
	// T1.2b R-2: Tx1 now needs the Fulfillment shipment snapshot outside the
	// transaction. The default fake returns "no shipment facts", which is the
	// normal case for a group that has not left the warehouse.
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(payments, &fakeFulfillmentPort{status: "CANCELLED"})
	return svc
}

func tccTestServiceWithFulfillment(t *testing.T, db *sql.DB, payments paymentPort, fulfillment fulfillmentPort) *Service {
	t.Helper()
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(payments, fulfillment)
	return svc
}

func seedTccOrder(t *testing.T, db *sql.DB, orderNo string, payCent, paidCent, freightCent, refunded int64, paymentNo string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO orders
		(order_no,user_id,status,total_cent,discount_cent,promotion_discount_cent,applied_promotions_json,
		 freight_cent,pay_cent,paid_cent,settlement_payment_no,refunded_cent,refund_status,refund_state,
		 receiver_name,receiver_phone,receiver_address,quote_id,client_request_id,
		 allocation_frozen_at,allocation_hash,package_count,warehouse_count,expire_at,created_at)
		VALUES (?,1,'PAID',?,0,0,'[]',?,?,?,?,?,'','NONE','A','1','R','','',NOW(3),'h',1,1,NOW(3),NOW(3))`,
		orderNo, payCent, freightCent, payCent, paidCent, paymentNo, refunded); err != nil {
		t.Fatal(err)
	}
}

func seedTccGroup(t *testing.T, db *sql.DB, orderNo, groupNo string, seq int, freightShare int64) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO order_fulfillment_groups
		(group_no, order_no, seq, warehouse_id, status, refund_state, reservation_state,
		 freight_cent, freight_share_cent, version)
		VALUES (?, ?, ?, ?, 'ALLOCATED', 'NONE', 'HELD', ?, ?, 1)`,
		groupNo, orderNo, seq, seq, freightShare, freightShare); err != nil {
		t.Fatal(err)
	}
}

func seedTccLine(t *testing.T, db *sql.DB, orderNo string, lineNo int, groupNo string, linePaid int64) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO order_items
		(order_no,line_no,sku_id,name,quantity,warehouse_id,fulfillment_group_no,line_paid_cent,unit_paid_cent)
		VALUES (?, ?, ?, 'x', 1, 1, ?, ?, ?)`,
		orderNo, lineNo, lineNo, groupNo, linePaid, linePaid); err != nil {
		t.Fatal(err)
	}
}

func seedTccCommitment(t *testing.T, db *sql.DB, orderNo, groupNo, refundID string, amount int64, state string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO order_refund_commitments
		(order_no,group_no,refund_id,amount_cent,state) VALUES (?, ?, ?, ?, ?)`,
		orderNo, groupNo, refundID, amount, state); err != nil {
		t.Fatal(err)
	}
}

// tccNoWrites proves the precheck changed no table/event/state.
func tccNoWrites(t *testing.T, db *sql.DB, orderNo string, versionBefore int64, wantStatus string) {
	t.Helper()
	var version int64
	var status, paymentNo, refundState string
	if err := db.QueryRow(`SELECT version, status, settlement_payment_no, refund_state
		FROM orders WHERE order_no = ?`, orderNo).Scan(&version, &status, &paymentNo, &refundState); err != nil {
		t.Fatal(err)
	}
	if version != versionBefore {
		t.Fatalf("orders.version changed %d -> %d", versionBefore, version)
	}
	if status != wantStatus || refundState != refundStateNone {
		t.Fatalf("order state mutated: status=%q (want %q) refund_state=%q", status, wantStatus, refundState)
	}
	for _, query := range []string{
		`SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = ?`,
		`SELECT COUNT(*) FROM order_refund_admissions WHERE order_no = ?`,
		`SELECT COUNT(*) FROM order_shipping_permits WHERE order_no = ?`,
	} {
		var count int
		if err := db.QueryRow(query, orderNo).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("precheck wrote rows (%s -> %d)", query, count)
		}
	}
}

func tccVersion(t *testing.T, db *sql.DB, orderNo string) int64 {
	t.Helper()
	var version int64
	if err := db.QueryRow(`SELECT version FROM orders WHERE order_no = ?`, orderNo).Scan(&version); err != nil {
		t.Fatal(err)
	}
	return version
}

func TestTccSumPrecheckMatchesMultiGroupOrder(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 10000, refunded: 0})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-SUM1", 10000, 10000, 500, 0, "PAY-SUM1")
	seedTccGroup(t, db, "ORD-SUM1", "G1", 1, 300)
	seedTccGroup(t, db, "ORD-SUM1", "G2", 2, 200)
	seedTccLine(t, db, "ORD-SUM1", 1, "G1", 5500)
	seedTccLine(t, db, "ORD-SUM1", 2, "G2", 4000)
	version := tccVersion(t, db, "ORD-SUM1")

	result, err := svc.precheckTccSum(ctx, "ORD-SUM1", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.SumGroupCapCent != 10000 || result.RemainingCapturedCent != 10000 {
		t.Fatalf("sum=%d remaining=%d, want 10000/10000", result.SumGroupCapCent, result.RemainingCapturedCent)
	}
	if result.ActiveGroupCount != 2 || result.NoCaptured || result.LegacyG1 {
		t.Fatalf("result=%+v", result)
	}
	caps := map[string]int64{}
	for _, group := range result.Groups {
		caps[group.GroupNo] = group.CapCent
	}
	if caps["G1"] != 5800 || caps["G2"] != 4200 {
		t.Fatalf("group caps=%v, want G1=5800 G2=4200", caps)
	}
	tccNoWrites(t, db, "ORD-SUM1", version, statusPaid)
}

func TestTccSumPrecheckMismatchBlocksWithoutStateChange(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 10000})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-SUM2", 10000, 10000, 500, 0, "PAY-SUM2")
	seedTccGroup(t, db, "ORD-SUM2", "G1", 1, 300)
	seedTccGroup(t, db, "ORD-SUM2", "G2", 2, 200)
	seedTccLine(t, db, "ORD-SUM2", 1, "G1", 5000)
	seedTccLine(t, db, "ORD-SUM2", 2, "G2", 3500) // Σ = 5000+300+3500+200 = 9000
	version := tccVersion(t, db, "ORD-SUM2")

	result, err := svc.precheckTccSum(ctx, "ORD-SUM2", "")
	if err == nil {
		t.Fatal("Σ mismatch was not rejected")
	}
	if !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("error code=%v, want FAILED_PRECONDITION: %v", apperr.From(err).Code, err)
	}
	details := apperr.From(err).Details
	if details["sum_group_cap_cent"] != int64(9000) || details["remaining_captured_cent"] != int64(10000) || details["diff_cent"] != int64(-1000) {
		t.Fatalf("details=%v", details)
	}
	if result == nil || result.DiffCent() != -1000 {
		t.Fatalf("result=%+v, want computed numbers on mismatch", result)
	}
	tccNoWrites(t, db, "ORD-SUM2", version, statusPaid)
}

func TestTccSumPrecheckPartiallyRefundedGroup(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 10000, refunded: 3000})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-SUM3", 10000, 10000, 0, 3000, "PAY-SUM3")
	seedTccGroup(t, db, "ORD-SUM3", "G1", 1, 0)
	seedTccLine(t, db, "ORD-SUM3", 1, "G1", 10000)
	seedTccCommitment(t, db, "ORD-SUM3", "G1", "R-SUM3", 3000, "SETTLED")
	version := tccVersion(t, db, "ORD-SUM3")

	result, err := svc.precheckTccSum(ctx, "ORD-SUM3", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.SumGroupCapCent != 7000 || result.RemainingCapturedCent != 7000 || result.ActiveGroupCount != 1 {
		t.Fatalf("result=%+v, want sum/remaining 7000 with one active group", result)
	}
	if result.Groups[0].CommittedCent != 3000 || result.Groups[0].CapCent != 7000 {
		t.Fatalf("group=%+v", result.Groups[0])
	}
	tccNoWrites(t, db, "ORD-SUM3", version, statusPaid)
}

func TestTccSumPrecheckNoActiveGroupsWhenFullyRefunded(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 10000, refunded: 10000})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-SUM4", 10000, 10000, 0, 10000, "PAY-SUM4")
	seedTccGroup(t, db, "ORD-SUM4", "G1", 1, 0)
	seedTccLine(t, db, "ORD-SUM4", 1, "G1", 10000)
	seedTccCommitment(t, db, "ORD-SUM4", "G1", "R-SUM4", 10000, "SETTLED")
	version := tccVersion(t, db, "ORD-SUM4")

	result, err := svc.precheckTccSum(ctx, "ORD-SUM4", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.SumGroupCapCent != 0 || result.RemainingCapturedCent != 0 || result.ActiveGroupCount != 0 {
		t.Fatalf("result=%+v, want zero sum with no active group", result)
	}
	tccNoWrites(t, db, "ORD-SUM4", version, statusPaid)
}

func TestTccSumPrecheckZeroOrderNeverCallsPayment(t *testing.T) {
	db := orderTestDB(t)
	// No Payment client at all: a `zero:` identity must not need one.
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-SUM5", 0, 0, 0, 0, "zero:ORD-SUM5")
	seedTccGroup(t, db, "ORD-SUM5", "G1", 1, 0)
	seedTccLine(t, db, "ORD-SUM5", 1, "G1", 0)
	version := tccVersion(t, db, "ORD-SUM5")

	result, err := svc.precheckTccSum(ctx, "ORD-SUM5", "")
	if err != nil {
		t.Fatal(err)
	}
	if !result.NoCaptured || result.CapturedCent != 0 || result.SumGroupCapCent != 0 || result.ActiveGroupCount != 0 {
		t.Fatalf("result=%+v, want the no-captured branch", result)
	}
	tccNoWrites(t, db, "ORD-SUM5", version, statusPaid)
}

func TestTccSumPrecheckHistoricalEmptyPaymentNo(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-SUM6", 10000, 10000, 0, 0, "")
	seedTccGroup(t, db, "ORD-SUM6", "G1", 1, 0)
	seedTccLine(t, db, "ORD-SUM6", 1, "G1", 10000)
	version := tccVersion(t, db, "ORD-SUM6")

	result, err := svc.precheckTccSum(ctx, "ORD-SUM6", "")
	if err == nil || result != nil {
		t.Fatalf("historical empty payment_no returned (%+v, %v)", result, err)
	}
	if !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("error code=%v, want FAILED_PRECONDITION: %v", apperr.From(err).Code, err)
	}
	if payments.getCalls != 0 || payments.listCalls != 0 {
		t.Fatalf("empty payment_no consulted Payment (get=%d list=%d)", payments.getCalls, payments.listCalls)
	}
	tccNoWrites(t, db, "ORD-SUM6", version, statusPaid)
}

func TestTccSumPrecheckPaymentUnavailableFailsClosed(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{getErr: errors.New("payment timeout")})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-SUM7", 10000, 10000, 0, 0, "PAY-SUM7")
	seedTccGroup(t, db, "ORD-SUM7", "G1", 1, 0)
	seedTccLine(t, db, "ORD-SUM7", 1, "G1", 10000)
	version := tccVersion(t, db, "ORD-SUM7")

	if _, err := svc.precheckTccSum(ctx, "ORD-SUM7", ""); err == nil {
		t.Fatal("precheck succeeded while Payment timed out")
	} else if !apperr.IsCode(err, apperr.CodeUnavailable) {
		t.Fatalf("error code=%v, want UNAVAILABLE: %v", apperr.From(err).Code, err)
	}
	tccNoWrites(t, db, "ORD-SUM7", version, statusPaid)
}

func TestTccSumPrecheckPayCentCapturedMismatchFailsClosed(t *testing.T) {
	db := orderTestDB(t)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-SUM8", 10000, 10000, 0, 0, "PAY-SUM8")
	seedTccGroup(t, db, "ORD-SUM8", "G1", 1, 0)
	seedTccLine(t, db, "ORD-SUM8", 1, "G1", 10000)
	version := tccVersion(t, db, "ORD-SUM8")

	svc := tccTestService(t, db, &fakePaymentPort{captured: 9000})
	if _, err := svc.precheckTccSum(ctx, "ORD-SUM8", ""); err == nil {
		t.Fatal("captured != pay_cent was accepted")
	} else if !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("error code=%v, want FAILED_PRECONDITION: %v", apperr.From(err).Code, err)
	}
	tccNoWrites(t, db, "ORD-SUM8", version, statusPaid)
}

// AC-40 narrowing at the Σ level: pay_cent=0 with a capture is not a zero
// order and must not be treated as "no captured".
func TestTccSumPrecheckZeroOrderWithCaptureIsNotZero(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 500})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-SUM9", 0, 0, 0, 0, "PAY-SUM9")
	seedTccGroup(t, db, "ORD-SUM9", "G1", 1, 0)
	seedTccLine(t, db, "ORD-SUM9", 1, "G1", 0)
	version := tccVersion(t, db, "ORD-SUM9")

	result, err := svc.precheckTccSum(ctx, "ORD-SUM9", "")
	if err == nil {
		t.Fatalf("zero order with a capture passed the precheck: %+v", result)
	}
	if !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("error code=%v, want FAILED_PRECONDITION: %v", apperr.From(err).Code, err)
	}
	tccNoWrites(t, db, "ORD-SUM9", version, statusPaid)
}

// Historical orders without fulfillment groups are calculated as one G1 group
// (AC-51/AC-65); initializing the permit is a Tx1 write, not a precheck write.
func TestTccSumPrecheckLegacyOrderWithoutGroupsUsesG1(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{captured: 10000})
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-SUM10", 10000, 10000, 2000, 0, "PAY-SUM10")
	seedTccLine(t, db, "ORD-SUM10", 1, "G1", 8000)
	version := tccVersion(t, db, "ORD-SUM10")

	result, err := svc.precheckTccSum(ctx, "ORD-SUM10", "")
	if err != nil {
		t.Fatal(err)
	}
	if !result.LegacyG1 || result.SumGroupCapCent != 10000 || result.ActiveGroupCount != 1 {
		t.Fatalf("result=%+v, want legacy G1 with cap 10000", result)
	}
	if result.Groups[0].GroupNo != defaultGroupNo || result.Groups[0].FreightCent != 2000 {
		t.Fatalf("group=%+v", result.Groups[0])
	}
	tccNoWrites(t, db, "ORD-SUM10", version, statusPaid)
}

// Tx1 recheck: the same snapshot is re-used inside the transaction, the order
// row is locked, no Payment round-trip happens and a stale snapshot fails.
func TestTccSumTxRecheckUsesSameSnapshot(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	seedTccOrder(t, db, "ORD-SUM11", 10000, 10000, 0, 0, "PAY-SUM11")
	seedTccGroup(t, db, "ORD-SUM11", "G1", 1, 0)
	seedTccLine(t, db, "ORD-SUM11", 1, "G1", 10000)
	version := tccVersion(t, db, "ORD-SUM11")

	record, err := findOrder(ctx, db, "ORD-SUM11")
	if err != nil || record == nil {
		t.Fatalf("load order: %v", err)
	}
	snapshot, err := svc.readTccPaymentSnapshot(ctx, record, "")
	if err != nil {
		t.Fatal(err)
	}
	if payments.getCalls != 1 {
		t.Fatalf("GetIntent calls=%d, want 1 outside the transaction", payments.getCalls)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	rechecked, err := svc.recheckTccSumTx(ctx, tx, "ORD-SUM11", snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if rechecked.SumGroupCapCent != 10000 || rechecked.RemainingCapturedCent != 10000 {
		t.Fatalf("recheck=%+v", rechecked)
	}
	if payments.getCalls != 1 {
		t.Fatalf("Tx1 recheck made a Payment call (calls=%d)", payments.getCalls)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	// A snapshot for another payment is stale and must fail closed.
	stale := &tccPaymentSnapshot{PaymentNo: "PAY-OTHER", CapturedCent: 10000}
	tx2, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx2.Rollback() }()
	if _, err := svc.recheckTccSumTx(ctx, tx2, "ORD-SUM11", stale); err == nil {
		t.Fatal("stale snapshot passed the Tx1 recheck")
	} else if !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("error code=%v, want FAILED_PRECONDITION: %v", apperr.From(err).Code, err)
	}
	tccNoWrites(t, db, "ORD-SUM11", version, statusPaid)
}

// T1.2 reuse: the compensation payment identity may come from the TCC request
// while the order is still PENDING_PAYMENT (captured before PAID). The precheck
// must take Payment facts for that identity instead of requiring
// settlement_payment_no to be written already.
func TestTccSumPrecheckUsesExplicitPaymentIdentity(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	if _, err := db.Exec(`INSERT INTO orders
		(order_no,user_id,status,total_cent,discount_cent,promotion_discount_cent,applied_promotions_json,
		 freight_cent,pay_cent,paid_cent,settlement_payment_no,refunded_cent,refund_status,refund_state,
		 receiver_name,receiver_phone,receiver_address,quote_id,client_request_id,
		 allocation_frozen_at,allocation_hash,package_count,warehouse_count,expire_at,created_at)
		VALUES ('ORD-SUM12',1,'PENDING_PAYMENT',10000,0,0,'[]',0,10000,0,'',0,'','NONE',
		 'A','1','R','','',NULL,'',1,1,NOW(3),NOW(3))`); err != nil {
		t.Fatal(err)
	}
	seedTccGroup(t, db, "ORD-SUM12", "G1", 1, 0)
	seedTccLine(t, db, "ORD-SUM12", 1, "G1", 10000)
	version := tccVersion(t, db, "ORD-SUM12")

	result, err := svc.precheckTccSum(ctx, "ORD-SUM12", "PAY-TCC12")
	if err != nil {
		t.Fatal(err)
	}
	if result.PaymentNo != "PAY-TCC12" || result.SumGroupCapCent != 10000 || result.RemainingCapturedCent != 10000 {
		t.Fatalf("result=%+v", result)
	}
	if payments.getCalls != 1 {
		t.Fatalf("GetIntent calls=%d, want 1", payments.getCalls)
	}
	tccNoWrites(t, db, "ORD-SUM12", version, statusPendingPayment)
}

// The precheck validates the payment identity of the order it is given.
func TestTccSumPrecheckMissingOrder(t *testing.T) {
	db := orderTestDB(t)
	svc := tccTestService(t, db, &fakePaymentPort{})
	if _, err := svc.precheckTccSum(context.Background(), "ORD-MISSING", ""); !apperr.IsCode(err, apperr.CodeNotFound) {
		t.Fatalf("err=%v, want NOT_FOUND", err)
	}
	if _, err := svc.precheckTccSum(context.Background(), "  ", ""); !apperr.IsCode(err, apperr.CodeInvalidArgument) {
		t.Fatalf("err=%v, want INVALID_ARGUMENT", err)
	}
}
