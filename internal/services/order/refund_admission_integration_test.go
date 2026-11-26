package order

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"

	"google.golang.org/grpc"
)

// fakePaymentPort is the order package's narrow Payment stand-in. It records
// calls so tests can prove a path never consulted Payment, and since T1.4b1 it
// also records Refund calls (payment evidence).
type fakePaymentPort struct {
	captured, refunded int64
	intents            []*pb.Intent
	getErr             error
	listErr            error
	getCalls           int
	listCalls          int
	// refundErrs fails the matching Refund call (by call index); nil entries
	// succeed. refundCalls records every request so duplicates are visible.
	refundErrs  []error
	refundCalls []*pb.RefundRequest
	// beforeGet runs before GetIntent returns; T1.6 tests use it to land a
	// concurrent payment confirmation between the snapshot and the order CAS.
	beforeGet func()

	// T1.4b1-R Payment facts. refundFacts is keyed by refund_id and is what
	// GetRefundByKey reads (absent => NotFound). RefundBatch mirrors C-04's
	// all-or-none contract and persists one reply per batch_id.
	refundFacts    map[string]*pb.Refund
	getRefundErr   error
	getRefundErrs  map[string]error
	getRefundCalls []string
	batchErrs      []error
	batchCalls     []*pb.RefundBatchRequest
	batchReplies   map[string]*pb.RefundBatchReply
	// beforeBatch runs inside RefundBatch after the call is recorded; tests use
	// it to land an external refund in the preflight -> batch window.
	beforeBatch func(*fakePaymentPort)
}

func (f *fakePaymentPort) GetIntent(context.Context, *pb.GetIntentRequest, ...grpc.CallOption) (*pb.Intent, error) {
	f.getCalls++
	if f.beforeGet != nil {
		f.beforeGet()
	}
	if f.getErr != nil {
		return nil, f.getErr
	}
	return &pb.Intent{PaymentNo: "PAY-1", CapturedCent: f.captured, RefundedCent: f.refunded}, nil
}

func (f *fakePaymentPort) ListOrderIntents(context.Context, *pb.ListOrderIntentsRequest, ...grpc.CallOption) (*pb.IntentList, error) {
	f.listCalls++
	if f.listErr != nil {
		return nil, f.listErr
	}
	return &pb.IntentList{Intents: f.intents}, nil
}

func (f *fakePaymentPort) Refund(_ context.Context, in *pb.RefundRequest, _ ...grpc.CallOption) (*pb.Empty, error) {
	index := len(f.refundCalls)
	f.refundCalls = append(f.refundCalls, in)
	if index < len(f.refundErrs) && f.refundErrs[index] != nil {
		return nil, f.refundErrs[index]
	}
	return &pb.Empty{}, nil
}

// GetRefundByKey returns the fake's refund fact or Payment's NOT_FOUND answer.
func (f *fakePaymentPort) GetRefundByKey(_ context.Context, in *pb.GetRefundByKeyRequest, _ ...grpc.CallOption) (*pb.Refund, error) {
	refundID := in.GetRefundId()
	f.getRefundCalls = append(f.getRefundCalls, refundID)
	if f.getRefundErr != nil {
		return nil, f.getRefundErr
	}
	if err := f.getRefundErrs[refundID]; err != nil {
		return nil, err
	}
	if fact := f.refundFacts[refundID]; fact != nil {
		return fact, nil
	}
	return nil, apperr.NotFound("refund not found")
}

// RefundBatch is the C-04 fake: one all-or-none transaction, batch replay from
// the persisted reply, mixed existing/new items and the deterministic errors
// Order must map to MANUAL_HOLD.
func (f *fakePaymentPort) RefundBatch(_ context.Context, in *pb.RefundBatchRequest, _ ...grpc.CallOption) (*pb.RefundBatchReply, error) {
	index := len(f.batchCalls)
	f.batchCalls = append(f.batchCalls, in)
	if f.beforeBatch != nil {
		f.beforeBatch(f)
	}
	if index < len(f.batchErrs) && f.batchErrs[index] != nil {
		return nil, f.batchErrs[index]
	}
	items := in.GetItems()
	if len(items) == 0 {
		return nil, fakeRefundBatchErr(apperr.CodeInvalidArgument, "INVALID_BATCH", "items must not be empty")
	}
	seen := make(map[string]struct{}, len(items))
	var total int64
	for _, item := range items {
		if item.GetRefundId() == "" || item.GetAmountCent() <= 0 {
			return nil, fakeRefundBatchErr(apperr.CodeInvalidArgument, "INVALID_BATCH", "invalid item")
		}
		if _, duplicate := seen[item.GetRefundId()]; duplicate {
			return nil, fakeRefundBatchErr(apperr.CodeInvalidArgument, "INVALID_BATCH", "duplicate refund_id")
		}
		seen[item.GetRefundId()] = struct{}{}
		total += item.GetAmountCent()
	}
	if total != in.GetExpectedTotalCent() {
		return nil, fakeRefundBatchErr(apperr.CodeInvalidArgument, "BATCH_SUM_MISMATCH", "expected_total mismatch")
	}
	if prior := f.batchReplies[in.GetBatchId()]; prior != nil {
		if err := sameFakeBatchItems(prior, items); err != nil {
			return nil, fakeRefundBatchErr(apperr.CodeConflict, "IDEMPOTENCY_CONFLICT", err.Error())
		}
		return prior, nil
	}
	results := make([]*pb.RefundBatchItemResult, 0, len(items))
	var existingSum, newSum int64
	for _, item := range items {
		if fact := f.refundFacts[item.GetRefundId()]; fact != nil {
			if fact.GetAmountCent() != item.GetAmountCent() {
				return nil, fakeRefundBatchErr(apperr.CodeConflict, "ITEM_CONFLICT", "refund amount differs")
			}
			if fact.GetStatus() != tccRefundStatusSucceeded && fact.GetStatus() != tccRefundStatusRefunded {
				return nil, fakeRefundBatchErr(apperr.CodeFailedPrecondition, "BATCH_STATE_INVALID", "refund not adoptable")
			}
			existingSum += item.GetAmountCent()
			results = append(results, &pb.RefundBatchItemResult{
				RefundId: item.GetRefundId(), RefundNo: fact.GetRefundNo(),
				Status: refundBatchStatusAlreadyRefunded, AmountCent: item.GetAmountCent(),
			})
			continue
		}
		newSum += item.GetAmountCent()
	}
	if newSum > f.captured-f.refunded {
		return nil, fakeRefundBatchErr(apperr.CodeFailedPrecondition, "INSUFFICIENT_REMAINING", "insufficient captured remaining")
	}
	status := refundBatchStatusAlreadyRefunded
	if newSum > 0 {
		status = refundBatchStatusRefunded
		f.refunded += newSum
		for _, item := range items {
			if f.refundFacts[item.GetRefundId()] != nil {
				continue
			}
			fact := &pb.Refund{
				RefundNo: fmt.Sprintf("RFD-%s", item.GetRefundId()), RefundId: item.GetRefundId(),
				PaymentNo: in.GetPaymentNo(), AmountCent: item.GetAmountCent(),
				Status: tccRefundStatusSucceeded, CreatedAtUnix: time.Now().Unix(),
				Reason: in.GetReason(),
			}
			f.addRefundFact(fact)
			results = append(results, &pb.RefundBatchItemResult{
				RefundId: item.GetRefundId(), RefundNo: fact.GetRefundNo(),
				Status: refundBatchStatusRefunded, AmountCent: item.GetAmountCent(),
			})
		}
	}
	if existingSum+newSum != total {
		return nil, fakeRefundBatchErr(apperr.CodeFailedPrecondition, "BATCH_STATE_INVALID", "item sum diverges")
	}
	reply := &pb.RefundBatchReply{
		BatchId: in.GetBatchId(), Status: status, RefundedCent: total, ItemResults: results,
	}
	if f.batchReplies == nil {
		f.batchReplies = make(map[string]*pb.RefundBatchReply)
	}
	f.batchReplies[in.GetBatchId()] = reply
	return reply, nil
}

func fakeRefundBatchErr(code apperr.Code, reason, msg string) error {
	return apperr.New(code, reason+": "+msg).WithDetails(map[string]any{"reason": reason})
}

func sameFakeBatchItems(reply *pb.RefundBatchReply, items []*pb.RefundBatchItem) error {
	if len(reply.GetItemResults()) != len(items) {
		return errors.New("different item count")
	}
	byID := make(map[string]int64, len(reply.GetItemResults()))
	for _, result := range reply.GetItemResults() {
		byID[result.GetRefundId()] = result.GetAmountCent()
	}
	for _, item := range items {
		amount, ok := byID[item.GetRefundId()]
		if !ok || amount != item.GetAmountCent() {
			return errors.New("different item set")
		}
	}
	return nil
}

func (f *fakePaymentPort) addRefundFact(fact *pb.Refund) {
	if f.refundFacts == nil {
		f.refundFacts = make(map[string]*pb.Refund)
	}
	f.refundFacts[fact.GetRefundId()] = fact
}

type fakeFulfillmentPort struct {
	mu          sync.Mutex
	status      string
	shipments   []*pb.Shipment
	listErr     error
	failCancels int
	// listCalls/cancelCalls are atomic because concurrent TCC admission tests
	// call the port from several goroutines under -race.
	listCalls   atomic.Int64
	cancelCalls atomic.Int64
}

func (f *fakeFulfillmentPort) CancelIfCreated(context.Context, *pb.CancelIfCreatedRequest, ...grpc.CallOption) (*pb.CancelIfCreatedReply, error) {
	f.cancelCalls.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failCancels > 0 {
		f.failCancels--
		return nil, errors.New("fulfillment cancel timeout")
	}
	return &pb.CancelIfCreatedReply{Status: f.status}, nil
}

func (f *fakeFulfillmentPort) setStatus(status string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.status = status
}

func (f *fakeFulfillmentPort) setFailCancels(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failCancels = n
}

func (f *fakeFulfillmentPort) ListShipments(context.Context, *pb.ListShipmentsRequest, ...grpc.CallOption) (*pb.ShipmentList, error) {
	f.listCalls.Add(1)
	if f.listErr != nil {
		return nil, f.listErr
	}
	return &pb.ShipmentList{Total: int64(len(f.shipments)), Shipments: f.shipments}, nil
}

func TestAdmitRefundPreShipAndIdempotent(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	ctx := context.Background()
	if _, err := db.Exec(`INSERT INTO orders
		(order_no,user_id,status,total_cent,discount_cent,promotion_discount_cent,applied_promotions_json,
		 freight_cent,pay_cent,paid_cent,settlement_payment_no,refunded_cent,refund_status,refund_state,
		 receiver_name,receiver_phone,receiver_address,quote_id,client_request_id,
		 allocation_frozen_at,allocation_hash,package_count,warehouse_count,expire_at,created_at)
		VALUES ('ORD-ADM',1,'PAID',10000,0,0,'[]',0,10000,10000,'PAY-1',0,'','NONE',
		 'A','1','R','','',NOW(3),'h',1,1,NOW(3),NOW(3))`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO order_items
		(order_no,line_no,sku_id,name,quantity,warehouse_id,fulfillment_group_no,line_paid_cent,unit_paid_cent)
		VALUES ('ORD-ADM',1,1,'x',1,1,'G1',10000,10000)`); err != nil {
		t.Fatal(err)
	}
	reply, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-ADM-1", OrderNo: "ORD-ADM", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetResult() != refundResultPre {
		t.Fatalf("result=%q, want PRE", reply.GetResult())
	}
	var permitState string
	if err := db.QueryRow(`SELECT state FROM order_shipping_permits WHERE order_no='ORD-ADM'`).Scan(&permitState); err != nil {
		t.Fatal(err)
	}
	if permitState != permitRevoked {
		t.Fatalf("permit=%q, want REVOKED", permitState)
	}
	again, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-ADM-1", OrderNo: "ORD-ADM", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
	})
	if err != nil || again.GetResult() != refundResultPre {
		t.Fatalf("idempotent retry = (%v, %v)", again, err)
	}
}

func TestAdmitRefundConsumedPermitTwoPhase(t *testing.T) {
	db := orderTestDB(t)
	fake := &fakeFulfillmentPort{status: "CANCELLED"}
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, fake)
	ctx := context.Background()
	if _, err := db.Exec(`INSERT INTO orders
		(order_no,user_id,status,total_cent,discount_cent,promotion_discount_cent,applied_promotions_json,
		 freight_cent,pay_cent,paid_cent,settlement_payment_no,refunded_cent,refund_status,refund_state,
		 receiver_name,receiver_phone,receiver_address,quote_id,client_request_id,
		 allocation_frozen_at,allocation_hash,package_count,warehouse_count,expire_at,created_at)
		VALUES ('ORD-ADM2',1,'PAID',10000,0,0,'[]',0,10000,10000,'PAY-1',0,'','NONE',
		 'A','1','R','','',NOW(3),'h',1,1,NOW(3),NOW(3))`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO order_items
		(order_no,line_no,sku_id,name,quantity,warehouse_id,fulfillment_group_no,line_paid_cent,unit_paid_cent)
		VALUES ('ORD-ADM2',1,1,'x',1,1,'G1',10000,10000)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO order_shipping_permits (order_no, fulfillment_group_no, state)
		VALUES ('ORD-ADM2','G1','CONSUMED')`); err != nil {
		t.Fatal(err)
	}
	reply, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-ADM-2", OrderNo: "ORD-ADM2", GroupNo: "G1", AmountCent: 10000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetResult() != refundResultPre {
		t.Fatalf("cancelled delivery result=%q, want PRE", reply.GetResult())
	}
	if _, err := db.Exec(`INSERT INTO order_shipping_permits (order_no, fulfillment_group_no, state)
		VALUES ('ORD-ADM3','G1','CONSUMED')`); err != nil {
		t.Fatal(err)
	}
	_, _ = db.Exec(`INSERT INTO orders (order_no,user_id,status,total_cent,pay_cent,paid_cent,settlement_payment_no,expire_at,applied_promotions_json)
		VALUES ('ORD-ADM3',1,'PAID',10000,10000,10000,'PAY-1',NOW(3),'[]')`)
	_, _ = db.Exec(`INSERT INTO order_items
		(order_no,line_no,sku_id,name,quantity,warehouse_id,fulfillment_group_no,line_paid_cent,unit_paid_cent)
		VALUES ('ORD-ADM3',1,1,'x',1,1,'G1',10000,10000)`)
	fake.status = "ALREADY_SHIPPED"
	reply, err = svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-ADM-3", OrderNo: "ORD-ADM3", GroupNo: "G1", AmountCent: 10000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetResult() != refundResultPost {
		t.Fatalf("shipped result=%q, want POST", reply.GetResult())
	}
}

// TestAdmitRefundZeroAmountOrder pins the P-03 regression and AC-40: a
// `zero:<order_no>` order is admitted with amount 0, revokes the unshipped
// permit, and never consults Payment (no intent exists for a zero order).
func TestAdmitRefundZeroAmountOrder(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{}
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(payments, &fakeFulfillmentPort{status: "CANCELLED"})
	ctx := context.Background()
	if _, err := db.Exec(`INSERT INTO orders
		(order_no,user_id,status,total_cent,pay_cent,paid_cent,settlement_payment_no,expire_at,applied_promotions_json)
		VALUES ('ORD-ZADM',1,'PAID',100,0,0,'zero:ORD-ZADM',NOW(3),'[]')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO order_items
		(order_no,line_no,sku_id,name,quantity,warehouse_id,fulfillment_group_no,line_paid_cent,unit_paid_cent)
		VALUES ('ORD-ZADM',1,1,'x',1,1,'G1',0,0)`); err != nil {
		t.Fatal(err)
	}
	reply, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-ZERO", OrderNo: "ORD-ZADM", GroupNo: "G1", AmountCent: 0, Source: "aftersale",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetResult() != refundResultPre {
		t.Fatalf("zero refund result=%q, want PRE", reply.GetResult())
	}
	if payments.getCalls != 0 || payments.listCalls != 0 {
		t.Fatalf("zero refund consulted Payment (get=%d list=%d)", payments.getCalls, payments.listCalls)
	}
	var permitState string
	if err := db.QueryRow(`SELECT state FROM order_shipping_permits WHERE order_no='ORD-ZADM'`).Scan(&permitState); err != nil {
		t.Fatal(err)
	}
	if permitState != permitRevoked {
		t.Fatalf("permit=%q, want REVOKED", permitState)
	}
	var admitted, committed int64
	if err := db.QueryRow(`SELECT amount_cent FROM order_refund_admissions WHERE refund_id='R-ZERO'`).Scan(&admitted); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT amount_cent FROM order_refund_commitments WHERE refund_id='R-ZERO'`).Scan(&committed); err != nil {
		t.Fatal(err)
	}
	if admitted != 0 || committed != 0 {
		t.Fatalf("zero refund amounts: admission=%d commitment=%d, want 0/0", admitted, committed)
	}
	// A zero order has no captured quota: any positive admission is rejected
	// before a second admission row is written.
	if _, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-ZERO-OVER", OrderNo: "ORD-ZADM", GroupNo: "G1", AmountCent: 1,
	}); err == nil {
		t.Fatal("positive refund admitted on a zero-amount order")
	} else if !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("error code=%v, want FAILED_PRECONDITION: %v", apperr.From(err).Code, err)
	}
	var overRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM order_refund_admissions WHERE refund_id='R-ZERO-OVER'`).Scan(&overRows); err != nil {
		t.Fatal(err)
	}
	if overRows != 0 {
		t.Fatalf("over-quota admission rows=%d, want 0", overRows)
	}
	if payments.getCalls != 0 || payments.listCalls != 0 {
		t.Fatalf("zero refund consulted Payment (get=%d list=%d)", payments.getCalls, payments.listCalls)
	}
}

// seedZeroRefundOrder creates the Order-side facts of a legitimately settled
// zero-amount order: pay_cent=0 and the `zero:<order_no>` settlement identity,
// which SettleZero only writes after Payment reported no captured intent.
func seedZeroRefundOrder(t *testing.T, db *sql.DB, orderNo string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO orders
		(order_no,user_id,status,total_cent,pay_cent,paid_cent,settlement_payment_no,
		 refunded_cent,refund_state,expire_at,applied_promotions_json)
		VALUES (?,1,'PAID',100,0,0,?,0,'NONE',NOW(3),'[]')`, orderNo, "zero:"+orderNo); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO order_items
		(order_no,line_no,sku_id,name,quantity,warehouse_id,fulfillment_group_no,line_paid_cent,unit_paid_cent)
		VALUES (?,1,1,'x',1,1,'G1',0,0)`, orderNo); err != nil {
		t.Fatal(err)
	}
}

// T1.4a / AC-67 / AC-68 / INV-50: the system_zero short-circuit writes the
// DONE execution fact and the PRE admission in one transaction, revokes the
// unused permit, and never consults Payment or creates a commitment.
func TestAdmitRefundSystemZeroShortCircuit(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 0}
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(payments, &fakeFulfillmentPort{status: "CANCELLED"})
	ctx := context.Background()
	seedZeroRefundOrder(t, db, "ORD-ZS")

	req := &pb.AdmitRefundRequest{
		RefundId: "AS-ZS", OrderNo: "ORD-ZS", GroupNo: "G1", AmountCent: 0, Source: refundSourceSystemZero,
	}
	reply, err := svc.AdmitRefund(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetState() != refundApplicationDone || reply.GetResult() != refundResultPre || reply.GetAmountCent() != 0 {
		t.Fatalf("zero admission = %s/%s/%d, want DONE/PRE/0",
			reply.GetState(), reply.GetResult(), reply.GetAmountCent())
	}
	if payments.getCalls != 0 || payments.listCalls != 0 {
		t.Fatalf("zero short-circuit consulted Payment (get=%d list=%d)", payments.getCalls, payments.listCalls)
	}
	var source, state, result, orderNo, groupNo string
	var amount, goods, freight int64
	var nonEffective int
	if err := db.QueryRow(`SELECT source, state, admission_result, order_no, group_no,
		amount_cent, goods_cent, freight_cent, non_effective_capture
		FROM refund_applications WHERE refund_id = 'AS-ZS'`).
		Scan(&source, &state, &result, &orderNo, &groupNo, &amount, &goods, &freight, &nonEffective); err != nil {
		t.Fatal(err)
	}
	if source != refundSourceSystemZero || state != refundApplicationDone || result != refundResultPre ||
		orderNo != "ORD-ZS" || groupNo != "G1" || amount != 0 || goods != 0 || freight != 0 || nonEffective != 0 {
		t.Fatalf("zero application = %s/%s/%s/%s/%s/%d/%d/%d/%d",
			source, state, result, orderNo, groupNo, amount, goods, freight, nonEffective)
	}
	var admissionResult, admissionState string
	var admissionAmount int64
	if err := db.QueryRow(`SELECT result, state, amount_cent FROM order_refund_admissions
		WHERE refund_id = 'AS-ZS'`).Scan(&admissionResult, &admissionState, &admissionAmount); err != nil {
		t.Fatal(err)
	}
	if admissionResult != refundResultPre || admissionState != refundResultPre || admissionAmount != 0 {
		t.Fatalf("zero admission row = %s/%s/%d", admissionResult, admissionState, admissionAmount)
	}
	var permitState string
	if err := db.QueryRow(`SELECT state FROM order_shipping_permits
		WHERE order_no = 'ORD-ZS' AND fulfillment_group_no = 'G1'`).Scan(&permitState); err != nil {
		t.Fatal(err)
	}
	if permitState != permitRevoked {
		t.Fatalf("zero permit = %s, want REVOKED", permitState)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitments WHERE order_no = 'ORD-ZS'`); got != 0 {
		t.Fatalf("zero short-circuit created %d commitments, want 0", got)
	}
	var status string
	var refunded, orderVersion int64
	if err := db.QueryRow(`SELECT status, refunded_cent, version FROM orders WHERE order_no = 'ORD-ZS'`).
		Scan(&status, &refunded, &orderVersion); err != nil {
		t.Fatal(err)
	}
	if status != statusPaid || refunded != 0 {
		t.Fatalf("zero short-circuit changed the order facts: status=%s refunded=%d", status, refunded)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events
		WHERE aggregate_id = 'ORD-ZS' AND event_type = 'order.refund_requested'`); got != 0 {
		t.Fatalf("zero short-circuit sent order.refund_requested: %d", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events
		WHERE aggregate_id = 'ORD-ZS' AND event_type = 'order.refund_admitted'`); got != 1 {
		t.Fatalf("zero admission events = %d, want 1", got)
	}

	// Idempotent repeat returns the same DONE fact without new rows.
	again, err := svc.AdmitRefund(ctx, req)
	if err != nil || again.GetState() != refundApplicationDone || again.GetResult() != refundResultPre {
		t.Fatalf("zero repeat = (%v, %v)", again, err)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_applications WHERE refund_id = 'AS-ZS'`); got != 1 {
		t.Fatalf("zero repeat duplicated the application: %d", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_admissions WHERE refund_id = 'AS-ZS'`); got != 1 {
		t.Fatalf("zero repeat duplicated the admission: %d", got)
	}
}

// T1.4a: an authorized-but-unshipped zero refund cancels the CREATED shipment
// and returns PRE; an already-shipped group returns POST. Both write DONE and
// never touch the permit state of a CONSUMED group.
func TestAdmitRefundSystemZeroConsumedPermit(t *testing.T) {
	for _, tc := range []struct {
		name       string
		cancel     string
		wantResult string
	}{
		{"cancelled", "CANCELLED", refundResultPre},
		{"already shipped", "ALREADY_SHIPPED", refundResultPost},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := orderTestDB(t)
			payments := &fakePaymentPort{}
			svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
			svc.EnableSettlementClients(payments, &fakeFulfillmentPort{status: tc.cancel})
			ctx := context.Background()
			orderNo := "ORD-ZS-CONSUMED-" + tc.cancel
			seedZeroRefundOrder(t, db, orderNo)
			if _, err := db.Exec(`INSERT INTO order_shipping_permits
				(order_no, fulfillment_group_no, state, shipment_no, version)
				VALUES (?, 'G1', 'CONSUMED', 'SHIP-ZS', 1)`, orderNo); err != nil {
				t.Fatal(err)
			}
			reply, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
				RefundId: "AS-" + orderNo, OrderNo: orderNo, GroupNo: "G1",
				AmountCent: 0, Source: refundSourceSystemZero,
			})
			if err != nil {
				t.Fatal(err)
			}
			if reply.GetState() != refundApplicationDone || reply.GetResult() != tc.wantResult {
				t.Fatalf("zero consumed admission = %s/%s, want DONE/%s",
					reply.GetState(), reply.GetResult(), tc.wantResult)
			}
			var permitState string
			var permitVersion int64
			if err := db.QueryRow(`SELECT state, version FROM order_shipping_permits
				WHERE order_no = ? AND fulfillment_group_no = 'G1'`, orderNo).
				Scan(&permitState, &permitVersion); err != nil {
				t.Fatal(err)
			}
			if permitState != permitConsumed || permitVersion != 1 {
				t.Fatalf("CONSUMED permit changed to %s/version %d", permitState, permitVersion)
			}
			var state, result string
			if err := db.QueryRow(`SELECT state, admission_result FROM refund_applications
				WHERE refund_id = ?`, "AS-"+orderNo).Scan(&state, &result); err != nil {
				t.Fatal(err)
			}
			if state != refundApplicationDone || result != tc.wantResult {
				t.Fatalf("zero application = %s/%s", state, result)
			}
			if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_commitments WHERE order_no = ?`, orderNo); got != 0 {
				t.Fatalf("zero consumed refund created %d commitments", got)
			}
			if payments.getCalls != 0 || payments.listCalls != 0 {
				t.Fatalf("zero consumed refund consulted Payment")
			}
		})
	}
}

// AC-67: the short-circuit is exclusive to system_zero + amount 0 on a zero
// order with the zero settlement identity; anything else fails closed before
// writing any row or permit change.
func TestAdmitRefundSystemZeroRejectsMismatchedFacts(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{}
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(payments, &fakeFulfillmentPort{status: "CANCELLED"})
	ctx := context.Background()
	seedZeroRefundOrder(t, db, "ORD-ZS-BAD")
	if _, err := db.Exec(`INSERT INTO orders
		(order_no,user_id,status,total_cent,pay_cent,paid_cent,settlement_payment_no,
		 refunded_cent,refund_state,expire_at,applied_promotions_json)
		VALUES ('ORD-ZS-PAID',1,'PAID',100,100,100,'PAY-ZS-PAID',0,'NONE',NOW(3),'[]')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO orders
		(order_no,user_id,status,total_cent,pay_cent,paid_cent,settlement_payment_no,
		 refunded_cent,refund_state,expire_at,applied_promotions_json)
		VALUES ('ORD-ZS-UNRESOLVED',1,'PAID',0,0,0,'',0,'NONE',NOW(3),'[]')`); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		orderNo string
		amount  int64
		source  string
	}{
		{"positive amount", "ORD-ZS-BAD", 1, refundSourceSystemZero},
		{"positive order", "ORD-ZS-PAID", 0, refundSourceSystemZero},
		{"no zero settlement identity", "ORD-ZS-UNRESOLVED", 0, refundSourceSystemZero},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
				RefundId: "AS-" + tc.name, OrderNo: tc.orderNo, GroupNo: "G1",
				AmountCent: tc.amount, Source: tc.source,
			})
			if err == nil {
				t.Fatal("mismatched zero refund was admitted")
			}
			if !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
				t.Fatalf("error = %v, want FAILED_PRECONDITION", err)
			}
			if got := countRows(t, db, `SELECT COUNT(*) FROM refund_applications WHERE refund_id = ?`, "AS-"+tc.name); got != 0 {
				t.Fatalf("mismatched zero refund wrote %d applications", got)
			}
			if got := countRows(t, db, `SELECT COUNT(*) FROM order_refund_admissions WHERE refund_id = ?`, "AS-"+tc.name); got != 0 {
				t.Fatalf("mismatched zero refund wrote %d admissions", got)
			}
			if got := countRows(t, db, `SELECT COUNT(*) FROM order_shipping_permits WHERE order_no = ?`, tc.orderNo); got != 0 {
				t.Fatalf("mismatched zero refund wrote permits")
			}
		})
	}
	// T2.4: historical zero orders may consult ListOrderIntents once to resolve
	// the zero identity; they must still never read an intent or refund money.
	if payments.getCalls != 0 || len(payments.refundCalls) != 0 {
		t.Fatalf("rejected zero refunds read/refunded Payment: get=%d refunds=%d", payments.getCalls, len(payments.refundCalls))
	}
}

// C-05 order/0021: the database refuses a system_zero row that is not a
// zero-amount, non-effective application.
func TestZeroApplicationCheckConstraint(t *testing.T) {
	db := orderTestDB(t)
	var constraints int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
		WHERE CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = 'refund_applications'
		  AND CONSTRAINT_NAME = 'chk_refund_app_system_zero' AND CONSTRAINT_TYPE = 'CHECK'`).Scan(&constraints); err != nil {
		t.Fatal(err)
	}
	if constraints != 1 {
		t.Fatalf("chk_refund_app_system_zero constraints = %d, want 1", constraints)
	}
	for _, tc := range []struct {
		name         string
		refundID     string
		amount       int64
		goods        int64
		freight      int64
		nonEffective int
	}{
		{"positive amount", "R-CHK-AMOUNT", 100, 100, 0, 0},
		{"non-effective capture", "R-CHK-NONEFFECTIVE", 0, 0, 0, 1},
		{"goods only", "R-CHK-GOODS", 100, 50, 50, 0},
	} {
		_, err := db.Exec(`INSERT INTO refund_applications
			(refund_id, source, order_no, group_no, non_effective_capture,
			 amount_cent, goods_cent, freight_cent, state)
			VALUES (?, 'system_zero', 'ORD-CHK', 'G1', ?, ?, ?, ?, 'DONE')`,
			tc.refundID, tc.nonEffective, tc.amount, tc.goods, tc.freight)
		if err == nil {
			t.Fatalf("%s: system_zero row passed the CHECK", tc.name)
		}
	}
	if _, err := db.Exec(`INSERT INTO refund_applications
		(refund_id, source, order_no, group_no, non_effective_capture,
		 amount_cent, goods_cent, freight_cent, state, admission_result)
		VALUES ('R-CHK-OK', 'system_zero', 'ORD-CHK', 'G1', 0, 0, 0, 0, 'DONE', 'PRE_SHIP_REVOKED')`); err != nil {
		t.Fatalf("valid system_zero row rejected: %v", err)
	}
}
