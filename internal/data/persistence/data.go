package persistence

import (
	"context"
	"time"

	"gin-kratos-gorm/internal/biz/query"
	"gin-kratos-gorm/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

type txKey struct{}

// Data owns the database handle. Repositories obtain a transaction-aware
// handle from the context, which keeps unit-of-work semantics without leaking
// GORM types into the domain layer.
type Data struct {
	db    *gorm.DB
	query *query.Query
	log   *log.Helper
}

func NewData(c *conf.Bootstrap, logger log.Logger, db *gorm.DB) (*Data, func(), error) {
	helper := log.NewHelper(logger)
	d := &Data{db: db, query: query.Use(db), log: helper}
	cleanup := func() {
		helper.Info("closing the data resources")
		if err := d.Close(); err != nil {
			helper.Errorf("close database: %v", err)
		}
	}
	return d, cleanup, nil
}

// InTx runs fn in a transaction. Nested calls reuse the existing transaction.
func (d *Data) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return fn(ctx)
	}
	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// DB returns the transaction bound to ctx, or the root connection.
func (d *Data) DB(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx.WithContext(ctx)
	}
	return d.db.WithContext(ctx)
}

// Q returns a generated query bound to the current transaction.
func (d *Data) Q(ctx context.Context) *query.Query {
	return query.Use(d.DB(ctx))
}

func (d *Data) Ping(ctx context.Context) error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return sqlDB.PingContext(pingCtx)
}

func (d *Data) Close() error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
