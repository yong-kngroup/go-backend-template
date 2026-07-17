package outbox

import (
	"context"
	"time"

	repositorytx "github.com/freeDog-wy/go-backend-template/internal/repository"

	"gorm.io/gorm"
)

// Store 定义 Outbox 事件的组件内存储契约。
type Store interface {
	Create(ctx context.Context, events ...*Event) error
	ListUnpublished(ctx context.Context, limit int) ([]*Event, error)
	MarkPublished(ctx context.Context, ids []uint, publishedAt time.Time) error
}

// Repository 基于 GORM 实现 Outbox 本地消息表读写。
type Repository struct {
	db *gorm.DB
}

var _ Store = (*Repository)(nil)

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Create 在当前事务上下文内批量写入待发布事件。
func (r *Repository) Create(ctx context.Context, events ...*Event) error {
	if len(events) == 0 {
		return nil
	}

	models := make([]*eventModel, 0, len(events))
	for _, event := range events {
		models = append(models, eventModelFromEvent(event))
	}

	return repositorytx.DB(ctx, r.db).Create(&models).Error
}

// ListUnpublished 按主键顺序抓取一批尚未投递的事件，供 cron publisher 扫描。
func (r *Repository) ListUnpublished(ctx context.Context, limit int) ([]*Event, error) {
	if limit <= 0 {
		limit = 100
	}

	var models []*eventModel
	if err := repositorytx.DB(ctx, r.db).
		Where("published_at IS NULL").
		Order("id ASC").
		Limit(limit).
		Find(&models).Error; err != nil {
		return nil, err
	}

	events := make([]*Event, 0, len(models))
	for _, model := range models {
		events = append(events, model.toEvent())
	}
	return events, nil
}

// MarkPublished 只更新仍未发布的记录，避免重复覆盖状态。
func (r *Repository) MarkPublished(ctx context.Context, ids []uint, publishedAt time.Time) error {
	if len(ids) == 0 {
		return nil
	}

	return repositorytx.DB(ctx, r.db).
		Model(&eventModel{}).
		Where("id IN ? AND published_at IS NULL", ids).
		Update("published_at", publishedAt).Error
}
