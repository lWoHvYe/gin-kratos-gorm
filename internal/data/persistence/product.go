package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gin-kratos-gorm/internal/biz"
	"gin-kratos-gorm/internal/biz/model"
	"gin-kratos-gorm/internal/pkg/apperr"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ProductRepo struct {
	data *Data
}

func NewProductRepo(data *Data) *ProductRepo {
	return &ProductRepo{data: data}
}

// DB exposes a transaction-aware handle for the DTM barrier path. It is only
// used by infrastructure adapters, never by domain code.
func (r *ProductRepo) DB(ctx context.Context) *gorm.DB {
	return r.data.DB(ctx)
}

func (r *ProductRepo) GetSpu(ctx context.Context, id int64) (*model.ProductSpu, error) {
	var spu model.ProductSpu
	err := r.data.DB(ctx).
		Preload("Skus").
		Where("id = ?", id).
		First(&spu).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.NotFound("product not found")
	}
	if err != nil {
		return nil, apperr.Internal("find product", err)
	}
	return &spu, nil
}

func (r *ProductRepo) ListSpus(ctx context.Context, filter biz.ProductFilter) ([]*model.ProductSpu, int64, error) {
	query := r.data.DB(ctx).Model(&model.ProductSpu{})
	if kw := strings.TrimSpace(filter.Keyword); kw != "" {
		like := "%" + kw + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", like, like)
	}
	if filter.CategoryID > 0 {
		query = query.Where("category_id = ?", filter.CategoryID)
	}
	if filter.OnSale != nil {
		query = query.Where("is_on_sale = ?", *filter.OnSale)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, apperr.Internal("count products", err)
	}
	if total == 0 {
		return []*model.ProductSpu{}, 0, nil
	}
	offset := (filter.Page - 1) * filter.PageSize
	var spus []*model.ProductSpu
	err := query.
		Preload("Skus").
		Order("id DESC").
		Offset(offset).
		Limit(filter.PageSize).
		Find(&spus).Error
	if err != nil {
		return nil, 0, apperr.Internal("list products", err)
	}
	return spus, total, nil
}

func (r *ProductRepo) CreateSpu(ctx context.Context, spu *model.ProductSpu) error {
	return r.data.InTx(ctx, func(ctx context.Context) error {
		skus := spu.Skus
		spu.Skus = nil
		if err := r.data.DB(ctx).Create(spu).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return apperr.Conflict("product or sku already exists")
			}
			return apperr.Internal("create product", err)
		}
		for i := range skus {
			skus[i].SpuID = spu.ID
		}
		if len(skus) > 0 {
			if err := r.data.DB(ctx).Create(&skus).Error; err != nil {
				if errors.Is(err, gorm.ErrDuplicatedKey) {
					return apperr.Conflict("sku code already exists")
				}
				return apperr.Internal("create skus", err)
			}
		}
		spu.Skus = skus
		return nil
	})
}

func (r *ProductRepo) UpdateSpu(ctx context.Context, spu *model.ProductSpu) error {
	updates := map[string]any{
		"name":        spu.Name,
		"description": spu.Description,
		"category_id": spu.CategoryID,
		"brand_id":    spu.BrandID,
		"main_image":  spu.MainImage,
	}
	result := r.data.DB(ctx).Model(&model.ProductSpu{}).Where("id = ?", spu.ID).Updates(updates)
	if result.Error != nil {
		return apperr.Internal("update product", result.Error)
	}
	return nil
}

func (r *ProductRepo) SetOnSale(ctx context.Context, id int64, onSale bool) error {
	result := r.data.DB(ctx).Model(&model.ProductSpu{}).Where("id = ?", id).UpdateColumn("is_on_sale", onSale)
	if result.Error != nil {
		return apperr.Internal("update product status", result.Error)
	}
	if result.RowsAffected == 0 {
		// MySQL reports 0 changed rows when the value is unchanged; distinguish
		// that from a missing product.
		var count int64
		if err := r.data.DB(ctx).Model(&model.ProductSpu{}).Where("id = ?", id).Count(&count).Error; err != nil {
			return apperr.Internal("check product", err)
		}
		if count == 0 {
			return apperr.NotFound("product not found")
		}
	}
	return nil
}

func (r *ProductRepo) AdjustStock(ctx context.Context, skuID int64, delta int32) (int64, error) {
	var newStock int64
	err := r.data.InTx(ctx, func(ctx context.Context) error {
		var sku model.ProductSku
		err := r.data.DB(ctx).Where("id = ?", skuID).First(&sku).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperr.NotFound("sku not found")
		}
		if err != nil {
			return apperr.Internal("find sku", err)
		}
		if delta < 0 && sku.Stock < int64(-delta) {
			return apperr.FailedPrecondition("insufficient stock")
		}
		result := r.data.DB(ctx).Model(&model.ProductSku{}).
			Where("id = ? AND stock + ? >= 0", skuID, delta).
			UpdateColumn("stock", gorm.Expr("stock + ?", delta))
		if result.Error != nil {
			return apperr.Internal("adjust stock", result.Error)
		}
		if result.RowsAffected == 0 {
			return apperr.FailedPrecondition("insufficient stock")
		}
		return r.data.DB(ctx).Model(&model.ProductSku{}).Select("stock").Where("id = ?", skuID).Scan(&newStock).Error
	})
	return newStock, err
}

func (r *ProductRepo) GetSkuSnapshot(ctx context.Context, skuID int64) (*biz.SkuSnapshot, error) {
	var snapshot biz.SkuSnapshot
	row := r.data.DB(ctx).
		Table("product_skus AS s").
		Select("s.id AS sku_id, s.spu_id, sp.name AS sku_name, sp.main_image AS sku_pic, s.price, s.stock, sp.is_on_sale AS on_sale").
		Joins("JOIN product_spus sp ON sp.id = s.spu_id AND sp.deleted_at IS NULL").
		Where("s.id = ? AND s.deleted_at IS NULL", skuID).
		Take(&snapshot)
	if errors.Is(row.Error, gorm.ErrRecordNotFound) {
		return nil, apperr.NotFound("sku not found")
	}
	if row.Error != nil {
		return nil, apperr.Internal("find sku", row.Error)
	}
	return &snapshot, nil
}

// DeductStock atomically deducts stock and records an idempotency log. The log
// and stock update are in the same transaction; a duplicate request is a no-op.
func (r *ProductRepo) DeductStock(ctx context.Context, orderSN string, skuID int64, num int32) error {
	if num <= 0 {
		return apperr.Invalid("quantity must be positive")
	}
	return r.data.InTx(ctx, func(ctx context.Context) error {
		var sku model.ProductSku
		err := r.data.DB(ctx).Where("id = ?", skuID).First(&sku).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperr.NotFound("sku not found")
		}
		if err != nil {
			return apperr.Internal("find sku", err)
		}

		logID := fmt.Sprintf("deduct:%s:%d", orderSN, skuID)
		entry := model.StockLog{ID: logID, OrderSn: orderSN, SkuID: skuID, Num: num, Action: "deduct"}
		result := r.data.DB(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&entry)
		if result.Error != nil {
			return apperr.Internal("write stock log", result.Error)
		}
		if result.RowsAffected == 0 {
			return nil // already deducted: idempotent success
		}

		update := r.data.DB(ctx).Model(&model.ProductSku{}).
			Where("id = ? AND stock >= ?", skuID, num).
			UpdateColumn("stock", gorm.Expr("stock - ?", num))
		if update.Error != nil {
			return apperr.Internal("deduct stock", update.Error)
		}
		if update.RowsAffected == 0 {
			return apperr.FailedPrecondition("insufficient stock")
		}
		return nil
	})
}

// RestoreStock is the idempotent compensation for DeductStock.
func (r *ProductRepo) RestoreStock(ctx context.Context, orderSN string, skuID int64, num int32) error {
	if num <= 0 {
		return apperr.Invalid("quantity must be positive")
	}
	return r.data.InTx(ctx, func(ctx context.Context) error {
		logID := fmt.Sprintf("restore:%s:%d", orderSN, skuID)
		entry := model.StockLog{ID: logID, OrderSn: orderSN, SkuID: skuID, Num: num, Action: "restore"}
		result := r.data.DB(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&entry)
		if result.Error != nil {
			return apperr.Internal("write stock log", result.Error)
		}
		if result.RowsAffected == 0 {
			return nil // already restored
		}
		update := r.data.DB(ctx).Model(&model.ProductSku{}).
			Where("id = ?", skuID).
			UpdateColumn("stock", gorm.Expr("stock + ?", num))
		if update.Error != nil {
			return apperr.Internal("restore stock", update.Error)
		}
		if update.RowsAffected == 0 {
			return apperr.NotFound("sku not found")
		}
		return nil
	})
}
