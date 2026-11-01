package biz

import (
	"context"
	"strings"

	"gin-kratos-gorm/internal/biz/model"
	"gin-kratos-gorm/internal/pkg/apperr"

	"github.com/go-kratos/kratos/v2/log"
)

// SkuSnapshot is the authoritative product data used when placing an order.
// It is deliberately a value object: prices and names are copied into the order
// so later product edits never change historical orders.
type SkuSnapshot struct {
	SkuID   int64
	SpuID   int64
	SkuName string
	SkuPic  string
	Price   float64
	Stock   int64
	OnSale  bool
}

// ProductCatalog is the order domain's view of the product domain. The
// monolith implements it with a local repository; split deployments implement
// it with a gRPC client.
type ProductCatalog interface {
	GetSkuSnapshot(ctx context.Context, skuID int64) (*SkuSnapshot, error)
	DeductStock(ctx context.Context, orderSN string, skuID int64, num int32) error
	RestoreStock(ctx context.Context, orderSN string, skuID int64, num int32) error
}

type ProductFilter struct {
	Page       int
	PageSize   int
	Keyword    string
	CategoryID int64
	OnSale     *bool
}

type ProductRepository interface {
	ProductCatalog
	GetSpu(ctx context.Context, id int64) (*model.ProductSpu, error)
	ListSpus(ctx context.Context, filter ProductFilter) ([]*model.ProductSpu, int64, error)
	CreateSpu(ctx context.Context, spu *model.ProductSpu) error
	UpdateSpu(ctx context.Context, spu *model.ProductSpu) error
	SetOnSale(ctx context.Context, id int64, onSale bool) error
	AdjustStock(ctx context.Context, skuID int64, delta int32) (int64, error)
}

type ProductUsecase struct {
	repo ProductRepository
	log  *log.Helper
}

func NewProductUsecase(repo ProductRepository, logger log.Logger) *ProductUsecase {
	return &ProductUsecase{repo: repo, log: log.NewHelper(logger)}
}

func (u *ProductUsecase) Get(ctx context.Context, id int64) (*model.ProductSpu, error) {
	if id <= 0 {
		return nil, apperr.Invalid("product id must be positive")
	}
	return u.repo.GetSpu(ctx, id)
}

func (u *ProductUsecase) GetSku(ctx context.Context, skuID int64) (*SkuSnapshot, error) {
	if skuID <= 0 {
		return nil, apperr.Invalid("sku id must be positive")
	}
	return u.repo.GetSkuSnapshot(ctx, skuID)
}

func (u *ProductUsecase) List(ctx context.Context, filter ProductFilter) ([]*model.ProductSpu, int64, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	return u.repo.ListSpus(ctx, filter)
}

func (u *ProductUsecase) Create(ctx context.Context, spu *model.ProductSpu) (*model.ProductSpu, error) {
	if spu == nil {
		return nil, apperr.Invalid("product is required")
	}
	spu.Name = strings.TrimSpace(spu.Name)
	if spu.Name == "" {
		return nil, apperr.Invalid("product name is required")
	}
	if len(spu.Skus) == 0 {
		return nil, apperr.Invalid("at least one sku is required")
	}
	if err := validateSkus(spu.Skus); err != nil {
		return nil, err
	}
	if err := u.repo.CreateSpu(ctx, spu); err != nil {
		return nil, err
	}
	return u.repo.GetSpu(ctx, spu.ID)
}

func (u *ProductUsecase) Update(ctx context.Context, spu *model.ProductSpu) (*model.ProductSpu, error) {
	if spu == nil || spu.ID <= 0 {
		return nil, apperr.Invalid("product id is required")
	}
	if strings.TrimSpace(spu.Name) == "" {
		return nil, apperr.Invalid("product name is required")
	}
	if err := u.repo.UpdateSpu(ctx, spu); err != nil {
		return nil, err
	}
	return u.repo.GetSpu(ctx, spu.ID)
}

func (u *ProductUsecase) SetOnSale(ctx context.Context, id int64, onSale bool) error {
	if id <= 0 {
		return apperr.Invalid("product id must be positive")
	}
	return u.repo.SetOnSale(ctx, id, onSale)
}

func (u *ProductUsecase) AdjustStock(ctx context.Context, skuID int64, delta int32) (int64, error) {
	if skuID <= 0 {
		return 0, apperr.Invalid("sku id must be positive")
	}
	if delta == 0 {
		return 0, apperr.Invalid("delta must not be zero")
	}
	return u.repo.AdjustStock(ctx, skuID, delta)
}

func validateSkus(skus []model.ProductSku) error {
	seen := make(map[string]struct{}, len(skus))
	for i := range skus {
		sku := &skus[i]
		if strings.TrimSpace(sku.SkuCode) == "" {
			return apperr.Invalid("sku code is required")
		}
		if _, ok := seen[sku.SkuCode]; ok {
			return apperr.Invalid("duplicate sku code %q", sku.SkuCode)
		}
		seen[sku.SkuCode] = struct{}{}
		if sku.Price <= 0 {
			return apperr.Invalid("sku %q price must be positive", sku.SkuCode)
		}
		if sku.Stock < 0 {
			return apperr.Invalid("sku %q stock must be >= 0", sku.SkuCode)
		}
	}
	return nil
}
