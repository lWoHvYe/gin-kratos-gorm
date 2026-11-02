package cart

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	pb "gin-kratos-gorm/api/gen/mall/v1"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// cartGorm is the read-only cart path. Cart mutations stay on database/sql so
// the cart row, item rows and outbox event share one explicit *sql.Tx.
type cartGorm struct {
	db *gorm.DB
}

type cartRow struct {
	ID     int64         `gorm:"column:id;primaryKey"`
	UserID int64         `gorm:"column:user_id"`
	Status string        `gorm:"column:status"`
	Items  []cartItemRow `gorm:"foreignKey:CartID"`
}

func (cartRow) TableName() string { return "carts" }

type cartItemRow struct {
	ID        int64  `gorm:"column:id;primaryKey"`
	CartID    int64  `gorm:"column:cart_id"`
	SkuID     int64  `gorm:"column:sku_id"`
	Name      string `gorm:"column:name"`
	PriceCent int64  `gorm:"column:price_cent"`
	Quantity  int32  `gorm:"column:quantity"`
	Selected  bool   `gorm:"column:selected"`
}

func (cartItemRow) TableName() string { return "cart_items" }

func newCartGorm(sqlDB *sql.DB) (*cartGorm, error) {
	gdb, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		return nil, fmt.Errorf("open gorm cart reader: %w", err)
	}
	return &cartGorm{db: gdb}, nil
}

func (g *cartGorm) GetCart(ctx context.Context, userID int64) (*pb.Cart, error) {
	var row cartRow
	err := g.db.WithContext(ctx).Preload("Items").Where("user_id = ?", userID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &pb.Cart{UserId: userID}, nil
	}
	if err != nil {
		return nil, err
	}
	items := make([]*pb.CartItem, 0, len(row.Items))
	for i := range row.Items {
		items = append(items, &pb.CartItem{
			SkuId: row.Items[i].SkuID, Name: row.Items[i].Name,
			PriceCent: row.Items[i].PriceCent, Quantity: row.Items[i].Quantity,
			Selected: row.Items[i].Selected,
		})
	}
	return &pb.Cart{UserId: userID, Items: items, TotalCent: totalCent(items)}, nil
}
