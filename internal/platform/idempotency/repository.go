package idempotency

import (
	"context"
	"strings"
	"time"

	repositorytx "github.com/freeDog-wy/go-backend-template/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repository 以 PostgreSQL 唯一约束实现 HTTP 写请求的幂等记录读写。
type Repository struct {
	db *gorm.DB
}

// Record 表示已领取的幂等请求及其可重放响应。
type Record struct {
	ID           uint
	RequestHash  string
	ResponseBody []byte
	StatusCode   int
	CompletedAt  *time.Time
}

// Store 原子地领取幂等键并保存首次请求的响应。
//
// 幂等命名空间由 actor、method、route 和 key 组成；requestHash 用于拒绝同一 key
// 对应不同请求体的情况。Claim 返回 claimed=false 时，调用方必须根据记录状态重放、
// 拒绝或报告处理中，而不能再次执行业务逻辑。
type Store interface {
	// Claim 返回已创建的记录和 true，或返回既有记录和 false。
	Claim(context.Context, uint, string, string, string, string) (*Record, bool, error)
	// Complete 只完成尚未完成的记录，保存原始响应以供同一请求重放。
	Complete(context.Context, uint, []byte, int) error
}

var _ Store = (*Repository)(nil)

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Claim 以 actor、method、route 和 key 创建记录；冲突时返回原记录而不覆盖请求哈希。
func (r *Repository) Claim(ctx context.Context, actorID uint, method, route, key, requestHash string) (*Record, bool, error) {
	candidate := recordModel{ActorUserID: actorID, Method: method, Route: route, Key: key, RequestHash: requestHash}
	db := repositorytx.DB(ctx, r.db).WithContext(ctx)
	result := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&candidate)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 1 {
		return recordFromModel(candidate), true, nil
	}

	var existing recordModel
	if err := db.Where("actor_user_id = ? AND method = ? AND route = ? AND key = ?", actorID, method, route, key).First(&existing).Error; err != nil {
		return nil, false, err
	}
	return recordFromModel(existing), false, nil
}

// Complete 只保存首次完成的响应，避免并发或重试覆盖已可重放的结果。
func (r *Repository) Complete(ctx context.Context, id uint, body []byte, statusCode int) error {
	now := time.Now().UTC()
	return repositorytx.DB(ctx, r.db).WithContext(ctx).
		Model(&recordModel{}).
		Where("id = ? AND completed_at IS NULL", id).
		Updates(map[string]any{"response_body": body, "status_code": statusCode, "completed_at": now}).Error
}

func recordFromModel(value recordModel) *Record {
	return &Record{
		ID:           value.ID,
		RequestHash:  strings.TrimSpace(value.RequestHash),
		ResponseBody: value.ResponseBody,
		StatusCode:   value.StatusCode,
		CompletedAt:  value.CompletedAt,
	}
}
