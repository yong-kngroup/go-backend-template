package verification

import (
	"context"
	"time"

	domainVerification "github.com/freeDog-wy/go-backend-template/internal/domain/verification"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
)

type Cron struct {
	repo   domainVerification.Repository
	logger logger.Logger
}

func NewCron(repo domainVerification.Repository, logger logger.Logger) *Cron {
	return &Cron{
		repo:   repo,
		logger: logger,
	}
}

func (c *Cron) CleanupExpiredTokens(ctx context.Context) error {
	now := time.Now()

	emailDeleted, err := c.repo.DeleteExpiredEmailVerificationTokens(ctx, now)
	if err != nil {
		return err
	}

	passwordResetDeleted, err := c.repo.DeleteExpiredPasswordResetTokens(ctx, now)
	if err != nil {
		return err
	}

	if c.logger != nil {
		c.logger.Info(
			"verification token cleanup completed",
			"email_verification_deleted",
			emailDeleted,
			"password_reset_deleted",
			passwordResetDeleted,
		)
	}

	return nil
}
