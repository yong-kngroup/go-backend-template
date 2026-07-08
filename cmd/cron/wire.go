package main

import (
	"context"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/config"
	"github.com/freeDog-wy/go-backend-template/internal/infra/database"
	"github.com/freeDog-wy/go-backend-template/internal/infra/logging"
	RepoVerification "github.com/freeDog-wy/go-backend-template/internal/repository/verification"
	UsecaseVerification "github.com/freeDog-wy/go-backend-template/internal/usecase/verification"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
	"github.com/freeDog-wy/go-backend-template/pkg/scheduler"
)

type CronApp struct {
	enabled bool
	logger  logger.Logger
	runner  *scheduler.Runner
}

func (a *CronApp) Run(ctx context.Context) error {
	if !a.enabled {
		a.logger.Info("cron is disabled by configuration")
		<-ctx.Done()
		return ctx.Err()
	}

	return a.runner.Run(ctx)
}

func initCronApp(cfg *config.Config) *CronApp {
	appLogger := logging.Init(cfg.App.Mode)
	runner := scheduler.New(appLogger)
	if cfg.Cron.Enabled {
		if cfg.Cron.VerificationCleanupIntervalSeconds <= 0 {
			panic("cron.verification_cleanup_interval_seconds must be greater than zero")
		}

		db := database.NewPostgresDB(cfg.Database.DSN)
		verificationRepo := RepoVerification.New(db)
		verificationCron := UsecaseVerification.NewCron(verificationRepo, appLogger)

		if err := runner.Register(scheduler.Job{
			Name:     "verification.cleanup_expired_tokens",
			Interval: time.Duration(cfg.Cron.VerificationCleanupIntervalSeconds) * time.Second,
			Run:      verificationCron.CleanupExpiredTokens,
		}); err != nil {
			panic("failed to register verification cleanup job: " + err.Error())
		}
	}

	return &CronApp{
		enabled: cfg.Cron.Enabled,
		logger:  appLogger,
		runner:  runner,
	}
}
