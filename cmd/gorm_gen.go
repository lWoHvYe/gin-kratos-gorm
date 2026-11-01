// Command gorm_gen regenerates GORM query/model code from an existing MySQL
// schema. It is a development tool; the application does not run it at startup.
//
// Usage:
//
//	GORM_GEN_DSN='root:root@tcp(127.0.0.1:3306)/gin_kratos?parseTime=True&loc=Local' go run cmd/gorm_gen.go
package main

import (
	"fmt"
	"os"

	"gorm.io/driver/mysql"
	"gorm.io/gen"
	"gorm.io/gen/field"
	"gorm.io/gorm"
)

func main() {
	dsn := os.Getenv("GORM_GEN_DSN")
	if dsn == "" {
		dsn = "root:root@tcp(127.0.0.1:3306)/gin_kratos?parseTime=True&loc=Local"
	}
	db, err := gorm.Open(mysql.Open(dsn))
	if err != nil {
		panic(fmt.Sprintf("connect database: %v", err))
	}

	g := gen.NewGenerator(gen.Config{
		OutPath:           "internal/biz/query",
		ModelPkgPath:      "internal/biz/model",
		Mode:              gen.WithDefaultQuery | gen.WithQueryInterface,
		FieldWithTypeTag:  true,
		FieldWithIndexTag: true,
	})
	g.UseDB(db)

	user := g.GenerateModel("users")
	role := g.GenerateModel("roles")
	userRole := g.GenerateModel("user_roles",
		gen.FieldRelate(field.BelongsTo, "User", user, &field.RelateConfig{}),
		gen.FieldRelate(field.BelongsTo, "Role", role, &field.RelateConfig{}),
	)

	sku := g.GenerateModel("product_skus")
	spu := g.GenerateModel("product_spus",
		gen.FieldRelate(field.HasMany, "Skus", sku, &field.RelateConfig{GORMTag: field.GormTag{"foreignKey": []string{"SpuID"}}}),
	)

	orderItem := g.GenerateModel("order_items")
	order := g.GenerateModel("orders",
		gen.FieldRelate(field.HasMany, "OrderItems", orderItem, &field.RelateConfig{GORMTag: field.GormTag{"foreignKey": []string{"OrderID"}}}),
	)

	stockLog := g.GenerateModel("stock_logs")
	outbox := g.GenerateModel("outbox_events")

	g.ApplyBasic(user, role, userRole, spu, sku, order, orderItem, stockLog, outbox)
	g.Execute()
}
