package main

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/freeDog-wy/go-backend-template/internal/config"
	"github.com/freeDog-wy/go-backend-template/internal/infra/database"
	"github.com/freeDog-wy/go-backend-template/internal/infra/logging"
	"github.com/freeDog-wy/go-backend-template/internal/infra/tracing"
	"github.com/freeDog-wy/go-backend-template/pkg/email"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"gorm.io/gorm"
)

type workerInfrastructure struct {
	tracerProvider *sdktrace.TracerProvider
	logger         logger.Logger
	db             *gorm.DB
	sqlDB          *sql.DB
	emailSender    email.Sender
}

func newWorkerInfrastructure(cfg *config.Config) (*workerInfrastructure, error) {
	tp, err := tracing.Init(cfg.App.Mode, cfg.Tracing.Endpoint, "go-backend-template-worker")
	if err != nil {
		return nil, fmt.Errorf("initialize tracing: %w", err)
	}
	appLogger := logging.Init(cfg.App.Mode)
	db, err := database.NewPostgresDB(cfg.Database.DSN)
	if err != nil {
		return nil, fmt.Errorf("initialize postgres: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get postgres health check handle: %w", err)
	}

	if cfg.App.Mode == "production" && !strings.EqualFold(strings.TrimSpace(cfg.Email.Mode), string(email.ModeSMTP)) {
		return nil, fmt.Errorf("email.mode must be smtp in production")
	}
	emailSender, err := email.New(email.Config{
		Mode:         email.Mode(cfg.Email.Mode),
		SmtpHost:     cfg.Email.SmtpHost,
		SmtpPort:     cfg.Email.SmtpPort,
		SmtpUser:     cfg.Email.SmtpUser,
		SmtpPassword: cfg.Email.SmtpPassword,
		FromAddress:  cfg.Email.FromAddress,
	})
	if err != nil {
		return nil, fmt.Errorf("initialize email sender: %w", err)
	}

	return &workerInfrastructure{
		tracerProvider: tp,
		logger:         appLogger,
		db:             db,
		sqlDB:          sqlDB,
		emailSender:    emailSender,
	}, nil
}
