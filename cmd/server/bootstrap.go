package main

import (
	"context"
	"fmt"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/config"
	hdlServiceToken "github.com/freeDog-wy/go-backend-template/internal/handler/service_token"
	infraOutbox "github.com/freeDog-wy/go-backend-template/internal/infra/outbox"
	svcAuth "github.com/freeDog-wy/go-backend-template/internal/usecase/auth"
	svcBootstrap "github.com/freeDog-wy/go-backend-template/internal/usecase/bootstrap"
	svcMCP "github.com/freeDog-wy/go-backend-template/internal/usecase/mcp"
)

func bootstrapServer(ctx context.Context, cfg *config.Config, services *serverServices) error {
	if err := services.bootstrap.BootstrapAdmin(ctx, svcBootstrap.BootstrapAdminCmd{
		Enabled:  cfg.BootstrapAdmin.Enabled,
		Name:     cfg.BootstrapAdmin.Name,
		Email:    cfg.BootstrapAdmin.Email,
		Password: cfg.BootstrapAdmin.Password,
	}); err != nil {
		return fmt.Errorf("bootstrap admin: %w", err)
	}
	return nil
}

func newMCPServiceTokenHandler(ctx context.Context, cfg *config.Config, infra *serverInfrastructure, repos *serverRepositories, eventBus *infraOutbox.EventBus) (*hdlServiceToken.Handler, error) {
	if !cfg.MCP.Enabled {
		return nil, nil
	}

	bootstrap := svcMCP.NewBootstrapService(infra.txManager, repos.mcpServiceAccount, repos.user, repos.authorization, infra.passwordHasher, infra.sessionStore, infra.logger)
	if err := bootstrap.Bootstrap(ctx, svcMCP.BootstrapCmd{
		Enabled:               true,
		Name:                  cfg.MCP.ServiceAccountName,
		Email:                 cfg.MCP.ServiceAccountEmail,
		ClientID:              cfg.MCP.ClientID,
		ClientSecret:          cfg.MCP.ClientSecret,
		RotationGrace:         time.Duration(cfg.MCP.SecretRotationGraceMinutes) * time.Minute,
		ServiceAccountEnabled: cfg.MCP.ServiceAccountEnabled,
	}); err != nil {
		return nil, fmt.Errorf("bootstrap mcp service account: %w", err)
	}

	service := svcAuth.NewServiceTokenService(
		repos.mcpServiceAccount,
		repos.user,
		infra.sessionStore,
		infra.passwordHasher,
		infra.tokenManager,
		eventBus,
		infra.logger,
		cfg.Auth.JWTIssuer,
		cfg.MCP.TokenAudience,
		time.Duration(cfg.MCP.AccessTokenTTLMinutes)*time.Minute,
	)
	return hdlServiceToken.New(service), nil
}
