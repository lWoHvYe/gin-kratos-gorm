package model

import "time"

const TableNameOutboxEvent = "outbox_events"

const (
	OutboxStatusPending   = "pending"
	OutboxStatusPublished = "published"
	OutboxStatusFailed    = "failed"
)

// OutboxEvent implements the transactional outbox pattern. Domain changes and
// the event are written in the same transaction; a background dispatcher then
// delivers the event at-least-once.
type OutboxEvent struct {
	ID            uint64     `gorm:"column:id;;primaryKey;autoIncrement:true" json:"id"`
	EventID       string     `gorm:"column:event_id;type:varchar(64);uniqueIndex:idx_outbox_event_id,priority:1" json:"event_id"`
	AggregateType string     `gorm:"column:aggregate_type;type:varchar(32);index:idx_outbox_aggregate,priority:1" json:"aggregate_type"`
	AggregateID   string     `gorm:"column:aggregate_id;type:varchar(64);index:idx_outbox_aggregate,priority:2" json:"aggregate_id"`
	EventType     string     `gorm:"column:event_type;type:varchar(64);index:idx_outbox_event_type,priority:1" json:"event_type"`
	Payload       string     `gorm:"column:payload;type:text" json:"payload"`
	Status        string     `gorm:"column:status;type:varchar(16);index:idx_outbox_status,priority:1" json:"status"`
	Attempts      int32      `gorm:"column:attempts;type:int;not null;default:0" json:"attempts"`
	LastError     string     `gorm:"column:last_error;type:varchar(1024)" json:"last_error"`
	AvailableAt   time.Time  `gorm:"column:available_at;index:idx_outbox_available_at,priority:1" json:"available_at"`
	CreatedAt     time.Time  `gorm:"column:created_at;" json:"created_at"`
	PublishedAt   *time.Time `gorm:"column:published_at;" json:"published_at"`
}

// TableName OutboxEvent's table name
func (*OutboxEvent) TableName() string {
	return TableNameOutboxEvent
}
