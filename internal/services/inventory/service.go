package inventory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/db"
	"gin-kratos-gorm/internal/platform/dtm"
)

const (
	// defaultWarehouseID is the warehouse seeded by 0001_init.sql. Requests that
	// omit warehouse_id operate on it.
	defaultWarehouseID int64 = 1
	// defaultReservationTTL is the hold lifetime when ttl_seconds is not set.
	defaultReservationTTL = 900 * time.Second
	// maxReservationTTL bounds ttl_seconds so a typo cannot hold stock forever.
	maxReservationTTL = 24 * time.Hour

	maxOrderNoLength = 64
)

// Service implements pb.InventoryServiceServer on top of MySQL. Stock mutations
// and outbox appends always share one *sql.Tx, opened by db.WithTx for direct
// calls and by the DTM barrier when the call is a saga branch.
type Service struct {
	pb.UnimplementedInventoryServiceServer
	db  *sql.DB
	log *slog.Logger
}

func New(database *sql.DB, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{db: database, log: logger}
}

var _ pb.InventoryServiceServer = (*Service)(nil)

type reserveInput struct {
	orderNo     string
	skuID       int64
	warehouseID int64
	quantity    int64
	ttl         time.Duration
}

// GetStock returns the on_hand, reserved and available (on_hand - reserved)
// counters of one sku in one warehouse.
func (s *Service) GetStock(ctx context.Context, req *pb.GetStockRequest) (*pb.Stock, error) {
	if req.GetSkuId() <= 0 {
		return nil, apperr.Invalid("sku_id is required")
	}
	warehouseID := normalizeWarehouse(req.GetWarehouseId())
	st, err := s.stock(ctx, s.db, req.GetSkuId(), warehouseID)
	if err != nil {
		return nil, err
	}
	return &pb.Stock{
		SkuId:       st.skuID,
		WarehouseId: st.warehouseID,
		OnHand:      st.onHand,
		Reserved:    st.reserved,
		Available:   st.available(),
	}, nil
}

// Reserve places a HELD reservation for (order_no, sku_id). It is idempotent:
// an existing HELD/CONFIRMED reservation is returned as-is, and a RELEASED one is
// a conflict because a reservation cannot be revived under the same order line.
func (s *Service) Reserve(ctx context.Context, req *pb.ReserveRequest) (*pb.ReserveReply, error) {
	in, err := normalizeReserve(req)
	if err != nil {
		return nil, err
	}

	existing, err := s.reservationByOrderSku(ctx, s.db, in.orderNo, in.skuID, false)
	switch {
	case err == nil:
		s.log.Debug("reserve replay", "order_no", in.orderNo, "sku_id", in.skuID, "status", existing.status)
		return reservationReply(existing)
	case apperr.IsCode(err, apperr.CodeNotFound):
		// First attempt for this order line.
	default:
		return nil, err
	}

	var created *reservation
	write := func(tx *sql.Tx) error {
		r, err := s.reserveTx(ctx, tx, in)
		if err != nil {
			return err
		}
		created = r
		return nil
	}
	if err := s.write(ctx, write); err != nil {
		if errors.Is(err, errReservationExists) {
			winner, readErr := s.reservationByOrderSku(ctx, s.db, in.orderNo, in.skuID, false)
			if readErr != nil {
				return nil, readErr
			}
			return reservationReply(winner)
		}
		return nil, err
	}
	if created == nil {
		// A replayed DTM branch is skipped by the barrier; the work was already
		// committed by an earlier attempt, so surface the committed reservation.
		r, err := s.reservationByOrderSku(ctx, s.db, in.orderNo, in.skuID, false)
		if err != nil {
			return nil, err
		}
		return reservationReply(r)
	}
	return reservationReply(created)
}

// Confirm turns a HELD reservation into CONFIRMED and consumes the reserved
// quantity from on_hand. Confirming twice is a no-op.
func (s *Service) Confirm(ctx context.Context, req *pb.ReserveRequest) (*pb.Empty, error) {
	in, err := normalizeReservationKey(req)
	if err != nil {
		return nil, err
	}
	if err := s.write(ctx, func(tx *sql.Tx) error {
		_, err := s.confirmTx(ctx, tx, in)
		return err
	}); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

// Release returns a HELD reservation to availability. Releasing twice is a
// no-op; releasing a CONFIRMED reservation is a conflict because stock was
// already consumed and restocking is a separate flow.
func (s *Service) Release(ctx context.Context, req *pb.ReserveRequest) (*pb.Empty, error) {
	in, err := normalizeReservationKey(req)
	if err != nil {
		return nil, err
	}
	if err := s.write(ctx, func(tx *sql.Tx) error {
		_, err := s.releaseTx(ctx, tx, in)
		return err
	}); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

// write runs fn in the DTM branch transaction when the call carries DTM branch
// metadata, otherwise in a plain local transaction. Business errors keep their
// apperr mapping; transport/barrier errors become INTERNAL.
func (s *Service) write(ctx context.Context, fn func(*sql.Tx) error) error {
	var err error
	if dtm.InBranch(ctx) {
		err = dtm.Branch(ctx, s.db, fn)
	} else {
		err = db.WithTx(ctx, s.db, fn)
	}
	if err == nil {
		return nil
	}
	var ae *apperr.Error
	if errors.As(err, &ae) || errors.Is(err, errReservationExists) {
		return err
	}
	return apperr.Internal("inventory transaction", err)
}

func reservationReply(r *reservation) (*pb.ReserveReply, error) {
	switch r.status {
	case statusHeld, statusConfirmed:
		return &pb.ReserveReply{ReservationNo: r.reservationNo, ExpiresAtUnix: r.expiresAt.Unix()}, nil
	case statusReleased:
		return nil, apperr.Conflict(fmt.Sprintf("reservation %s for order %s was released", r.reservationNo, r.orderNo))
	default:
		return nil, apperr.Internal("unexpected reservation status", fmt.Errorf("status %q", r.status))
	}
}

func normalizeReserve(req *pb.ReserveRequest) (reserveInput, error) {
	in, err := normalizeReservationKey(req)
	if err != nil {
		return reserveInput{}, err
	}
	if req.GetQuantity() <= 0 {
		return reserveInput{}, apperr.Invalid("quantity must be positive")
	}
	in.quantity = int64(req.GetQuantity())
	in.warehouseID = normalizeWarehouse(req.GetWarehouseId())
	in.ttl = defaultReservationTTL
	if secs := req.GetTtlSeconds(); secs < 0 {
		return reserveInput{}, apperr.Invalid("ttl_seconds must not be negative")
	} else if secs > 0 {
		in.ttl = time.Duration(secs) * time.Second
		if in.ttl > maxReservationTTL {
			return reserveInput{}, apperr.Invalid("ttl_seconds must be at most %d", int64(maxReservationTTL/time.Second))
		}
	}
	return in, nil
}

func normalizeReservationKey(req *pb.ReserveRequest) (reserveInput, error) {
	orderNo := strings.TrimSpace(req.GetOrderNo())
	if orderNo == "" {
		return reserveInput{}, apperr.Invalid("order_no is required")
	}
	if len(orderNo) > maxOrderNoLength {
		return reserveInput{}, apperr.Invalid("order_no must be at most %d characters", maxOrderNoLength)
	}
	if req.GetSkuId() <= 0 {
		return reserveInput{}, apperr.Invalid("sku_id is required")
	}
	if req.GetQuantity() < 0 {
		return reserveInput{}, apperr.Invalid("quantity must not be negative")
	}
	return reserveInput{orderNo: orderNo, skuID: req.GetSkuId(), quantity: int64(req.GetQuantity())}, nil
}

func normalizeWarehouse(warehouseID int64) int64 {
	if warehouseID <= 0 {
		return defaultWarehouseID
	}
	return warehouseID
}
