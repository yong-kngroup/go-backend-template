package audit

import (
	"context"

	domainAudit "github.com/freeDog-wy/go-backend-template/internal/domain/audit"
	"github.com/freeDog-wy/go-backend-template/internal/infra/mq"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
)

type Consumer struct {
	repo   domainAudit.Repository
	logger logger.Logger
}

func NewConsumer(repo domainAudit.Repository, logger logger.Logger) *Consumer {
	return &Consumer{
		repo:   repo,
		logger: logger,
	}
}

func (c *Consumer) OnLogRequested(ctx context.Context, evt domainAudit.LogRequested) error {
	log, err := domainAudit.NewAuditLog(
		evt.ActorUserID,
		evt.TargetType,
		evt.TargetID,
		evt.Action,
		evt.Result,
		evt.IP,
		evt.UserAgent,
		mq.TraceIDFromContext(ctx),
		evt.Metadata,
	)
	if err != nil {
		return err
	}

	if err := c.repo.Create(ctx, log); err != nil {
		if c.logger != nil {
			c.logger.Error("audit log persist failed", "action", evt.Action, "error", err)
		}
		return err
	}
	return nil
}
