package verification

import (
	"context"
	"errors"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	domainVerification "github.com/freeDog-wy/go-backend-template/internal/domain/verification"
	"github.com/freeDog-wy/go-backend-template/internal/infra/database"
	modelVerification "github.com/freeDog-wy/go-backend-template/internal/model/verification"

	"gorm.io/gorm"
)

// Repository 实现 domain/verification.Repository。
type Repository struct {
	db *gorm.DB
}

var _ domainVerification.Repository = (*Repository)(nil)

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) g(ctx context.Context) gorm.Interface[modelVerification.EmailVerificationToken] {
	return gorm.G[modelVerification.EmailVerificationToken](database.DB(ctx, r.db))
}

func (r *Repository) passwordResetG(ctx context.Context) gorm.Interface[modelVerification.PasswordResetToken] {
	return gorm.G[modelVerification.PasswordResetToken](database.DB(ctx, r.db))
}

func (r *Repository) Create(ctx context.Context, token *domainVerification.EmailVerificationToken) error {
	m := modelVerification.EmailVerificationTokenFromEntity(token)
	return r.g(ctx).Create(ctx, m)
}

func (r *Repository) FindActiveByTokenHash(ctx context.Context, tokenHash string, now time.Time) (*domainVerification.EmailVerificationToken, error) {
	m, err := r.g(ctx).
		Where("token_hash = ? AND consumed_at IS NULL AND expires_at > ?", tokenHash, now).
		First(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, shared.ErrNotFound
		}
		return nil, err
	}
	return m.ToEntity(), nil
}

func (r *Repository) InvalidateByUserID(ctx context.Context, userID uint, now time.Time) error {
	return database.DB(ctx, r.db).
		Model(&modelVerification.EmailVerificationToken{}).
		Where("user_id = ? AND consumed_at IS NULL", userID).
		Update("consumed_at", now).Error
}

func (r *Repository) Update(ctx context.Context, token *domainVerification.EmailVerificationToken) error {
	m := modelVerification.EmailVerificationTokenFromEntity(token)
	return database.DB(ctx, r.db).
		Model(&modelVerification.EmailVerificationToken{}).
		Where("id = ?", m.ID).
		Updates(map[string]any{
			"consumed_at": m.ConsumedAt,
		}).Error
}

func (r *Repository) DeleteExpiredEmailVerificationTokens(ctx context.Context, now time.Time) (int64, error) {
	result := database.DB(ctx, r.db).
		Where("expires_at <= ?", now).
		Delete(&modelVerification.EmailVerificationToken{})
	return result.RowsAffected, result.Error
}

func (r *Repository) CreatePasswordReset(ctx context.Context, token *domainVerification.PasswordResetToken) error {
	m := modelVerification.PasswordResetTokenFromEntity(token)
	return r.passwordResetG(ctx).Create(ctx, m)
}

func (r *Repository) FindActivePasswordResetByTokenHash(ctx context.Context, tokenHash string, now time.Time) (*domainVerification.PasswordResetToken, error) {
	m, err := r.passwordResetG(ctx).
		Where("token_hash = ? AND consumed_at IS NULL AND expires_at > ?", tokenHash, now).
		First(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, shared.ErrNotFound
		}
		return nil, err
	}
	return m.ToEntity(), nil
}

func (r *Repository) InvalidatePasswordResetByUserID(ctx context.Context, userID uint, now time.Time) error {
	return database.DB(ctx, r.db).
		Model(&modelVerification.PasswordResetToken{}).
		Where("user_id = ? AND consumed_at IS NULL", userID).
		Update("consumed_at", now).Error
}

func (r *Repository) UpdatePasswordReset(ctx context.Context, token *domainVerification.PasswordResetToken) error {
	m := modelVerification.PasswordResetTokenFromEntity(token)
	return database.DB(ctx, r.db).
		Model(&modelVerification.PasswordResetToken{}).
		Where("id = ?", m.ID).
		Updates(map[string]any{
			"consumed_at": m.ConsumedAt,
		}).Error
}

func (r *Repository) DeleteExpiredPasswordResetTokens(ctx context.Context, now time.Time) (int64, error) {
	result := database.DB(ctx, r.db).
		Where("expires_at <= ?", now).
		Delete(&modelVerification.PasswordResetToken{})
	return result.RowsAffected, result.Error
}
