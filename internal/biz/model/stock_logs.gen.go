package model

import "time"

const TableNameStockLog = "stock_logs"

// StockLog is an idempotency record for stock mutations. The primary key is
// deterministic ("deduct:<order_sn>:<sku_id>" / "restore:<order_sn>:<sku_id>")
// so duplicate requests from retries or DTM compensation are no-ops.
type StockLog struct {
	ID        string    `gorm:"column:id;type:varchar(128);primaryKey;comment:幂等ID" json:"id"`
	OrderSn   string    `gorm:"column:order_sn;type:varchar(64);index:idx_stock_logs_order_sn,priority:1" json:"order_sn"`
	SkuID     int64     `gorm:"column:sku_id;type:bigint;not null;index:idx_stock_logs_sku_id,priority:1" json:"sku_id"`
	Num       int32     `gorm:"column:num;type:int;not null" json:"num"`
	Action    string    `gorm:"column:action;type:varchar(16);not null" json:"action"`
	CreatedAt time.Time `gorm:"column:created_at;" json:"created_at"`
}

// TableName StockLog's table name
func (*StockLog) TableName() string {
	return TableNameStockLog
}
