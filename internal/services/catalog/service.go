package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"
	"gin-kratos-gorm/internal/platform/apperr"
	"gin-kratos-gorm/internal/platform/mq"
	"gin-kratos-gorm/internal/platform/outbox"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

type Service struct {
	pb.UnimplementedCatalogServiceServer
	db    *sql.DB
	mq    *mq.Client
	cache *redis.Client
	gorm  *gormReader
	log   *slog.Logger
}

func New(db *sql.DB, client *mq.Client, cache *redis.Client, logger *slog.Logger) *Service {
	s := &Service{db: db, mq: client, cache: cache, log: logger}
	reader, err := newGormReader(db)
	if err != nil {
		if logger != nil {
			logger.Warn("catalog gorm reader disabled", "error", err)
		}
	} else {
		s.gorm = reader
	}
	return s
}

const productCacheKey = "catalog:product:"

func (s *Service) cacheGetProduct(ctx context.Context, id int64) (*pb.Product, bool) {
	if s.cache == nil {
		return nil, false
	}
	raw, err := s.cache.Get(ctx, fmt.Sprintf("%s%d", productCacheKey, id)).Bytes()
	if err != nil {
		return nil, false
	}
	var p pb.Product
	if err := proto.Unmarshal(raw, &p); err != nil {
		return nil, false
	}
	return &p, true
}

func (s *Service) cachePutProduct(ctx context.Context, p *pb.Product) {
	if s.cache == nil || p == nil {
		return
	}
	raw, err := proto.Marshal(p)
	if err != nil {
		return
	}
	_ = s.cache.Set(ctx, fmt.Sprintf("%s%d", productCacheKey, p.GetId()), raw, 60*time.Second).Err()
}

func (s *Service) cacheEvictProduct(ctx context.Context, id int64) {
	if s.cache != nil {
		_ = s.cache.Del(ctx, fmt.Sprintf("%s%d", productCacheKey, id)).Err()
	}
}

func (s *Service) GetProduct(ctx context.Context, req *pb.GetProductRequest) (*pb.Product, error) {
	if p, ok := s.cacheGetProduct(ctx, req.GetId()); ok {
		return p, nil
	}
	var p *pb.Product
	var err error
	if s.gorm != nil {
		p, err = s.gorm.GetProduct(ctx, req.GetId())
		if err != nil {
			return nil, apperr.Internal("get product", err)
		}
		if p == nil {
			return nil, apperr.NotFound("product not found")
		}
	} else {
		p, err = s.loadProduct(ctx, req.GetId())
		if err != nil {
			return nil, err
		}
	}
	s.cachePutProduct(ctx, p)
	return p, nil
}

func (s *Service) ListProducts(ctx context.Context, req *pb.ListProductsRequest) (*pb.ProductList, error) {
	if req.GetPage() <= 0 {
		req.Page = 1
	}
	if req.GetPageSize() <= 0 || req.GetPageSize() > 100 {
		req.PageSize = 20
	}
	if s.gorm != nil {
		list, err := s.gorm.ListProducts(ctx, req)
		if err != nil {
			return nil, apperr.Internal("list products", err)
		}
		return list, nil
	}
	where := "WHERE status = 'ON_SALE'"
	args := []any{}
	if kw := strings.TrimSpace(req.GetKeyword()); kw != "" {
		where += " AND (name LIKE ? OR subtitle LIKE ?)"
		args = append(args, "%"+kw+"%", "%"+kw+"%")
	}
	if req.GetCategoryId() > 0 {
		where += " AND category_id = ?"
		args = append(args, req.GetCategoryId())
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM spus "+where, args...).Scan(&total); err != nil {
		return nil, apperr.Internal("count products", err)
	}
	args = append(args, req.GetPageSize(), (req.GetPage()-1)*req.GetPageSize())
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM spus "+where+" ORDER BY id DESC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, apperr.Internal("list products", err)
	}
	defer rows.Close()
	reply := &pb.ProductList{Total: total}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, apperr.Internal("scan product id", err)
		}
		p, err := s.loadProduct(ctx, id)
		if err != nil {
			return nil, err
		}
		reply.Products = append(reply.Products, p)
	}
	return reply, rows.Err()
}

func (s *Service) GetSku(ctx context.Context, req *pb.GetSkuRequest) (*pb.Sku, error) {
	if s.gorm != nil {
		sku, err := s.gorm.GetSku(ctx, req.GetSkuId())
		if err != nil {
			return nil, apperr.Internal("get sku", err)
		}
		if sku == nil {
			return nil, apperr.NotFound("sku not found")
		}
		return sku, nil
	}
	var sku pb.Sku
	err := s.db.QueryRowContext(ctx, `SELECT id, spu_id, sku_code, specs_json, price_cent, stock, status FROM skus WHERE id = ?`, req.GetSkuId()).
		Scan(&sku.Id, &sku.SpuId, &sku.SkuCode, &sku.SpecsJson, &sku.PriceCent, &sku.Stock, &sku.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("sku not found")
	}
	if err != nil {
		return nil, apperr.Internal("get sku", err)
	}
	return &sku, nil
}

func (s *Service) CreateCategory(ctx context.Context, req *pb.CreateCategoryRequest) (*pb.Category, error) {
	c := req.GetCategory()
	if c == nil || strings.TrimSpace(c.GetName()) == "" {
		return nil, apperr.Invalid("category name is required")
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO categories (parent_id, name, level, sort, status) VALUES (?, ?, ?, ?, ?)`,
		c.GetParentId(), c.GetName(), c.GetLevel(), c.GetSort(), defaultString(c.GetStatus(), "ENABLED"))
	if err != nil {
		return nil, apperr.Internal("create category", err)
	}
	c.Id, _ = res.LastInsertId()
	return c, nil
}

func (s *Service) ListCategories(ctx context.Context, _ *pb.Empty) (*pb.CategoryList, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, parent_id, name, level, sort, status FROM categories ORDER BY sort, id`)
	if err != nil {
		return nil, apperr.Internal("list categories", err)
	}
	defer rows.Close()
	reply := &pb.CategoryList{}
	for rows.Next() {
		var c pb.Category
		if err := rows.Scan(&c.Id, &c.ParentId, &c.Name, &c.Level, &c.Sort, &c.Status); err != nil {
			return nil, apperr.Internal("scan category", err)
		}
		reply.Categories = append(reply.Categories, &c)
	}
	return reply, rows.Err()
}

func (s *Service) CreateProduct(ctx context.Context, req *pb.CreateProductRequest) (*pb.Product, error) {
	if strings.TrimSpace(req.GetName()) == "" || req.GetCategoryId() <= 0 || len(req.GetSkus()) == 0 {
		return nil, apperr.Invalid("name, category_id and skus are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin product", err)
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `INSERT INTO spus (category_id, brand_id, name, subtitle, description, main_image, status) VALUES (?, ?, ?, ?, ?, ?, 'DRAFT')`,
		req.GetCategoryId(), req.GetBrandId(), req.GetName(), req.GetSubtitle(), req.GetDescription(), req.GetMainImage())
	if err != nil {
		return nil, apperr.Internal("create spu", err)
	}
	spuID, _ := res.LastInsertId()
	for _, in := range req.GetSkus() {
		if strings.TrimSpace(in.GetSkuCode()) == "" || in.GetPriceCent() <= 0 {
			return nil, apperr.Invalid("sku_code and price_cent are required")
		}
		specs := in.GetSpecsJson()
		if specs == "" {
			specs = "{}"
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO skus (spu_id, sku_code, specs_json, price_cent, stock) VALUES (?, ?, ?, ?, ?)`,
			spuID, in.GetSkuCode(), specs, in.GetPriceCent(), in.GetStock()); err != nil {
			return nil, apperr.Internal("create sku", err)
		}
	}
	payload, _ := json.Marshal(map[string]any{"spu_id": spuID, "status": "DRAFT"})
	if err := outbox.Append(ctx, tx, outbox.Event{Topic: "shop.catalog.product.changed.v1", AggregateType: "spu", AggregateID: itoa(spuID), EventType: "product.created", Payload: payload}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, apperr.Internal("commit product", err)
	}
	p, err := s.loadProduct(ctx, spuID)
	if err != nil {
		return nil, err
	}
	s.cachePutProduct(ctx, p)
	return p, nil
}

func (s *Service) SetProductStatus(ctx context.Context, req *pb.SetProductStatusRequest) (*pb.Empty, error) {
	if req.GetId() <= 0 || (req.GetStatus() != "ON_SALE" && req.GetStatus() != "OFF_SALE" && req.GetStatus() != "DRAFT") {
		return nil, apperr.Invalid("valid id and status are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin status", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE spus SET status = ?, version = version + 1 WHERE id = ?`, req.GetStatus(), req.GetId()); err != nil {
		return nil, apperr.Internal("set status", err)
	}
	payload, _ := json.Marshal(map[string]any{"spu_id": req.GetId(), "status": req.GetStatus()})
	if err := outbox.Append(ctx, tx, outbox.Event{Topic: "shop.catalog.product.changed.v1", AggregateType: "spu", AggregateID: itoa(req.GetId()), EventType: "product.status_changed", Payload: payload}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, apperr.Internal("commit status", err)
	}
	s.cacheEvictProduct(ctx, req.GetId())
	return &pb.Empty{}, nil
}

func (s *Service) loadProduct(ctx context.Context, id int64) (*pb.Product, error) {
	var p pb.Product
	err := s.db.QueryRowContext(ctx, `SELECT id, category_id, brand_id, name, subtitle, description, main_image, status FROM spus WHERE id = ?`, id).
		Scan(&p.Id, &p.CategoryId, &p.BrandId, &p.Name, &p.Subtitle, &p.Description, &p.MainImage, &p.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("product not found")
	}
	if err != nil {
		return nil, apperr.Internal("get product", err)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, spu_id, sku_code, specs_json, price_cent, stock, status FROM skus WHERE spu_id = ? ORDER BY id`, id)
	if err != nil {
		return nil, apperr.Internal("get skus", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sku pb.Sku
		if err := rows.Scan(&sku.Id, &sku.SpuId, &sku.SkuCode, &sku.SpecsJson, &sku.PriceCent, &sku.Stock, &sku.Status); err != nil {
			return nil, apperr.Internal("scan sku", err)
		}
		if p.MinPriceCent == 0 || sku.PriceCent < p.MinPriceCent {
			p.MinPriceCent = sku.PriceCent
		}
		p.Skus = append(p.Skus, &sku)
	}
	return &p, rows.Err()
}

func defaultString(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
func itoa(v int64) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.TrimSpace(formatInt(v)), " ", ""))
}
func formatInt(v int64) string { return fmt.Sprintf("%d", v) }
