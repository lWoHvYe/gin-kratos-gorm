package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	pb "gin-kratos-gorm/api/gen/mall/v1"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// gormReader is the read side of catalog. The transactional write path stays on
// database/sql so outbox rows and mutations share an explicit *sql.Tx; GORM is
// used where associations and query composition make it a net win.
type gormReader struct {
	db *gorm.DB
}

type gormSPU struct {
	ID           int64           `gorm:"column:id;primaryKey"`
	CategoryID   int64           `gorm:"column:category_id"`
	BrandID      int64           `gorm:"column:brand_id"`
	Name         string          `gorm:"column:name"`
	Subtitle     string          `gorm:"column:subtitle"`
	Description  string          `gorm:"column:description"`
	MainImage    string          `gorm:"column:main_image"`
	Status       string          `gorm:"column:status"`
	MinPriceCent int64           `gorm:"column:min_price_cent"`
	Skus         []gormSKU       `gorm:"foreignKey:SpuID"`
	Images       []gormImage     `gorm:"foreignKey:SpuID"`
	Attributes   []gormAttribute `gorm:"foreignKey:SpuID"`
}

func (gormSPU) TableName() string { return "spus" }

type gormSKU struct {
	ID        int64  `gorm:"column:id;primaryKey"`
	SpuID     int64  `gorm:"column:spu_id"`
	SkuCode   string `gorm:"column:sku_code"`
	SpecsJSON string `gorm:"column:specs_json"`
	PriceCent int64  `gorm:"column:price_cent"`
	Stock     int64  `gorm:"column:stock"`
	Status    string `gorm:"column:status"`
	ImageURL  string `gorm:"column:image_url"`
}

func (gormSKU) TableName() string { return "skus" }

type gormImage struct {
	ID    int64  `gorm:"column:id;primaryKey"`
	SpuID int64  `gorm:"column:spu_id"`
	URL   string `gorm:"column:url"`
	Sort  int32  `gorm:"column:sort"`
}

func (gormImage) TableName() string { return "spu_images" }

type gormAttribute struct {
	ID    int64  `gorm:"column:id;primaryKey"`
	SpuID int64  `gorm:"column:spu_id"`
	Name  string `gorm:"column:name"`
	Value string `gorm:"column:value"`
	Sort  int32  `gorm:"column:sort"`
}

func (gormAttribute) TableName() string { return "spu_attributes" }

func newGormReader(sqlDB *sql.DB) (*gormReader, error) {
	gdb, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		return nil, fmt.Errorf("open gorm catalog reader: %w", err)
	}
	return &gormReader{db: gdb}, nil
}

func (g *gormReader) GetProduct(ctx context.Context, id int64, activeOnly bool) (*pb.Product, error) {
	var row gormSPU
	q := g.db.WithContext(ctx).
		Preload("Skus", func(db *gorm.DB) *gorm.DB {
			if activeOnly {
				return db.Where("status = ?", "ENABLED").Order("id")
			}
			return db.Order("id")
		}).
		Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort, id") }).
		Preload("Attributes", func(db *gorm.DB) *gorm.DB { return db.Order("sort, id") })
	err := q.First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if activeOnly && row.Status != "ON_SALE" {
		return nil, nil
	}
	return row.toPB(), nil
}

func (g *gormReader) GetSku(ctx context.Context, id int64) (*pb.Sku, error) {
	var row gormSKU
	err := g.db.WithContext(ctx).First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return row.toPB(), nil
}

func (g *gormReader) ListProducts(ctx context.Context, req *pb.ListProductsRequest, categoryIDs []int64) (*pb.ProductList, error) {
	base := func() *gorm.DB {
		query := g.db.WithContext(ctx).Model(&gormSPU{}).Where("status = ?", "ON_SALE")
		if kw := strings.TrimSpace(req.GetKeyword()); kw != "" {
			like := "%" + kw + "%"
			query = query.Where("(name LIKE ? OR subtitle LIKE ? OR EXISTS (SELECT 1 FROM skus s WHERE s.spu_id = spus.id AND s.sku_code LIKE ? AND s.status = 'ENABLED'))", like, like, like)
		}
		if len(categoryIDs) > 0 {
			query = query.Where("category_id IN ?", categoryIDs)
		}
		if req.GetBrandId() > 0 {
			query = query.Where("brand_id = ?", req.GetBrandId())
		}
		if req.GetMinPriceCent() > 0 {
			query = query.Where("min_price_cent >= ?", req.GetMinPriceCent())
		}
		if req.GetMaxPriceCent() > 0 {
			query = query.Where("min_price_cent <= ?", req.GetMaxPriceCent())
		}
		return query
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, err
	}
	order := "id DESC"
	switch req.GetSort() {
	case "price_asc":
		order = "min_price_cent ASC, id DESC"
	case "price_desc":
		order = "min_price_cent DESC, id DESC"
	case "newest", "":
		order = "id DESC"
	}
	var rows []gormSPU
	if err := base().
		Preload("Skus", func(db *gorm.DB) *gorm.DB {
			return db.Where("status = ? AND price_cent > 0", "ENABLED").Order("id")
		}).
		Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort, id") }).
		Preload("Attributes", func(db *gorm.DB) *gorm.DB { return db.Order("sort, id") }).
		Order(order).
		Limit(int(req.GetPageSize())).Offset(int((req.GetPage() - 1) * req.GetPageSize())).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := &pb.ProductList{Total: total, Products: make([]*pb.Product, 0, len(rows))}
	for i := range rows {
		out.Products = append(out.Products, rows[i].toPB())
	}
	return out, nil
}

func (g *gormReader) ListAdminProducts(ctx context.Context, req *pb.ListAdminProductsRequest) (*pb.ProductList, error) {
	base := func() *gorm.DB {
		query := g.db.WithContext(ctx).Model(&gormSPU{}).Where("status <> ?", "DELETED")
		if kw := strings.TrimSpace(req.GetKeyword()); kw != "" {
			like := "%" + kw + "%"
			query = query.Where("(name LIKE ? OR subtitle LIKE ?)", like, like)
		}
		if req.GetCategoryId() > 0 {
			query = query.Where("category_id = ?", req.GetCategoryId())
		}
		if req.GetBrandId() > 0 {
			query = query.Where("brand_id = ?", req.GetBrandId())
		}
		if st := strings.TrimSpace(req.GetStatus()); st != "" {
			query = query.Where("status = ?", st)
		}
		return query
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []gormSPU
	if err := base().
		Preload("Skus", func(db *gorm.DB) *gorm.DB { return db.Order("id") }).
		Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort, id") }).
		Preload("Attributes", func(db *gorm.DB) *gorm.DB { return db.Order("sort, id") }).
		Order("id DESC").
		Limit(int(req.GetPageSize())).Offset(int((req.GetPage() - 1) * req.GetPageSize())).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := &pb.ProductList{Total: total, Products: make([]*pb.Product, 0, len(rows))}
	for i := range rows {
		out.Products = append(out.Products, rows[i].toPB())
	}
	return out, nil
}

func (row *gormSPU) toPB() *pb.Product {
	p := &pb.Product{
		Id: row.ID, CategoryId: row.CategoryID, BrandId: row.BrandID,
		Name: row.Name, Subtitle: row.Subtitle, Description: row.Description,
		MainImage: row.MainImage, Status: row.Status, MinPriceCent: row.MinPriceCent,
	}
	for i := range row.Skus {
		sku := row.Skus[i].toPB()
		if sku.Status == "ENABLED" && sku.PriceCent > 0 && (p.MinPriceCent == 0 || sku.PriceCent < p.MinPriceCent) {
			p.MinPriceCent = sku.PriceCent
		}
		p.Skus = append(p.Skus, sku)
	}
	for i := range row.Images {
		p.Images = append(p.Images, &pb.ProductImage{Id: row.Images[i].ID, Url: row.Images[i].URL, Sort: row.Images[i].Sort})
	}
	for i := range row.Attributes {
		p.Attributes = append(p.Attributes, &pb.ProductAttribute{Id: row.Attributes[i].ID, Name: row.Attributes[i].Name, Value: row.Attributes[i].Value, Sort: row.Attributes[i].Sort})
	}
	return p
}

func (row *gormSKU) toPB() *pb.Sku {
	return &pb.Sku{
		Id: row.ID, SpuId: row.SpuID, SkuCode: row.SkuCode, SpecsJson: row.SpecsJSON,
		PriceCent: row.PriceCent, Stock: row.Stock, Status: row.Status, ImageUrl: row.ImageURL,
	}
}
