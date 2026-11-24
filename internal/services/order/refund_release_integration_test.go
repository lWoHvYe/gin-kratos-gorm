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

// serialPaymentPort serializes the shared fake for concurrent tests so
// `go test -race` sees no fake-internal data race.
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

// seedReleaseOrder writes the frozen order facts a refund admission needs.
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
		seedReleasePermit(t, db, orderNo, "G1", permitState)
	}
}

func seedReleaseGroup(t *testing.T, db *sql.DB, orderNo, groupNo string, seq int, status, refundState string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO order_fulfillment_groups
		(group_no,order_no,seq,warehouse_id,status,refund_state,reservation_state,
		 freight_cent,freight_share_cent,version,created_at,updated_at)
		VALUES (?,?,?,1,?,?, 'HELD',0,0,1,NOW(3),NOW(3))`,
		groupNo, orderNo, seq, status, refundState); err != nil {
		t.Fatalf("seed group %s/%s: %v", orderNo, groupNo, err)
	}
}

func seedReleasePermit(t *testing.T, db *sql.DB, orderNo, groupNo, state string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO order_shipping_permits (order_no, fulfillment_group_no, state)
		VALUES (?,?,?)`, orderNo, groupNo, state); err != nil {
		t.Fatalf("seed permit %s/%s: %v", orderNo, groupNo, err)
	}
}

func markGroupRefunded(t *testing.T, db *sql.DB, orderNo, groupNo string) {
	t.Helper()
	if _, err := db.Exec(`UPDATE order_fulfillment_groups
		SET refund_state='REFUNDED', version=version+1 WHERE order_no=? AND group_no=?`,
		orderNo, groupNo); err != nil {
		t.Fatal(err)
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

func releaseAdmissionCount(t *testing.T, db *sql.DB, refundID string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM order_refund_admissions WHERE refund_id=?`, refundID).Scan(&n); err != nil {
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

func TestOrderLevelFullReleaseAfterLastGroup(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	ctx := context.Background()
	// Shared reservation key (sku 1, warehouse 1) split across two groups:
	// the full release must aggregate the whole order quantity, not G2's 1.
	seedReleaseOrder(t, db, "ORD-ONE", 10000, 10000, "PAY-1", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 2, warehouseID: 1, paidCent: 6000, groupNo: "G1"},
		{lineNo: 2, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 4000, groupNo: "G2"},
	}, "")
	seedReleaseGroup(t, db, "ORD-ONE", "G1", 1, "ALLOCATED", refundStateNone)
	seedReleaseGroup(t, db, "ORD-ONE", "G2", 2, "ALLOCATED", refundStateNone)
	if _, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-ONE-G1", OrderNo: "ORD-ONE", GroupNo: "G1", AmountCent: 6000, Source: "aftersale",
	}); err != nil {
		t.Fatal(err)
	}
	if got := len(loadRefundReleaseEvents(t, db, "ORD-ONE")); got != 0 {
		t.Fatalf("G1 (not last) wrote %d release event(s)", got)
	}
	markGroupRefunded(t, db, "ORD-ONE", "G1")
	reply, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-ONE-G2", OrderNo: "ORD-ONE", GroupNo: "G2", AmountCent: 4000, Source: "aftersale",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetResult() != refundResultPre {
		t.Fatalf("G2 admission=%q, want PRE", reply.GetResult())
	}
	releases := loadRefundReleaseEvents(t, db, "ORD-ONE")
	if len(releases) != 1 {
		t.Fatalf("release events=%d, want 1", len(releases))
	}
	ev := releases[0]
	if ev.ReleaseScope != events.RefundReleaseScopeOrderReservationFull {
		t.Fatalf("release_scope=%q", ev.ReleaseScope)
	}
	if ev.ReleaseID != "rr:R-ONE-G2" || ev.GroupNo != "G2" || ev.OrderNo != "ORD-ONE" {
		t.Fatalf("release identity = %+v", ev)
	}
	if len(ev.Items) != 1 || ev.Items[0] != (events.RefundReleaseItem{SkuID: 1, Quantity: 3, WarehouseID: 1}) {
		t.Fatalf("items=%+v, want order-level {1 3 1}", ev.Items)
	}
	if len(ev.CoveredGroupNos) != 2 || ev.CoveredGroupNos[0] != "G1" || ev.CoveredGroupNos[1] != "G2" {
		t.Fatalf("covered_group_nos=%v", ev.CoveredGroupNos)
	}
	if ev.Source != events.RefundReleaseSourceAfterSale || ev.Reason != events.RefundReleaseReasonAfterSaleRefund {
		t.Fatalf("source/reason=%s/%s", ev.Source, ev.Reason)
	}
	if refundAdmittedCount(t, db, "ORD-ONE") != 2 {
		t.Fatalf("refund_admitted=%d, want 2", refundAdmittedCount(t, db, "ORD-ONE"))
	}
}

func TestAdmitRefundSingleGroupEmitsImmediately(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	ctx := context.Background()
	seedReleaseOrder(t, db, "ORD-SINGLE", 10000, 10000, "PAY-1", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 4000, groupNo: "G1"},
		{lineNo: 2, skuID: 2, quantity: 2, warehouseID: 3, paidCent: 6000, groupNo: "G1"},
	}, "")
	reply, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-SINGLE", OrderNo: "ORD-SINGLE", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.GetResult() != refundResultPre {
		t.Fatalf("result=%q", reply.GetResult())
	}
	releases := loadRefundReleaseEvents(t, db, "ORD-SINGLE")
	if len(releases) != 1 {
		t.Fatalf("release events=%d, want 1", len(releases))
	}
	if len(releases[0].Items) != 2 ||
		releases[0].Items[0] != (events.RefundReleaseItem{SkuID: 1, Quantity: 1, WarehouseID: 1}) ||
		releases[0].Items[1] != (events.RefundReleaseItem{SkuID: 2, Quantity: 2, WarehouseID: 3}) {
		t.Fatalf("items=%+v", releases[0].Items)
	}
}

// Two active groups: both admissions defer, and the scan emits exactly one
// order-level release after both groups are refunded. Concurrent scans must not
// double append.
func TestDeferredReleaseScanEmitsOnceAfterBothGroups(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&serialPaymentPort{fakePaymentPort: &fakePaymentPort{captured: 10000}}, &fakeFulfillmentPort{status: "CANCELLED"})
	ctx := context.Background()
	seedReleaseOrder(t, db, "ORD-SCAN", 10000, 10000, "PAY-1", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 5000, groupNo: "G1"},
		{lineNo: 2, skuID: 2, quantity: 1, warehouseID: 1, paidCent: 5000, groupNo: "G2"},
	}, "")
	seedReleaseGroup(t, db, "ORD-SCAN", "G1", 1, "ALLOCATED", refundStateNone)
	seedReleaseGroup(t, db, "ORD-SCAN", "G2", 2, "ALLOCATED", refundStateNone)
	for _, req := range []*pb.AdmitRefundRequest{
		{RefundId: "R-SCAN-G1", OrderNo: "ORD-SCAN", GroupNo: "G1", AmountCent: 5000, Source: "aftersale"},
		{RefundId: "R-SCAN-G2", OrderNo: "ORD-SCAN", GroupNo: "G2", AmountCent: 5000, Source: "aftersale"},
	} {
		if _, err := svc.AdmitRefund(ctx, req); err != nil {
			t.Fatal(err)
		}
	}
	if got := len(loadRefundReleaseEvents(t, db, "ORD-SCAN")); got != 0 {
		t.Fatalf("deferred admissions wrote %d release event(s)", got)
	}
	markGroupRefunded(t, db, "ORD-SCAN", "G1")
	markGroupRefunded(t, db, "ORD-SCAN", "G2")
	var wg sync.WaitGroup
	errs := make([]error, 3)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = svc.runRefundReleaseReconcileOnce(ctx)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("scan %d: %v", i, err)
		}
	}
	releases := loadRefundReleaseEvents(t, db, "ORD-SCAN")
	if len(releases) != 1 {
		t.Fatalf("release events=%d, want exactly 1", len(releases))
	}
	if len(releases[0].Items) != 2 {
		t.Fatalf("full items=%+v", releases[0].Items)
	}
	if err := svc.runRefundReleaseReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if got := len(loadRefundReleaseEvents(t, db, "ORD-SCAN")); got != 1 {
		t.Fatalf("rescan appended duplicate release events=%d", got)
	}
}

// One POST/shipped group blocks automatic full release; the admission still
// succeeds and the scan stays manual.
func TestSharedPOSTGroupBlocksAutomaticRelease(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	ctx := context.Background()
	seedReleaseOrder(t, db, "ORD-POSTSHARE", 10000, 10000, "PAY-1", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 5000, groupNo: "G1"},
		{lineNo: 2, skuID: 2, quantity: 1, warehouseID: 2, paidCent: 5000, groupNo: "G2"},
	}, "")
	seedReleaseGroup(t, db, "ORD-POSTSHARE", "G1", 1, "ALLOCATED", refundStateNone)
	seedReleaseGroup(t, db, "ORD-POSTSHARE", "G2", 2, statusShipped, refundStateNone)
	if _, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-POSTSHARE-G1", OrderNo: "ORD-POSTSHARE", GroupNo: "G1", AmountCent: 5000, Source: "aftersale",
	}); err != nil {
		t.Fatal(err)
	}
	if got := len(loadRefundReleaseEvents(t, db, "ORD-POSTSHARE")); got != 0 {
		t.Fatalf("POST group allowed %d automatic release event(s)", got)
	}
	if err := svc.runRefundReleaseReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if got := len(loadRefundReleaseEvents(t, db, "ORD-POSTSHARE")); got != 0 {
		t.Fatalf("scan released %d event(s) despite a POST group", got)
	}
}

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

// The release append runs inside the admission transaction: a failure rolls
// the whole admission back.
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
	if n := releaseAdmissionCount(t, db, "R-ROLLBACK"); n != 0 {
		t.Fatalf("admission rows=%d, want 0", n)
	}
	if refundAdmittedCount(t, db, "ORD-ROLLBACK") != 0 {
		t.Fatal("refund_admitted leaked after rollback")
	}
	var permits int
	if err := db.QueryRow(`SELECT COUNT(*) FROM order_shipping_permits WHERE order_no='ORD-ROLLBACK'`).Scan(&permits); err != nil {
		t.Fatal(err)
	}
	if permits != 0 || len(loadRefundReleaseEvents(t, db, "ORD-ROLLBACK")) != 0 {
		t.Fatalf("rollback leaked permits=%d release=%d", permits, len(loadRefundReleaseEvents(t, db, "ORD-ROLLBACK")))
	}
}

func TestAdmitRefundSystemZeroWritesOrderLevelRelease(t *testing.T) {
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
	if releases[0].ReleaseScope != events.RefundReleaseScopeOrderReservationFull ||
		releases[0].Source != events.RefundReleaseSourceSystemZero {
		t.Fatalf("zero release = %+v", releases[0])
	}
	if payments.getCalls != 0 || payments.listCalls != 0 {
		t.Fatalf("zero release consulted Payment (get=%d list=%d)", payments.getCalls, payments.listCalls)
	}
	if _, err := svc.AdmitRefund(ctx, req); err != nil {
		t.Fatal(err)
	}
	if got := len(loadRefundReleaseEvents(t, db, "ORD-ZREL")); got != 1 {
		t.Fatalf("replay appended release events=%d, want 1", got)
	}
}

func TestAdmitRefundSystemZeroPostWritesNoReleaseOutbox(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 0}, &fakeFulfillmentPort{status: "ALREADY_SHIPPED"})
	ctx := context.Background()
	seedZeroRefundOrder(t, db, "ORD-ZPOST")
	seedReleasePermit(t, db, "ORD-ZPOST", "G1", permitConsumed)
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

func TestAdmitRefundRemoteAdmissionPreWritesOrderLevelRelease(t *testing.T) {
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
	releases := loadRefundReleaseEvents(t, db, "ORD-REMOTE")
	if len(releases) != 1 || releases[0].ReleaseScope != events.RefundReleaseScopeOrderReservationFull {
		t.Fatalf("remote PRE release=%+v", releases)
	}
}

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
		if err == nil || !strings.Contains(err.Error(), "REFUND_RELEASE_SOURCE_UNSUPPORTED") {
			t.Fatalf("source %q error = %v", source, err)
		}
		if n := releaseAdmissionCount(t, db, refundID); n != 0 {
			t.Fatalf("source %q leaked admissions=%d", source, n)
		}
	}
	if refundAdmittedCount(t, db, "ORD-SRC") != 0 || len(loadRefundReleaseEvents(t, db, "ORD-SRC")) != 0 {
		t.Fatal("unsupported source wrote release/refund_admitted rows")
	}
}

func TestAdmitRefundEmptySourceDefaultsToAfterSale(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	ctx := context.Background()
	seedReleaseOrder(t, db, "ORD-EMPTY-SRC", 10000, 10000, "PAY-1", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 10000, groupNo: "G1"},
	}, "")
	if _, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-EMPTY-SRC", OrderNo: "ORD-EMPTY-SRC", GroupNo: "G1", AmountCent: 10000, Source: "",
	}); err != nil {
		t.Fatal(err)
	}
	releases := loadRefundReleaseEvents(t, db, "ORD-EMPTY-SRC")
	if len(releases) != 1 || releases[0].Source != events.RefundReleaseSourceAfterSale {
		t.Fatalf("empty source release=%+v", releases)
	}
}

func TestAdmitRefundSystemShortageSource(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	ctx := context.Background()
	seedReleaseOrder(t, db, "ORD-SHORT", 10000, 10000, "PAY-1", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 10000, groupNo: "G1"},
	}, "")
	if _, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-SHORT", OrderNo: "ORD-SHORT", GroupNo: "G1", AmountCent: 10000, Source: "system_shortage",
	}); err != nil {
		t.Fatal(err)
	}
	releases := loadRefundReleaseEvents(t, db, "ORD-SHORT")
	if len(releases) != 1 || releases[0].Source != events.RefundReleaseSourceSystemShortage ||
		releases[0].Reason != events.RefundReleaseReasonShortageRefund {
		t.Fatalf("shortage release=%+v", releases)
	}
}

// The deferred scan reconstructs system_shortage from the resolved shortage
// case when there is no refund_applications row.
func TestDeferredReleaseScanUsesShortageSource(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	ctx := context.Background()
	seedReleaseOrder(t, db, "ORD-SHORT-SCAN", 10000, 10000, "PAY-1", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 5000, groupNo: "G1"},
		{lineNo: 2, skuID: 2, quantity: 1, warehouseID: 2, paidCent: 5000, groupNo: "G2"},
	}, "")
	seedReleaseGroup(t, db, "ORD-SHORT-SCAN", "G1", 1, "ALLOCATED", refundStateNone)
	seedReleaseGroup(t, db, "ORD-SHORT-SCAN", "G2", 2, "ALLOCATED", refundStateNone)
	if _, err := db.Exec(`INSERT INTO order_shortage_cases
		(order_no,group_no,status,resolution,reason,version,created_at,updated_at)
		VALUES ('ORD-SHORT-SCAN','G1','RESOLVED_REFUND','REFUND','shortage',1,NOW(3),NOW(3))`); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-SHORT-SCAN-G1", OrderNo: "ORD-SHORT-SCAN", GroupNo: "G1", AmountCent: 5000, Source: "system_shortage",
	}); err != nil {
		t.Fatal(err)
	}
	markGroupRefunded(t, db, "ORD-SHORT-SCAN", "G1")
	markGroupRefunded(t, db, "ORD-SHORT-SCAN", "G2")
	if err := svc.runRefundReleaseReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	releases := loadRefundReleaseEvents(t, db, "ORD-SHORT-SCAN")
	if len(releases) != 1 {
		t.Fatalf("release events=%d, want 1", len(releases))
	}
	if releases[0].Source != events.RefundReleaseSourceSystemShortage {
		t.Fatalf("scan source=%q, want system_shortage (refund_id=%s)", releases[0].Source, releases[0].RefundID)
	}
}

func TestAdmitRefundMissingWarehouseRejectsRelease(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	ctx := context.Background()
	seedReleaseOrder(t, db, "ORD-NOWH", 10000, 10000, "PAY-1", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 0, paidCent: 10000, groupNo: "G1"},
	}, "")
	if _, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-NOWH", OrderNo: "ORD-NOWH", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
	}); err == nil {
		t.Fatal("admission succeeded without warehouse_id")
	}
	if releaseAdmissionCount(t, db, "R-NOWH") != 0 || len(loadRefundReleaseEvents(t, db, "ORD-NOWH")) != 0 {
		t.Fatal("missing warehouse leaked admission/release")
	}
}

func TestAdmitRefundMissingGroupRejectsRelease(t *testing.T) {
	db := orderTestDB(t)
	svc := New(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil)
	svc.EnableSettlementClients(&fakePaymentPort{captured: 10000}, &fakeFulfillmentPort{status: "CANCELLED"})
	ctx := context.Background()
	seedReleaseOrder(t, db, "ORD-NOGROUP", 10000, 10000, "PAY-1", []releaseTestItem{
		{lineNo: 1, skuID: 1, quantity: 1, warehouseID: 1, paidCent: 10000, groupNo: ""},
	}, "")
	if _, err := svc.AdmitRefund(ctx, &pb.AdmitRefundRequest{
		RefundId: "R-NOGROUP", OrderNo: "ORD-NOGROUP", GroupNo: "G1", AmountCent: 10000, Source: "aftersale",
	}); err == nil {
		t.Fatal("admission succeeded without fulfillment_group_no")
	}
	if releaseAdmissionCount(t, db, "R-NOGROUP") != 0 || len(loadRefundReleaseEvents(t, db, "ORD-NOGROUP")) != 0 {
		t.Fatal("missing group leaked admission/release")
	}
}

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
	if err == nil || !strings.Contains(err.Error(), "no order items") {
		t.Fatalf("error=%v, want no-order-items fail-closed", err)
	}
	if releaseAdmissionCount(t, db, "R-NOITEM") != 0 || len(loadRefundReleaseEvents(t, db, "ORD-NOITEM")) != 0 {
		t.Fatal("no-items admission leaked rows")
	}
}
