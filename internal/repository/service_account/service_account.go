package serviceaccount

import (
	"context"
	"errors"

	domainServiceAccount "github.com/freeDog-wy/go-backend-template/internal/domain/service_account"
	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	modelServiceAccount "github.com/freeDog-wy/go-backend-template/internal/model/service_account"
	repositorytx "github.com/freeDog-wy/go-backend-template/internal/repository"
	"gorm.io/gorm"
)

type ServiceAccountRepository struct{ db *gorm.DB }

var _ domainServiceAccount.Repository = (*ServiceAccountRepository)(nil)

func New(db *gorm.DB) *ServiceAccountRepository {
	return &ServiceAccountRepository{db: db}
}

func (r *ServiceAccountRepository) Create(ctx context.Context, account *domainServiceAccount.ServiceAccount) error {
	model := modelServiceAccount.FromEntity(account)
	if err := repositorytx.DB(ctx, r.db).Create(model).Error; err != nil {
		return err
	}
	account.AssignID(model.ID)
	return nil
}

func (r *ServiceAccountRepository) Update(ctx context.Context, account *domainServiceAccount.ServiceAccount) error {
	return repositorytx.DB(ctx, r.db).Save(modelServiceAccount.FromEntity(account)).Error
}

func (r *ServiceAccountRepository) FindByClientID(ctx context.Context, clientID string) (*domainServiceAccount.ServiceAccount, error) {
	return r.find(ctx, "client_id = ?", clientID)
}

func (r *ServiceAccountRepository) FindByUserID(ctx context.Context, userID uint) (*domainServiceAccount.ServiceAccount, error) {
	return r.find(ctx, "user_id = ?", userID)
}

func (r *ServiceAccountRepository) find(ctx context.Context, query string, value any) (*domainServiceAccount.ServiceAccount, error) {
	var model modelServiceAccount.ServiceAccount
	if err := repositorytx.DB(ctx, r.db).Where(query, value).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, shared.ErrNotFound
		}
		return nil, err
	}
	return model.ToEntity(), nil
}
