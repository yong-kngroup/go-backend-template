package main

import (
	"context"
	"strings"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/config"
	"github.com/freeDog-wy/go-backend-template/internal/infra/database"
	"github.com/freeDog-wy/go-backend-template/internal/infra/logging"
	"github.com/freeDog-wy/go-backend-template/internal/infra/mq"
	RepoOutbox "github.com/freeDog-wy/go-backend-template/internal/repository/outbox"
	RepoVerification "github.com/freeDog-wy/go-backend-template/internal/repository/verification"
	UsecaseMessaging "github.com/freeDog-wy/go-backend-template/internal/usecase/messaging"
	UsecaseSupport "github.com/freeDog-wy/go-backend-template/internal/usecase/support"
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
		if cfg.Cron.OutboxPublishIntervalSeconds <= 0 {
			panic("cron.outbox_publish_interval_seconds must be greater than zero")
		}
		if cfg.Cron.OutboxBatchSize <= 0 {
			panic("cron.outbox_batch_size must be greater than zero")
		}
		if cfg.Cron.VerificationCleanupIntervalSeconds <= 0 {
			panic("cron.verification_cleanup_interval_seconds must be greater than zero")
		}

		db := database.NewPostgresDB(cfg.Database.DSN)

		outboxRepo := RepoOutbox.New(db)
		publisher := mq.NewPublisherFromConfig(cfg, appLogger)
		outboxPublisher := UsecaseSupport.NewOutboxPublisher(
			outboxRepo,
			mq.NewOutboxPublisherAdapter(publisher),
			appLogger,
			cfg.Cron.OutboxBatchSize,
		)
		verificationRepo := RepoVerification.New(db)
		verificationCron := UsecaseVerification.NewCron(verificationRepo, appLogger)

		if err := runner.Register(scheduler.Job{
			Name:     "outbox.publish_pending_events",
			Interval: time.Duration(cfg.Cron.OutboxPublishIntervalSeconds) * time.Second,
			Run:      outboxPublisher.PublishPending,
		}); err != nil {
			panic("failed to register outbox publisher job: " + err.Error())
		}

		if err := runner.Register(scheduler.Job{
			Name:     "verification.cleanup_expired_tokens",
			Interval: time.Duration(cfg.Cron.VerificationCleanupIntervalSeconds) * time.Second,
			Run:      verificationCron.CleanupExpiredTokens,
		}); err != nil {
			panic("failed to register verification cleanup job: " + err.Error())
		}

		registerKafkaDLQJobs(cfg, appLogger, runner)
	}

	return &CronApp{
		enabled: cfg.Cron.Enabled,
		logger:  appLogger,
		runner:  runner,
	}
}

func registerKafkaDLQJobs(cfg *config.Config, appLogger logger.Logger, runner *scheduler.Runner) {
	if cfg.Cron.DLQInspectionEnabled {
		if cfg.Cron.DLQInspectionIntervalSeconds <= 0 {
			panic("cron.dlq_inspection_interval_seconds must be greater than zero")
		}
		if cfg.Cron.DLQInspectionBatchSize <= 0 {
			panic("cron.dlq_inspection_batch_size must be greater than zero")
		}
		if strings.TrimSpace(cfg.Cron.DLQInspectionGroup) == "" {
			panic("cron.dlq_inspection_group must not be empty")
		}

		inspector := mq.NewDeadLetterInspectorFromConfig(cfg, cfg.Cron.DLQInspectionGroup, appLogger)
		service := UsecaseMessaging.NewDeadLetterUsecase(
			inspector,
			nil,
			appLogger,
			cfg.Cron.DLQInspectionBatchSize,
			0,
			"",
		)
		if err := runner.Register(scheduler.Job{
			Name:     "mq.dlq.inspect",
			Interval: time.Duration(cfg.Cron.DLQInspectionIntervalSeconds) * time.Second,
			Run:      service.InspectDeadLetters,
		}); err != nil {
			panic("failed to register dlq inspection job: " + err.Error())
		}
	}

	if cfg.Cron.DLQReplayEnabled {
		if cfg.Cron.DLQReplayIntervalSeconds <= 0 {
			panic("cron.dlq_replay_interval_seconds must be greater than zero")
		}
		if cfg.Cron.DLQReplayBatchSize <= 0 {
			panic("cron.dlq_replay_batch_size must be greater than zero")
		}
		if strings.TrimSpace(cfg.Cron.DLQReplayGroup) == "" {
			panic("cron.dlq_replay_group must not be empty")
		}

		replayer := mq.NewDeadLetterReplayerFromConfig(cfg, cfg.Cron.DLQReplayGroup, appLogger)
		service := UsecaseMessaging.NewDeadLetterUsecase(
			nil,
			replayer,
			appLogger,
			0,
			cfg.Cron.DLQReplayBatchSize,
			mq.ResolveDeadLetterReplayTargetFromConfig(cfg),
		)
		if err := runner.Register(scheduler.Job{
			Name:     "mq.dlq.replay",
			Interval: time.Duration(cfg.Cron.DLQReplayIntervalSeconds) * time.Second,
			Run:      service.ReplayDeadLetters,
		}); err != nil {
			panic("failed to register dlq replay job: " + err.Error())
		}
	}
}
