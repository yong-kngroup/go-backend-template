package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/freeDog-wy/go-backend-template/internal/config"
	domainMedia "github.com/freeDog-wy/go-backend-template/internal/domain/media"
	"github.com/freeDog-wy/go-backend-template/internal/infra/logging"
	"github.com/freeDog-wy/go-backend-template/internal/infra/postgres"
	infraStorage "github.com/freeDog-wy/go-backend-template/internal/infra/storage"
	"github.com/freeDog-wy/go-backend-template/internal/infra/tracing"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
	"github.com/freeDog-wy/go-backend-template/pkg/scheduler"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"gorm.io/gorm"
)

type cronInfrastructure struct {
	tracerProvider *sdktrace.TracerProvider
	logger         logger.Logger
	runner         *scheduler.Runner
}

type cronRuntimeInfrastructure struct {
	db    *gorm.DB
	sqlDB *sql.DB
}

func newCronInfrastructure(cfg *config.Config) (*cronInfrastructure, error) {
	tp, err := tracing.Init(cfg.App.Mode, cfg.Tracing.Endpoint, "go-backend-template-cron")
	if err != nil {
		return nil, fmt.Errorf("initialize tracing: %w", err)
	}
	appLogger := logging.Init(cfg.App.Mode)
	return &cronInfrastructure{
		tracerProvider: tp,
		logger:         appLogger,
		runner:         scheduler.New(appLogger),
	}, nil
}

func newCronRuntimeInfrastructure(cfg *config.Config) (*cronRuntimeInfrastructure, error) {
	db, err := postgres.Open(cfg.Database.DSN)
	if err != nil {
		return nil, fmt.Errorf("initialize postgres: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get postgres health check handle: %w", err)
	}
	return &cronRuntimeInfrastructure{db: db, sqlDB: sqlDB}, nil
}

func newCronMediaStorage(cfg *config.Config, appLogger logger.Logger) (domainMedia.Storage, bool, error) {
	storage, err := infraStorage.NewS3(context.Background(), infraStorage.Options{
		Endpoint:          cfg.Storage.S3.Endpoint,
		Region:            cfg.Storage.S3.Region,
		AccessKeyID:       cfg.Storage.S3.AccessKeyID,
		SecretAccessKey:   cfg.Storage.S3.SecretAccessKey,
		Bucket:            cfg.Storage.S3.Bucket,
		PublicBaseURL:     cfg.Storage.S3.PublicBaseURL,
		Prefix:            cfg.Storage.S3.Prefix,
		UsePathStyle:      cfg.Storage.S3.UsePathStyle,
		PresignTTLMinutes: cfg.Storage.S3.PresignTTLMinutes,
	})
	if errors.Is(err, infraStorage.ErrNotConfigured) {
		appLogger.Info("media upload cleanup is disabled because S3 storage is not configured")
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("initialize S3 storage for media cleanup: %w", err)
	}
	return storage, true, nil
}
