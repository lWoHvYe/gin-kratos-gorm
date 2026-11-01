package persistence

import (
	"context"
	"errors"
	"time"

	"gin-kratos-gorm/internal/biz"
	"gin-kratos-gorm/internal/biz/model"
	"gin-kratos-gorm/internal/pkg/apperr"

	"gorm.io/gorm"
)

type OrderRepo struct {
	data *Data
}

func NewOrderRepo(data *Data) *OrderRepo {
	return &OrderRepo{data: data}
}

func (r *OrderRepo) Create(ctx context.Context, order *model.Order, items []*model.OrderItem, event *biz.DomainEvent) error {
	if order == nil {
		return apperr.Invalid("order is required")
	}
	return r.data.InTx(ctx, func(ctx context.Context) error {
		if err := r.data.DB(ctx).Create(order).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return apperr.Conflict("order already exists")
			}
			return apperr.Internal("create order", err)
		}
		for _, item := range items {
			item.OrderID = order.ID
			item.OrderSn = order.OrderSn
		}
		if len(items) > 0 {
			if err := r.data.DB(ctx).Create(&items).Error; err != nil {
				return apperr.Internal("create order items", err)
			}
		}
		order.OrderItems = derefItems(items)
		if event != nil {
			if err := insertOutbox(ctx, r.data.DB(ctx), event); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *OrderRepo) GetBySn(ctx context.Context, orderSN string) (*model.Order, error) {
	var order model.Order
	err := r.data.DB(ctx).
		Preload("OrderItems").
		Where("order_sn = ?", orderSN).
		First(&order).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.NotFound("order not found")
	}
	if err != nil {
		return nil, apperr.Internal("find order", err)
	}
	return &order, nil
}

func (r *OrderRepo) GetByClientRequestID(ctx context.Context, userID int64, requestID string) (*model.Order, error) {
	var order model.Order
	err := r.data.DB(ctx).
		Preload("OrderItems").
		Where("user_id = ? AND client_request_id = ?", userID, requestID).
		First(&order).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.NotFound("order not found")
	}
	if err != nil {
		return nil, apperr.Internal("find order by request id", err)
	}
	return &order, nil
}

func (r *OrderRepo) List(ctx context.Context, filter biz.OrderFilter) ([]*model.Order, int64, error) {
	query := r.data.DB(ctx).Model(&model.Order{})
	if filter.UserID > 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}
	if filter.OrderSN != "" {
		query = query.Where("order_sn = ?", filter.OrderSN)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, apperr.Internal("count orders", err)
	}
	if total == 0 {
		return []*model.Order{}, 0, nil
	}
	offset := (filter.Page - 1) * filter.PageSize
	var orders []*model.Order
	err := query.
		Preload("OrderItems").
		Order("id DESC").
		Offset(offset).
		Limit(filter.PageSize).
		Find(&orders).Error
	if err != nil {
		return nil, 0, apperr.Internal("list orders", err)
	}
	return orders, total, nil
}

// UpdateStatus performs a compare-and-set transition. It returns false when the
// order is no longer in the expected status, which makes concurrent pay/cancel
// safe without row locks.
func (r *OrderRepo) UpdateStatus(ctx context.Context, orderSN string, from, to int32, fields map[string]any, event *biz.DomainEvent) (bool, error) {
	updated := false
	err := r.data.InTx(ctx, func(ctx context.Context) error {
		updates := map[string]any{"status": to}
		for k, v := range fields {
			updates[k] = v
		}
		result := r.data.DB(ctx).Model(&model.Order{}).
			Where("order_sn = ? AND status = ?", orderSN, from).
			Updates(updates)
		if result.Error != nil {
			return apperr.Internal("update order status", result.Error)
		}
		if result.RowsAffected == 0 {
			return nil
		}
		updated = true
		if event != nil {
			if err := insertOutbox(ctx, r.data.DB(ctx), event); err != nil {
				return err
			}
		}
		return nil
	})
	return updated, err
}

func (r *OrderRepo) ListExpired(ctx context.Context, now time.Time, limit int) ([]*model.Order, error) {
	var orders []*model.Order
	err := r.data.DB(ctx).
		Where("status = ? AND expire_at IS NOT NULL AND expire_at <= ?", biz.OrderStatusPendingPayment, now).
		Order("expire_at ASC").
		Limit(limit).
		Find(&orders).Error
	if err != nil {
		return nil, apperr.Internal("list expired orders", err)
	}
	return orders, nil
}

func derefItems(items []*model.OrderItem) []model.OrderItem {
	result := make([]model.OrderItem, 0, len(items))
	for _, item := range items {
		if item != nil {
			result = append(result, *item)
		}
	}
	return result
}
