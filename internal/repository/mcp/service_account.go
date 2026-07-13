package mcp

import (
	"context"
	"errors"

	domainMCP "github.com/freeDog-wy/go-backend-template/internal/domain/mcp"
	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	"github.com/freeDog-wy/go-backend-template/internal/infra/database"
	modelMCP "github.com/freeDog-wy/go-backend-template/internal/model/mcp"
	"gorm.io/gorm"
)

type ServiceAccountRepository struct{ db *gorm.DB }

var _ domainMCP.ServiceAccountRepository = (*ServiceAccountRepository)(nil)

func NewServiceAccountRepository(db *gorm.DB) *ServiceAccountRepository {
	return &ServiceAccountRepository{db: db}
}

func (r *ServiceAccountRepository) Create(ctx context.Context, account *domainMCP.ServiceAccount) error {
	model := modelMCP.FromEntity(account)
	if err := database.DB(ctx, r.db).Create(model).Error; err != nil {
		return err
	}
	account.AssignID(model.ID)
	return nil
}

func (r *ServiceAccountRepository) Update(ctx context.Context, account *domainMCP.ServiceAccount) error {
	return database.DB(ctx, r.db).Save(modelMCP.FromEntity(account)).Error
}

func (r *ServiceAccountRepository) FindByClientID(ctx context.Context, clientID string) (*domainMCP.ServiceAccount, error) {
	return r.find(ctx, "client_id = ?", clientID)
}

func (r *ServiceAccountRepository) FindByUserID(ctx context.Context, userID uint) (*domainMCP.ServiceAccount, error) {
	return r.find(ctx, "user_id = ?", userID)
}

func (r *ServiceAccountRepository) find(ctx context.Context, query string, value any) (*domainMCP.ServiceAccount, error) {
	var model modelMCP.ServiceAccount
	if err := database.DB(ctx, r.db).Where(query, value).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, shared.ErrNotFound
		}
		return nil, err
	}
	return model.ToEntity(), nil
}
