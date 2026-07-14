package main

import (
	"context"
	"fmt"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/config"
	"github.com/freeDog-wy/go-backend-template/internal/infra/database"
	"github.com/freeDog-wy/go-backend-template/internal/infra/mq"
	repoMedia "github.com/freeDog-wy/go-backend-template/internal/repository/media"
	repoOutbox "github.com/freeDog-wy/go-backend-template/internal/repository/outbox"
	repoVerification "github.com/freeDog-wy/go-backend-template/internal/repository/verification"
	svcMedia "github.com/freeDog-wy/go-backend-template/internal/usecase/media"
	svcSupport "github.com/freeDog-wy/go-backend-template/internal/usecase/support"
	svcVerification "github.com/freeDog-wy/go-backend-template/internal/usecase/verification"
	"github.com/freeDog-wy/go-backend-template/pkg/scheduler"
)

type cronJobServices struct {
	outboxPublisher  *svcSupport.OutboxPublisher
	verificationCron *svcVerification.Cron
}

func validateCronConfig(cfg *config.Config) error {
	if cfg.Cron.OutboxPublishIntervalSeconds <= 0 {
		return fmt.Errorf("cron.outbox_publish_interval_seconds must be greater than zero")
	}
	if cfg.Cron.OutboxBatchSize <= 0 {
		return fmt.Errorf("cron.outbox_batch_size must be greater than zero")
	}
	if cfg.Cron.VerificationCleanupIntervalSeconds <= 0 {
		return fmt.Errorf("cron.verification_cleanup_interval_seconds must be greater than zero")
	}
	return nil
}

func registerCronJobs(cfg *config.Config, infra *cronInfrastructure, runtime *cronRuntimeInfrastructure) error {
	services, err := newCronJobServices(cfg, infra, runtime)
	if err != nil {
		return err
	}
	if err := registerMediaCleanupJob(cfg, infra, runtime); err != nil {
		return err
	}
	if err := registerCoreCronJobs(cfg, infra.runner, services); err != nil {
		return err
	}
	return registerKafkaDLQJobs(cfg, infra.logger, infra.runner)
}

func newCronJobServices(cfg *config.Config, infra *cronInfrastructure, runtime *cronRuntimeInfrastructure) (*cronJobServices, error) {
	publisher, err := mq.NewPublisher(mq.KafkaOptions{
		Brokers:  cfg.MQ.Kafka.Brokers,
		Topic:    cfg.MQ.EventsName,
		ClientID: cfg.MQ.Kafka.ClientID,
	}, infra.logger)
	if err != nil {
		return nil, fmt.Errorf("initialize kafka publisher: %w", err)
	}
	return &cronJobServices{
		outboxPublisher: svcSupport.NewOutboxPublisher(
			repoOutbox.New(runtime.db),
			mq.NewOutboxPublisherAdapter(publisher),
			infra.logger,
			cfg.Cron.OutboxBatchSize,
		),
		verificationCron: svcVerification.NewCron(repoVerification.New(runtime.db), infra.logger),
	}, nil
}

func registerCoreCronJobs(cfg *config.Config, runner *scheduler.Runner, services *cronJobServices) error {
	if err := runner.Register(scheduler.Job{
		Name:     "outbox.publish_pending_events",
		Interval: time.Duration(cfg.Cron.OutboxPublishIntervalSeconds) * time.Second,
		Run:      services.outboxPublisher.PublishPending,
	}); err != nil {
		return fmt.Errorf("register outbox publisher job: %w", err)
	}
	if err := runner.Register(scheduler.Job{
		Name:     "verification.cleanup_expired_tokens",
		Interval: time.Duration(cfg.Cron.VerificationCleanupIntervalSeconds) * time.Second,
		Run:      services.verificationCron.CleanupExpiredTokens,
	}); err != nil {
		return fmt.Errorf("register verification cleanup job: %w", err)
	}
	return nil
}

func registerMediaCleanupJob(cfg *config.Config, infra *cronInfrastructure, runtime *cronRuntimeInfrastructure) error {
	storage, enabled, err := newCronMediaStorage(cfg, infra.logger)
	if err != nil || !enabled {
		return err
	}
	if cfg.Cron.MediaUploadCleanupIntervalSeconds <= 0 {
		return fmt.Errorf("cron.media_upload_cleanup_interval_seconds must be greater than zero")
	}
	if cfg.Cron.MediaUploadCleanupBatchSize <= 0 {
		return fmt.Errorf("cron.media_upload_cleanup_batch_size must be greater than zero")
	}
	service := svcMedia.New(database.NewTxManager(runtime.db), repoMedia.New(runtime.db), storage)
	if err := infra.runner.Register(scheduler.Job{
		Name:     "media.cleanup_stale_uploads",
		Interval: time.Duration(cfg.Cron.MediaUploadCleanupIntervalSeconds) * time.Second,
		Run: func(ctx context.Context) error {
			_, err := service.CleanupStaleUploads(ctx, cfg.Cron.MediaUploadCleanupBatchSize)
			return err
		},
	}); err != nil {
		return fmt.Errorf("register media cleanup job: %w", err)
	}
	return nil
}
