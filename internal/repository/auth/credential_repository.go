package auth

import (
	"context"
	"errors"

	domainAuth "github.com/freeDog-wy/go-backend-template/internal/domain/auth"
	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	modelAuth "github.com/freeDog-wy/go-backend-template/internal/model/auth"
	repositorytx "github.com/freeDog-wy/go-backend-template/internal/repository"

	"gorm.io/gorm"
)

// CredentialRepository 保存用户密码凭据。
type CredentialRepository struct {
	db *gorm.DB
}

var _ domainAuth.CredentialRepository = (*CredentialRepository)(nil)

func New(db *gorm.DB) *CredentialRepository {
	return &CredentialRepository{db: db}
}

func (r *CredentialRepository) g(ctx context.Context) gorm.Interface[modelAuth.UserCredential] {
	return gorm.G[modelAuth.UserCredential](repositorytx.DB(ctx, r.db))
}

func (r *CredentialRepository) Create(ctx context.Context, credential *domainAuth.UserCredential) error {
	return r.g(ctx).Create(ctx, modelAuth.FromEntity(credential))
}

func (r *CredentialRepository) FindByUserID(ctx context.Context, userID uint) (*domainAuth.UserCredential, error) {
	m, err := r.g(ctx).Where("user_id = ?", userID).First(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, shared.ErrNotFound
		}
		return nil, err
	}
	return m.ToEntity(), nil
}

func (r *CredentialRepository) Update(ctx context.Context, credential *domainAuth.UserCredential) error {
	m := modelAuth.FromEntity(credential)
	return repositorytx.DB(ctx, r.db).
		Model(&modelAuth.UserCredential{}).
		Where("user_id = ?", m.UserID).
		Updates(map[string]any{
			"password_hash":       m.PasswordHash,
			"password_changed_at": m.PasswordChangedAt,
			"updated_at":          m.UpdatedAt,
		}).Error
}
