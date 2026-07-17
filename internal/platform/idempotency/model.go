package idempotency

import "time"

// recordModel 是 idempotency_records 表对应的组件内部 GORM 模型。
type recordModel struct {
	ID           uint   `gorm:"primaryKey"`
	ActorUserID  uint   `gorm:"not null;uniqueIndex:uk_idempotency_records_request"`
	Method       string `gorm:"type:varchar(10);not null;uniqueIndex:uk_idempotency_records_request"`
	Route        string `gorm:"type:varchar(255);not null;uniqueIndex:uk_idempotency_records_request"`
	Key          string `gorm:"type:varchar(200);not null;uniqueIndex:uk_idempotency_records_request"`
	RequestHash  string `gorm:"type:char(64);not null"`
	ResponseBody []byte `gorm:"type:bytea"`
	StatusCode   int    `gorm:"not null;default:200"`
	CompletedAt  *time.Time
	CreatedAt    time.Time `gorm:"not null"`
}

func (recordModel) TableName() string { return "idempotency_records" }
