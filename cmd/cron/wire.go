package main

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/config"
	HdlHealth "github.com/freeDog-wy/go-backend-template/internal/handler/health"
	"github.com/freeDog-wy/go-backend-template/internal/infra/database"
	"github.com/freeDog-wy/go-backend-template/internal/infra/logging"
	"github.com/freeDog-wy/go-backend-template/internal/infra/mq"
	InfraStorage "github.com/freeDog-wy/go-backend-template/internal/infra/storage"
	"github.com/freeDog-wy/go-backend-template/internal/infra/tracing"
	RepoMedia "github.com/freeDog-wy/go-backend-template/internal/repository/media"
	RepoOutbox "github.com/freeDog-wy/go-backend-template/internal/repository/outbox"
	RepoVerification "github.com/freeDog-wy/go-backend-template/internal/repository/verification"
	UsecaseMedia "github.com/freeDog-wy/go-backend-template/internal/usecase/media"
	UsecaseMessaging "github.com/freeDog-wy/go-backend-template/internal/usecase/messaging"
	UsecaseSupport "github.com/freeDog-wy/go-backend-template/internal/usecase/support"
	UsecaseVerification "github.com/freeDog-wy/go-backend-template/internal/usecase/verification"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
	"github.com/freeDog-wy/go-backend-template/pkg/scheduler"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"gorm.io/gorm"
)

type CronApp struct {
	enabled     bool
	logger      logger.Logger
	runner      *scheduler.Runner
	probeServer *HdlHealth.Server
	running     atomic.Bool
	tp          *sdktrace.TracerProvider
}

func (a *CronApp) Run(ctx context.Context) error {
	a.running.Store(true)
	defer a.running.Store(false)
	if !a.enabled {
		a.logger.Info("cron is disabled by configuration")
		<-ctx.Done()
		return ctx.Err()
	}

	return a.runner.Run(ctx)
}

func (a *CronApp) ServeProbe() error {
	return a.probeServer.Serve()
}

func (a *CronApp) Shutdown(ctx context.Context) error {
	err := a.probeServer.Shutdown(ctx)
	tracing.Shutdown(ctx, a.tp)
	return err
}

func initCronApp(cfg *config.Config) *CronApp {
	tp, err := tracing.Init(cfg.App.Mode, cfg.Tracing.Endpoint, "go-backend-template-cron")
	if err != nil {
		panic("failed to init tracing: " + err.Error())
	}

	appLogger := logging.Init(cfg.App.Mode)
	runner := scheduler.New(appLogger)
	cronApp := &CronApp{
		enabled: cfg.Cron.Enabled,
		logger:  appLogger,
		runner:  runner,
		tp:      tp,
	}
	checks := map[string]HdlHealth.Checker{
		"scheduler": HdlHealth.CheckFunc(func(context.Context) error {
			if !cronApp.running.Load() {
				return errors.New("scheduler loop is not running")
			}
			return nil
		}),
	}
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
		sqlDB, err := db.DB()
		if err != nil {
			panic("failed to get database health check handle: " + err.Error())
		}
		checks["database"] = HdlHealth.CheckFunc(sqlDB.PingContext)
		checks["kafka"] = HdlHealth.CheckFunc(func(ctx context.Context) error {
			return mq.PingKafka(ctx, cfg.MQ.Kafka.Brokers)
		})

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
		registerMediaCleanupJob(cfg, appLogger, runner, db)

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

	cronApp.probeServer = HdlHealth.NewServer(cfg.Cron.Probe.Address(), checks, 2*time.Second)
	return cronApp
}

func registerMediaCleanupJob(cfg *config.Config, appLogger logger.Logger, runner *scheduler.Runner, db *gorm.DB) {
	if cfg.Storage.S3.Endpoint == "" || cfg.Storage.S3.AccessKeyID == "" || cfg.Storage.S3.SecretAccessKey == "" || cfg.Storage.S3.Bucket == "" {
		appLogger.Info("media upload cleanup is disabled because S3 storage is not configured")
		return
	}
	if cfg.Cron.MediaUploadCleanupIntervalSeconds <= 0 {
		panic("cron.media_upload_cleanup_interval_seconds must be greater than zero")
	}
	if cfg.Cron.MediaUploadCleanupBatchSize <= 0 {
		panic("cron.media_upload_cleanup_batch_size must be greater than zero")
	}
	storage, err := InfraStorage.NewS3(context.Background(), cfg.Storage.S3)
	if err != nil {
		panic("failed to initialize S3 storage for media cleanup: " + err.Error())
	}
	service := UsecaseMedia.New(database.NewTxManager(db), RepoMedia.New(db), storage)
	if err := runner.Register(scheduler.Job{
		Name:     "media.cleanup_expired_uploads",
		Interval: time.Duration(cfg.Cron.MediaUploadCleanupIntervalSeconds) * time.Second,
		Run: func(ctx context.Context) error {
			_, err := service.CleanupExpiredUploads(ctx, cfg.Cron.MediaUploadCleanupBatchSize)
			return err
		},
	}); err != nil {
		panic("failed to register media cleanup job: " + err.Error())
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
