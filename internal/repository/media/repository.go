package media

import (
	"context"
	"errors"
	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	"github.com/freeDog-wy/go-backend-template/internal/infra/database"
	model "github.com/freeDog-wy/go-backend-template/internal/model/media"
	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) *Repository { return &Repository{db: db} }
func (r *Repository) Create(ctx context.Context, a *model.Asset) error {
	return database.DB(ctx, r.db).Create(a).Error
}
func (r *Repository) Find(ctx context.Context, id uint) (*model.Asset, error) {
	var a model.Asset
	if err := database.DB(ctx, r.db).Where("deleted_at IS NULL").First(&a, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, shared.ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}
func (r *Repository) MarkReady(ctx context.Context, id uint, mime string, size int64) error {
	res := database.DB(ctx, r.db).Model(&model.Asset{}).Where("id = ? AND status = 'pending'", id).Updates(map[string]any{"status": "ready", "mime_type": mime, "size_bytes": size})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return shared.ErrNotFound
	}
	return nil
}
func (r *Repository) MarkFailed(ctx context.Context, id uint) error {
	return database.DB(ctx, r.db).Model(&model.Asset{}).Where("id = ?", id).Update("status", "failed").Error
}
