package fulfillment

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/id"
	"gin-kratos-gorm/internal/platform/mq"
	"gin-kratos-gorm/internal/platform/outbox"

	"github.com/go-sql-driver/mysql"
)

// Shipment lifecycle. All transitions are forward-only:
//
//	CREATED -> SHIPPED -> RECEIVED
const (
	statusCreated  = "CREATED"
	statusShipped  = "SHIPPED"
	statusReceived = "RECEIVED"
)

// Outbox subjects, see docs/contracts.md.
const (
	topicShipmentCreated   = "shop.shipment.created.v1"
	topicShipmentShipped   = "shop.shipment.shipped.v1"
	topicShipmentDelivered = "shop.shipment.delivered.v1"
)

// aggregateShipment is the outbox aggregate type for shipments.
const aggregateShipment = "shipment"

type Service struct {
	pb.UnimplementedFulfillmentServiceServer
	db  *sql.DB
	mq  *mq.Client
	log *slog.Logger
}

func New(db *sql.DB, client *mq.Client, logger *slog.Logger) *Service {
	return &Service{db: db, mq: client, log: logger}
}

// CreateShipment registers a parcel for an order. It is idempotent by order_no:
// a repeated call for an order that already has a shipment returns the existing
// shipment instead of creating a duplicate. Shipment numbers are additionally
// unique, so a retried request can never book the same parcel twice.
func (s *Service) CreateShipment(ctx context.Context, req *pb.CreateShipmentRequest) (*pb.Shipment, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	items, err := validateCreateShipment(orderNo, req.GetItems())
	if err != nil {
		return nil, err
	}
	carrier := strings.TrimSpace(req.GetCarrier())
	trackingNo := strings.TrimSpace(req.GetTrackingNo())
	warehouseID := req.GetWarehouseId()

	existing, err := s.shipmentNoByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	if existing != "" {
		return s.loadShipment(ctx, existing)
	}

	shipmentNo := id.Shipment()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin create shipment", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `INSERT INTO shipments (shipment_no, order_no, warehouse_id, carrier, tracking_no, status)
		VALUES (?, ?, ?, ?, ?, ?)`, shipmentNo, orderNo, warehouseID, carrier, trackingNo, statusCreated)
	if err != nil {
		var me *mysql.MySQLError
		if errors.As(err, &me) && me.Number == 1062 {
			// Concurrent create for the same order: release our transaction and
			// return the shipment that won the race.
			_ = tx.Rollback()
			winner, lookupErr := s.shipmentNoByOrderNo(ctx, orderNo)
			if lookupErr != nil {
				return nil, lookupErr
			}
			if winner != "" {
				return s.loadShipment(ctx, winner)
			}
			return nil, apperr.Conflict("shipment already exists for order " + orderNo)
		}
		return nil, apperr.Internal("create shipment", err)
	}
	shipmentID, err := res.LastInsertId()
	if err != nil {
		return nil, apperr.Internal("shipment id", err)
	}
	for _, it := range items {
		if _, err := tx.ExecContext(ctx, `INSERT INTO shipment_items (shipment_id, sku_id, quantity) VALUES (?, ?, ?)`,
			shipmentID, it.SkuID, it.Quantity); err != nil {
			return nil, apperr.Internal("create shipment item", err)
		}
	}
	payload, err := json.Marshal(shipmentEvent{
		ShipmentNo: shipmentNo, OrderNo: orderNo, WarehouseID: warehouseID,
		Carrier: carrier, TrackingNo: trackingNo, Status: statusCreated, Items: items,
	})
	if err != nil {
		return nil, apperr.Internal("marshal shipment event", err)
	}
	if err := outbox.Append(ctx, tx, outbox.Event{
		Topic: topicShipmentCreated, AggregateType: aggregateShipment,
		AggregateID: shipmentNo, EventType: "shipment.created", Payload: payload,
	}); err != nil {
		return nil, apperr.Internal("append shipment created", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, apperr.Internal("commit create shipment", err)
	}
	return s.loadShipment(ctx, shipmentNo)
}

// Ship moves a shipment from CREATED to SHIPPED and emits
// shop.shipment.shipped.v1. Repeating the call on an already shipped shipment is
// a no-op that returns the current aggregate.
func (s *Service) Ship(ctx context.Context, req *pb.ShipRequest) (*pb.Shipment, error) {
	return s.advance(ctx, strings.TrimSpace(req.GetShipmentNo()), topicShipmentShipped, "shipment.shipped", shipOutcome)
}

// Receive moves a shipment from SHIPPED to RECEIVED and emits
// shop.shipment.delivered.v1. Repeating the call on a received shipment is a
// no-op that returns the current aggregate.
func (s *Service) Receive(ctx context.Context, req *pb.ReceiveRequest) (*pb.Shipment, error) {
	return s.advance(ctx, strings.TrimSpace(req.GetShipmentNo()), topicShipmentDelivered, "shipment.delivered", receiveOutcome)
}

func (s *Service) GetShipment(ctx context.Context, req *pb.GetShipmentRequest) (*pb.Shipment, error) {
	shipmentNo := strings.TrimSpace(req.GetShipmentNo())
	if shipmentNo == "" {
		return nil, apperr.Invalid("shipment_no is required")
	}
	return s.loadShipment(ctx, shipmentNo)
}

// GetShipmentByOrder lets the C-end receive flow resolve the parcel from the
// order number, so the client does not need to know the internal shipment_no.
func (s *Service) GetShipmentByOrder(ctx context.Context, req *pb.GetShipmentByOrderRequest) (*pb.Shipment, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	shipmentNo, err := s.shipmentNoByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	if shipmentNo == "" {
		return nil, apperr.NotFound("shipment not found for order")
	}
	return s.loadShipment(ctx, shipmentNo)
}

// advance applies a forward-only transition under a row lock. The outbox row is
// written in the same transaction as the status change, so domain events are
// never lost or emitted for rolled-back updates.
func (s *Service) advance(
	ctx context.Context,
	shipmentNo, topic, eventType string,
	transition func(current string) (next string, changed bool, err error),
) (*pb.Shipment, error) {
	if shipmentNo == "" {
		return nil, apperr.Invalid("shipment_no is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin shipment transition", err)
	}
	defer func() { _ = tx.Rollback() }()

	var (
		shipmentID  int64
		status      string
		orderNo     string
		warehouseID int64
		carrier     string
		trackingNo  string
	)
	err = tx.QueryRowContext(ctx, `SELECT id, status, order_no, warehouse_id, carrier, tracking_no
		FROM shipments WHERE shipment_no = ? FOR UPDATE`, shipmentNo).
		Scan(&shipmentID, &status, &orderNo, &warehouseID, &carrier, &trackingNo)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("shipment not found")
	}
	if err != nil {
		return nil, apperr.Internal("lock shipment", err)
	}

	next, changed, err := transition(status)
	if err != nil {
		return nil, err
	}
	if changed {
		items, err := shipmentItemsTx(ctx, tx, shipmentID)
		if err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE shipments SET status = ?, version = version + 1 WHERE id = ?`,
			next, shipmentID); err != nil {
			return nil, apperr.Internal("update shipment status", err)
		}
		payload, err := json.Marshal(shipmentEvent{
			ShipmentNo: shipmentNo, OrderNo: orderNo, WarehouseID: warehouseID,
			Carrier: carrier, TrackingNo: trackingNo, Status: next, Items: items,
		})
		if err != nil {
			return nil, apperr.Internal("marshal shipment event", err)
		}
		if err := outbox.Append(ctx, tx, outbox.Event{
			Topic: topic, AggregateType: aggregateShipment,
			AggregateID: shipmentNo, EventType: eventType, Payload: payload,
		}); err != nil {
			return nil, apperr.Internal("append "+eventType, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, apperr.Internal("commit shipment transition", err)
	}
	return s.loadShipment(ctx, shipmentNo)
}

func (s *Service) shipmentNoByOrderNo(ctx context.Context, orderNo string) (string, error) {
	var shipmentNo string
	err := s.db.QueryRowContext(ctx, `SELECT shipment_no FROM shipments WHERE order_no = ? ORDER BY id LIMIT 1`, orderNo).Scan(&shipmentNo)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", apperr.Internal("lookup shipment by order", err)
	}
	return shipmentNo, nil
}

func (s *Service) loadShipment(ctx context.Context, shipmentNo string) (*pb.Shipment, error) {
	var (
		sh        pb.Shipment
		createdAt time.Time
	)
	err := s.db.QueryRowContext(ctx, `SELECT shipment_no, order_no, status, carrier, tracking_no, created_at
		FROM shipments WHERE shipment_no = ?`, shipmentNo).
		Scan(&sh.ShipmentNo, &sh.OrderNo, &sh.Status, &sh.Carrier, &sh.TrackingNo, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("shipment not found")
	}
	if err != nil {
		return nil, apperr.Internal("get shipment", err)
	}
	sh.CreatedAtUnix = createdAt.Unix()
	rows, err := s.db.QueryContext(ctx, `SELECT sku_id, quantity FROM shipment_items WHERE shipment_id =
		(SELECT id FROM shipments WHERE shipment_no = ?) ORDER BY id`, shipmentNo)
	if err != nil {
		return nil, apperr.Internal("list shipment items", err)
	}
	defer rows.Close()
	for rows.Next() {
		var it pb.ShipmentItem
		if err := rows.Scan(&it.SkuId, &it.Quantity); err != nil {
			return nil, apperr.Internal("scan shipment item", err)
		}
		sh.Items = append(sh.Items, &it)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal("iterate shipment items", err)
	}
	return &sh, nil
}

func shipmentItemsTx(ctx context.Context, tx *sql.Tx, shipmentID int64) ([]shipmentItem, error) {
	rows, err := tx.QueryContext(ctx, `SELECT sku_id, quantity FROM shipment_items WHERE shipment_id = ? ORDER BY id`, shipmentID)
	if err != nil {
		return nil, apperr.Internal("load shipment items", err)
	}
	defer rows.Close()
	items := make([]shipmentItem, 0, 4)
	for rows.Next() {
		var it shipmentItem
		if err := rows.Scan(&it.SkuID, &it.Quantity); err != nil {
			return nil, apperr.Internal("scan shipment item", err)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal("iterate shipment items", err)
	}
	return items, nil
}

// shipOutcome returns the status after Ship. Retrying on an already shipped
// shipment is idempotent; shipping a received parcel is a client error.
func shipOutcome(current string) (string, bool, error) {
	switch current {
	case statusCreated:
		return statusShipped, true, nil
	case statusShipped:
		return statusShipped, false, nil
	case statusReceived:
		return "", false, apperr.Failed("shipment already received")
	default:
		return "", false, apperr.Failed(fmt.Sprintf("invalid shipment status %q", current))
	}
}

// receiveOutcome returns the status after Receive.
func receiveOutcome(current string) (string, bool, error) {
	switch current {
	case statusShipped:
		return statusReceived, true, nil
	case statusReceived:
		return statusReceived, false, nil
	case statusCreated:
		return "", false, apperr.Failed("shipment has not been shipped yet")
	default:
		return "", false, apperr.Failed(fmt.Sprintf("invalid shipment status %q", current))
	}
}

// shipmentItem is the persisted/event view of a shipment line.
type shipmentItem struct {
	SkuID    int64 `json:"sku_id"`
	Quantity int32 `json:"quantity"`
}

// shipmentEvent is the outbox payload for shop.shipment.* events.
type shipmentEvent struct {
	ShipmentNo  string         `json:"shipment_no"`
	OrderNo     string         `json:"order_no"`
	WarehouseID int64          `json:"warehouse_id"`
	Carrier     string         `json:"carrier"`
	TrackingNo  string         `json:"tracking_no"`
	Status      string         `json:"status"`
	Items       []shipmentItem `json:"items"`
}

// validateCreateShipment validates the request and merges duplicate SKU lines so
// that one shipment carries at most one row per SKU.
func validateCreateShipment(orderNo string, items []*pb.ShipmentItem) ([]shipmentItem, error) {
	if orderNo == "" {
		return nil, apperr.Invalid("order_no is required")
	}
	if len(items) == 0 {
		return nil, apperr.Invalid("at least one shipment item is required")
	}
	merged := make([]shipmentItem, 0, len(items))
	index := make(map[int64]int, len(items))
	for _, in := range items {
		if in == nil {
			return nil, apperr.Invalid("shipment item must not be nil")
		}
		skuID := in.GetSkuId()
		if skuID <= 0 {
			return nil, apperr.Invalid("sku_id must be positive")
		}
		if in.GetQuantity() <= 0 {
			return nil, apperr.Invalid("quantity must be positive for sku %d", skuID)
		}
		if pos, ok := index[skuID]; ok {
			sum := int64(merged[pos].Quantity) + int64(in.GetQuantity())
			if sum > math.MaxInt32 {
				return nil, apperr.Invalid("quantity overflow for sku %d", skuID)
			}
			merged[pos].Quantity = int32(sum)
			continue
		}
		index[skuID] = len(merged)
		merged = append(merged, shipmentItem{SkuID: skuID, Quantity: in.GetQuantity()})
	}
	return merged, nil
}
