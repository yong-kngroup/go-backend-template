package main

import (
	"context"
	"encoding/json"

	"github.com/freeDog-wy/go-backend-template/internal/config"
	domainAudit "github.com/freeDog-wy/go-backend-template/internal/domain/audit"
	domainIdentity "github.com/freeDog-wy/go-backend-template/internal/domain/identity"
	domainVerification "github.com/freeDog-wy/go-backend-template/internal/domain/verification"
	"github.com/freeDog-wy/go-backend-template/internal/infra/database"
	"github.com/freeDog-wy/go-backend-template/internal/infra/logging"
	"github.com/freeDog-wy/go-backend-template/internal/infra/mq"
	RepoAudit "github.com/freeDog-wy/go-backend-template/internal/repository/audit"
	RepoConsumption "github.com/freeDog-wy/go-backend-template/internal/repository/consumption"
	SvcAudit "github.com/freeDog-wy/go-backend-template/internal/usecase/audit"
	SvcVerification "github.com/freeDog-wy/go-backend-template/internal/usecase/verification"
	"github.com/freeDog-wy/go-backend-template/pkg/email"
)

// Worker 事件消费者进程。
type Worker struct {
	consumer mq.Consumer
}

// Run 启动消费者，阻塞直到 ctx 取消。
func (w *Worker) Run(ctx context.Context) error {
	return w.consumer.Run(ctx)
}

func initWorker(cfg *config.Config) *Worker {
	appLogger := logging.Init(cfg.App.Mode)
	db := database.NewPostgresDB(cfg.Database.DSN)

	emailSender := email.New(email.Config{
		SmtpHost:     cfg.Email.SmtpHost,
		SmtpPort:     cfg.Email.SmtpPort,
		SmtpUser:     cfg.Email.SmtpUser,
		SmtpPassword: cfg.Email.SmtpPassword,
		FromAddress:  cfg.Email.FromAddress,
	})

	verificationConsumer := SvcVerification.NewConsumer(emailSender, cfg.Email.SiteBaseURL, appLogger)
	auditConsumer := SvcAudit.NewConsumer(RepoAudit.New(db), appLogger)

	consumer := mq.NewConsumerFromConfig(cfg, RepoConsumption.New(db), appLogger)

	consumer.Handle("user.registered", func(ctx context.Context, message mq.Message) error {
		var evt domainIdentity.Registered
		if err := json.Unmarshal(message.Payload, &evt); err != nil {
			return err
		}
		return nil
	})

	consumer.Handle("user.email_verification_requested", func(ctx context.Context, message mq.Message) error {
		var evt domainVerification.EmailVerificationRequested
		if err := json.Unmarshal(message.Payload, &evt); err != nil {
			return err
		}
		return verificationConsumer.OnEmailVerificationRequested(ctx, evt)
	})

	consumer.Handle("user.password_reset_requested", func(ctx context.Context, message mq.Message) error {
		var evt domainVerification.PasswordResetRequested
		if err := json.Unmarshal(message.Payload, &evt); err != nil {
			return err
		}
		return verificationConsumer.OnPasswordResetRequested(ctx, evt)
	})

	consumer.Handle("audit.log.requested", func(ctx context.Context, message mq.Message) error {
		var evt domainAudit.LogRequested
		if err := json.Unmarshal(message.Payload, &evt); err != nil {
			return err
		}
		return auditConsumer.OnLogRequested(ctx, evt)
	})

	return &Worker{consumer: consumer}
}
