package order

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"

	"google.golang.org/grpc"
)

// T1.6 evidence (SPEC-0002 v1.9 AC-46/AC-65/AC-67, ADR-0007 Amd1 T1.6 Addendum
// §2/§3). The guarantees are transactional, so the tests run against real MySQL
// (TEST_MYSQL_DSN).

// t16InventoryPort records the order-level inventory compensation and can land a
// concurrent cancel exactly inside MarkPaid's Confirm window.
type t16InventoryPort struct {
	db                 *sql.DB
	cancelOnConfirm    bool
	confirmCalls       int
	cancelConfirmCalls int
	cancelConfirmErr   error
	cancelConfirmItems []*pb.OrderItem
}

func (f *t16InventoryPort) AllocateOrder(context.Context, *pb.AllocateOrderRequest, ...grpc.CallOption) (*pb.AllocateOrderReply, error) {
	return &pb.AllocateOrderReply{}, nil
}

func (f *t16InventoryPort) Confirm(_ context.Context, in *pb.ReserveRequest, _ ...grpc.CallOption) (*pb.Empty, error) {
	f.confirmCalls++
	if f.cancelOnConfirm {
		f.cancelOnConfirm = false
		if _, err := f.db.Exec(`UPDATE orders SET status = ?, version = version + 1, updated_at = NOW(3)
			WHERE order_no = ? AND status = ?`, statusCancelled, in.GetOrderNo(), statusPendingPayment); err != nil {
			return nil, err
		}
	}
	return &pb.Empty{}, nil
}

func (f *t16InventoryPort) Release(context.Context, *pb.ReserveRequest, ...grpc.CallOption) (*pb.Empty, error) {
	return &pb.Empty{}, nil
}

func (f *t16InventoryPort) CreateSeckillReservation(context.Context, *pb.CreateSeckillReservationRequest, ...grpc.CallOption) (*pb.ReserveReply, error) {
	return &pb.ReserveReply{}, nil
}

func (f *t16InventoryPort) ReleaseSeckillReservation(context.Context, *pb.ReleaseSeckillReservationRequest, ...grpc.CallOption) (*pb.Empty, error) {
	return &pb.Empty{}, nil
}

func (f *t16InventoryPort) TccCancelConfirm(_ context.Context, in *pb.TccPaymentRequest, _ ...grpc.CallOption) (*pb.Empty, error) {
	f.cancelConfirmCalls++
	f.cancelConfirmItems = append(f.cancelConfirmItems, in.GetItems()...)
	if f.cancelConfirmErr != nil {
		return nil, f.cancelConfirmErr
	}
	return &pb.Empty{}, nil
}

func (f *t16InventoryPort) CheckOrderReady(context.Context, *pb.CheckOrderReadyRequest, ...grpc.CallOption) (*pb.CheckOrderReadyReply, error) {
	return &pb.CheckOrderReadyReply{Ready: true}, nil
}

func t16OutboxPayloads(t *testing.T, db *sql.DB, orderNo, eventType string) []map[string]any {
	t.Helper()
	rows, err := db.Query(`SELECT payload FROM outbox_events WHERE aggregate_id = ? AND event_type = ? ORDER BY id`,
		orderNo, eventType)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var payload map[string]any
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatal(err)
		}
		out = append(out, payload)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func t16AssertNoGroupLedger(t *testing.T, db *sql.DB, orderNo string) {
	t.Helper()
	for _, table := range []string{
		"refund_batches", "refund_applications", "order_refund_admissions",
		"order_refund_commitments", "order_shipping_permits",
	} {
		if got := countRows(t, db, `SELECT COUNT(*) FROM `+table+` WHERE order_no = ?`, orderNo); got != 0 {
			t.Fatalf("T1.6 wrote %s rows: %d", table, got)
		}
	}
}

func TestT1_6PendingCapturedCASWins(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	inventory := &t16InventoryPort{db: db}
	svc := New(db, nil, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(payments, &fakeFulfillmentPort{status: "CANCELLED"})
	svc.inventory = inventory
	ctx := context.Background()
	const orderNo = "ORD-T16-PENDING-WIN"
	seedTccPendingOrder(t, db, orderNo, 10000, 0, 0)
	seedTccGroup(t, db, orderNo, "G1", 1, 0)
	seedTccLine(t, db, orderNo, 1, "G1", 10000)

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: orderNo, PaymentNo: "PAY-T16-WIN", PaidCent: 10000,
	}); err != nil {
		t.Fatalf("pending captured cancel: %v", err)
	}

	var status, refundState string
	var paidCent int64
	if err := db.QueryRow(`SELECT status, refund_state, paid_cent FROM orders WHERE order_no = ?`, orderNo).
		Scan(&status, &refundState, &paidCent); err != nil {
		t.Fatal(err)
	}
	if status != statusCancelled || refundState != refundStateNone {
		t.Fatalf("order = %s/%s, want CANCELLED/NONE", status, refundState)
	}
	if paidCent != 0 {
		t.Fatalf("pre-PAID capture leaked into orders.paid_cent: %d", paidCent)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = ? AND event_type = 'order.cancelled'`, orderNo); got != 1 {
		t.Fatalf("order.cancelled = %d, want 1", got)
	}
	commands := t16OutboxPayloads(t, db, orderNo, "order.refund_requested")
	if len(commands) != 1 {
		t.Fatalf("payment cancel commands = %d, want 1", len(commands))
	}
	command := commands[0]
	if command["refund_id"] != "pay:PAY-T16-WIN:cancel" {
		t.Fatalf("refund_id = %v", command["refund_id"])
	}
	if command["source"] != paymentCancelSource {
		t.Fatalf("source = %v", command["source"])
	}
	if command["non_effective_capture"] != true {
		t.Fatalf("non_effective_capture = %v", command["non_effective_capture"])
	}
	if command["amount_cent"] != float64(10000) || command["paid_cent"] != float64(10000) {
		t.Fatalf("amount/paid = %v/%v, want 10000/10000", command["amount_cent"], command["paid_cent"])
	}
	t16AssertNoGroupLedger(t, db, orderNo)
	// The authoritative captured amount is read from Payment before the CAS;
	// Order itself never calls Payment.Refund.
	if payments.getCalls == 0 {
		t.Fatal("pending captured cancel did not read the authoritative Payment snapshot")
	}
	if len(payments.refundCalls) != 0 {
		t.Fatalf("Order called Payment.Refund directly: %d", len(payments.refundCalls))
	}

	// Repeat is idempotent for the lifecycle: no second order.cancelled. The
	// command may be re-emitted with the same fixed refund_id, which Payment
	// deduplicates.
	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: orderNo, PaymentNo: "PAY-T16-WIN", PaidCent: 10000,
	}); err != nil {
		t.Fatalf("repeat pending captured cancel: %v", err)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = ? AND event_type = 'order.cancelled'`, orderNo); got != 1 {
		t.Fatalf("repeat wrote order.cancelled = %d, want 1", got)
	}
	commands = t16OutboxPayloads(t, db, orderNo, "order.refund_requested")
	for _, command := range commands {
		if command["refund_id"] != "pay:PAY-T16-WIN:cancel" {
			t.Fatalf("repeat used a different refund_id: %v", command["refund_id"])
		}
	}
}

func TestT1_6PendingCapturedCASLostToPaymentContinuesGroupFanout(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	var once sync.Once
	payments.beforeGet = func() {
		// Payment confirmation wins between the authoritative snapshot and the
		// order CAS, exactly the AC-46 race the branch must lose safely.
		once.Do(func() {
			if _, err := db.Exec(`UPDATE orders SET status = ?, settlement_payment_no = ?, paid_cent = ?, version = version + 1
				WHERE order_no = ?`, statusPaid, "PAY-T16-LOST", 10000, "ORD-T16-PENDING-LOST"); err != nil {
				t.Errorf("simulate payment win: %v", err)
			}
		})
	}
	var logs bytes.Buffer
	svc := New(db, nil, slog.New(slog.NewTextHandler(&logs, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(payments, &fakeFulfillmentPort{status: "CANCELLED"})
	ctx := context.Background()
	const orderNo = "ORD-T16-PENDING-LOST"
	seedTccPendingOrder(t, db, orderNo, 10000, 0, 0)
	seedTccGroup(t, db, orderNo, "G1", 1, 0)
	seedTccLine(t, db, orderNo, 1, "G1", 10000)

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: orderNo, PaymentNo: "PAY-T16-LOST", PaidCent: 10000,
	}); err != nil {
		// The test fixture must exist before the goroutine helper is useful.
		t.Fatalf("CAS-lost compensation: %v", err)
	}
	if !strings.Contains(logs.String(), tccErrCancelCASLost) {
		t.Fatalf("CAS-lost reason %s not logged; logs=%s", tccErrCancelCASLost, logs.String())
	}
	var status, refundState string
	if err := db.QueryRow(`SELECT status, refund_state FROM orders WHERE order_no = ?`, orderNo).
		Scan(&status, &refundState); err != nil {
		t.Fatal(err)
	}
	if status != statusPaid || refundState != refundStateRefunding {
		t.Fatalf("order = %s/%s, want PAID/REFUNDING (group fan-out)", status, refundState)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = ?`, orderNo); got != 1 {
		t.Fatalf("group refund batches = %d, want 1", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = ? AND event_type = 'order.refund_requested'`, orderNo); got != 0 {
		t.Fatalf("CAS lost to PAID still wrote payment-level commands: %d", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = ? AND event_type = 'order.cancelled'`, orderNo); got != 0 {
		t.Fatalf("CAS lost to PAID wrote order.cancelled: %d", got)
	}
}

func TestT1_6PendingCapturedOutboxFailureRollsBack(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	const orderNo = "ORD-T16-PENDING-OUTBOX"
	seedTccPendingOrder(t, db, orderNo, 10000, 0, 0)
	seedTccGroup(t, db, orderNo, "G1", 1, 0)
	seedTccLine(t, db, orderNo, 1, "G1", 10000)
	version := tccVersion(t, db, orderNo)

	_, _ = db.Exec(`DROP TRIGGER IF EXISTS t16_fail_refund_cmd`)
	if _, err := db.Exec(`CREATE TRIGGER t16_fail_refund_cmd BEFORE INSERT ON outbox_events
		FOR EACH ROW
		BEGIN
			IF NEW.event_type = 'order.refund_requested' THEN
				SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'injected payment-cancel outbox failure';
			END IF;
		END`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP TRIGGER IF EXISTS t16_fail_refund_cmd`) })

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: orderNo, PaymentNo: "PAY-T16-OUTBOX", PaidCent: 10000,
	}); err == nil {
		t.Fatal("outbox failure was reported as a successful cancel")
	}

	var status string
	var versionAfter int64
	if err := db.QueryRow(`SELECT status, version FROM orders WHERE order_no = ?`, orderNo).
		Scan(&status, &versionAfter); err != nil {
		t.Fatal(err)
	}
	if status != statusPendingPayment || versionAfter != version {
		t.Fatalf("outbox failure left a partial cancel: status=%s version=%d want PENDING/%d", status, versionAfter, version)
	}
	for _, eventType := range []string{"order.cancelled", "order.refund_requested"} {
		if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = ? AND event_type = ?`, orderNo, eventType); got != 0 {
			t.Fatalf("%s survived the rollback: %d", eventType, got)
		}
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM order_status_logs WHERE order_no = ?`, orderNo); got != 0 {
		t.Fatalf("status log survived the rollback: %d", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = ?`, orderNo); got != 0 {
		t.Fatalf("outbox failure created a group batch: %d", got)
	}
	if len(payments.refundCalls) != 0 {
		t.Fatalf("Order called Payment.Refund directly: %d", len(payments.refundCalls))
	}
}

func TestT1_6PendingCapturedAlreadyRefundedWritesNoCommand(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 1000, refunded: 1000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	const orderNo = "ORD-T16-PENDING-FULL"
	seedTccPendingOrder(t, db, orderNo, 1000, 0, 0)
	seedTccGroup(t, db, orderNo, "G1", 1, 0)
	seedTccLine(t, db, orderNo, 1, "G1", 1000)

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: orderNo, PaymentNo: "PAY-T16-FULL", PaidCent: 1000,
	}); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM orders WHERE order_no = ?`, orderNo).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != statusCancelled {
		t.Fatalf("status = %s, want CANCELLED", status)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = ? AND event_type = 'order.cancelled'`, orderNo); got != 1 {
		t.Fatalf("order.cancelled = %d, want 1", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = ? AND event_type = 'order.refund_requested'`, orderNo); got != 0 {
		t.Fatalf("fully refunded capture still wrote a command: %d", got)
	}
	t16AssertNoGroupLedger(t, db, orderNo)
}

func TestT1_6CancelledCapturedWritesCommand(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 1000}
	inventory := &t16InventoryPort{db: db}
	svc := New(db, nil, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(payments, &fakeFulfillmentPort{status: "CANCELLED"})
	svc.inventory = inventory
	ctx := context.Background()
	const orderNo = "ORD-T16-CANCELLED"
	seedTccOrder(t, db, orderNo, 1000, 1000, 0, 0, "PAY-T16-CAN")
	seedTccLine(t, db, orderNo, 1, "G1", 1000)
	setTccOrderStatus(t, db, orderNo, statusCancelled)

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: orderNo, PaymentNo: "PAY-T16-CAN", PaidCent: 1000,
	}); err != nil {
		t.Fatalf("cancelled captured cancel: %v", err)
	}
	commands := t16OutboxPayloads(t, db, orderNo, "order.refund_requested")
	if len(commands) != 1 {
		t.Fatalf("payment cancel commands = %d, want 1", len(commands))
	}
	if commands[0]["refund_id"] != "pay:PAY-T16-CAN:cancel" {
		t.Fatalf("refund_id = %v", commands[0]["refund_id"])
	}
	// The order.cancelled event belongs to the cancel that already won; this
	// repair must not create a second one or any group ledger.
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = ? AND event_type = 'order.cancelled'`, orderNo); got != 0 {
		t.Fatalf("cancelled repair wrote order.cancelled: %d", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM refund_batches WHERE order_no = ?`, orderNo); got != 0 {
		t.Fatalf("cancelled repair wrote group batches: %d", got)
	}
	if inventory.cancelConfirmCalls != 1 {
		t.Fatalf("cancelled repair TccCancelConfirm calls = %d, want 1", inventory.cancelConfirmCalls)
	}
	if len(payments.refundCalls) != 0 {
		t.Fatalf("Order called Payment.Refund directly: %d", len(payments.refundCalls))
	}
}

// B3: a nil Inventory client is a configuration error for the direct
// cancel-after-confirm compensation; it must not silently write a command.
func TestT1_6CancelledCapturedRequiresInventory(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 1000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	const orderNo = "ORD-T16-CANCELLED-NO-INV"
	seedTccOrder(t, db, orderNo, 1000, 1000, 0, 0, "PAY-T16-CAN-NO-INV")
	setTccOrderStatus(t, db, orderNo, statusCancelled)

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: orderNo, PaymentNo: "PAY-T16-CAN-NO-INV", PaidCent: 1000,
	}); err == nil {
		t.Fatal("nil inventory client was accepted for cancel-after-confirm compensation")
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = ? AND event_type = 'order.refund_requested'`, orderNo); got != 0 {
		t.Fatalf("nil inventory wrote a payment command: %d", got)
	}
}

// A payment confirmation that already confirmed inventory and then lost the
// cancel CAS must restock through TccCancelConfirm and persist the command with
// the authoritative captured amount (not the order/request amount).
func TestT1_6RefundCancelledAfterConfirmRestocksAndPersistsCommand(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 800}
	inventory := &t16InventoryPort{db: db, cancelOnConfirm: true}
	svc := New(db, nil, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(payments, &fakeFulfillmentPort{status: "CANCELLED"})
	svc.inventory = inventory
	ctx := context.Background()
	const orderNo = "ORD-T16-AFTER-CONFIRM"
	seedTccPendingOrder(t, db, orderNo, 1000, 0, 0)
	seedTccGroup(t, db, orderNo, "G1", 1, 0)
	seedTccLine(t, db, orderNo, 1, "G1", 1000)

	_, err := svc.MarkPaid(ctx, &pb.MarkPaidRequest{OrderNo: orderNo, PaymentNo: "PAY-T16-AFC", PaidCent: 1000})
	if err == nil || !apperr.IsCode(err, apperr.CodeConflict) {
		t.Fatalf("MarkPaid = %v, want CONFLICT after the cancel won", err)
	}
	if inventory.confirmCalls == 0 {
		t.Fatal("MarkPaid did not confirm inventory before the CAS")
	}
	if inventory.cancelConfirmCalls != 1 {
		t.Fatalf("TccCancelConfirm calls = %d, want 1 (CONFIRMED restock)", inventory.cancelConfirmCalls)
	}
	commands := t16OutboxPayloads(t, db, orderNo, "order.refund_requested")
	if len(commands) != 1 {
		t.Fatalf("payment cancel commands = %d, want 1", len(commands))
	}
	command := commands[0]
	if command["refund_id"] != "pay:PAY-T16-AFC:cancel" {
		t.Fatalf("refund_id = %v", command["refund_id"])
	}
	if command["amount_cent"] != float64(800) || command["paid_cent"] != float64(800) {
		t.Fatalf("amount/paid = %v/%v, want authoritative captured 800", command["amount_cent"], command["paid_cent"])
	}
	if command["source"] != paymentCancelSource || command["non_effective_capture"] != true {
		t.Fatalf("command source/non_effective = %v/%v", command["source"], command["non_effective_capture"])
	}
	if len(payments.refundCalls) != 0 {
		t.Fatalf("Order called Payment.Refund directly: %d", len(payments.refundCalls))
	}

	// Repeat is idempotent: the same fixed refund_id is reused and the restock
	// compensation is replayed, never a second refund identity.
	updated, err := findOrder(ctx, db, orderNo)
	if err != nil || updated == nil {
		t.Fatalf("reload cancelled order: %v", err)
	}
	if err := svc.refundCancelledAfterConfirm(ctx, updated, "PAY-T16-AFC"); err != nil {
		t.Fatal(err)
	}
	commands = t16OutboxPayloads(t, db, orderNo, "order.refund_requested")
	if len(commands) < 2 {
		t.Fatalf("repeat did not persist a replayable command: %d", len(commands))
	}
	for _, command := range commands {
		if command["refund_id"] != "pay:PAY-T16-AFC:cancel" {
			t.Fatalf("repeat changed refund_id: %v", command["refund_id"])
		}
	}
	if inventory.cancelConfirmCalls != 2 {
		t.Fatalf("repeat TccCancelConfirm calls = %d, want 2 (idempotent replay)", inventory.cancelConfirmCalls)
	}
}

func TestT1_6RefundCancelledAfterConfirmWithoutCaptureWritesNothing(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 1000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	const orderNo = "ORD-T16-AFC-NOCAP"
	seedTccOrder(t, db, orderNo, 1000, 1000, 0, 0, "PAY-T16-AFC-NOCAP")
	setTccOrderStatus(t, db, orderNo, statusCancelled)
	record, err := findOrder(ctx, db, orderNo)
	if err != nil || record == nil {
		t.Fatalf("seed cancelled order: %v", err)
	}

	if err := svc.refundCancelledAfterConfirm(ctx, record, ""); err != nil {
		t.Fatal(err)
	}
	payments.captured = 0
	if err := svc.refundCancelledAfterConfirm(ctx, record, "PAY-T16-AFC-NOCAP"); err != nil {
		t.Fatal(err)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = ? AND event_type = 'order.refund_requested'`, orderNo); got != 0 {
		t.Fatalf("no-capture path wrote commands: %d", got)
	}
	if payments.getCalls != 1 {
		t.Fatalf("no-capture path Payment reads = %d, want 1 authoritative check", payments.getCalls)
	}
}

// AC-46 / T1.6 §2.1: concurrent cancels of the same captured PENDING_PAYMENT
// order have a single order-CAS winner. Losers converge to CANCELLED and only
// replay the same fixed refund identity; no second order.cancelled is written.
func TestT1_6ConcurrentPendingCapturedCancelSingleWinner(t *testing.T) {
	db := orderTestDB(t)
	payments := &countingPaymentPort{captured: 10000}
	inventory := &t16InventoryPort{db: db}
	svc := New(db, nil, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(payments, &fakeFulfillmentPort{status: "CANCELLED"})
	svc.inventory = inventory
	ctx := context.Background()
	const orderNo = "ORD-T16-PENDING-RACE"
	seedTccPendingOrder(t, db, orderNo, 10000, 0, 0)
	seedTccGroup(t, db, orderNo, "G1", 1, 0)
	seedTccLine(t, db, orderNo, 1, "G1", 10000)
	version := tccVersion(t, db, orderNo)

	const callers = 6
	var wg sync.WaitGroup
	errs := make([]error, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, errs[index] = svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
				OrderNo: orderNo, PaymentNo: "PAY-T16-RACE", PaidCent: 10000,
			})
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent cancel %d: %v", i, err)
		}
	}
	var status string
	var versionAfter int64
	if err := db.QueryRow(`SELECT status, version FROM orders WHERE order_no = ?`, orderNo).
		Scan(&status, &versionAfter); err != nil {
		t.Fatal(err)
	}
	if status != statusCancelled || versionAfter != version+1 {
		t.Fatalf("order = %s/version %d, want CANCELLED/%d (single CAS winner)", status, versionAfter, version+1)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = ? AND event_type = 'order.cancelled'`, orderNo); got != 1 {
		t.Fatalf("order.cancelled = %d, want 1", got)
	}
	commands := t16OutboxPayloads(t, db, orderNo, "order.refund_requested")
	if len(commands) == 0 {
		t.Fatal("no payment-level cancel command was persisted")
	}
	for _, command := range commands {
		if command["refund_id"] != "pay:PAY-T16-RACE:cancel" {
			t.Fatalf("concurrent caller used a different refund_id: %v", command["refund_id"])
		}
	}
	t16AssertNoGroupLedger(t, db, orderNo)
}

// B2 + B1: PENDING_PAYMENT must read the authoritative Payment snapshot even
// when req.paid_cent=0, and the order.cancelled payload must carry the line's
// warehouse_id (W2) so Inventory cannot compensate W1 by default.
func TestT1_6PendingCapturedPaidCentZeroUsesAuthorityAndWarehouse(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 10000}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	const orderNo = "ORD-T16-PAIDZERO-W2"
	seedTccPendingOrder(t, db, orderNo, 10000, 0, 0)
	seedTccGroup(t, db, orderNo, "G1", 1, 0)
	if _, err := db.Exec(`INSERT INTO order_items
		(order_no,line_no,sku_id,name,quantity,warehouse_id,fulfillment_group_no,line_paid_cent,unit_paid_cent)
		VALUES (?, 1, 1, 'x', 1, 2, 'G1', 10000, 10000)`, orderNo); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: orderNo, PaymentNo: "PAY-T16-PAIDZERO", PaidCent: 0,
	}); err != nil {
		t.Fatalf("paid_cent=0 with authoritative capture: %v", err)
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM orders WHERE order_no = ?`, orderNo).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != statusCancelled {
		t.Fatalf("status = %s, want CANCELLED", status)
	}
	commands := t16OutboxPayloads(t, db, orderNo, "order.refund_requested")
	if len(commands) != 1 {
		t.Fatalf("payment commands = %d, want 1", len(commands))
	}
	if commands[0]["amount_cent"] != float64(10000) || commands[0]["refund_id"] != "pay:PAY-T16-PAIDZERO:cancel" {
		t.Fatalf("command = %+v", commands[0])
	}
	cancelled := t16OutboxPayloads(t, db, orderNo, "order.cancelled")
	if len(cancelled) != 1 {
		t.Fatalf("order.cancelled = %d, want 1", len(cancelled))
	}
	items, _ := cancelled[0]["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("order.cancelled items = %+v", cancelled[0]["items"])
	}
	item, _ := items[0].(map[string]any)
	if item["warehouse_id"] != float64(2) {
		t.Fatalf("order.cancelled item warehouse_id = %v, want 2", item["warehouse_id"])
	}
	if payments.getCalls == 0 {
		t.Fatal("PENDING cancel did not read the authoritative Payment snapshot")
	}
}

// B1/B3: the durable Inventory compensation payload must carry warehouse_id and
// must return append failures to the caller instead of only logging them.
func TestT1_6QueueInventoryCompensationCarriesWarehouseAndReturnsError(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil, nil, nil, nil)
	ctx := context.Background()
	const orderNo = "ORD-T16-COMP-QUEUE"

	if err := svc.queueInventoryCompensation(ctx, orderNo, []*pb.OrderItem{
		{SkuId: 1, WarehouseId: 2, Quantity: 1},
	}); err != nil {
		t.Fatal(err)
	}
	events := t16OutboxPayloads(t, db, orderNo, "inventory.compensation_requested")
	if len(events) != 1 {
		t.Fatalf("compensation events = %d, want 1", len(events))
	}
	items, _ := events[0]["items"].([]any)
	item, _ := items[0].(map[string]any)
	if item["warehouse_id"] != float64(2) {
		t.Fatalf("compensation item warehouse_id = %v, want 2", item["warehouse_id"])
	}

	if err := svc.queueInventoryCompensation(ctx, "ORD-T16-COMP-MISSING", []*pb.OrderItem{
		{SkuId: 1, Quantity: 1},
	}); err == nil {
		t.Fatal("compensation without warehouse_id was accepted")
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = 'ORD-T16-COMP-MISSING'`); got != 0 {
		t.Fatalf("missing-warehouse compensation wrote outbox rows: %d", got)
	}

	_, _ = db.Exec(`DROP TRIGGER IF EXISTS t16_fail_compensation`)
	if _, err := db.Exec(`CREATE TRIGGER t16_fail_compensation BEFORE INSERT ON outbox_events
		FOR EACH ROW
		BEGIN
			IF NEW.event_type = 'inventory.compensation_requested' THEN
				SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'injected compensation outbox failure';
			END IF;
		END`); err != nil {
		t.Fatalf("create compensation failure trigger: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP TRIGGER IF EXISTS t16_fail_compensation`) })
	if err := svc.queueInventoryCompensation(ctx, orderNo, []*pb.OrderItem{
		{SkuId: 1, WarehouseId: 2, Quantity: 1},
	}); err == nil {
		t.Fatal("compensation outbox failure was not returned")
	}
}

// Ruling 3: an Inventory direct-compensation failure is persisted, but the
// payment command is still written so captured money is not blocked by stock.
func TestT1_6CancelAfterConfirmInventoryFailureStillRefunds(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 1000}
	inventory := &t16InventoryPort{db: db, cancelConfirmErr: apperr.Unavailable("inventory down", nil)}
	svc := New(db, nil, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(payments, &fakeFulfillmentPort{status: "CANCELLED"})
	svc.inventory = inventory
	ctx := context.Background()
	const orderNo = "ORD-T16-AFC-INVFAIL"
	seedTccOrder(t, db, orderNo, 1000, 1000, 0, 0, "PAY-T16-AFC-INVFAIL")
	seedTccLine(t, db, orderNo, 1, "G1", 1000)
	setTccOrderStatus(t, db, orderNo, statusCancelled)
	record, err := findOrder(ctx, db, orderNo)
	if err != nil || record == nil {
		t.Fatalf("reload cancelled order: %v", err)
	}

	if err := svc.refundCancelledAfterConfirm(ctx, record, "PAY-T16-AFC-INVFAIL"); err != nil {
		t.Fatal(err)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = ? AND event_type = 'inventory.compensation_requested'`, orderNo); got != 1 {
		t.Fatalf("durable inventory compensation = %d, want 1", got)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = ? AND event_type = 'order.refund_requested'`, orderNo); got != 1 {
		t.Fatalf("payment command = %d, want 1", got)
	}
}

// B3: if the durable Inventory compensation cannot be persisted, the direct
// compensation is a real error and no payment command may be written.
func TestT1_6CancelAfterConfirmCompensationAppendFailureBlocksRefund(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 1000}
	inventory := &t16InventoryPort{db: db, cancelConfirmErr: apperr.Unavailable("inventory down", nil)}
	svc := New(db, nil, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(payments, &fakeFulfillmentPort{status: "CANCELLED"})
	svc.inventory = inventory
	ctx := context.Background()
	const orderNo = "ORD-T16-AFC-COMPFAIL"
	seedTccOrder(t, db, orderNo, 1000, 1000, 0, 0, "PAY-T16-AFC-COMPFAIL")
	seedTccLine(t, db, orderNo, 1, "G1", 1000)
	setTccOrderStatus(t, db, orderNo, statusCancelled)
	record, err := findOrder(ctx, db, orderNo)
	if err != nil || record == nil {
		t.Fatalf("reload cancelled order: %v", err)
	}

	_, _ = db.Exec(`DROP TRIGGER IF EXISTS t16_fail_compensation`)
	if _, err := db.Exec(`CREATE TRIGGER t16_fail_compensation BEFORE INSERT ON outbox_events
		FOR EACH ROW
		BEGIN
			IF NEW.event_type = 'inventory.compensation_requested' THEN
				SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'injected compensation outbox failure';
			END IF;
		END`); err != nil {
		t.Fatalf("create compensation failure trigger: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP TRIGGER IF EXISTS t16_fail_compensation`) })

	if err := svc.refundCancelledAfterConfirm(ctx, record, "PAY-T16-AFC-COMPFAIL"); err == nil {
		t.Fatal("unpersisted inventory compensation did not fail the refund path")
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = ? AND event_type = 'order.refund_requested'`, orderNo); got != 0 {
		t.Fatalf("unpersisted compensation still wrote a payment command: %d", got)
	}
}

// B2/裁决 5: an unavailable Payment authority must fail closed before any
// order/cancel/refund write, even when req.paid_cent=0.
func TestT1_6PendingCapturedPaymentUnavailableFailsClosed(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{getErr: apperr.Unavailable("payment intent timeout", nil)}
	svc := tccTestService(t, db, payments)
	ctx := context.Background()
	const orderNo = "ORD-T16-PENDING-UNAVAILABLE"
	seedTccPendingOrder(t, db, orderNo, 10000, 0, 0)
	seedTccGroup(t, db, orderNo, "G1", 1, 0)
	seedTccLine(t, db, orderNo, 1, "G1", 10000)
	version := tccVersion(t, db, orderNo)

	if _, err := svc.TccCancelMarkPaid(ctx, &pb.TccPaymentRequest{
		OrderNo: orderNo, PaymentNo: "PAY-T16-UNAVAILABLE", PaidCent: 0,
	}); err == nil {
		t.Fatal("unavailable Payment authority was accepted")
	}
	var status string
	var versionAfter int64
	if err := db.QueryRow(`SELECT status, version FROM orders WHERE order_no = ?`, orderNo).
		Scan(&status, &versionAfter); err != nil {
		t.Fatal(err)
	}
	if status != statusPendingPayment || versionAfter != version {
		t.Fatalf("Payment-unavailable cancel mutated order: %s/version %d", status, versionAfter)
	}
	if got := countRows(t, db, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = ?`, orderNo); got != 0 {
		t.Fatalf("Payment-unavailable cancel wrote outbox events: %d", got)
	}
}
