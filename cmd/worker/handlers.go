package main

import (
	"context"
	"encoding/json"

	"github.com/freeDog-wy/go-backend-template/internal/config"
	domainAudit "github.com/freeDog-wy/go-backend-template/internal/domain/audit"
	domainIdentity "github.com/freeDog-wy/go-backend-template/internal/domain/identity"
	domainVerification "github.com/freeDog-wy/go-backend-template/internal/domain/verification"
	"github.com/freeDog-wy/go-backend-template/internal/infra/mq"
	svcAudit "github.com/freeDog-wy/go-backend-template/internal/usecase/audit"
	svcVerification "github.com/freeDog-wy/go-backend-template/internal/usecase/verification"
)

type workerEventConsumers struct {
	verification *svcVerification.EmailVerificationConsumer
	audit        *svcAudit.Consumer
}

func newWorkerEventConsumers(cfg *config.Config, infra *workerInfrastructure, repos *workerRepositories) *workerEventConsumers {
	return &workerEventConsumers{
		verification: svcVerification.NewConsumer(infra.emailSender, cfg.Email.SiteBaseURL, infra.logger),
		audit:        svcAudit.NewConsumer(repos.audit, infra.logger),
	}
}

func registerWorkerHandlers(consumer mq.Consumer, handlers *workerEventConsumers) {
	consumer.Handle("user.registered", consumeUserRegistered)
	consumer.Handle("user.email_verification_requested", func(ctx context.Context, message mq.Message) error {
		var event domainVerification.EmailVerificationRequested
		if err := json.Unmarshal(message.Payload, &event); err != nil {
			return err
		}
		return handlers.verification.OnEmailVerificationRequested(ctx, event)
	})
	consumer.Handle("user.password_reset_requested", func(ctx context.Context, message mq.Message) error {
		var event domainVerification.PasswordResetRequested
		if err := json.Unmarshal(message.Payload, &event); err != nil {
			return err
		}
		return handlers.verification.OnPasswordResetRequested(ctx, event)
	})
	consumer.Handle("audit.log.requested", func(ctx context.Context, message mq.Message) error {
		var event domainAudit.LogRequested
		if err := json.Unmarshal(message.Payload, &event); err != nil {
			return err
		}
		return handlers.audit.OnLogRequested(ctx, event)
	})
}

func consumeUserRegistered(_ context.Context, message mq.Message) error {
	var event domainIdentity.Registered
	return json.Unmarshal(message.Payload, &event)
}
