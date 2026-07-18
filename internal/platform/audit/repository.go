package audit

import (
	"context"

	repositorytx "github.com/freeDog-wy/go-backend-template/internal/repository"

	"gorm.io/gorm"
)

// Writer persists audit records.
type Writer interface {
	Create(ctx context.Context, log *AuditLog) error
}

// Repository persists audit records with GORM.
type Repository struct {
	db *gorm.DB
}

var _ Writer = (*Repository)(nil)

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) g(ctx context.Context) gorm.Interface[logModel] {
	return gorm.G[logModel](repositorytx.DB(ctx, r.db))
}

func (r *Repository) Create(ctx context.Context, log *AuditLog) error {
	return r.g(ctx).Create(ctx, logModelFromLog(log))
}
