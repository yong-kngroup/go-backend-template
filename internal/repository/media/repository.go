package media

import (
	"context"
	"errors"
	domainMedia "github.com/freeDog-wy/go-backend-template/internal/domain/media"
	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	"github.com/freeDog-wy/go-backend-template/internal/infra/database"
	model "github.com/freeDog-wy/go-backend-template/internal/model/media"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
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
func (r *Repository) List(ctx context.Context, limit, offset int) ([]model.Asset, int64, error) {
	var total int64
	db := database.DB(ctx, r.db).Model(&model.Asset{}).Where("deleted_at IS NULL")
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var assets []model.Asset
	if err := db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&assets).Error; err != nil {
		return nil, 0, err
	}
	return assets, total, nil
}

func (r *Repository) ListReadyPublic(ctx context.Context, locale string, ids []uint) ([]domainMedia.PublicAsset, error) {
	if len(ids) == 0 {
		return []domainMedia.PublicAsset{}, nil
	}
	type row struct {
		ID        uint
		ObjectKey string
		AltText   string
		Title     string
	}
	var rows []row
	err := database.DB(ctx, r.db).Table("media_assets").
		Select("media_assets.id, media_assets.object_key, COALESCE(media_translations.alt_text, '') AS alt_text, COALESCE(media_translations.title, '') AS title").
		Joins("LEFT JOIN media_translations ON media_translations.media_id = media_assets.id AND media_translations.locale = ?", locale).
		Where("media_assets.id IN ? AND media_assets.status = 'ready' AND media_assets.deleted_at IS NULL", ids).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	assets := make([]domainMedia.PublicAsset, 0, len(rows))
	for _, row := range rows {
		assets = append(assets, domainMedia.PublicAsset{ID: row.ID, ObjectKey: row.ObjectKey, AltText: row.AltText, Title: row.Title})
	}
	return assets, nil
}
func (r *Repository) UpsertTranslation(ctx context.Context, t *model.Translation) error {
	return database.DB(ctx, r.db).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "media_id"}, {Name: "locale"}}, DoUpdates: clause.Assignments(map[string]any{"alt_text": t.AltText, "title": t.Title, "updated_at": time.Now()})}).Create(t).Error
}
