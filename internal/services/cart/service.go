package cart

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/db"
	"gin-kratos-gorm/internal/platform/outbox"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// maxItemQuantity caps the quantity accepted by a single AddItem/UpdateItem
	// call, which keeps totals comfortably inside int64.
	maxItemQuantity = 9999

	// cartEventTopic carries cart mutations through the outbox.
	cartEventTopic = "shop.cart.updated.v1"

	eventItemAdded       = "cart.item_added"
	eventItemUpdated     = "cart.item_updated"
	eventItemRemoved     = "cart.item_removed"
	eventSelectedCleared = "cart.selected_cleared"
)

// Service implements pb.CartServiceServer.
//
// The service is intentionally free of authentication: user_id is supplied by
// the gateway in every request, so the caller is trusted to pass the
// authenticated user.
type Service struct {
	pb.UnimplementedCartServiceServer
	db      *sql.DB
	catalog pb.CatalogServiceClient
	repo    *repo
	gorm    *cartGorm
	log     *slog.Logger
}

// New builds a cart service. catalog is the catalog gRPC client used to resolve
// sku price and display name.
func New(database *sql.DB, catalog pb.CatalogServiceClient, logger *slog.Logger) *Service {
	s := &Service{
		db:      database,
		catalog: catalog,
		repo:    newRepo(database),
		log:     logger,
	}
	if reader, err := newCartGorm(database); err != nil {
		if logger != nil {
			logger.Warn("cart gorm reader disabled", "error", err)
		}
	} else {
		s.gorm = reader
	}
	return s
}

// GetCart returns the user's cart. A user without a cart gets an empty cart;
// reading never creates rows.
func (s *Service) GetCart(ctx context.Context, req *pb.GetCartRequest) (*pb.Cart, error) {
	if req.GetUserId() <= 0 {
		return nil, apperr.Invalid("user_id is required")
	}
	return s.loadCart(ctx, req.GetUserId())
}

// AddItem resolves the sku in catalog and upserts the cart line, accumulating
// quantity for an already present (cart, sku) pair. The full cart is returned.
func (s *Service) AddItem(ctx context.Context, req *pb.AddItemRequest) (*pb.Cart, error) {
	userID, skuID, quantity := req.GetUserId(), req.GetSkuId(), req.GetQuantity()
	if userID <= 0 || skuID <= 0 {
		return nil, apperr.Invalid("user_id and sku_id are required")
	}
	if quantity <= 0 {
		return nil, apperr.Invalid("quantity must be greater than 0")
	}
	if quantity > maxItemQuantity {
		return nil, apperr.Invalid("quantity must not exceed %d", maxItemQuantity)
	}
	sku, err := s.fetchSku(ctx, skuID)
	if err != nil {
		return nil, err
	}
	item := &pb.CartItem{
		SkuId:     skuID,
		Name:      s.skuDisplayName(ctx, sku),
		PriceCent: sku.GetPriceCent(),
		Quantity:  quantity,
		Selected:  true,
	}

	var cart *pb.Cart
	err = db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		cartID, err := s.repo.ensureCart(ctx, tx, userID)
		if err != nil {
			return apperr.Internal("ensure cart", err)
		}
		if err := s.repo.upsertItem(ctx, tx, cartID, item); err != nil {
			return apperr.Internal("upsert cart item", err)
		}
		if err := s.appendEvent(ctx, tx, eventItemAdded, userID, skuID); err != nil {
			return err
		}
		cart, err = s.cartFrom(ctx, tx, userID, cartID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return cart, nil
}

// UpdateItem sets quantity and selected for an existing line. A quantity of
// zero or less removes the line. The full cart is returned.
//
// proto3 has no field presence for bool, so selected is always applied:
// selected=false unselects the line.
func (s *Service) UpdateItem(ctx context.Context, req *pb.UpdateItemRequest) (*pb.Cart, error) {
	userID, skuID, quantity := req.GetUserId(), req.GetSkuId(), req.GetQuantity()
	if userID <= 0 || skuID <= 0 {
		return nil, apperr.Invalid("user_id and sku_id are required")
	}
	if quantity > maxItemQuantity {
		return nil, apperr.Invalid("quantity must not exceed %d", maxItemQuantity)
	}

	var cart *pb.Cart
	err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		cartID, err := s.repo.cartID(ctx, tx, userID)
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound("cart item not found")
		}
		if err != nil {
			return apperr.Internal("query cart", err)
		}
		exists, err := s.repo.itemExists(ctx, tx, cartID, skuID)
		if err != nil {
			return apperr.Internal("query cart item", err)
		}
		if !exists {
			return apperr.NotFound("cart item not found")
		}

		eventType := eventItemUpdated
		if quantity <= 0 {
			if _, err := s.repo.deleteItem(ctx, tx, cartID, skuID); err != nil {
				return apperr.Internal("delete cart item", err)
			}
			eventType = eventItemRemoved
		} else if err := s.repo.updateItem(ctx, tx, cartID, skuID, quantity, req.GetSelected()); err != nil {
			return apperr.Internal("update cart item", err)
		}
		if err := s.appendEvent(ctx, tx, eventType, userID, skuID); err != nil {
			return err
		}
		cart, err = s.cartFrom(ctx, tx, userID, cartID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return cart, nil
}

// RemoveItem deletes a line. Removal is idempotent: removing a sku that is not
// in the cart (or from a user without a cart) succeeds and returns the cart.
func (s *Service) RemoveItem(ctx context.Context, req *pb.RemoveItemRequest) (*pb.Cart, error) {
	userID, skuID := req.GetUserId(), req.GetSkuId()
	if userID <= 0 || skuID <= 0 {
		return nil, apperr.Invalid("user_id and sku_id are required")
	}

	var cart *pb.Cart
	err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		cartID, err := s.repo.cartID(ctx, tx, userID)
		if errors.Is(err, sql.ErrNoRows) {
			cart = &pb.Cart{UserId: userID}
			return nil
		}
		if err != nil {
			return apperr.Internal("query cart", err)
		}
		deleted, err := s.repo.deleteItem(ctx, tx, cartID, skuID)
		if err != nil {
			return apperr.Internal("delete cart item", err)
		}
		if deleted > 0 {
			if err := s.appendEvent(ctx, tx, eventItemRemoved, userID, skuID); err != nil {
				return err
			}
		}
		cart, err = s.cartFrom(ctx, tx, userID, cartID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return cart, nil
}

// ClearSelected removes the selected lines after a successful checkout. The
// caller treats failures as non-fatal; the order already exists.
func (s *Service) ClearSelected(ctx context.Context, req *pb.ClearCartRequest) (*pb.Empty, error) {
	userID := req.GetUserId()
	if userID <= 0 {
		return nil, apperr.Invalid("user_id is required")
	}
	err := db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		cartID, err := s.repo.cartID(ctx, tx, userID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return apperr.Internal("query cart", err)
		}
		res, err := tx.ExecContext(ctx, `DELETE FROM cart_items WHERE cart_id = ? AND selected = 1`, cartID)
		if err != nil {
			return apperr.Internal("clear selected cart items", err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			return s.appendEvent(ctx, tx, eventSelectedCleared, userID, 0)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

func (s *Service) loadCart(ctx context.Context, userID int64) (*pb.Cart, error) {
	if s.gorm != nil {
		return s.gorm.GetCart(ctx, userID)
	}
	cartID, err := s.repo.cartID(ctx, s.db, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return &pb.Cart{UserId: userID}, nil
	}
	if err != nil {
		return nil, apperr.Internal("query cart", err)
	}
	return s.cartFrom(ctx, s.db, userID, cartID)
}

func (s *Service) cartFrom(ctx context.Context, q querier, userID, cartID int64) (*pb.Cart, error) {
	items, err := s.repo.items(ctx, q, cartID)
	if err != nil {
		return nil, apperr.Internal("query cart items", err)
	}
	return &pb.Cart{UserId: userID, Items: items, TotalCent: totalCent(items)}, nil
}

// fetchSku loads a sku from catalog and maps transport errors to domain errors.
func (s *Service) fetchSku(ctx context.Context, skuID int64) (*pb.Sku, error) {
	if s.catalog == nil {
		return nil, apperr.Unavailable("catalog client is not configured", nil)
	}
	sku, err := s.catalog.GetSku(ctx, &pb.GetSkuRequest{SkuId: skuID})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, apperr.NotFound("sku not found")
		}
		return nil, apperr.Unavailable("fetch sku from catalog", err)
	}
	if sku.GetId() == 0 {
		sku.Id = skuID
	}
	if sku.GetPriceCent() < 0 {
		return nil, apperr.Failed("sku price is invalid")
	}
	return sku, nil
}

// skuDisplayName resolves the name stored on the cart line. pb.Sku carries no
// name, so the owning product name is used and sku_code is the fallback.
func (s *Service) skuDisplayName(ctx context.Context, sku *pb.Sku) string {
	if sku.GetSpuId() > 0 {
		product, err := s.catalog.GetProduct(ctx, &pb.GetProductRequest{Id: sku.GetSpuId()})
		if err != nil {
			if s.log != nil {
				s.log.Warn("fetch product name", "spu_id", sku.GetSpuId(), "error", err)
			}
		} else if name := strings.TrimSpace(product.GetName()); name != "" {
			return name
		}
	}
	if code := strings.TrimSpace(sku.GetSkuCode()); code != "" {
		return code
	}
	return strconv.FormatInt(sku.GetId(), 10)
}

func (s *Service) appendEvent(ctx context.Context, tx *sql.Tx, eventType string, userID, skuID int64) error {
	event := outbox.Event{
		Topic:         cartEventTopic,
		AggregateType: "cart",
		AggregateID:   strconv.FormatInt(userID, 10),
		EventType:     eventType,
		Payload:       map[string]any{"user_id": userID, "sku_id": skuID},
	}
	if err := outbox.Append(ctx, tx, event); err != nil {
		return apperr.Internal("append cart event", err)
	}
	return nil
}

// totalCent sums price_cent*quantity over the selected lines only. Lines with a
// non-positive quantity or a negative price are ignored.
func totalCent(items []*pb.CartItem) int64 {
	var total int64
	for _, item := range items {
		if item == nil || !item.GetSelected() {
			continue
		}
		if item.GetQuantity() <= 0 || item.GetPriceCent() < 0 {
			continue
		}
		total += item.GetPriceCent() * int64(item.GetQuantity())
	}
	return total
}
