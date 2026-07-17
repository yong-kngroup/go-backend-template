package idempotency

import (
	"context"
	"strings"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/infra/database"
	infraIdempotency "github.com/freeDog-wy/go-backend-template/internal/infra/idempotency"
	modelIdempotency "github.com/freeDog-wy/go-backend-template/internal/model/idempotency"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repository 以 PostgreSQL 唯一约束实现 HTTP 写请求的幂等记录读写。
type Repository struct {
	db *gorm.DB
}

var _ infraIdempotency.Store = (*Repository)(nil)

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Claim 以 actor、method、route 和 key 创建记录；冲突时返回原记录而不覆盖请求哈希。
func (r *Repository) Claim(ctx context.Context, actorID uint, method, route, key, requestHash string) (*infraIdempotency.Record, bool, error) {
	candidate := modelIdempotency.Record{ActorUserID: actorID, Method: method, Route: route, Key: key, RequestHash: requestHash}
	db := database.DB(ctx, r.db).WithContext(ctx)
	result := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&candidate)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 1 {
		return recordFromModel(candidate), true, nil
	}

	var existing modelIdempotency.Record
	if err := db.Where("actor_user_id = ? AND method = ? AND route = ? AND key = ?", actorID, method, route, key).First(&existing).Error; err != nil {
		return nil, false, err
	}
	return recordFromModel(existing), false, nil
}

// Complete 只保存首次完成的响应，避免并发或重试覆盖已可重放的结果。
func (r *Repository) Complete(ctx context.Context, id uint, body []byte, statusCode int) error {
	now := time.Now().UTC()
	return database.DB(ctx, r.db).WithContext(ctx).
		Model(&modelIdempotency.Record{}).
		Where("id = ? AND completed_at IS NULL", id).
		Updates(map[string]any{"response_body": body, "status_code": statusCode, "completed_at": now}).Error
}

func recordFromModel(value modelIdempotency.Record) *infraIdempotency.Record {
	return &infraIdempotency.Record{
		ID:           value.ID,
		RequestHash:  strings.TrimSpace(value.RequestHash),
		ResponseBody: value.ResponseBody,
		StatusCode:   value.StatusCode,
		CompletedAt:  value.CompletedAt,
	}
}
