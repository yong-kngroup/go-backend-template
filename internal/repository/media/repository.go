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
func (r *Repository) SetUploadExpiresAt(ctx context.Context, id uint, expiresAt time.Time) error {
	res := database.DB(ctx, r.db).Model(&model.Asset{}).Where("id = ? AND status = 'pending'", id).Update("upload_expires_at", expiresAt)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return shared.ErrNotFound
	}
	return nil
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
func (r *Repository) MarkReady(ctx context.Context, id uint, mime string, size int64, width, height int, now time.Time) error {
	res := database.DB(ctx, r.db).Model(&model.Asset{}).Where("id = ? AND status = 'pending' AND (upload_expires_at IS NULL OR upload_expires_at > ?)", id, now).Updates(map[string]any{"status": "ready", "mime_type": mime, "size_bytes": size, "width": width, "height": height})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return shared.ErrNotFound
	}
	return nil
}
func (r *Repository) MarkExpired(ctx context.Context, id uint, now time.Time) error {
	return database.DB(ctx, r.db).Model(&model.Asset{}).Where("id = ? AND status = 'pending' AND upload_expires_at <= ?", id, now).Update("status", "expired").Error
}
func (r *Repository) ClaimExpired(ctx context.Context, now, retryBefore time.Time, batchSize int) ([]model.Asset, error) {
	if batchSize <= 0 {
		return []model.Asset{}, nil
	}
	assets := make([]model.Asset, 0, batchSize)
	err := database.DB(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("(status = 'pending' AND upload_expires_at <= ?) OR (status = 'expired' AND (cleanup_claimed_at IS NULL OR cleanup_claimed_at <= ?))", now, retryBefore).Order("upload_expires_at NULLS FIRST, id").Limit(batchSize).Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Find(&assets).Error; err != nil {
			return err
		}
		if len(assets) == 0 {
			return nil
		}
		ids := make([]uint, 0, len(assets))
		for _, asset := range assets {
			ids = append(ids, asset.ID)
		}
		return tx.Model(&model.Asset{}).Where("id IN ? AND status IN ('pending', 'expired')", ids).Updates(map[string]any{"status": "expired", "cleanup_claimed_at": now}).Error
	})
	return assets, err
}
func (r *Repository) MarkDeleted(ctx context.Context, id uint, now time.Time) error {
	res := database.DB(ctx, r.db).Model(&model.Asset{}).Where("id = ? AND status = 'expired'", id).Updates(map[string]any{"status": "deleted", "deleted_at": now, "cleanup_last_error": "", "cleanup_claimed_at": nil})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return shared.ErrNotFound
	}
	return nil
}
func (r *Repository) RecordCleanupFailure(ctx context.Context, id uint, message string) error {
	return database.DB(ctx, r.db).Model(&model.Asset{}).Where("id = ? AND status = 'expired'", id).Updates(map[string]any{"cleanup_attempts": gorm.Expr("cleanup_attempts + 1"), "cleanup_last_error": message, "cleanup_claimed_at": nil}).Error
}
func (r *Repository) MarkFailed(ctx context.Context, id uint) error {
	res := database.DB(ctx, r.db).Model(&model.Asset{}).Where("id = ? AND status = 'pending'", id).Update("status", "failed")
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return shared.ErrNotFound
	}
	return nil
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
