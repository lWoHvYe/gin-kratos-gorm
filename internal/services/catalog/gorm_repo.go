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
	ID          int64     `gorm:"column:id;primaryKey"`
	CategoryID  int64     `gorm:"column:category_id"`
	BrandID     int64     `gorm:"column:brand_id"`
	Name        string    `gorm:"column:name"`
	Subtitle    string    `gorm:"column:subtitle"`
	Description string    `gorm:"column:description"`
	MainImage   string    `gorm:"column:main_image"`
	Status      string    `gorm:"column:status"`
	Skus        []gormSKU `gorm:"foreignKey:SpuID"`
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
}

func (gormSKU) TableName() string { return "skus" }

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

func (g *gormReader) GetProduct(ctx context.Context, id int64) (*pb.Product, error) {
	var row gormSPU
	err := g.db.WithContext(ctx).Preload("Skus").First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
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

func (g *gormReader) ListProducts(ctx context.Context, req *pb.ListProductsRequest) (*pb.ProductList, error) {
	base := func() *gorm.DB {
		query := g.db.WithContext(ctx).Model(&gormSPU{}).Where("status = ?", "ON_SALE")
		if kw := strings.TrimSpace(req.GetKeyword()); kw != "" {
			like := "%" + kw + "%"
			query = query.Where("name LIKE ? OR subtitle LIKE ?", like, like)
		}
		if req.GetCategoryId() > 0 {
			query = query.Where("category_id = ?", req.GetCategoryId())
		}
		return query
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []gormSPU
	if err := base().Preload("Skus").Order("id DESC").
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
		MainImage: row.MainImage, Status: row.Status,
	}
	for i := range row.Skus {
		sku := row.Skus[i].toPB()
		if p.MinPriceCent == 0 || sku.PriceCent < p.MinPriceCent {
			p.MinPriceCent = sku.PriceCent
		}
		p.Skus = append(p.Skus, sku)
	}
	return p
}

func (row *gormSKU) toPB() *pb.Sku {
	return &pb.Sku{
		Id: row.ID, SpuId: row.SpuID, SkuCode: row.SkuCode, SpecsJson: row.SpecsJSON,
		PriceCent: row.PriceCent, Stock: row.Stock, Status: row.Status,
	}
}
