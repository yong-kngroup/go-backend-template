package idempotency

import (
	"context"
	"time"

	handlerMiddleware "github.com/freeDog-wy/go-backend-template/internal/handler/middleware"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type record struct {
	ID           uint   `gorm:"primaryKey"`
	ActorUserID  uint   `gorm:"not null;uniqueIndex:idx_idempotency_request"`
	Method       string `gorm:"type:varchar(10);not null;uniqueIndex:idx_idempotency_request"`
	Route        string `gorm:"type:varchar(255);not null;uniqueIndex:idx_idempotency_request"`
	Key          string `gorm:"type:varchar(200);not null;uniqueIndex:idx_idempotency_request"`
	RequestHash  string `gorm:"type:char(64);not null"`
	ResponseBody []byte `gorm:"type:bytea"`
	StatusCode   int
	CompletedAt  *time.Time
	CreatedAt    time.Time `gorm:"not null"`
}

func (record) TableName() string { return "idempotency_records" }

// Store 以数据库唯一约束实现 IdempotencyStore，允许多个应用实例安全竞争同一个键。
type Store struct{ db *gorm.DB }

func New(db *gorm.DB) *Store { return &Store{db: db} }

// Claim 以 actor、method、route 和 key 创建记录；冲突时返回原记录而不覆盖请求哈希。
func (s *Store) Claim(ctx context.Context, actorID uint, method, route, key, requestHash string) (*handlerMiddleware.IdempotencyRecord, bool, error) {
	candidate := record{ActorUserID: actorID, Method: method, Route: route, Key: key, RequestHash: requestHash}
	result := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&candidate)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 1 {
		return fromModel(candidate), true, nil
	}
	var existing record
	if err := s.db.WithContext(ctx).Where("actor_user_id = ? AND method = ? AND route = ? AND key = ?", actorID, method, route, key).First(&existing).Error; err != nil {
		return nil, false, err
	}
	return fromModel(existing), false, nil
}

// Complete 只保存首次完成的响应，避免并发或重试覆盖已可重放的结果。
func (s *Store) Complete(ctx context.Context, id uint, body []byte, statusCode int) error {
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Model(&record{}).Where("id = ? AND completed_at IS NULL", id).Updates(map[string]any{"response_body": body, "status_code": statusCode, "completed_at": now}).Error
}

func fromModel(value record) *handlerMiddleware.IdempotencyRecord {
	return &handlerMiddleware.IdempotencyRecord{ID: value.ID, RequestHash: value.RequestHash, ResponseBody: value.ResponseBody, StatusCode: value.StatusCode, CompletedAt: value.CompletedAt}
}

var _ handlerMiddleware.IdempotencyStore = (*Store)(nil)
