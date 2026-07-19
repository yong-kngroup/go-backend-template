package main

import (
	"context"
	"encoding/json"

	"github.com/freeDog-wy/go-backend-template/internal/config"
	domainIdentity "github.com/freeDog-wy/go-backend-template/internal/domain/identity"
	domainVerification "github.com/freeDog-wy/go-backend-template/internal/domain/verification"
	"github.com/freeDog-wy/go-backend-template/internal/infra/kafka/consumer"
	"github.com/freeDog-wy/go-backend-template/internal/infra/kafka/event"
	svcVerification "github.com/freeDog-wy/go-backend-template/internal/usecase/verification"
)

type workerEventConsumers struct {
	verification *svcVerification.EmailVerificationConsumer
}

func newWorkerEventConsumers(cfg *config.Config, infra *workerInfrastructure) *workerEventConsumers {
	return &workerEventConsumers{
		verification: svcVerification.NewConsumer(infra.emailSender, cfg.Email.SiteBaseURL, infra.logger),
	}
}

func registerWorkerHandlers(consumer consumer.Consumer, handlers *workerEventConsumers) {
	consumer.Handle("user.registered", consumeUserRegistered)
	consumer.Handle("user.email_verification_requested", func(ctx context.Context, message event.Event) error {
		var event domainVerification.EmailVerificationRequested
		if err := json.Unmarshal(message.Payload, &event); err != nil {
			return err
		}
		return handlers.verification.OnEmailVerificationRequested(ctx, event)
	})
	consumer.Handle("user.password_reset_requested", func(ctx context.Context, message event.Event) error {
		var event domainVerification.PasswordResetRequested
		if err := json.Unmarshal(message.Payload, &event); err != nil {
			return err
		}
		return handlers.verification.OnPasswordResetRequested(ctx, event)
	})
}

func consumeUserRegistered(_ context.Context, message event.Event) error {
	var event domainIdentity.Registered
	return json.Unmarshal(message.Payload, &event)
}
