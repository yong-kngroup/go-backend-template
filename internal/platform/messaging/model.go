package messaging

import "time"

// RecordModel 是 message_consumptions 表对应的组件内部 GORM 模型。
type RecordModel struct {
	ID            uint       `gorm:"primaryKey"`
	ConsumerGroup string     `gorm:"size:191;not null;uniqueIndex:uk_consumer_group_message_key"`
	MessageKey    string     `gorm:"size:191;not null;uniqueIndex:uk_consumer_group_message_key"`
	EventName     string     `gorm:"size:191;index;not null"`
	TraceID       string     `gorm:"size:64;index"`
	Status        string     `gorm:"size:32;index;not null"`
	AttemptCount  int        `gorm:"not null;default:1"`
	LastError     string     `gorm:"type:text"`
	LockedUntil   *time.Time `gorm:"index"`
	ProcessedAt   *time.Time `gorm:"index"`
	CreatedAt     time.Time  `gorm:"index;not null"`
	UpdatedAt     time.Time  `gorm:"index;not null"`
}

func (RecordModel) TableName() string {
	return "message_consumptions"
}
