package fulfillment

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"testing"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
)

func deliveryService(db *sql.DB, orders pb.OrderServiceClient, opts Options) *Service {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewWithOptions(db, nil, logger, orders, opts)
}

func seedDeliveryShipment(t *testing.T, db *sql.DB, shipmentNo, orderNo, status, trackStatus string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO shipments
		(shipment_no, order_no, fulfillment_group_no, warehouse_id, carrier, carrier_code, tracking_no,
		 status, track_status, shipped_at, version)
		VALUES (?, ?, 'G1', 1, 'SF Express', 'SF', ?, ?, ?, NOW(3), 1)`,
		shipmentNo, orderNo, "TN-"+shipmentNo, status, trackStatus); err != nil {
		t.Fatal(err)
	}
}

func seedCarrier(t *testing.T, db *sql.DB, code string, enabled bool) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO carriers (code, name, enabled) VALUES (?, ?, ?)`, code, code+" Express", enabled); err != nil {
		t.Fatal(err)
	}
}

// AC-17: only an enabled registry carrier can create/ship a parcel.
func TestAC17CarrierRegistryRejectsUnknownCarrier(t *testing.T) {
	db := fulfillmentTestDB(t)
	seedCarrier(t, db, "SF", true)
	svc := deliveryService(db, &fakePermitOrder{order: &pb.Order{OrderNo: "ORD-C", UserId: 7}}, Options{CarrierRegistryEnabled: true})
	ctx := context.Background()
	_, err := svc.CreateShipment(ctx, &pb.CreateShipmentRequest{
		OrderNo: "ORD-C", CarrierCode: "UNKNOWN", TrackingNo: "X1",
		Items: []*pb.ShipmentItem{{SkuId: 1, Quantity: 1}},
	})
	if !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("unregistered carrier err = %v", err)
	}
	shipment, err := svc.CreateShipment(ctx, &pb.CreateShipmentRequest{
		OrderNo: "ORD-C", CarrierCode: "SF", TrackingNo: "X1",
		Items: []*pb.ShipmentItem{{SkuId: 1, Quantity: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if shipment.GetCarrierCode() != "SF" || shipment.GetCarrier() != "SF Express" {
		t.Fatalf("shipment carrier = %+v", shipment)
	}
}

// AC-18: the generated tracking_key unique index blocks a reused waybill.
func TestAC18DuplicateTrackingIsRejected(t *testing.T) {
	db := fulfillmentTestDB(t)
	seedCarrier(t, db, "SF", true)
	svc := deliveryService(db, &fakePermitOrder{order: &pb.Order{OrderNo: "ORD-D"}}, Options{CarrierRegistryEnabled: true})
	ctx := context.Background()
	if _, err := svc.CreateShipment(ctx, &pb.CreateShipmentRequest{
		OrderNo: "ORD-D", CarrierCode: "SF", TrackingNo: "DUP",
		Items: []*pb.ShipmentItem{{SkuId: 1, Quantity: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	_, err := svc.CreateShipment(ctx, &pb.CreateShipmentRequest{
		OrderNo: "ORD-E", CarrierCode: "SF", TrackingNo: "DUP",
		Items: []*pb.ShipmentItem{{SkuId: 1, Quantity: 1}},
	})
	if !apperr.IsCode(err, apperr.CodeConflict) {
		t.Fatalf("duplicate tracking err = %v", err)
	}
}

// AC-19: DELIVERED is terminal; a later IN_TRANSIT event cannot move it back.
func TestAC19TrackingIsMonotonic(t *testing.T) {
	db := fulfillmentTestDB(t)
	seedDeliveryShipment(t, db, "SHIP-T", "ORD-T", statusShipped, trackShipped)
	svc := deliveryService(db, nil, Options{})
	ctx := context.Background()
	if _, err := svc.RecordTrackingEvent(ctx, &pb.RecordTrackingEventRequest{
		ShipmentNo: "SHIP-T", CarrierCode: "SF", TrackingNo: "TN-SHIP-T", EventId: "E1", Status: trackDelivered,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RecordTrackingEvent(ctx, &pb.RecordTrackingEventRequest{
		ShipmentNo: "SHIP-T", CarrierCode: "SF", TrackingNo: "TN-SHIP-T", EventId: "E2", Status: trackInTransit,
	}); err != nil {
		t.Fatal(err)
	}
	shipment, err := svc.GetShipment(ctx, &pb.GetShipmentRequest{ShipmentNo: "SHIP-T"})
	if err != nil {
		t.Fatal(err)
	}
	if shipment.GetTrackStatus() != trackDelivered || shipment.GetDeliverySource() != sourceCarrierDelivered {
		t.Fatalf("shipment = %+v", shipment)
	}
	var evidence int
	if err := db.QueryRow(`SELECT COUNT(*) FROM delivery_evidence WHERE shipment_no = 'SHIP-T'`).Scan(&evidence); err != nil {
		t.Fatal(err)
	}
	if evidence != 1 {
		t.Fatalf("delivery evidence = %d", evidence)
	}
}

// AC-20: T+1 auto-confirm only works when the carrier DELIVERED evidence
// exists; it creates a receipt, not a fake delivery source.
func TestAC20AutoConfirmT1(t *testing.T) {
	db := fulfillmentTestDB(t)
	seedDeliveryShipment(t, db, "SHIP-T1", "ORD-T1", statusShipped, trackShipped)
	svc := deliveryService(db, nil, Options{DeliveryAutoConfirmEnabled: true})
	ctx := context.Background()
	if _, err := svc.RecordTrackingEvent(ctx, &pb.RecordTrackingEventRequest{
		ShipmentNo: "SHIP-T1", CarrierCode: "SF", TrackingNo: "TN-SHIP-T1", EventId: "D1", Status: trackDelivered,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE delivery_auto_confirm_jobs SET due_at = DATE_SUB(NOW(3), INTERVAL 1 MINUTE)
		WHERE shipment_no = 'SHIP-T1'`); err != nil {
		t.Fatal(err)
	}
	if err := svc.processAutoConfirmJob(ctx, "SHIP-T1"); err != nil {
		t.Fatal(err)
	}
	var source, state string
	if err := db.QueryRow(`SELECT r.source, j.state FROM shipment_receipts r
		JOIN delivery_auto_confirm_jobs j ON j.shipment_no = r.shipment_no WHERE r.shipment_no = 'SHIP-T1'`).
		Scan(&source, &state); err != nil {
		t.Fatal(err)
	}
	if source != receiptAutoT1 || state != autoConfirmDone {
		t.Fatalf("source=%s state=%s", source, state)
	}
	// A repeated job execution is idempotent.
	if err := svc.processAutoConfirmJob(ctx, "SHIP-T1"); err != nil {
		t.Fatal(err)
	}
	var receipts int
	if err := db.QueryRow(`SELECT COUNT(*) FROM shipment_receipts WHERE shipment_no = 'SHIP-T1'`).Scan(&receipts); err != nil {
		t.Fatal(err)
	}
	if receipts != 1 {
		t.Fatalf("receipts = %d", receipts)
	}
}

// AC-21 / AC-37: T+7 confirms parcels with no tracking at all and never claims
// the parcel was signed for; the UI label is "已自动确认收货（无物流轨迹）".
func testAutoConfirmT7DoesNotFakeDelivery(t *testing.T) {
	db := fulfillmentTestDB(t)
	seedDeliveryShipment(t, db, "SHIP-T7", "ORD-T7", statusShipped, trackCreated)
	if _, err := db.Exec(`INSERT INTO delivery_auto_confirm_jobs (shipment_no, reason, due_at, state)
		VALUES ('SHIP-T7', 'T7_NO_TRACK', DATE_SUB(NOW(3), INTERVAL 1 MINUTE), 'PENDING')`); err != nil {
		t.Fatal(err)
	}
	svc := deliveryService(db, nil, Options{DeliveryAutoConfirmEnabled: true})
	ctx := context.Background()
	if err := svc.processAutoConfirmJob(ctx, "SHIP-T7"); err != nil {
		t.Fatal(err)
	}
	var source string
	var deliverySource string
	var track string
	if err := db.QueryRow(`SELECT r.source, s.delivery_source, s.track_status FROM shipment_receipts r
		JOIN shipments s ON s.shipment_no = r.shipment_no WHERE r.shipment_no = 'SHIP-T7'`).
		Scan(&source, &deliverySource, &track); err != nil {
		t.Fatal(err)
	}
	if source != receiptAutoT7 || deliverySource != "" || track == trackDelivered {
		t.Fatalf("source=%s delivery=%s track=%s", source, deliverySource, track)
	}
}

// AC-22/AC-23: a user dispute freezes auto-confirm and only the resolution
// endpoint can write the final receipt.
func TestAC22ReceiptDisputeFreezesAutoConfirm(t *testing.T) {
	db := fulfillmentTestDB(t)
	order := &pb.Order{OrderNo: "ORD-DIS", UserId: 9}
	seedDeliveryShipment(t, db, "SHIP-DIS", "ORD-DIS", statusShipped, trackShipped)
	svc := deliveryService(db, &fakePermitOrder{order: order}, Options{DeliveryAutoConfirmEnabled: true})
	ctx := context.Background()
	if _, err := svc.RecordTrackingEvent(ctx, &pb.RecordTrackingEventRequest{
		ShipmentNo: "SHIP-DIS", CarrierCode: "SF", TrackingNo: "TN-SHIP-DIS", EventId: "D1", Status: trackDelivered,
	}); err != nil {
		t.Fatal(err)
	}
	dispute, err := svc.OpenReceiptDispute(ctx, &pb.OpenReceiptDisputeRequest{
		ShipmentNo: "SHIP-DIS", UserId: 9, Reason: "not received",
	})
	if err != nil {
		t.Fatal(err)
	}
	if dispute.GetStatus() != disputeOpen {
		t.Fatalf("dispute = %+v", dispute)
	}
	if _, err := db.Exec(`UPDATE delivery_auto_confirm_jobs SET state = 'PENDING',
		due_at = DATE_SUB(NOW(3), INTERVAL 1 MINUTE) WHERE shipment_no = 'SHIP-DIS'`); err != nil {
		t.Fatal(err)
	}
	if err := svc.processAutoConfirmJob(ctx, "SHIP-DIS"); err != nil {
		t.Fatal(err)
	}
	var receipts int
	if err := db.QueryRow(`SELECT COUNT(*) FROM shipment_receipts WHERE shipment_no = 'SHIP-DIS'`).Scan(&receipts); err != nil {
		t.Fatal(err)
	}
	if receipts != 0 {
		t.Fatalf("auto confirm bypassed an open dispute: %d receipts", receipts)
	}
	if _, err := svc.ResolveReceiptDispute(ctx, &pb.ResolveReceiptDisputeRequest{
		DisputeId: dispute.GetId(), Resolution: disputeUpheld, Reason: "delivery proof invalid", Operator: "cs-1",
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM shipment_receipts WHERE shipment_no = 'SHIP-DIS'`).Scan(&receipts); err != nil {
		t.Fatal(err)
	}
	if receipts != 0 {
		t.Fatalf("upheld dispute still confirmed receipt")
	}
	if _, err := svc.Receive(ctx, &pb.ReceiveRequest{ShipmentNo: "SHIP-DIS"}); !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("direct receive err = %v", err)
	}
}

// AC-24: an exception only opens an operation case; it does not refund or
// change the forward tracking status.
func TestAC24ExceptionDoesNotChangeTrackStatusOrRefund(t *testing.T) {
	db := fulfillmentTestDB(t)
	seedDeliveryShipment(t, db, "SHIP-EX", "ORD-EX", statusShipped, trackShipped)
	svc := deliveryService(db, nil, Options{})
	ctx := context.Background()
	if _, err := svc.RecordTrackingEvent(ctx, &pb.RecordTrackingEventRequest{
		ShipmentNo: "SHIP-EX", CarrierCode: "SF", TrackingNo: "TN-SHIP-EX", EventId: "L1",
		ExceptionType: "LOST", OccurredAtUnix: time.Now().Unix(),
	}); err != nil {
		t.Fatal(err)
	}
	var track, exception string
	if err := db.QueryRow(`SELECT track_status, exception_state FROM shipments WHERE shipment_no = 'SHIP-EX'`).
		Scan(&track, &exception); err != nil {
		t.Fatal(err)
	}
	if track != trackShipped || exception != exceptionOpen {
		t.Fatalf("track=%s exception=%s", track, exception)
	}
	var cases int
	if err := db.QueryRow(`SELECT COUNT(*) FROM shipment_operation_cases WHERE shipment_no = 'SHIP-EX'`).Scan(&cases); err != nil {
		t.Fatal(err)
	}
	if cases != 1 {
		t.Fatalf("operation cases = %d", cases)
	}
}

// AC-35: the 60s grace after due_at lets an appeal win; no receipt is written
// before the grace expires.
func TestAC35AppealGraceBeforeAutoConfirm(t *testing.T) {
	db := fulfillmentTestDB(t)
	seedDeliveryShipment(t, db, "SHIP-GRACE", "ORD-GRACE", statusShipped, trackShipped)
	svc := deliveryService(db, nil, Options{DeliveryAutoConfirmEnabled: true})
	ctx := context.Background()
	if _, err := svc.RecordTrackingEvent(ctx, &pb.RecordTrackingEventRequest{
		ShipmentNo: "SHIP-GRACE", CarrierCode: "SF", TrackingNo: "TN-SHIP-GRACE", EventId: "D1", Status: trackDelivered,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE delivery_auto_confirm_jobs SET due_at = DATE_SUB(NOW(3), INTERVAL 30 SECOND)
		WHERE shipment_no = 'SHIP-GRACE'`); err != nil {
		t.Fatal(err)
	}
	if err := svc.processAutoConfirmJob(ctx, "SHIP-GRACE"); err != nil {
		t.Fatal(err)
	}
	var receipts int
	if err := db.QueryRow(`SELECT COUNT(*) FROM shipment_receipts WHERE shipment_no = 'SHIP-GRACE'`).Scan(&receipts); err != nil {
		t.Fatal(err)
	}
	if receipts != 0 {
		t.Fatalf("auto confirm ignored the 60s appeal grace: %d receipts", receipts)
	}
	if _, err := db.Exec(`UPDATE delivery_auto_confirm_jobs SET due_at = DATE_SUB(NOW(3), INTERVAL 61 SECOND),
		state = 'PENDING' WHERE shipment_no = 'SHIP-GRACE'`); err != nil {
		t.Fatal(err)
	}
	if err := svc.processAutoConfirmJob(ctx, "SHIP-GRACE"); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM shipment_receipts WHERE shipment_no = 'SHIP-GRACE'`).Scan(&receipts); err != nil {
		t.Fatal(err)
	}
	if receipts != 1 {
		t.Fatalf("receipts = %d", receipts)
	}
}

// AC-36: after auto confirmation, a 15-day dispute is accepted but never rolls
// back the receipt fact.
func TestAC36PostReceiptDisputeDoesNotRollback(t *testing.T) {
	db := fulfillmentTestDB(t)
	order := &fakePermitOrder{order: &pb.Order{OrderNo: "ORD-36", UserId: 9}}
	seedDeliveryShipment(t, db, "SHIP-36", "ORD-36", statusShipped, trackShipped)
	svc := deliveryService(db, order, Options{DeliveryAutoConfirmEnabled: true})
	ctx := context.Background()
	if _, err := svc.RecordTrackingEvent(ctx, &pb.RecordTrackingEventRequest{
		ShipmentNo: "SHIP-36", CarrierCode: "SF", TrackingNo: "TN-SHIP-36", EventId: "D1", Status: trackDelivered,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE delivery_auto_confirm_jobs SET due_at = DATE_SUB(NOW(3), INTERVAL 2 MINUTE)
		WHERE shipment_no = 'SHIP-36'`); err != nil {
		t.Fatal(err)
	}
	if err := svc.processAutoConfirmJob(ctx, "SHIP-36"); err != nil {
		t.Fatal(err)
	}
	dispute, err := svc.OpenReceiptDispute(ctx, &pb.OpenReceiptDisputeRequest{
		ShipmentNo: "SHIP-36", UserId: 9, Reason: "not received after auto confirm",
	})
	if err != nil {
		t.Fatal(err)
	}
	if dispute.GetStatus() != disputeOpen {
		t.Fatalf("dispute = %+v", dispute)
	}
	if _, err := svc.ResolveReceiptDispute(ctx, &pb.ResolveReceiptDisputeRequest{
		DisputeId: dispute.GetId(), Resolution: disputeUpheld, Reason: "proof invalid", Operator: "cs-2",
	}); err != nil {
		t.Fatal(err)
	}
	var receipts int
	if err := db.QueryRow(`SELECT COUNT(*) FROM shipment_receipts WHERE shipment_no = 'SHIP-36'`).Scan(&receipts); err != nil {
		t.Fatal(err)
	}
	if receipts != 1 {
		t.Fatalf("upheld post-receipt dispute rolled back the receipt: %d", receipts)
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM shipments WHERE shipment_no = 'SHIP-36'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != statusReceived {
		t.Fatalf("shipment status = %s", status)
	}
}

// AC-29: order detail can show each group's warehouse/carrier/tracking.
func TestAC29GroupShipmentExposesTracking(t *testing.T) {
	db := fulfillmentTestDB(t)
	seedDeliveryShipment(t, db, "SHIP-29", "ORD-29", statusShipped, trackShipped)
	svc := deliveryService(db, nil, Options{})
	list, err := svc.ListShipments(context.Background(), &pb.ListShipmentsRequest{OrderNo: "ORD-29"})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.GetShipments()) != 1 {
		t.Fatalf("shipments = %+v", list.GetShipments())
	}
	shipment := list.GetShipments()[0]
	if shipment.GetFulfillmentGroupNo() != "G1" || shipment.GetWarehouseId() != 1 ||
		shipment.GetCarrierCode() != "SF" || shipment.GetTrackingNo() != "TN-SHIP-29" {
		t.Fatalf("shipment = %+v", shipment)
	}
}

// AC-21: the no-tracking parcel is auto-confirmed at T+7.
func TestAC21AutoConfirmT7(t *testing.T) { testAutoConfirmT7DoesNotFakeDelivery(t) }

// AC-37: the AUTO_T7 receipt never produces a delivery source and must not be
// rendered as "已签收".
func TestAC37AutoT7DoesNotClaimDelivered(t *testing.T) { testAutoConfirmT7DoesNotFakeDelivery(t) }

// AC-23: no operator can directly mark a shipment received.
func TestAC23DirectReceiveRejected(t *testing.T) {
	db := fulfillmentTestDB(t)
	svc := deliveryService(db, nil, Options{})
	_, err := svc.Receive(context.Background(), &pb.ReceiveRequest{ShipmentNo: "SHIP-NO-RECEIVE"})
	if !apperr.IsCode(err, apperr.CodeFailedPrecondition) {
		t.Fatalf("direct receive err = %v", err)
	}
}
