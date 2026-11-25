package aftersales

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// zeroTestOrder is the Order projection of a zero-amount settlement: pay_cent=0
// and the synthetic `zero:<order_no>` identity that proves "no captured intent"
// (SPEC-0002 v1.9 AC-67). Order remains the owner of this fact; AfterSales only
// reads it to choose the zero short-circuit.
func zeroTestOrder() *pb.Order {
	return &pb.Order{
		OrderNo: "ORD-ZERO", UserId: 1, Status: "PAID", PayCent: 0, PaidCent: 0,
		SettlementPaymentNo: "zero:ORD-ZERO", RefundState: "NONE",
		Items: []*pb.OrderItem{{
			LineNo: 1, SkuId: 1, Quantity: 1, WarehouseId: 1, GroupNo: "G1",
			LinePaidCent: 0, UnitPaidCent: 0,
		}},
		FulfillmentGroups: []*pb.OrderFulfillmentGroup{{GroupNo: "G1", WarehouseId: 1, Status: "ALLOCATED"}},
	}
}

// fakeZeroOrderService emulates Order's zero short-circuit. It records every
// admission with the durable task status observed at call time, can fail without
// writing S19, and can simulate "S19 committed but the reply was lost".
type fakeZeroOrderService struct {
	pb.OrderServiceClient
	order        *pb.Order
	db           *sql.DB
	admitErr     error
	failAfterS19 int
	s19          map[string]*pb.AdmitRefundReply
	calls        []zeroAdmitCall
}

type zeroAdmitCall struct {
	RefundID   string
	OrderNo    string
	GroupNo    string
	Source     string
	AmountCent int64
	TaskStatus string
}

func (f *fakeZeroOrderService) GetOrder(context.Context, *pb.GetOrderRequest, ...grpc.CallOption) (*pb.Order, error) {
	return f.order, nil
}

func (f *fakeZeroOrderService) AdmitRefund(_ context.Context, in *pb.AdmitRefundRequest, _ ...grpc.CallOption) (*pb.AdmitRefundReply, error) {
	call := zeroAdmitCall{
		RefundID: in.GetRefundId(), OrderNo: in.GetOrderNo(), GroupNo: in.GetGroupNo(),
		Source: in.GetSource(), AmountCent: in.GetAmountCent(),
	}
	if f.db != nil {
		_ = f.db.QueryRow(`SELECT status FROM after_sale_refund_tasks WHERE after_sale_no = ?`, in.GetRefundId()).
			Scan(&call.TaskStatus)
	}
	f.calls = append(f.calls, call)
	if f.s19 == nil {
		f.s19 = make(map[string]*pb.AdmitRefundReply)
	}
	if f.admitErr != nil {
		// CancelIfCreated / admission failure: no S19 is written.
		return nil, f.admitErr
	}
	if existing, ok := f.s19[in.GetRefundId()]; ok {
		return existing, nil
	}
	reply := &pb.AdmitRefundReply{
		RefundId: in.GetRefundId(), Result: "PRE_SHIP_REVOKED",
		State: zeroRefundApplicationDone, AmountCent: 0, Version: 1,
	}
	f.s19[in.GetRefundId()] = reply
	if f.failAfterS19 > 0 {
		f.failAfterS19--
		// S19 is committed, but the caller sees a transport failure.
		return nil, status.Error(codes.Unavailable, "admission reply lost after S19 commit")
	}
	return reply, nil
}

func newZeroTestService(t *testing.T, db *sql.DB, order *fakeZeroOrderService, payment *fakePaymentService) *Service {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(db, nil, payment, &fakeInventoryService{}, order, logger)
}

// seedZeroAfterSale inserts the persisted after-sale, item, pending line
// reservation and (optionally) the durable refund task. T1.4b2b starts from the
// persisted intent; the zero-order creation/approval gap is tracked separately.
func seedZeroAfterSale(t *testing.T, db *sql.DB, afterSaleNo, typ, status, restockState string, withTask bool) {
	t.Helper()
	res, err := db.Exec("INSERT INTO after_sales (after_sale_no, order_no, user_id, `type`, status, amount_cent, "+
		"effective_amount_cent, amount_state, restock_state, reason, source) "+
		"VALUES (?, 'ORD-ZERO', 1, ?, ?, 0, 0, 'ACTIVE', ?, 'zero test', 'USER')",
		afterSaleNo, typ, status, restockState)
	if err != nil {
		t.Fatalf("seed zero after sale: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO after_sale_items
		(after_sale_id, sku_id, quantity, warehouse_id, group_no, line_no,
		 line_paid_cent_snapshot, unit_paid_cent_snapshot, pending_amount_cent)
		VALUES (?, 1, 1, 1, 'G1', 1, 0, 0, 0)`, id); err != nil {
		t.Fatalf("seed zero after sale item: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO after_sale_line_ledger
		(order_no, line_no, refunded_quantity, refunded_amount_cent, pending_quantity, pending_amount_cent)
		VALUES ('ORD-ZERO', 1, 0, 0, 1, 0)`); err != nil {
		t.Fatalf("seed zero line ledger: %v", err)
	}
	if withTask {
		if _, err := db.Exec(`INSERT INTO after_sale_refund_tasks (after_sale_no, status, attempts)
			VALUES (?, 'PENDING', 0)`, afterSaleNo); err != nil {
			t.Fatalf("seed zero refund task: %v", err)
		}
	}
}

func assertZeroRefunded(t *testing.T, db *sql.DB, afterSaleNo string) {
	t.Helper()
	var (
		afterStatus, amountState, refundNo, taskStatus string
		effective, refunded                            int64
		attempts                                       int
	)
	if err := db.QueryRow(`SELECT status, effective_amount_cent, refunded_cent, amount_state, refund_no
		FROM after_sales WHERE after_sale_no = ?`, afterSaleNo).
		Scan(&afterStatus, &effective, &refunded, &amountState, &refundNo); err != nil {
		t.Fatal(err)
	}
	if afterStatus != statusRefunded || effective != 0 || refunded != 0 ||
		amountState != amountConsumed || refundNo != "" {
		t.Fatalf("refunded projection = %s/%d/%d/%s/%q", afterStatus, effective, refunded, amountState, refundNo)
	}
	if err := db.QueryRow(`SELECT status, attempts FROM after_sale_refund_tasks WHERE after_sale_no = ?`, afterSaleNo).
		Scan(&taskStatus, &attempts); err != nil {
		t.Fatal(err)
	}
	if taskStatus != taskDone {
		t.Fatalf("refund task = %s attempts=%d", taskStatus, attempts)
	}
	var pendingQty, refundedQty, refundedCent int64
	if err := db.QueryRow(`SELECT pending_quantity, refunded_quantity, refunded_amount_cent
		FROM after_sale_line_ledger WHERE order_no = 'ORD-ZERO' AND line_no = 1`).
		Scan(&pendingQty, &refundedQty, &refundedCent); err != nil {
		t.Fatal(err)
	}
	if pendingQty != 0 || refundedQty != 1 || refundedCent != 0 {
		t.Fatalf("line ledger = pending %d refunded %d/%d", pendingQty, refundedQty, refundedCent)
	}
}

func countAfterSaleEvents(t *testing.T, db *sql.DB, eventType, afterSaleNo string) int {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM outbox_events WHERE event_type = ? AND aggregate_id = ?`,
		eventType, afterSaleNo).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

// TestZeroAfterSaleRefundTaskCompletesWithoutPayment covers the AC-67/AC-68
// happy path: the refund task is persisted and claimed before Order admission,
// S19 DONE is written by Order, and the local projection becomes
// REFUNDED/0/0/CONSUMED/” with task DONE and no Payment call.
func TestZeroAfterSaleRefundTaskCompletesWithoutPayment(t *testing.T) {
	db := afterSaleTestDB(t)
	ctx := context.Background()
	order := &fakeZeroOrderService{order: zeroTestOrder(), db: db, s19: map[string]*pb.AdmitRefundReply{}}
	payment := &fakePaymentService{captured: 0}
	svc := newZeroTestService(t, db, order, payment)
	seedZeroAfterSale(t, db, "AS-ZERO-OK", typeRefundOnly, statusApproved, restockNone, false)

	as, err := svc.Refund(ctx, &pb.RefundAfterSaleRequest{AfterSaleNo: "AS-ZERO-OK"})
	if err != nil {
		t.Fatalf("zero refund: %v", err)
	}
	if as.GetStatus() != statusRefunded || as.GetEffectiveAmountCent() != 0 ||
		as.GetRefundedCent() != 0 || as.GetAmountState() != amountConsumed || as.GetRefundNo() != "" {
		t.Fatalf("reply = %+v", as)
	}
	if len(order.calls) != 1 {
		t.Fatalf("admission calls = %d", len(order.calls))
	}
	call := order.calls[0]
	if call.Source != zeroAfterSaleSource || call.AmountCent != 0 || call.GroupNo != "G1" {
		t.Fatalf("admission = %+v", call)
	}
	// R-C: the durable task must be claimed before the remote admission runs.
	if call.TaskStatus != taskProcessing {
		t.Fatalf("task status during admission = %q", call.TaskStatus)
	}
	if payment.refundCalls != 0 {
		t.Fatalf("payment called %d times for a zero refund", payment.refundCalls)
	}
	if got := countAfterSaleEvents(t, db, "aftersale.completed", "AS-ZERO-OK"); got != 1 {
		t.Fatalf("completed events = %d", got)
	}
	assertZeroRefunded(t, db, "AS-ZERO-OK")
}

// TestZeroAfterSaleCancelFailureRetriesWithoutS19 proves that a CancelIfCreated
// failure leaves the persisted task PENDING for retry, never writes S19, never
// calls Payment and never fabricates REFUNDED.
func TestZeroAfterSaleCancelFailureRetriesWithoutS19(t *testing.T) {
	db := afterSaleTestDB(t)
	ctx := context.Background()
	order := &fakeZeroOrderService{
		order: zeroTestOrder(), db: db, s19: map[string]*pb.AdmitRefundReply{},
		admitErr: status.Error(codes.Unavailable, "cancel created shipment failed"),
	}
	payment := &fakePaymentService{captured: 0}
	svc := newZeroTestService(t, db, order, payment)
	seedZeroAfterSale(t, db, "AS-ZERO-CANCEL", typeRefundOnly, statusApproved, restockNone, false)

	if _, err := svc.Refund(ctx, &pb.RefundAfterSaleRequest{AfterSaleNo: "AS-ZERO-CANCEL"}); err == nil {
		t.Fatal("expected queued refund error")
	}
	var afterStatus, taskStatus, amountState, refundNo string
	var attempts int
	if err := db.QueryRow(`SELECT status, amount_state, refund_no FROM after_sales WHERE after_sale_no = 'AS-ZERO-CANCEL'`).
		Scan(&afterStatus, &amountState, &refundNo); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT status, attempts FROM after_sale_refund_tasks WHERE after_sale_no = 'AS-ZERO-CANCEL'`).
		Scan(&taskStatus, &attempts); err != nil {
		t.Fatal(err)
	}
	if afterStatus != statusRefunding || amountState == amountConsumed || refundNo != "" {
		t.Fatalf("after sale after failure = %s/%s/%q", afterStatus, amountState, refundNo)
	}
	if taskStatus != taskPending || attempts != 1 {
		t.Fatalf("task after failure = %s attempts=%d", taskStatus, attempts)
	}
	if len(order.s19) != 0 {
		t.Fatalf("S19 written despite admission failure: %d", len(order.s19))
	}
	if payment.refundCalls != 0 {
		t.Fatalf("payment calls = %d", payment.refundCalls)
	}
	if got := countAfterSaleEvents(t, db, "aftersale.completed", "AS-ZERO-CANCEL"); got != 0 {
		t.Fatalf("completed events after failure = %d", got)
	}

	order.admitErr = nil
	if err := svc.processRefundTask(ctx, "AS-ZERO-CANCEL"); err != nil {
		t.Fatalf("retry zero refund: %v", err)
	}
	assertZeroRefunded(t, db, "AS-ZERO-CANCEL")
	if payment.refundCalls != 0 {
		t.Fatalf("payment calls after retry = %d", payment.refundCalls)
	}
}

// TestZeroAfterSaleS19WrittenThenCrashIsIdempotent simulates S19 committed by
// Order while AfterSales loses the reply/crashes before the local transaction.
// The retry must reuse the existing S19 and finish REFUNDED exactly once.
func TestZeroAfterSaleS19WrittenThenCrashIsIdempotent(t *testing.T) {
	db := afterSaleTestDB(t)
	ctx := context.Background()
	order := &fakeZeroOrderService{
		order: zeroTestOrder(), db: db, s19: map[string]*pb.AdmitRefundReply{}, failAfterS19: 1,
	}
	payment := &fakePaymentService{captured: 0}
	svc := newZeroTestService(t, db, order, payment)
	seedZeroAfterSale(t, db, "AS-ZERO-CRASH", typeRefundOnly, statusRefunding, restockNone, true)

	if err := svc.processRefundTask(ctx, "AS-ZERO-CRASH"); err == nil {
		t.Fatal("expected the lost admission reply to fail the first attempt")
	}
	var afterStatus, taskStatus string
	var attempts int
	if err := db.QueryRow(`SELECT status FROM after_sales WHERE after_sale_no = 'AS-ZERO-CRASH'`).Scan(&afterStatus); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT status, attempts FROM after_sale_refund_tasks WHERE after_sale_no = 'AS-ZERO-CRASH'`).
		Scan(&taskStatus, &attempts); err != nil {
		t.Fatal(err)
	}
	if afterStatus == statusRefunded || taskStatus != taskPending || attempts != 1 {
		t.Fatalf("after first (crashed) attempt = %s/%s/%d", afterStatus, taskStatus, attempts)
	}
	if len(order.s19) != 1 {
		t.Fatalf("S19 count after crash = %d", len(order.s19))
	}
	if got := countAfterSaleEvents(t, db, "aftersale.completed", "AS-ZERO-CRASH"); got != 0 {
		t.Fatalf("completed events after crash = %d", got)
	}

	if err := svc.processRefundTask(ctx, "AS-ZERO-CRASH"); err != nil {
		t.Fatalf("idempotent retry: %v", err)
	}
	assertZeroRefunded(t, db, "AS-ZERO-CRASH")
	if len(order.calls) != 2 {
		t.Fatalf("admission calls = %d", len(order.calls))
	}
	if len(order.s19) != 1 {
		t.Fatalf("retry wrote a second S19: %d", len(order.s19))
	}
	if got := countAfterSaleEvents(t, db, "aftersale.completed", "AS-ZERO-CRASH"); got != 1 {
		t.Fatalf("completed events after retry = %d", got)
	}
	if payment.refundCalls != 0 {
		t.Fatalf("payment calls = %d", payment.refundCalls)
	}
}

// TestZeroReturnRefundRequiresRestockBeforeAdmission proves the zero path does
// not bypass the return-refund stock gate: no admission while restock_state is
// not DONE, then admission after the existing restock worker finishes.
func TestZeroReturnRefundRequiresRestockBeforeAdmission(t *testing.T) {
	db := afterSaleTestDB(t)
	ctx := context.Background()
	order := &fakeZeroOrderService{order: zeroTestOrder(), db: db, s19: map[string]*pb.AdmitRefundReply{}}
	svc := newZeroTestService(t, db, order, &fakePaymentService{captured: 0})
	seedZeroAfterSale(t, db, "AS-ZERO-RETURN", typeReturnRefund, statusRefunding, restockPending, true)

	if err := svc.processRefundTask(ctx, "AS-ZERO-RETURN"); err == nil {
		t.Fatal("expected the restock gate to block the zero refund")
	}
	if len(order.calls) != 0 {
		t.Fatalf("admission called before restock: %d calls", len(order.calls))
	}
	var afterStatus, taskStatus string
	if err := db.QueryRow(`SELECT status FROM after_sales WHERE after_sale_no = 'AS-ZERO-RETURN'`).Scan(&afterStatus); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT status FROM after_sale_refund_tasks WHERE after_sale_no = 'AS-ZERO-RETURN'`).Scan(&taskStatus); err != nil {
		t.Fatal(err)
	}
	if afterStatus != statusRefunding || taskStatus != taskPending {
		t.Fatalf("after restock gate = %s/%s", afterStatus, taskStatus)
	}

	if _, err := db.Exec(`UPDATE after_sales SET restock_state = ? WHERE after_sale_no = 'AS-ZERO-RETURN'`, restockDone); err != nil {
		t.Fatal(err)
	}
	if err := svc.processRefundTask(ctx, "AS-ZERO-RETURN"); err != nil {
		t.Fatalf("zero refund after restock: %v", err)
	}
	if len(order.calls) != 1 || order.calls[0].Source != zeroAfterSaleSource {
		t.Fatalf("admission after restock = %+v", order.calls)
	}
	assertZeroRefunded(t, db, "AS-ZERO-RETURN")
}

// TestZeroRefundTaskExhaustionFailsPermanent verifies that repeated admission
// failures terminate the task as FAILED_PERMANENT (alertable) and never fake
// REFUNDED/CONSUMED or write S19.
func TestZeroRefundTaskExhaustionFailsPermanent(t *testing.T) {
	db := afterSaleTestDB(t)
	ctx := context.Background()
	order := &fakeZeroOrderService{
		order: zeroTestOrder(), db: db, s19: map[string]*pb.AdmitRefundReply{},
		admitErr: errors.New("cancel created shipment permanently rejected"),
	}
	svc := newZeroTestService(t, db, order, &fakePaymentService{captured: 0})
	seedZeroAfterSale(t, db, "AS-ZERO-EXHAUST", typeRefundOnly, statusRefunding, restockNone, true)

	for attempt := 1; attempt <= maxTaskAttempts; attempt++ {
		if err := svc.processRefundTask(ctx, "AS-ZERO-EXHAUST"); err == nil {
			t.Fatalf("attempt %d unexpectedly succeeded", attempt)
		}
	}
	var taskStatus, lastError, afterStatus, amountState, refundNo string
	var attempts int
	if err := db.QueryRow(`SELECT status, attempts, last_error FROM after_sale_refund_tasks
		WHERE after_sale_no = 'AS-ZERO-EXHAUST'`).Scan(&taskStatus, &attempts, &lastError); err != nil {
		t.Fatal(err)
	}
	if taskStatus != taskFailed || attempts != maxTaskAttempts || lastError == "" {
		t.Fatalf("exhausted task = %s attempts=%d error=%q", taskStatus, attempts, lastError)
	}
	if err := db.QueryRow(`SELECT status, amount_state, refund_no FROM after_sales
		WHERE after_sale_no = 'AS-ZERO-EXHAUST'`).Scan(&afterStatus, &amountState, &refundNo); err != nil {
		t.Fatal(err)
	}
	if afterStatus == statusRefunded || amountState == amountConsumed || refundNo != "" {
		t.Fatalf("permanent failure faked completion: %s/%s/%q", afterStatus, amountState, refundNo)
	}
	if len(order.s19) != 0 {
		t.Fatalf("S19 written on permanent failure: %d", len(order.s19))
	}
	if got := countAfterSaleEvents(t, db, "aftersale.completed", "AS-ZERO-EXHAUST"); got != 0 {
		t.Fatalf("completed events on permanent failure = %d", got)
	}
	// FAILED_PERMANENT is terminal for the worker; a rescan must not refund.
	if err := svc.processRefundTask(ctx, "AS-ZERO-EXHAUST"); err != nil {
		t.Fatalf("failed task rescan = %v", err)
	}
	assertNotRefunded(t, db, "AS-ZERO-EXHAUST")
}

// TestZeroRefundDoesNotRequirePaymentClient is the strongest "no Payment" proof:
// the service is built with a nil Payment client and the zero refund still
// completes through Order's zero short-circuit.
func TestZeroRefundDoesNotRequirePaymentClient(t *testing.T) {
	db := afterSaleTestDB(t)
	ctx := context.Background()
	order := &fakeZeroOrderService{order: zeroTestOrder(), db: db, s19: map[string]*pb.AdmitRefundReply{}}
	svc := newZeroTestService(t, db, order, nil)
	seedZeroAfterSale(t, db, "AS-ZERO-NOPAY", typeRefundOnly, statusRefunding, restockNone, true)

	if err := svc.processRefundTask(ctx, "AS-ZERO-NOPAY"); err != nil {
		t.Fatalf("zero refund without payment client: %v", err)
	}
	assertZeroRefunded(t, db, "AS-ZERO-NOPAY")
}

// TestPositiveRefundAdmitsAfterTaskPersisted is the positive-amount regression
// for the R-C reorder: Refund persists REFUNDING + task, then the worker admits
// (source=aftersales, amount>0) and Payment is still called exactly once.
func TestPositiveRefundAdmitsAfterTaskPersisted(t *testing.T) {
	db := afterSaleTestDB(t)
	svc, order, payment, _ := newAfterSaleTestService(t, db, "PAID")
	order.db = db
	as := createAfterSale(t, svc, typeRefundOnly, 600)
	approveAfterSale(t, svc, as.GetAfterSaleNo(), 600)

	refunded, err := svc.Refund(context.Background(), &pb.RefundAfterSaleRequest{AfterSaleNo: as.GetAfterSaleNo()})
	if err != nil {
		t.Fatalf("positive refund: %v", err)
	}
	if refunded.GetStatus() != statusRefunded || refunded.GetRefundedCent() != 600 {
		t.Fatalf("positive refund reply = %+v", refunded)
	}
	if order.taskStatusAtAdmit != taskProcessing {
		t.Fatalf("task status during positive admission = %q", order.taskStatusAtAdmit)
	}
	if order.sourceAtAdmit != "aftersale" || order.amountAtAdmit != 600 {
		t.Fatalf("positive admission = %s/%d", order.sourceAtAdmit, order.amountAtAdmit)
	}
	if payment.refundCalls != 1 || len(payment.refundAmount) != 1 || payment.refundAmount[0] != 600 {
		t.Fatalf("payment refunds = %d %v", payment.refundCalls, payment.refundAmount)
	}
}

func assertNotRefunded(t *testing.T, db *sql.DB, afterSaleNo string) {
	t.Helper()
	var status string
	if err := db.QueryRow(`SELECT status FROM after_sales WHERE after_sale_no = ?`, afterSaleNo).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status == statusRefunded {
		t.Fatalf("after sale %s was marked REFUNDED", afterSaleNo)
	}
}

// --- T1.4b2b2: zero after-sale creation / approval entry ---------------------

func positiveTestOrder() *pb.Order {
	return &pb.Order{
		OrderNo: "ORD-POS", UserId: 1, Status: "PAID", PayCent: 1000, PaidCent: 1000,
		SettlementPaymentNo: "PAY-POS", RefundState: "NONE",
		Items: []*pb.OrderItem{{
			LineNo: 1, SkuId: 1, Quantity: 1, WarehouseId: 1, GroupNo: "G1",
			LinePaidCent: 1000, UnitPaidCent: 1000,
		}},
		FulfillmentGroups: []*pb.OrderFulfillmentGroup{{GroupNo: "G1", WarehouseId: 1, Status: "ALLOCATED"}},
	}
}

// nonZeroIdentityOrder has pay_cent=0 but not the `zero:<order_no>` settlement
// identity, so it is NOT a zero order; captured=0 must fail closed.
func nonZeroIdentityOrder() *pb.Order {
	order := zeroTestOrder()
	order.OrderNo = "ORD-NONZERO"
	order.SettlementPaymentNo = "PAY-OTHER"
	return order
}

func newServiceForOrder(t *testing.T, db *sql.DB, orders pb.OrderServiceClient, payment *fakePaymentService) *Service {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(db, nil, payment, &fakeInventoryService{}, orders, logger)
}

// TestZeroAfterSaleEndToEndCreateApproveRefundWithoutPayment drives the public
// entry points: CreateAfterSale -> Approve -> Refund/worker -> REFUNDED/0/
// CONSUMED, with zero Payment lookups/refunds and no amount ledger.
func TestZeroAfterSaleEndToEndCreateApproveRefundWithoutPayment(t *testing.T) {
	db := afterSaleTestDB(t)
	ctx := context.Background()
	order := &fakeZeroOrderService{order: zeroTestOrder(), db: db, s19: map[string]*pb.AdmitRefundReply{}}
	payment := &fakePaymentService{captured: 0}
	svc := newServiceForOrder(t, db, order, payment)

	created, err := svc.CreateAfterSale(ctx, &pb.CreateAfterSaleRequest{
		OrderNo: "ORD-ZERO", UserId: 1, Type: typeRefundOnly, AmountCent: 0, Reason: "zero after-sale",
		Items: []*pb.AfterSaleItem{{SkuId: 1, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("create zero after sale: %v", err)
	}
	if created.GetStatus() != statusRequested || created.GetEffectiveAmountCent() != 0 ||
		created.GetAmountState() != amountActive {
		t.Fatalf("created zero after sale = %+v", created)
	}
	var ledgers, tasks int
	if err := db.QueryRow(`SELECT COUNT(*) FROM after_sale_amount_ledger WHERE order_no = 'ORD-ZERO'`).Scan(&ledgers); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM after_sale_refund_tasks`).Scan(&tasks); err != nil {
		t.Fatal(err)
	}
	if ledgers != 0 || tasks != 0 {
		t.Fatalf("zero create wrote ledger/task: ledgers=%d tasks=%d", ledgers, tasks)
	}

	approved, err := svc.Approve(ctx, &pb.ApproveRequest{AfterSaleNo: created.GetAfterSaleNo(), Approved: true})
	if err != nil {
		t.Fatalf("approve zero after sale: %v", err)
	}
	if approved.GetStatus() != statusApproved || approved.GetEffectiveAmountCent() != 0 ||
		approved.GetAmountState() != amountActive {
		t.Fatalf("approved zero after sale = %+v", approved)
	}
	if payment.lookupCalls != 0 || payment.refundCalls != 0 {
		t.Fatalf("zero create/approve called Payment: lookups=%d refunds=%d", payment.lookupCalls, payment.refundCalls)
	}

	refunded, err := svc.Refund(ctx, &pb.RefundAfterSaleRequest{AfterSaleNo: created.GetAfterSaleNo()})
	if err != nil {
		t.Fatalf("refund zero after sale: %v", err)
	}
	if refunded.GetStatus() != statusRefunded {
		t.Fatalf("zero refund reply = %+v", refunded)
	}
	assertZeroRefunded(t, db, created.GetAfterSaleNo())
	if len(order.calls) != 1 {
		t.Fatalf("admission calls = %d", len(order.calls))
	}
	if call := order.calls[0]; call.Source != zeroAfterSaleSource || call.AmountCent != 0 || call.TaskStatus != taskProcessing {
		t.Fatalf("zero admission = %+v", call)
	}
	if payment.lookupCalls != 0 || payment.refundCalls != 0 {
		t.Fatalf("zero refund called Payment: lookups=%d refunds=%d", payment.lookupCalls, payment.refundCalls)
	}
	if got := countAfterSaleEvents(t, db, "aftersale.completed", created.GetAfterSaleNo()); got != 1 {
		t.Fatalf("completed events = %d", got)
	}
}

// TestZeroAfterSaleApprovalRejectsPositiveAmount covers AC-67: a zero order's
// after-sale cannot be turned into a positive-amount refund.
func TestZeroAfterSaleApprovalRejectsPositiveAmount(t *testing.T) {
	db := afterSaleTestDB(t)
	ctx := context.Background()
	order := &fakeZeroOrderService{order: zeroTestOrder(), db: db, s19: map[string]*pb.AdmitRefundReply{}}
	svc := newServiceForOrder(t, db, order, &fakePaymentService{captured: 0})
	created, err := svc.CreateAfterSale(ctx, &pb.CreateAfterSaleRequest{
		OrderNo: "ORD-ZERO", UserId: 1, Type: typeRefundOnly, AmountCent: 0, Reason: "zero after-sale",
		Items: []*pb.AfterSaleItem{{SkuId: 1, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("create zero after sale: %v", err)
	}
	_, err = svc.Approve(ctx, &pb.ApproveRequest{
		AfterSaleNo: created.GetAfterSaleNo(), Approved: true, AmountCent: 100, Reason: "adjust",
	})
	if !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("zero approval with positive amount = %v", err)
	}
	var status string
	var effective int64
	if err := db.QueryRow(`SELECT status, effective_amount_cent FROM after_sales WHERE after_sale_no = ?`,
		created.GetAfterSaleNo()).Scan(&status, &effective); err != nil {
		t.Fatal(err)
	}
	if status != statusRequested || effective != 0 {
		t.Fatalf("after rejected zero approval = %s/%d", status, effective)
	}
}

// TestZeroSystemShortageRefundUsesZeroTask covers createSystemRefund on a
// zero-settled order: it creates the system after-sale + durable task with
// amount 0, never calls Payment, and the worker completes it via the zero path.
func TestZeroSystemShortageRefundUsesZeroTask(t *testing.T) {
	db := afterSaleTestDB(t)
	ctx := context.Background()
	order := &fakeZeroOrderService{order: zeroTestOrder(), db: db, s19: map[string]*pb.AdmitRefundReply{}}
	payment := &fakePaymentService{captured: 0}
	svc := newServiceForOrder(t, db, order, payment)
	event := shortageResolvedEvent{
		OrderNo: "ORD-ZERO", GroupNo: "G1", Status: shortageResolved, Resolution: "REFUND",
		ReasonCode: "SHORTAGE_DEFAULT",
		Items: []struct {
			LineNo   int32 `json:"line_no"`
			SkuID    int64 `json:"sku_id"`
			Quantity int32 `json:"quantity"`
		}{{LineNo: 1, SkuID: 1, Quantity: 1}},
	}
	if err := svc.createSystemRefund(ctx, event); err != nil {
		t.Fatalf("zero system refund: %v", err)
	}
	if err := svc.createSystemRefund(ctx, event); err != nil {
		t.Fatalf("idempotent zero system refund: %v", err)
	}
	var afterSaleNo, status, source string
	var effective int64
	if err := db.QueryRow(`SELECT after_sale_no, status, source, effective_amount_cent FROM after_sales
		WHERE order_no = 'ORD-ZERO'`).Scan(&afterSaleNo, &status, &source, &effective); err != nil {
		t.Fatal(err)
	}
	if status != statusApproved || source != sourceSystem || effective != 0 {
		t.Fatalf("zero system refund = %s/%s/%d", status, source, effective)
	}
	var afterSales, tasks, ledgers int
	if err := db.QueryRow(`SELECT COUNT(*) FROM after_sales WHERE order_no = 'ORD-ZERO'`).Scan(&afterSales); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM after_sale_refund_tasks`).Scan(&tasks); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM after_sale_amount_ledger WHERE order_no = 'ORD-ZERO'`).Scan(&ledgers); err != nil {
		t.Fatal(err)
	}
	if afterSales != 1 || tasks != 1 || ledgers != 0 {
		t.Fatalf("zero system rows = after_sales %d tasks %d ledgers %d", afterSales, tasks, ledgers)
	}
	if payment.lookupCalls != 0 || payment.refundCalls != 0 {
		t.Fatalf("zero system refund called Payment: lookups=%d refunds=%d", payment.lookupCalls, payment.refundCalls)
	}
	if err := svc.processRefundTask(ctx, afterSaleNo); err != nil {
		t.Fatalf("zero system worker: %v", err)
	}
	assertZeroRefunded(t, db, afterSaleNo)
	if payment.refundCalls != 0 {
		t.Fatalf("payment calls = %d", payment.refundCalls)
	}
}

// TestNonZeroOrderZeroAmountRequestIsNotZeroAfterSale proves a user-supplied 0
// on a positive order still computes the line refund (positive path) and pays
// through Payment; it never becomes a zero after-sale.
func TestNonZeroOrderZeroAmountRequestIsNotZeroAfterSale(t *testing.T) {
	db := afterSaleTestDB(t)
	ctx := context.Background()
	order := &fakeOrderService{order: positiveTestOrder(), db: db}
	payment := &fakePaymentService{captured: 1000}
	svc := newServiceForOrder(t, db, order, payment)

	created, err := svc.CreateAfterSale(ctx, &pb.CreateAfterSaleRequest{
		OrderNo: "ORD-POS", UserId: 1, Type: typeRefundOnly, AmountCent: 0, Reason: "user typed zero",
		Items: []*pb.AfterSaleItem{{SkuId: 1, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("positive create: %v", err)
	}
	if created.GetEffectiveAmountCent() != 1000 || created.GetAmountState() != amountActive {
		t.Fatalf("positive create with amount 0 = %+v", created)
	}
	if _, err := svc.Approve(ctx, &pb.ApproveRequest{AfterSaleNo: created.GetAfterSaleNo(), Approved: true}); err != nil {
		t.Fatalf("positive approve: %v", err)
	}
	if _, err := svc.Refund(ctx, &pb.RefundAfterSaleRequest{AfterSaleNo: created.GetAfterSaleNo()}); err != nil {
		t.Fatalf("positive refund: %v", err)
	}
	if payment.refundCalls != 1 || len(payment.refundAmount) != 1 || payment.refundAmount[0] != 1000 {
		t.Fatalf("positive payment refunds = %d %v", payment.refundCalls, payment.refundAmount)
	}
	if len(order.blocked) != 1 || order.sourceAtAdmit != "aftersale" || order.amountAtAdmit != 1000 {
		t.Fatalf("positive admission = %v src=%s amount=%d", order.blocked, order.sourceAtAdmit, order.amountAtAdmit)
	}
}

// TestNonZeroOrderFreeLineZeroAmountFailsClosed: a user-supplied 0 for a
// zero-priced line on a positive order is not a zero after-sale and must fail
// closed instead of entering the zero path.
func TestNonZeroOrderFreeLineZeroAmountFailsClosed(t *testing.T) {
	db := afterSaleTestDB(t)
	order := &fakeOrderService{order: positiveTestOrder()}
	order.order.Items[0].LinePaidCent = 0
	order.order.Items[0].UnitPaidCent = 0
	svc := newServiceForOrder(t, db, order, &fakePaymentService{captured: 1000})
	_, err := svc.CreateAfterSale(context.Background(), &pb.CreateAfterSaleRequest{
		OrderNo: "ORD-POS", UserId: 1, Type: typeRefundOnly, AmountCent: 0, Reason: "free line",
		Items: []*pb.AfterSaleItem{{SkuId: 1, Quantity: 1}},
	})
	if !apperr.IsCode(err, apperr.CodeInvalidArgument) {
		t.Fatalf("positive free-line zero request = %v", err)
	}
	var afterSales int
	if err := db.QueryRow(`SELECT COUNT(*) FROM after_sales`).Scan(&afterSales); err != nil {
		t.Fatal(err)
	}
	if afterSales != 0 {
		t.Fatalf("failed positive zero request created %d after-sales", afterSales)
	}
}

// TestCapturedZeroNonZeroIdentityFailsClosed covers the boundary: pay_cent=0
// without the `zero:<order_no>` identity is not a zero order, so create,
// approve and the system shortage refund all fail closed with no rows/tasks.
func TestCapturedZeroNonZeroIdentityFailsClosed(t *testing.T) {
	db := afterSaleTestDB(t)
	ctx := context.Background()
	order := &fakeOrderService{order: nonZeroIdentityOrder(), db: db}
	payment := &fakePaymentService{captured: 0}
	svc := newServiceForOrder(t, db, order, payment)

	if _, err := svc.CreateAfterSale(ctx, &pb.CreateAfterSaleRequest{
		OrderNo: "ORD-NONZERO", UserId: 1, Type: typeRefundOnly, AmountCent: 0, Reason: "not zero",
		Items: []*pb.AfterSaleItem{{SkuId: 1, Quantity: 1}},
	}); !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("non-zero identity create = %v", err)
	}
	event := shortageResolvedEvent{
		OrderNo: "ORD-NONZERO", GroupNo: "G1", Status: shortageResolved, Resolution: "REFUND",
		ReasonCode: "SHORTAGE_DEFAULT",
		Items: []struct {
			LineNo   int32 `json:"line_no"`
			SkuID    int64 `json:"sku_id"`
			Quantity int32 `json:"quantity"`
		}{{LineNo: 1, SkuID: 1, Quantity: 1}},
	}
	if err := svc.createSystemRefund(ctx, event); !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("non-zero identity system refund = %v", err)
	}
	var afterSales, tasks int
	if err := db.QueryRow(`SELECT COUNT(*) FROM after_sales`).Scan(&afterSales); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM after_sale_refund_tasks`).Scan(&tasks); err != nil {
		t.Fatal(err)
	}
	if afterSales != 0 || tasks != 0 {
		t.Fatalf("fail-closed rows = after_sales %d tasks %d", afterSales, tasks)
	}
	if payment.refundCalls != 0 {
		t.Fatalf("payment calls = %d", payment.refundCalls)
	}

	if _, err := db.Exec(`INSERT INTO after_sales (after_sale_no, order_no, user_id, ` + "`type`" + `, status,
		amount_cent, effective_amount_cent, amount_state, restock_state, reason, source)
		VALUES ('AS-NONZERO', 'ORD-NONZERO', 1, 'REFUND_ONLY', 'REQUESTED', 0, 0, 'ACTIVE', 'NONE', 'x', 'USER')`); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Approve(ctx, &pb.ApproveRequest{AfterSaleNo: "AS-NONZERO", Approved: true}); !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("non-zero identity approve = %v", err)
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM after_sales WHERE after_sale_no = 'AS-NONZERO'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != statusRequested {
		t.Fatalf("fail-closed approve changed status to %s", status)
	}
}
