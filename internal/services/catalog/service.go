package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
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

// ---------------------------------------------------------------- read side

func (s *Service) GetProduct(ctx context.Context, req *pb.GetProductRequest) (*pb.Product, error) {
	if req.GetId() <= 0 {
		return nil, apperr.Invalid("id is required")
	}
	if p, ok := s.cacheGetProduct(ctx, req.GetId()); ok {
		return p, nil
	}
	var (
		p   *pb.Product
		err error
	)
	if s.gorm != nil {
		p, err = s.gorm.GetProduct(ctx, req.GetId(), true)
	} else {
		p, err = s.loadProduct(ctx, req.GetId(), true)
	}
	if err != nil {
		return nil, apperr.Internal("get product", err)
	}
	if p == nil {
		return nil, apperr.NotFound("product not found")
	}
	s.cachePutProduct(ctx, p)
	return p, nil
}

func (s *Service) GetAdminProduct(ctx context.Context, req *pb.GetProductRequest) (*pb.Product, error) {
	if req.GetId() <= 0 {
		return nil, apperr.Invalid("id is required")
	}
	p, err := s.loadProduct(ctx, req.GetId(), false)
	if err != nil {
		return nil, apperr.Internal("get admin product", err)
	}
	if p == nil || p.GetStatus() == "DELETED" {
		return nil, apperr.NotFound("product not found")
	}
	return p, nil
}

func (s *Service) ListProducts(ctx context.Context, req *pb.ListProductsRequest) (*pb.ProductList, error) {
	normalizePage(&req.Page, &req.PageSize)
	ids, err := s.categoryIDs(ctx, req.GetCategoryId())
	if err != nil {
		return nil, err
	}
	if req.GetCategoryId() > 0 && len(ids) == 0 {
		return &pb.ProductList{}, nil
	}
	if s.gorm != nil {
		list, err := s.gorm.ListProducts(ctx, req, ids)
		if err != nil {
			return nil, apperr.Internal("list products", err)
		}
		return list, nil
	}
	return s.listProductsSQL(ctx, req, ids)
}

func (s *Service) ListAdminProducts(ctx context.Context, req *pb.ListAdminProductsRequest) (*pb.ProductList, error) {
	normalizePage(&req.Page, &req.PageSize)
	if s.gorm != nil {
		list, err := s.gorm.ListAdminProducts(ctx, req)
		if err != nil {
			return nil, apperr.Internal("list admin products", err)
		}
		return list, nil
	}
	return s.listAdminProductsSQL(ctx, req)
}

func (s *Service) GetSku(ctx context.Context, req *pb.GetSkuRequest) (*pb.Sku, error) {
	if req.GetSkuId() <= 0 {
		return nil, apperr.Invalid("sku_id is required")
	}
	sku, err := s.loadSku(ctx, req.GetSkuId(), true)
	if err != nil {
		return nil, apperr.Internal("get sku", err)
	}
	if sku == nil {
		return nil, apperr.NotFound("sku not found")
	}
	return sku, nil
}

// loadSku is shared by the public SKU endpoint and the admin write methods.
// activeOnly enforces the C-end rule: the SKU and its parent SPU must both be
// sellable. Admin writes pass false so a DRAFT/disabled SKU can be read back.
func (s *Service) loadSku(ctx context.Context, id int64, activeOnly bool) (*pb.Sku, error) {
	query := `SELECT k.id, k.spu_id, k.sku_code, k.specs_json, k.price_cent, k.stock, k.status, k.image_url,
		p.category_id, p.brand_id
		FROM skus k JOIN spus p ON p.id = k.spu_id WHERE k.id = ?`
	if activeOnly {
		query += " AND k.status = 'ENABLED' AND p.status = 'ON_SALE'"
	}
	var sku pb.Sku
	err := s.db.QueryRowContext(ctx, query, id).
		Scan(&sku.Id, &sku.SpuId, &sku.SkuCode, &sku.SpecsJson, &sku.PriceCent, &sku.Stock, &sku.Status, &sku.ImageUrl,
			&sku.CategoryId, &sku.BrandId)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sku, nil
}

// ---------------------------------------------------------------- category

func (s *Service) CreateCategory(ctx context.Context, req *pb.CreateCategoryRequest) (*pb.Category, error) {
	c := req.GetCategory()
	if c == nil || strings.TrimSpace(c.GetName()) == "" {
		return nil, apperr.Invalid("category name is required")
	}
	parentID := c.GetParentId()
	level := int32(1)
	if parentID > 0 {
		var parentLevel int32
		err := s.db.QueryRowContext(ctx, `SELECT level FROM categories WHERE id = ?`, parentID).Scan(&parentLevel)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("parent category not found")
		}
		if err != nil {
			return nil, apperr.Internal("load parent category", err)
		}
		level = parentLevel + 1
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO categories (parent_id, name, level, sort, status) VALUES (?, ?, ?, ?, ?)`,
		parentID, strings.TrimSpace(c.GetName()), level, c.GetSort(), defaultString(c.GetStatus(), "ENABLED"))
	if err != nil {
		return nil, apperr.Internal("create category", err)
	}
	c.Id, _ = res.LastInsertId()
	c.Level = level
	return c, nil
}

func (s *Service) UpdateCategory(ctx context.Context, req *pb.UpdateCategoryRequest) (*pb.Category, error) {
	c := req.GetCategory()
	if c == nil || c.GetId() <= 0 || strings.TrimSpace(c.GetName()) == "" {
		return nil, apperr.Invalid("category id and name are required")
	}
	if c.GetParentId() == c.GetId() {
		return nil, apperr.Invalid("category cannot be its own parent")
	}
	if c.GetParentId() > 0 {
		var cyclic int
		if err := s.db.QueryRowContext(ctx, `WITH RECURSIVE descendants AS (
			SELECT id FROM categories WHERE id = ?
			UNION ALL
			SELECT child.id FROM categories child JOIN descendants d ON child.parent_id = d.id
		) SELECT COUNT(*) FROM descendants WHERE id = ?`, c.GetId(), c.GetParentId()).Scan(&cyclic); err != nil {
			return nil, apperr.Internal("check category cycle", err)
		}
		if cyclic > 0 {
			return nil, apperr.Invalid("category cannot be moved below its own descendant")
		}
	}
	level := int32(1)
	if c.GetParentId() > 0 {
		var parentLevel int32
		err := s.db.QueryRowContext(ctx, `SELECT level FROM categories WHERE id = ?`, c.GetParentId()).Scan(&parentLevel)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("parent category not found")
		}
		if err != nil {
			return nil, apperr.Internal("load parent category", err)
		}
		level = parentLevel + 1
	}
	var currentStatus string
	if err := s.db.QueryRowContext(ctx, `SELECT status FROM categories WHERE id = ?`, c.GetId()).Scan(&currentStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("category not found")
		}
		return nil, apperr.Internal("query category", err)
	}
	status := strings.TrimSpace(c.GetStatus())
	if status == "" {
		status = currentStatus
	}
	if status != "ENABLED" && status != "DISABLED" {
		return nil, apperr.Invalid("invalid category status")
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE categories SET parent_id = ?, name = ?, level = ?, sort = ?, status = ? WHERE id = ?`,
		c.GetParentId(), strings.TrimSpace(c.GetName()), level, c.GetSort(), status, c.GetId()); err != nil {
		return nil, apperr.Internal("update category", err)
	}
	c.Level = level
	return c, nil
}

func (s *Service) DeleteCategory(ctx context.Context, req *pb.DeleteCategoryRequest) (*pb.Empty, error) {
	if req.GetId() <= 0 {
		return nil, apperr.Invalid("id is required")
	}
	var children, products int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM categories WHERE parent_id = ?`, req.GetId()).Scan(&children); err != nil {
		return nil, apperr.Internal("count child categories", err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM spus WHERE category_id = ? AND status <> 'DELETED'`, req.GetId()).Scan(&products); err != nil {
		return nil, apperr.Internal("count category products", err)
	}
	if children > 0 || products > 0 {
		return nil, apperr.Failed("category has children or products")
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM categories WHERE id = ?`, req.GetId())
	if err != nil {
		return nil, apperr.Internal("delete category", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, apperr.NotFound("category not found")
	}
	return &pb.Empty{}, nil
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

// ---------------------------------------------------------------- brand

func (s *Service) CreateBrand(ctx context.Context, req *pb.CreateBrandRequest) (*pb.Brand, error) {
	b := req.GetBrand()
	if b == nil || strings.TrimSpace(b.GetName()) == "" {
		return nil, apperr.Invalid("brand name is required")
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO brands (name, logo, status) VALUES (?, ?, ?)`,
		strings.TrimSpace(b.GetName()), b.GetLogo(), defaultString(b.GetStatus(), "ENABLED"))
	if err != nil {
		return nil, apperr.Internal("create brand", err)
	}
	b.Id, _ = res.LastInsertId()
	return b, nil
}

func (s *Service) UpdateBrand(ctx context.Context, req *pb.UpdateBrandRequest) (*pb.Brand, error) {
	b := req.GetBrand()
	if b == nil || b.GetId() <= 0 || strings.TrimSpace(b.GetName()) == "" {
		return nil, apperr.Invalid("brand id and name are required")
	}
	var currentStatus, currentLogo string
	if err := s.db.QueryRowContext(ctx, `SELECT status, logo FROM brands WHERE id = ?`, b.GetId()).Scan(&currentStatus, &currentLogo); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("brand not found")
		}
		return nil, apperr.Internal("query brand", err)
	}
	status := strings.TrimSpace(b.GetStatus())
	if status == "" {
		status = currentStatus
	}
	if status != "ENABLED" && status != "DISABLED" {
		return nil, apperr.Invalid("invalid brand status")
	}
	logo := b.GetLogo()
	if strings.TrimSpace(logo) == "" {
		logo = currentLogo
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE brands SET name = ?, logo = ?, status = ? WHERE id = ?`,
		strings.TrimSpace(b.GetName()), logo, status, b.GetId()); err != nil {
		return nil, apperr.Internal("update brand", err)
	}
	return b, nil
}

func (s *Service) DeleteBrand(ctx context.Context, req *pb.DeleteBrandRequest) (*pb.Empty, error) {
	if req.GetId() <= 0 {
		return nil, apperr.Invalid("id is required")
	}
	var products int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM spus WHERE brand_id = ? AND status <> 'DELETED'`, req.GetId()).Scan(&products); err != nil {
		return nil, apperr.Internal("count brand products", err)
	}
	if products > 0 {
		return nil, apperr.Failed("brand has products")
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM brands WHERE id = ?`, req.GetId())
	if err != nil {
		return nil, apperr.Internal("delete brand", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, apperr.NotFound("brand not found")
	}
	return &pb.Empty{}, nil
}

func (s *Service) ListBrands(ctx context.Context, _ *pb.Empty) (*pb.BrandList, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, logo, status FROM brands ORDER BY id`)
	if err != nil {
		return nil, apperr.Internal("list brands", err)
	}
	defer rows.Close()
	reply := &pb.BrandList{}
	for rows.Next() {
		var b pb.Brand
		if err := rows.Scan(&b.Id, &b.Name, &b.Logo, &b.Status); err != nil {
			return nil, apperr.Internal("scan brand", err)
		}
		reply.Brands = append(reply.Brands, &b)
	}
	return reply, rows.Err()
}

// ---------------------------------------------------------------- product writes

func (s *Service) CreateProduct(ctx context.Context, req *pb.CreateProductRequest) (*pb.Product, error) {
	if strings.TrimSpace(req.GetName()) == "" || req.GetCategoryId() <= 0 || len(req.GetSkus()) == 0 {
		return nil, apperr.Invalid("name, category_id and skus are required")
	}
	if err := s.requireCategory(ctx, s.db, req.GetCategoryId()); err != nil {
		return nil, err
	}
	if req.GetBrandId() > 0 {
		if err := s.requireBrand(ctx, s.db, req.GetBrandId()); err != nil {
			return nil, err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin product", err)
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `INSERT INTO spus (category_id, brand_id, name, subtitle, description, main_image, status, min_price_cent) VALUES (?, ?, ?, ?, ?, ?, 'DRAFT', 0)`,
		req.GetCategoryId(), req.GetBrandId(), strings.TrimSpace(req.GetName()), req.GetSubtitle(), req.GetDescription(), req.GetMainImage())
	if err != nil {
		return nil, apperr.Internal("create spu", err)
	}
	spuID, _ := res.LastInsertId()
	for _, in := range req.GetSkus() {
		if err := validateSkuInput(in); err != nil {
			return nil, err
		}
		specs := in.GetSpecsJson()
		if strings.TrimSpace(specs) == "" {
			specs = "{}"
		}
		// stock is a projection owned by inventory; product creation must not
		// invent stock. The admin initializes inventory before publishing.
		if _, err := tx.ExecContext(ctx, `INSERT INTO skus (spu_id, sku_code, specs_json, price_cent, stock, status, image_url) VALUES (?, ?, ?, ?, 0, 'ENABLED', ?)`,
			spuID, strings.TrimSpace(in.GetSkuCode()), specs, in.GetPriceCent(), in.GetImageUrl()); err != nil {
			return nil, apperr.Internal("create sku", err)
		}
	}
	if err := insertMedia(ctx, tx, spuID, req.GetImages(), req.GetAttributes()); err != nil {
		return nil, err
	}
	if err := recalcMinPrice(ctx, tx, spuID); err != nil {
		return nil, err
	}
	if err := appendProductEvent(ctx, tx, spuID, "DRAFT", "product.created"); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, apperr.Internal("commit product", err)
	}
	p, err := s.loadProduct(ctx, spuID, false)
	if err != nil {
		return nil, err
	}
	s.cacheEvictProduct(ctx, spuID)
	return p, nil
}

func (s *Service) UpdateProduct(ctx context.Context, req *pb.UpdateProductRequest) (*pb.Product, error) {
	if req.GetId() <= 0 || strings.TrimSpace(req.GetName()) == "" || req.GetCategoryId() <= 0 {
		return nil, apperr.Invalid("id, name and category_id are required")
	}
	if err := s.requireCategory(ctx, s.db, req.GetCategoryId()); err != nil {
		return nil, err
	}
	if req.GetBrandId() > 0 {
		if err := s.requireBrand(ctx, s.db, req.GetBrandId()); err != nil {
			return nil, err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin update product", err)
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `UPDATE spus SET category_id = ?, brand_id = ?, name = ?, subtitle = ?, description = ?, main_image = ?, version = version + 1 WHERE id = ? AND status <> 'DELETED'`,
		req.GetCategoryId(), req.GetBrandId(), strings.TrimSpace(req.GetName()), req.GetSubtitle(), req.GetDescription(), req.GetMainImage(), req.GetId())
	if err != nil {
		return nil, apperr.Internal("update spu", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, apperr.NotFound("product not found")
	}
	// Repeated fields have presence in the Go request: nil means the client
	// omitted the field, while an explicit empty array means "clear". Preserve
	// media on partial updates; replace only when the corresponding field was
	// supplied.
	if req.Images != nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM spu_images WHERE spu_id = ?`, req.GetId()); err != nil {
			return nil, apperr.Internal("replace images", err)
		}
		if err := insertMedia(ctx, tx, req.GetId(), req.GetImages(), nil); err != nil {
			return nil, err
		}
	}
	if req.Attributes != nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM spu_attributes WHERE spu_id = ?`, req.GetId()); err != nil {
			return nil, apperr.Internal("replace attributes", err)
		}
		if err := insertMedia(ctx, tx, req.GetId(), nil, req.GetAttributes()); err != nil {
			return nil, err
		}
	}
	if err := recalcMinPrice(ctx, tx, req.GetId()); err != nil {
		return nil, err
	}
	if err := appendProductEvent(ctx, tx, req.GetId(), "", "product.updated"); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, apperr.Internal("commit update product", err)
	}
	s.cacheEvictProduct(ctx, req.GetId())
	return s.loadProduct(ctx, req.GetId(), false)
}

func (s *Service) DeleteProduct(ctx context.Context, req *pb.DeleteProductRequest) (*pb.Empty, error) {
	if req.GetId() <= 0 {
		return nil, apperr.Invalid("id is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin delete product", err)
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `UPDATE spus SET status = 'DELETED', version = version + 1 WHERE id = ? AND status <> 'DELETED'`, req.GetId())
	if err != nil {
		return nil, apperr.Internal("delete product", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, apperr.NotFound("product not found")
	}
	if err := appendProductEvent(ctx, tx, req.GetId(), "DELETED", "product.deleted"); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, apperr.Internal("commit delete product", err)
	}
	s.cacheEvictProduct(ctx, req.GetId())
	return &pb.Empty{}, nil
}

func (s *Service) SetProductStatus(ctx context.Context, req *pb.SetProductStatusRequest) (*pb.Empty, error) {
	if req.GetId() <= 0 || !validProductStatus(req.GetStatus()) {
		return nil, apperr.Invalid("valid id and status are required")
	}
	if req.GetStatus() == "ON_SALE" {
		var enabled int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM skus WHERE spu_id = ? AND status = 'ENABLED' AND price_cent > 0`, req.GetId()).Scan(&enabled); err != nil {
			return nil, apperr.Internal("count active skus", err)
		}
		if enabled == 0 {
			return nil, apperr.Failed("product has no active sku")
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin set status", err)
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `UPDATE spus SET status = ?, version = version + 1 WHERE id = ? AND status <> 'DELETED'`, req.GetStatus(), req.GetId())
	if err != nil {
		return nil, apperr.Internal("set status", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, apperr.NotFound("product not found")
	}
	if err := appendProductEvent(ctx, tx, req.GetId(), req.GetStatus(), "product.status_changed"); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, apperr.Internal("commit set status", err)
	}
	s.cacheEvictProduct(ctx, req.GetId())
	return &pb.Empty{}, nil
}

// ---------------------------------------------------------------- sku writes

func (s *Service) CreateSku(ctx context.Context, req *pb.CreateSkuRequest) (*pb.Sku, error) {
	if req.GetSpuId() <= 0 || req.GetSku() == nil {
		return nil, apperr.Invalid("spu_id and sku are required")
	}
	if err := validateSkuInput(req.GetSku()); err != nil {
		return nil, err
	}
	if err := s.requireProduct(ctx, s.db, req.GetSpuId()); err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin create sku", err)
	}
	defer func() { _ = tx.Rollback() }()
	specs := req.GetSku().GetSpecsJson()
	if strings.TrimSpace(specs) == "" {
		specs = "{}"
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO skus (spu_id, sku_code, specs_json, price_cent, stock, status, image_url) VALUES (?, ?, ?, ?, 0, 'ENABLED', ?)`,
		req.GetSpuId(), strings.TrimSpace(req.GetSku().GetSkuCode()), specs, req.GetSku().GetPriceCent(), req.GetSku().GetImageUrl())
	if err != nil {
		return nil, apperr.Internal("create sku", err)
	}
	id, _ := res.LastInsertId()
	if err := recalcMinPrice(ctx, tx, req.GetSpuId()); err != nil {
		return nil, err
	}
	if err := appendProductEvent(ctx, tx, req.GetSpuId(), "", "sku.created"); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, apperr.Internal("commit create sku", err)
	}
	s.cacheEvictProduct(ctx, req.GetSpuId())
	return s.loadSku(ctx, id, false)
}

func (s *Service) UpdateSku(ctx context.Context, req *pb.UpdateSkuRequest) (*pb.Sku, error) {
	if req.GetId() <= 0 {
		return nil, apperr.Invalid("id is required")
	}
	var (
		spuID         int64
		currentStatus string
	)
	if err := s.db.QueryRowContext(ctx, `SELECT spu_id, status FROM skus WHERE id = ? AND status <> 'DELETED'`, req.GetId()).Scan(&spuID, &currentStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("sku not found")
		}
		return nil, apperr.Internal("load sku", err)
	}
	if strings.TrimSpace(req.GetSkuCode()) == "" || req.GetPriceCent() <= 0 {
		return nil, apperr.Invalid("sku_code and positive price_cent are required")
	}
	specs := req.GetSpecsJson()
	if strings.TrimSpace(specs) == "" {
		specs = "{}"
	}
	if !json.Valid([]byte(specs)) {
		return nil, apperr.Invalid("specs_json must be valid JSON")
	}
	status := strings.TrimSpace(req.GetStatus())
	if status == "" {
		status = currentStatus
	}
	if status != "ENABLED" && status != "DISABLED" {
		return nil, apperr.Invalid("invalid sku status")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperr.Internal("begin update sku", err)
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `UPDATE skus SET sku_code = ?, specs_json = ?, price_cent = ?, status = ?, image_url = ? WHERE id = ?`,
		strings.TrimSpace(req.GetSkuCode()), specs, req.GetPriceCent(), status, req.GetImageUrl(), req.GetId())
	if err != nil {
		return nil, apperr.Internal("update sku", err)
	}
	if err := recalcMinPrice(ctx, tx, spuID); err != nil {
		return nil, err
	}
	if err := appendProductEvent(ctx, tx, spuID, "", "sku.updated"); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, apperr.Internal("commit update sku", err)
	}
	s.cacheEvictProduct(ctx, spuID)
	return s.loadSku(ctx, req.GetId(), false)
}

func (s *Service) DeleteSku(ctx context.Context, req *pb.DeleteSkuRequest) (*pb.Empty, error) {
	if req.GetId() <= 0 {
		return nil, apperr.Invalid("id is required")
	}
	var spuID int64
	if err := s.db.QueryRowContext(ctx, `SELECT spu_id FROM skus WHERE id = ? AND status <> 'DELETED'`, req.GetId()).Scan(&spuID); err != nil {
		return nil, apperr.NotFound("sku not found")
	}
	if err := s.setSkuStatus(ctx, req.GetId(), "DELETED"); err != nil {
		return nil, err
	}
	s.cacheEvictProduct(ctx, spuID)
	return &pb.Empty{}, nil
}

func (s *Service) SetSkuStatus(ctx context.Context, req *pb.SetSkuStatusRequest) (*pb.Sku, error) {
	if req.GetId() <= 0 || (req.GetStatus() != "ENABLED" && req.GetStatus() != "DISABLED") {
		return nil, apperr.Invalid("valid id and status are required")
	}
	var spuID int64
	if err := s.db.QueryRowContext(ctx, `SELECT spu_id FROM skus WHERE id = ? AND status <> 'DELETED'`, req.GetId()).Scan(&spuID); err != nil {
		return nil, apperr.NotFound("sku not found")
	}
	if err := s.setSkuStatus(ctx, req.GetId(), req.GetStatus()); err != nil {
		return nil, err
	}
	s.cacheEvictProduct(ctx, spuID)
	return s.loadSku(ctx, req.GetId(), false)
}

func (s *Service) setSkuStatus(ctx context.Context, id int64, status string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return apperr.Internal("begin sku status", err)
	}
	defer func() { _ = tx.Rollback() }()
	var spuID int64
	if err := tx.QueryRowContext(ctx, `SELECT spu_id FROM skus WHERE id = ?`, id).Scan(&spuID); err != nil {
		return apperr.NotFound("sku not found")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE skus SET status = ? WHERE id = ?`, status, id); err != nil {
		return apperr.Internal("set sku status", err)
	}
	if err := recalcMinPrice(ctx, tx, spuID); err != nil {
		return err
	}
	if err := appendProductEvent(ctx, tx, spuID, "", "sku.status_changed"); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return apperr.Internal("commit sku status", err)
	}
	return nil
}

// ---------------------------------------------------------------- helpers

func normalizePage(page, size *int32) {
	if *page <= 0 {
		*page = 1
	}
	if *size <= 0 || *size > 100 {
		*size = 20
	}
}

func validProductStatus(v string) bool {
	switch v {
	case "DRAFT", "ON_SALE", "OFF_SALE":
		return true
	default:
		return false
	}
}

func validateSkuInput(in *pb.SkuInput) error {
	if in == nil || strings.TrimSpace(in.GetSkuCode()) == "" || in.GetPriceCent() <= 0 {
		return apperr.Invalid("sku_code and price_cent are required")
	}
	if in.GetStock() != 0 {
		return apperr.Invalid("sku stock is managed by inventory; use inventory.adjust before publishing")
	}
	specs := in.GetSpecsJson()
	if strings.TrimSpace(specs) == "" {
		specs = "{}"
	}
	if !json.Valid([]byte(specs)) {
		return apperr.Invalid("specs_json must be valid JSON for sku %s", in.GetSkuCode())
	}
	return nil
}

func insertMedia(ctx context.Context, tx *sql.Tx, spuID int64, images []*pb.ProductImage, attrs []*pb.ProductAttribute) error {
	for _, img := range images {
		if img == nil || strings.TrimSpace(img.GetUrl()) == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO spu_images (spu_id, url, sort) VALUES (?, ?, ?)`, spuID, img.GetUrl(), img.GetSort()); err != nil {
			return apperr.Internal("insert product image", err)
		}
	}
	for _, attr := range attrs {
		if attr == nil || strings.TrimSpace(attr.GetName()) == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO spu_attributes (spu_id, name, value, sort) VALUES (?, ?, ?, ?)`, spuID, attr.GetName(), attr.GetValue(), attr.GetSort()); err != nil {
			return apperr.Internal("insert product attribute", err)
		}
	}
	return nil
}

func recalcMinPrice(ctx context.Context, tx *sql.Tx, spuID int64) error {
	// Serialize concurrent SKU mutations for the same SPU. Without this lock two
	// transactions can each compute a minimum from their own snapshot and the
	// last writer may store a stale price used by C-end filters.
	var locked int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM spus WHERE id = ? FOR UPDATE`, spuID).Scan(&locked); err != nil {
		return apperr.Internal("lock spu for min price", err)
	}
	var min sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT MIN(price_cent) FROM skus WHERE spu_id = ? AND status = 'ENABLED' AND price_cent > 0`, spuID).Scan(&min); err != nil {
		return apperr.Internal("recalculate min price", err)
	}
	value := int64(0)
	if min.Valid {
		value = min.Int64
	}
	if _, err := tx.ExecContext(ctx, `UPDATE spus SET min_price_cent = ? WHERE id = ?`, value, spuID); err != nil {
		return apperr.Internal("update min price", err)
	}
	var enabled int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM skus WHERE spu_id = ? AND status = 'ENABLED'`, spuID).Scan(&enabled); err != nil {
		return apperr.Internal("count enabled skus", err)
	}
	if enabled == 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE spus SET status = 'OFF_SALE', version = version + 1 WHERE id = ? AND status = 'ON_SALE'`, spuID); err != nil {
			return apperr.Internal("unpublish empty product", err)
		}
	}
	return nil
}

func appendProductEvent(ctx context.Context, tx *sql.Tx, spuID int64, status, eventType string) error {
	payload, _ := json.Marshal(map[string]any{"spu_id": spuID, "status": status})
	return outbox.Append(ctx, tx, outbox.Event{
		Topic: "shop.catalog.product.changed.v1", AggregateType: "spu",
		AggregateID: strconv.FormatInt(spuID, 10), EventType: eventType, Payload: payload,
	})
}

func (s *Service) appendStandaloneProductEvent(ctx context.Context, spuID int64, status, eventType string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return apperr.Internal("begin product event", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := appendProductEvent(ctx, tx, spuID, status, eventType); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return apperr.Internal("commit product event", err)
	}
	return nil
}

func (s *Service) requireCategory(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id int64) error {
	var exists int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM categories WHERE id = ? AND status = 'ENABLED'`, id).Scan(&exists); err != nil {
		return apperr.Internal("query category", err)
	}
	if exists == 0 {
		return apperr.NotFound("category not found or disabled")
	}
	return nil
}

func (s *Service) requireBrand(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id int64) error {
	var exists int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM brands WHERE id = ? AND status = 'ENABLED'`, id).Scan(&exists); err != nil {
		return apperr.Internal("query brand", err)
	}
	if exists == 0 {
		return apperr.NotFound("brand not found or disabled")
	}
	return nil
}

func (s *Service) requireProduct(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id int64) error {
	var exists int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM spus WHERE id = ? AND status <> 'DELETED'`, id).Scan(&exists); err != nil {
		return apperr.Internal("query product", err)
	}
	if exists == 0 {
		return apperr.NotFound("product not found")
	}
	return nil
}

func (s *Service) categoryIDs(ctx context.Context, root int64) ([]int64, error) {
	if root <= 0 {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `WITH RECURSIVE tree AS (
		SELECT id FROM categories WHERE id = ?
		UNION ALL
		SELECT c.id FROM categories c JOIN tree t ON c.parent_id = t.id
	) SELECT id FROM tree`, root)
	if err != nil {
		return nil, apperr.Internal("load category tree", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, apperr.Internal("scan category id", err)
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return ids, rows.Err()
}

func (s *Service) loadProduct(ctx context.Context, id int64, activeOnly bool) (*pb.Product, error) {
	var p pb.Product
	err := s.db.QueryRowContext(ctx, `SELECT id, category_id, brand_id, name, subtitle, description, main_image, status, min_price_cent FROM spus WHERE id = ?`, id).
		Scan(&p.Id, &p.CategoryId, &p.BrandId, &p.Name, &p.Subtitle, &p.Description, &p.MainImage, &p.Status, &p.MinPriceCent)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, apperr.Internal("get product", err)
	}
	if activeOnly && p.Status != "ON_SALE" {
		return nil, nil
	}
	skuQuery := `SELECT id, spu_id, sku_code, specs_json, price_cent, stock, status, image_url FROM skus WHERE spu_id = ?`
	if activeOnly {
		skuQuery += ` AND status = 'ENABLED'`
	}
	skuQuery += ` ORDER BY id`
	rows, err := s.db.QueryContext(ctx, skuQuery, id)
	if err != nil {
		return nil, apperr.Internal("get skus", err)
	}
	for rows.Next() {
		var sku pb.Sku
		if err := rows.Scan(&sku.Id, &sku.SpuId, &sku.SkuCode, &sku.SpecsJson, &sku.PriceCent, &sku.Stock, &sku.Status, &sku.ImageUrl); err != nil {
			rows.Close()
			return nil, apperr.Internal("scan sku", err)
		}
		if sku.Status == "ENABLED" && sku.PriceCent > 0 && (p.MinPriceCent == 0 || sku.PriceCent < p.MinPriceCent) {
			p.MinPriceCent = sku.PriceCent
		}
		p.Skus = append(p.Skus, &sku)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	imgRows, err := s.db.QueryContext(ctx, `SELECT id, url, sort FROM spu_images WHERE spu_id = ? ORDER BY sort, id`, id)
	if err != nil {
		return nil, apperr.Internal("get product images", err)
	}
	for imgRows.Next() {
		var img pb.ProductImage
		if err := imgRows.Scan(&img.Id, &img.Url, &img.Sort); err != nil {
			imgRows.Close()
			return nil, apperr.Internal("scan product image", err)
		}
		p.Images = append(p.Images, &img)
	}
	imgRows.Close()
	attrRows, err := s.db.QueryContext(ctx, `SELECT id, name, value, sort FROM spu_attributes WHERE spu_id = ? ORDER BY sort, id`, id)
	if err != nil {
		return nil, apperr.Internal("get product attributes", err)
	}
	for attrRows.Next() {
		var attr pb.ProductAttribute
		if err := attrRows.Scan(&attr.Id, &attr.Name, &attr.Value, &attr.Sort); err != nil {
			attrRows.Close()
			return nil, apperr.Internal("scan product attribute", err)
		}
		p.Attributes = append(p.Attributes, &attr)
	}
	attrRows.Close()
	return &p, attrRows.Err()
}

func (s *Service) listProductsSQL(ctx context.Context, req *pb.ListProductsRequest, categoryIDs []int64) (*pb.ProductList, error) {
	where := "WHERE status = 'ON_SALE'"
	args := []any{}
	if kw := strings.TrimSpace(req.GetKeyword()); kw != "" {
		where += " AND (name LIKE ? OR subtitle LIKE ?)"
		args = append(args, "%"+kw+"%", "%"+kw+"%")
	}
	if len(categoryIDs) > 0 {
		where += " AND category_id IN (" + placeholders(len(categoryIDs)) + ")"
		for _, id := range categoryIDs {
			args = append(args, id)
		}
	}
	if req.GetBrandId() > 0 {
		where += " AND brand_id = ?"
		args = append(args, req.GetBrandId())
	}
	if req.GetMinPriceCent() > 0 {
		where += " AND min_price_cent >= ?"
		args = append(args, req.GetMinPriceCent())
	}
	if req.GetMaxPriceCent() > 0 {
		where += " AND min_price_cent <= ?"
		args = append(args, req.GetMaxPriceCent())
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM spus "+where, args...).Scan(&total); err != nil {
		return nil, apperr.Internal("count products", err)
	}
	order := "id DESC"
	if req.GetSort() == "price_asc" {
		order = "min_price_cent ASC, id DESC"
	} else if req.GetSort() == "price_desc" {
		order = "min_price_cent DESC, id DESC"
	}
	args = append(args, req.GetPageSize(), (req.GetPage()-1)*req.GetPageSize())
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM spus "+where+" ORDER BY "+order+" LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, apperr.Internal("list products", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, apperr.Internal("scan product id", err)
		}
		ids = append(ids, id)
	}
	reply := &pb.ProductList{Total: total}
	for _, id := range ids {
		p, err := s.loadProduct(ctx, id, true)
		if err != nil {
			return nil, err
		}
		if p != nil {
			reply.Products = append(reply.Products, p)
		}
	}
	return reply, rows.Err()
}

func (s *Service) listAdminProductsSQL(ctx context.Context, req *pb.ListAdminProductsRequest) (*pb.ProductList, error) {
	where := "WHERE status <> 'DELETED'"
	args := []any{}
	if kw := strings.TrimSpace(req.GetKeyword()); kw != "" {
		where += " AND (name LIKE ? OR subtitle LIKE ?)"
		args = append(args, "%"+kw+"%", "%"+kw+"%")
	}
	if req.GetCategoryId() > 0 {
		where += " AND category_id = ?"
		args = append(args, req.GetCategoryId())
	}
	if req.GetBrandId() > 0 {
		where += " AND brand_id = ?"
		args = append(args, req.GetBrandId())
	}
	if st := strings.TrimSpace(req.GetStatus()); st != "" {
		where += " AND status = ?"
		args = append(args, st)
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM spus "+where, args...).Scan(&total); err != nil {
		return nil, apperr.Internal("count admin products", err)
	}
	args = append(args, req.GetPageSize(), (req.GetPage()-1)*req.GetPageSize())
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM spus "+where+" ORDER BY id DESC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, apperr.Internal("list admin products", err)
	}
	defer rows.Close()
	reply := &pb.ProductList{Total: total}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, apperr.Internal("scan admin product id", err)
		}
		p, err := s.loadProduct(ctx, id, false)
		if err != nil {
			return nil, err
		}
		if p != nil {
			reply.Products = append(reply.Products, p)
		}
	}
	return reply, rows.Err()
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func defaultString(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
