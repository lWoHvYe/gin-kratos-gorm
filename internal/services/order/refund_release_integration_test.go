package order

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/events"

	"google.golang.org/grpc"
)

// serialPaymentPort serializes the shared fake for the concurrent admission
// test so `go test -race` sees no fake-internal data race.
type serialPaymentPort struct {
	*fakePaymentPort
	mu sync.Mutex
}

func (p *serialPaymentPort) GetIntent(ctx context.Context, in *pb.GetIntentRequest, opts ...grpc.CallOption) (*pb.Intent, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.fakePaymentPort.GetIntent(ctx, in, opts...)
}

type releaseTestItem struct {
	lineNo      int
	skuID       int64
	quantity    int64
	warehouseID int64
	paidCent    int64
	groupNo     string
}

// seedReleaseOrder writes the frozen order facts an unshipped-group refund
// admission needs: paid order, group lines and an optional permit.
func seedReleaseOrder(t *testing.T, db *sql.DB, orderNo string, totalCent, payCent int64, settlement string, items []releaseTestItem, permitState string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO orders
		(order_no,user_id,status,total_cent,discount_cent,promotion_discount_cent,applied_promotions_json,
		 freight_cent,pay_cent,paid_cent,settlement_payment_no,refunded_cent,refund_status,refund_state,
		 receiver_name,receiver_phone,receiver_address,quote_id,client_request_id,
		 allocation_frozen_at,allocation_hash,package_count,warehouse_count,expire_at,created_at)
		VALUES (?,1,'PAID',?,0,0,'[]',0,?,?,?,0,'','NONE','A','1','R','','',NOW(3),'h',1,1,NOW(3),NOW(3))`,
		orderNo, totalCent, payCent, payCent, settlement); err != nil {
		t.Fatalf("seed order %s: %v", orderNo, err)
	}
	for _, item := range items {
		if _, err := db.Exec(`INSERT INTO order_items
			(order_no,line_no,sku_id,name,quantity,warehouse_id,fulfillment_group_no,line_paid_cent,unit_paid_cent)
			VALUES (?,?,?,'x',?,?,?,?,?)`,
			orderNo, item.lineNo, item.skuID, item.quantity, item.warehouseID, item.groupNo, item.paidCent, item.paidCent); err != nil {
			t.Fatalf("seed order item %s/%d: %v", orderNo, item.lineNo, err)
		}
	}
	if permitState != "" {
		if _, err := db.Exec(`INSERT INTO order_shipping_permits (order_no, fulfillment_group_no, state)
			VALUES (?, 'G1', ?)`, orderNo, permitState); err != nil {
			t.Fatalf("seed permit %s: %v", orderNo, err)
		}
	}
}

func loadRefundReleaseEvents(t *testing.T, db *sql.DB, orderNo string) []events.RefundReleaseRequested {
	t.Helper()
	rows, err := db.Query(`SELECT payload FROM outbox_events
		WHERE event_type = ? AND aggregate_id = ? ORDER BY id`, events.RefundReleaseEventType, orderNo)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []events.RefundReleaseRequested
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		if err := events.ValidateJSON(events.RefundReleaseEventType, raw); err != nil {
			t.Fatalf("stored release event violates the registry: %v", err)
		}
		var ev events.RefundReleaseRequested
		if err := json.Unmarshal(raw, &ev); err != nil {
			t.Fatal(err)
		}
		out = append(out, ev)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func refundAdmittedCount(t *testing.T, db *sql.DB, orderNo string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM outbox_events
		WHERE event_type = 'order.refund_admitted' AND aggregate_id = ?`, orderNo).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRefundReleaseSourceMapping(t *testing.T) {
	cases := []struct {
		in     string
		source string
		reason string
	}{
		{"tcc", events.RefundReleaseSourceTCC, events.RefundReleaseReasonTCCCompensation},
		{"system_zero", events.RefundReleaseSourceSystemZero, events.RefundReleaseReasonZeroAfterSale},
		{"system_shortage", events.RefundReleaseSourceSystemShortage, events.RefundReleaseReasonShortageRefund},
		{"aftersale", events.RefundReleaseSourceAfterSale, events.RefundReleaseReasonAfterSaleRefund},
		{"", events.RefundReleaseSourceAfterSale, events.RefundReleaseReasonAfterSaleRefund},
		{"  ", events.RefundReleaseSourceAfterSale, events.RefundReleaseReasonAfterSaleRefund},
	}
	for _, tc := range cases {
		source, reason, err := refundReleaseSource(tc.in)
		if err != nil {
			t.Fatalf("source %q rejected: %v", tc.in, err)
		}
		if source != tc.source || reason != tc.reason {
			t.Fatalf("source %q -> %s/%s, want %s/%s", tc.in, source, reason, tc.source, tc.reason)
		}
	}
	for _, raw := range []string{"payment", "legacy_expired", "unknown", "system"} {
		if _, _, err := refundReleaseSource(raw); err == nil {
			t.Fatalf("unsupported source %q was accepted", raw)
		} else if !strings.Contains(err.Error(), "REFUND_RELEASE_SOURCE_UNSUPPORTED") {
			t.Fatalf("source %q error = %v, want REFUND_RELEASE_SOURCE_UNSUPPORTED", raw, err)
		}
	}
}

// AC-50 / INV-44 (T1.4b2a): PRE + HELD.
// The admission transaction carries the durable release request,
// aggregated per (sku, warehouse), and a replay does not append a second one.
func TestAdmitRefundPreWritesReleaseOutbox(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	ctx := context.Background()
	seedReleaseOrder(t, db, "ORD-REL", 10000, 10000, "PAY-1", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 3000, groupNo: "G1"},
		{lineNo: 2, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 3000, groupNo: "G1"},
		{lineNo: 3, skuID: 2, quantity: 1, warehouseID: 3, paidCent: 4000, groupNo: "G1"},
	}, "")
	req := &pb.AdmitRefundRequest{
		RefundId: "R-REL", OrderNo: "ORD-REL", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
	}
	reply, err := svc.AdmitRefund(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetResult() != refundResultPre || reply.GetState() != refundResultPre {
		t.Fatalf("admission = %s/%s, want PRE/PRE", reply.GetResult(), reply.GetState())
	}
	releases := loadRefundReleaseEvents(t, db, "ORD-REL")
	if len(releases) != 1 {
		t.Fatalf("release events=%d, want 1", len(releases))
	}
	ev := releases[0]
	if ev.ReleaseID != "rr:R-REL" || ev.RefundID != "R-REL" || ev.OrderNo != "ORD-REL" || ev.GroupNo != "G1" {
		t.Fatalf("release identity = %+v", ev)
	}
	if ev.Source != events.RefundReleaseSourceAfterSale || ev.Reason != events.RefundReleaseReasonAfterSaleRefund {
		t.Fatalf("source/reason = %s/%s", ev.Source, ev.Reason)
	}
	if len(ev.Items) != 2 ||
		ev.Items[0] != (events.RefundReleaseItem{SkuID: 1, Quantity: 2, WarehouseID: 1}) ||
		ev.Items[1] != (events.RefundReleaseItem{SkuID: 2, Quantity: 1, WarehouseID: 3}) {
		t.Fatalf("release items = %+v, want [{1 2 1} {2 1 3}]", ev.Items)
	}
	if refundAdmittedCount(t, db, "ORD-REL") != 1 {
		t.Fatal("refund_admitted event missing")
	}
	if _, err := svc.AdmitRefund(ctx, req); err != nil {
		t.Fatal(err)
	}
	if got := len(loadRefundReleaseEvents(t, db, "ORD-REL")); got != 1 {
		t.Fatalf("release events after replay=%d, want 1", got)
	}
}

// AC-50 / INV-44: POST + CONFIRMED leaves the warehouse, so no release event is
// written; stock returns through the AfterSales restock flow.
func TestAdmitRefundPostDoesNotWriteReleaseOutbox(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "ALREADY_SHIPPED"})
	ctx := context.Background()
	seedReleaseOrder(t, db, "ORD-POST", 10000, 10000, "PAY-1", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 10000, groupNo: "G1"},
	}, permitConsumed)
	reply, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-POST", OrderNo: "ORD-POST", GroupNo: "G1", AmountCent: 1000, Source: "aftersale",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetResult() != refundResultPost {
		t.Fatalf("result=%q, want POST", reply.GetResult())
	}
	if got := len(loadRefundReleaseEvents(t, db, "ORD-POST")); got != 0 {
		t.Fatalf("POST admission wrote %d release event(s)", got)
	}
}

// A failed release append must roll the whole admission back: no admission, no
// commitment, no refund_admitted event and the permit stays unusable.
func TestAdmitRefundReleaseOutboxFailureRollsBackAdmission(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	ctx := context.Background()
	seedReleaseOrder(t, db, "ORD-ROLLBACK", 10000, 10000, "PAY-1", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 10000, groupNo: "G1"},
	}, "")
	if _, err := db.Exec(`CREATE TRIGGER trg_block_refund_release BEFORE INSERT ON outbox_events
		FOR EACH ROW BEGIN
			IF NEW.event_type = 'inventory.refund_release_requested' THEN
				SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'injected release append failure';
			END IF;
		END`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP TRIGGER IF EXISTS trg_block_refund_release`) })

	_, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-ROLLBACK", OrderNo: "ORD-ROLLBACK", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
	})
	if err == nil {
		t.Fatal("admission succeeded although the release append failed")
	}
	var admissions, commitments, releaseEvents int
	if err := db.QueryRow(`SELECT COUNT(*) FROM order_refund_admissions WHERE refund_id='R-ROLLBACK'`).Scan(&admissions); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM order_refund_commitments WHERE refund_id='R-ROLLBACK'`).Scan(&commitments); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM outbox_events WHERE event_type=?`, events.RefundReleaseEventType).Scan(&releaseEvents); err != nil {
		t.Fatal(err)
	}
	if admissions != 0 || commitments != 0 || releaseEvents != 0 || refundAdmittedCount(t, db, "ORD-ROLLBACK") != 0 {
		t.Fatalf("rollback leaked admission=%d commitment=%d release=%d", admissions, commitments, releaseEvents)
	}
	var permitCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM order_shipping_permits WHERE order_no='ORD-ROLLBACK'`).Scan(&permitCount); err != nil {
		t.Fatal(err)
	}
	if permitCount != 0 {
		t.Fatalf("permit rows after rollback=%d, want 0", permitCount)
	}
}

// Zero and positive refunds share the INV-44 event; zero only changes source,
// reason and the Payment short-circuit.
func TestAdmitRefundSystemZeroWritesReleaseOutbox(t *testing.T) {
	db := orderTestDB(t)
	payments := &fakePaymentPort{captured: 0}
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(payments, &fakeFulfillmentPort{status: "CANCELLED"})
	ctx := context.Background()
	seedZeroRefundOrder(t, db, "ORD-ZREL")
	req := &pb.AdmitRefundRequest{
		RefundId: "AS-ZREL", OrderNo: "ORD-ZREL", GroupNo: "G1", AmountCent: 0, Source: refundSourceSystemZero,
	}
	if _, err := svc.AdmitRefund(ctx, req); err != nil {
		t.Fatal(err)
	}
	releases := loadRefundReleaseEvents(t, db, "ORD-ZREL")
	if len(releases) != 1 {
		t.Fatalf("release events=%d, want 1", len(releases))
	}
	if releases[0].Source != events.RefundReleaseSourceSystemZero || releases[0].Reason != events.RefundReleaseReasonZeroAfterSale {
		t.Fatalf("zero release source/reason = %s/%s", releases[0].Source, releases[0].Reason)
	}
	if payments.getCalls != 0 || payments.listCalls != 0 {
		t.Fatalf("zero release consulted Payment (get=%d list=%d)", payments.getCalls, payments.listCalls)
	}
	if _, err := svc.AdmitRefund(ctx, req); err != nil {
		t.Fatal(err)
	}
	if got := len(loadRefundReleaseEvents(t, db, "ORD-ZREL")); got != 1 {
		t.Fatalf("release events after replay=%d, want 1", got)
	}
}

func TestAdmitRefundSystemZeroPostWritesNoReleaseOutbox(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 0}, &fakeFulfillmentPort{status: "ALREADY_SHIPPED"})
	ctx := context.Background()
	seedZeroRefundOrder(t, db, "ORD-ZPOST")
	if _, err := db.Exec(`INSERT INTO order_shipping_permits (order_no, fulfillment_group_no, state)
		VALUES ('ORD-ZPOST','G1',?)`, permitConsumed); err != nil {
		t.Fatal(err)
	}
	reply, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "AS-ZPOST", OrderNo: "ORD-ZPOST", GroupNo: "G1", AmountCent: 0, Source: refundSourceSystemZero,
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetResult() != refundResultPost {
		t.Fatalf("result=%q, want POST", reply.GetResult())
	}
	if got := len(loadRefundReleaseEvents(t, db, "ORD-ZPOST")); got != 0 {
		t.Fatalf("zero POST wrote %d release event(s)", got)
	}
}

// The two-phase CONSUMED -> CancelIfCreated(CANCELLED) resume is a PRE
// admission, so its Tx2 must append the release exactly once.
func TestAdmitRefundRemoteAdmissionPreWritesReleaseOutbox(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	ctx := context.Background()
	seedReleaseOrder(t, db, "ORD-REMOTE", 10000, 10000, "PAY-1", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 10000, groupNo: "G1"},
	}, permitConsumed)
	reply, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-REMOTE", OrderNo: "ORD-REMOTE", GroupNo: "G1", AmountCent: 1000, Source: "aftersale",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetResult() != refundResultPre {
		t.Fatalf("result=%q, want PRE", reply.GetResult())
	}
	if got := len(loadRefundReleaseEvents(t, db, "ORD-REMOTE")); got != 1 {
		t.Fatalf("release events=%d, want 1", got)
	}
}

// Concurrent admissions of the same refund_id converge on one admission and
// one release request through the order row lock.
func TestAdmitRefundConcurrentWritesOneReleaseOutbox(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&serialPaymentPort{fakePaymentPort: &fakePaymentPort{captured: 10000}}, &fakeFulfillmentPort{status: "CANCELLED"})
	ctx := context.Background()
	seedReleaseOrder(t, db, "ORD-CONC", 10000, 10000, "PAY-1", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 10000, groupNo: "G1"},
	}, "")
	req := &pb.AdmitRefundRequest{
		RefundId: "R-CONC", OrderNo: "ORD-CONC", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
	}
	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = svc.AdmitRefund(ctx, req)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent admission %d: %v", i, err)
		}
	}
	if got := len(loadRefundReleaseEvents(t, db, "ORD-CONC")); got != 1 {
		t.Fatalf("release events=%d, want 1", got)
	}
	if refundAdmittedCount(t, db, "ORD-CONC") != 1 {
		t.Fatal("refund_admitted event duplicated")
	}
	var admissions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM order_refund_admissions WHERE refund_id='R-CONC'`).Scan(&admissions); err != nil {
		t.Fatal(err)
	}
	if admissions != 1 {
		t.Fatalf("admissions=%d, want 1", admissions)
	}
}

// An order line without a real warehouse must fail closed before any release
// request is written; the event contract forbids defaulting to W1.
func TestAdmitRefundMissingWarehouseRejectsRelease(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	ctx := context.Background()
	seedReleaseOrder(t, db, "ORD-NOWH", 10000, 10000, "PAY-1", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 0, paidCent: 10000, groupNo: "G1"},
	}, "")
	_, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-NOWH", OrderNo: "ORD-NOWH", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
	})
	if err == nil {
		t.Fatal("admission succeeded without warehouse_id")
	}
	var admissions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM order_refund_admissions WHERE refund_id='R-NOWH'`).Scan(&admissions); err != nil {
		t.Fatal(err)
	}
	if admissions != 0 {
		t.Fatalf("admission rows=%d, want 0", admissions)
	}
	if got := len(loadRefundReleaseEvents(t, db, "ORD-NOWH")); got != 0 {
		t.Fatalf("release events=%d, want 0", got)
	}
}

// Architect ruling (2026-11-25): only tcc|system_zero|system_shortage|aftersale
// (and empty as aftersale) are releasable sources. Any other explicit value must
// reject the admission with REFUND_RELEASE_SOURCE_UNSUPPORTED and write zero
// rows; it must never be relabelled as aftersale.
func TestAdmitRefundUnsupportedReleaseSourceRejects(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	ctx := context.Background()
	seedReleaseOrder(t, db, "ORD-SRC", 10000, 10000, "PAY-1", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 10000, groupNo: "G1"},
	}, "")
	for _, source := range []string{"payment", "legacy_expired", "unknown", "system"} {
		refundID := "R-SRC-" + source
		_, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
			RefundId: refundID, OrderNo: "ORD-SRC", GroupNo: "G1", AmountCent: 10000, Source: source,
		})
		if err == nil {
			t.Fatalf("source %q was accepted", source)
		}
		if !strings.Contains(err.Error(), "REFUND_RELEASE_SOURCE_UNSUPPORTED") {
			t.Fatalf("source %q error = %v, want REFUND_RELEASE_SOURCE_UNSUPPORTED", source, err)
		}
		var admissions, commitments int
		if err := db.QueryRow(`SELECT COUNT(*) FROM order_refund_admissions WHERE refund_id=?`, refundID).Scan(&admissions); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM order_refund_commitments WHERE refund_id=?`, refundID).Scan(&commitments); err != nil {
			t.Fatal(err)
		}
		if admissions != 0 || commitments != 0 || refundAdmittedCount(t, db, "ORD-SRC") != 0 {
			t.Fatalf("source %q leaked admission=%d commitment=%d", source, admissions, commitments)
		}
		if got := len(loadRefundReleaseEvents(t, db, "ORD-SRC")); got != 0 {
			t.Fatalf("source %q wrote %d release event(s)", source, got)
		}
		var permits int
		if err := db.QueryRow(`SELECT COUNT(*) FROM order_shipping_permits WHERE order_no='ORD-SRC'`).Scan(&permits); err != nil {
			t.Fatal(err)
		}
		if permits != 0 {
			t.Fatalf("source %q leaked %d permit row(s)", source, permits)
		}
	}
}

// Empty source stays backward compatible as an ordinary after-sale refund.
func TestAdmitRefundEmptySourceDefaultsToAfterSale(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	ctx := context.Background()
	seedReleaseOrder(t, db, "ORD-EMPTY-SRC", 10000, 10000, "PAY-1", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 10000, groupNo: "G1"},
	}, "")
	reply, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-EMPTY-SRC", OrderNo: "ORD-EMPTY-SRC", GroupNo: "G1", AmountCent: 10000, Source: "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetResult() != refundResultPre {
		t.Fatalf("result=%q, want PRE", reply.GetResult())
	}
	releases := loadRefundReleaseEvents(t, db, "ORD-EMPTY-SRC")
	if len(releases) != 1 {
		t.Fatalf("release events=%d, want 1", len(releases))
	}
	if releases[0].Source != events.RefundReleaseSourceAfterSale || releases[0].Reason != events.RefundReleaseReasonAfterSaleRefund {
		t.Fatalf("empty source mapped to %s/%s", releases[0].Source, releases[0].Reason)
	}
	if releases[0].ReleaseID != "rr:R-EMPTY-SRC" {
		t.Fatalf("release_id=%q", releases[0].ReleaseID)
	}
}

// An unshipped group with no frozen order lines cannot produce a release event;
// admission must fail closed with zero writes instead of writing an empty event.
func TestAdmitRefundNoOrderItemsFailsClosed(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	ctx := context.Background()
	seedReleaseOrder(t, db, "ORD-NOITEM", 10000, 10000, "PAY-1", nil, "")
	if _, err := db.Exec(`INSERT INTO order_fulfillment_groups
		(group_no,order_no,seq,warehouse_id,status,reservation_state,freight_cent,freight_share_cent,version,created_at,updated_at)
		VALUES ('G1','ORD-NOITEM',1,1,'ALLOCATED','HELD',500,500,1,NOW(3),NOW(3))`); err != nil {
		t.Fatal(err)
	}
	_, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-NOITEM", OrderNo: "ORD-NOITEM", GroupNo: "G1", AmountCent: 500, Source: "aftersale",
	})
	if err == nil {
		t.Fatal("admission without order items succeeded")
	}
	if !strings.Contains(err.Error(), "no order items to release") {
		t.Fatalf("error = %v, want the no-items fail-closed cause", err)
	}
	var admissions, permits int
	if err := db.QueryRow(`SELECT COUNT(*) FROM order_refund_admissions WHERE refund_id='R-NOITEM'`).Scan(&admissions); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM order_shipping_permits WHERE order_no='ORD-NOITEM'`).Scan(&permits); err != nil {
		t.Fatal(err)
	}
	if admissions != 0 || permits != 0 || refundAdmittedCount(t, db, "ORD-NOITEM") != 0 {
		t.Fatalf("no-items admission leaked admissions=%d permits=%d", admissions, permits)
	}
	if got := len(loadRefundReleaseEvents(t, db, "ORD-NOITEM")); got != 0 {
		t.Fatalf("no-items admission wrote %d release event(s)", got)
	}
}
