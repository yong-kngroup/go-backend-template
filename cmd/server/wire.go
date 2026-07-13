package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/config"
	domainMedia "github.com/freeDog-wy/go-backend-template/internal/domain/media"
	"github.com/freeDog-wy/go-backend-template/internal/handler"
	HdlAdminCMS "github.com/freeDog-wy/go-backend-template/internal/handler/admin_cms"
	HdlAdminMedia "github.com/freeDog-wy/go-backend-template/internal/handler/admin_media"
	HdlAdminRole "github.com/freeDog-wy/go-backend-template/internal/handler/admin_role"
	HdlAdminUser "github.com/freeDog-wy/go-backend-template/internal/handler/admin_user"
	HdlAuth "github.com/freeDog-wy/go-backend-template/internal/handler/auth"
	HdlCaptcha "github.com/freeDog-wy/go-backend-template/internal/handler/captcha"
	HdlHealth "github.com/freeDog-wy/go-backend-template/internal/handler/health"
	HdlMe "github.com/freeDog-wy/go-backend-template/internal/handler/me"
	"github.com/freeDog-wy/go-backend-template/internal/handler/middleware"
	HdlPublicContent "github.com/freeDog-wy/go-backend-template/internal/handler/public_content"
	HdlServiceToken "github.com/freeDog-wy/go-backend-template/internal/handler/service_token"
	"github.com/freeDog-wy/go-backend-template/internal/infra/cache"
	"github.com/freeDog-wy/go-backend-template/internal/infra/crypto"
	"github.com/freeDog-wy/go-backend-template/internal/infra/database"
	"github.com/freeDog-wy/go-backend-template/internal/infra/idempotency"
	"github.com/freeDog-wy/go-backend-template/internal/infra/logging"
	InfraOutbox "github.com/freeDog-wy/go-backend-template/internal/infra/outbox"
	"github.com/freeDog-wy/go-backend-template/internal/infra/storage"
	infraToken "github.com/freeDog-wy/go-backend-template/internal/infra/token"
	"github.com/freeDog-wy/go-backend-template/internal/infra/tracing"
	RepoAuth "github.com/freeDog-wy/go-backend-template/internal/repository/auth"
	RepoAuthorization "github.com/freeDog-wy/go-backend-template/internal/repository/authorization"
	RepoCMS "github.com/freeDog-wy/go-backend-template/internal/repository/cms"
	RepoIdentity "github.com/freeDog-wy/go-backend-template/internal/repository/identity"
	RepoMCP "github.com/freeDog-wy/go-backend-template/internal/repository/mcp"
	RepoMedia "github.com/freeDog-wy/go-backend-template/internal/repository/media"
	RepoOutbox "github.com/freeDog-wy/go-backend-template/internal/repository/outbox"
	RepoVerification "github.com/freeDog-wy/go-backend-template/internal/repository/verification"
	svcAuth "github.com/freeDog-wy/go-backend-template/internal/usecase/auth"
	SvcAuthorization "github.com/freeDog-wy/go-backend-template/internal/usecase/authorization"
	SvcBootstrap "github.com/freeDog-wy/go-backend-template/internal/usecase/bootstrap"
	SvcCMS "github.com/freeDog-wy/go-backend-template/internal/usecase/cms"
	SvcIdentity "github.com/freeDog-wy/go-backend-template/internal/usecase/identity"
	SvcMCP "github.com/freeDog-wy/go-backend-template/internal/usecase/mcp"
	SvcMedia "github.com/freeDog-wy/go-backend-template/internal/usecase/media"
	SvcVerification "github.com/freeDog-wy/go-backend-template/internal/usecase/verification"
	"github.com/freeDog-wy/go-backend-template/pkg/captcha"
	"github.com/freeDog-wy/go-backend-template/pkg/ratelimit"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type App struct {
	server *http.Server
	tp     *sdktrace.TracerProvider
}

func (a *App) Run() error {
	return a.server.ListenAndServe()
}

func (a *App) Shutdown(ctx context.Context) error {
	if err := a.server.Shutdown(ctx); err != nil {
		return err
	}
	tracing.Shutdown(ctx, a.tp)
	return nil
}

func initApp(cfg *config.Config) (*App, error) {
	tp, err := tracing.Init(cfg.App.Mode, cfg.Tracing.Endpoint, "go-backend-template-server")
	if err != nil {
		return nil, fmt.Errorf("initialize tracing: %w", err)
	}

	appLogger := logging.Init(cfg.App.Mode)

	rdb, err := cache.NewRedis(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		return nil, fmt.Errorf("initialize redis: %w", err)
	}

	captchaGenerator := captcha.NewWithStore(captcha.Config{
		Width:  cfg.Captcha.Width,
		Height: cfg.Captcha.Height,
		Length: cfg.Captcha.Length,
	}, captcha.NewRedisStore(rdb, "captcha:", 5*time.Minute))

	db, err := database.NewPostgresDB(cfg.Database.DSN)
	if err != nil {
		return nil, fmt.Errorf("initialize postgres: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get postgres health check handle: %w", err)
	}
	txManager := database.NewTxManager(db)

	credentialRepo := RepoAuth.New(db)
	authorizationRepo := RepoAuthorization.New(db)
	userRepo := RepoIdentity.New(db)
	outboxRepo := RepoOutbox.New(db)
	verifyRepo := RepoVerification.New(db)
	cmsRepo := RepoCMS.New(db)
	mediaRepo := RepoMedia.New(db)
	mcpServiceAccountRepo := RepoMCP.NewServiceAccountRepository(db)
	idempotencyStore := idempotency.New(db)

	pwdHasher := crypto.NewBcryptHasher(0)
	eventBus := InfraOutbox.NewEventBus(outboxRepo)
	sessionStore := cache.NewRefreshSessionStore(rdb)
	rateLimiter := ratelimit.NewRateLimiter(rdb, "rate_limit")
	tokenManager, err := infraToken.NewJWTManager(cfg.Auth.JWTIssuer, cfg.Auth.JWTAudience, cfg.Auth.JWTSecret)
	if err != nil {
		return nil, fmt.Errorf("initialize jwt manager: %w", err)
	}
	if cfg.MCP.Enabled {
		if strings.TrimSpace(cfg.MCP.TokenAudience) == "" || cfg.MCP.AccessTokenTTLMinutes <= 0 {
			return nil, fmt.Errorf("validate mcp configuration: token audience and positive token ttl are required")
		}
		tokenManager.AllowAudience(cfg.MCP.TokenAudience)
	}

	verificationSvc := SvcVerification.New(txManager, userRepo, verifyRepo, credentialRepo, pwdHasher, sessionStore, eventBus, appLogger)
	authorizationSvc := SvcAuthorization.New(txManager, authorizationRepo, userRepo, eventBus, appLogger)
	bootstrapSvc := SvcBootstrap.New(txManager, userRepo, authorizationRepo, credentialRepo, pwdHasher, appLogger)
	identitySvc := SvcIdentity.New(txManager, userRepo, authorizationRepo, credentialRepo, pwdHasher, captchaGenerator, verificationSvc, appLogger, eventBus)
	authSvc := svcAuth.New(
		userRepo,
		credentialRepo,
		sessionStore,
		pwdHasher,
		tokenManager,
		eventBus,
		appLogger,
		cfg.Auth.JWTIssuer,
		cfg.Auth.JWTAudience,
		time.Duration(cfg.Auth.AccessTokenTTLMinutes)*time.Minute,
		time.Duration(cfg.Auth.RefreshTokenTTLHours)*time.Hour,
	)
	cmsSvc := SvcCMS.New(txManager, cmsRepo, eventBus)
	var mediaStorage domainMedia.Storage
	s3Storage, err := storage.NewS3(context.Background(), storage.Options{
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
	if err != nil && !errors.Is(err, storage.ErrNotConfigured) {
		return nil, fmt.Errorf("initialize S3 storage: %w", err)
	}
	if err == nil {
		mediaStorage = s3Storage
	}
	mediaSvc := SvcMedia.New(txManager, mediaRepo, mediaStorage)
	cmsSvc.SetMediaFinder(mediaSvc)
	cmsSvc.SetPublicMediaFinder(mediaSvc)
	if err := bootstrapSvc.BootstrapAdmin(context.Background(), SvcBootstrap.BootstrapAdminCmd{
		Enabled:  cfg.BootstrapAdmin.Enabled,
		Name:     cfg.BootstrapAdmin.Name,
		Email:    cfg.BootstrapAdmin.Email,
		Password: cfg.BootstrapAdmin.Password,
	}); err != nil {
		return nil, fmt.Errorf("bootstrap admin: %w", err)
	}
	var serviceTokenHdl *HdlServiceToken.Handler
	if cfg.MCP.Enabled {
		mcpBootstrapSvc := SvcMCP.NewBootstrapService(txManager, mcpServiceAccountRepo, userRepo, authorizationRepo, pwdHasher, sessionStore, appLogger)
		if err := mcpBootstrapSvc.Bootstrap(context.Background(), SvcMCP.BootstrapCmd{
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
		serviceTokenSvc := svcAuth.NewServiceTokenService(
			mcpServiceAccountRepo,
			userRepo,
			sessionStore,
			pwdHasher,
			tokenManager,
			eventBus,
			appLogger,
			cfg.Auth.JWTIssuer,
			cfg.MCP.TokenAudience,
			time.Duration(cfg.MCP.AccessTokenTTLMinutes)*time.Minute,
		)
		serviceTokenHdl = HdlServiceToken.New(serviceTokenSvc)
	}

	captchaHdl := HdlCaptcha.New(captchaGenerator)
	healthHdl := HdlHealth.New(map[string]HdlHealth.Checker{
		"database": HdlHealth.CheckFunc(sqlDB.PingContext),
		"redis": HdlHealth.CheckFunc(func(ctx context.Context) error {
			return rdb.Ping(ctx).Err()
		}),
	}, 2*time.Second)
	authHdl := HdlAuth.New(authSvc, authorizationSvc, identitySvc, verificationSvc)
	adminRoleHdl := HdlAdminRole.New(authSvc, authorizationSvc, authorizationSvc)
	adminUserHdl := HdlAdminUser.New(authSvc, authorizationSvc, authorizationSvc, identitySvc)
	meHdl := HdlMe.New(authSvc, authSvc, identitySvc)
	adminCMSHdl := HdlAdminCMS.New(authSvc, authorizationSvc, cmsSvc)
	adminCMSHdl.SetIdempotency(middleware.Idempotency(idempotencyStore))
	adminMediaHdl := HdlAdminMedia.New(authSvc, authorizationSvc, mediaSvc)
	publicContentHdl := HdlPublicContent.New(cmsSvc)

	registry := handler.NewRegistry()
	registry.Add(healthHdl)
	registry.Add(captchaHdl)
	registry.Add(authHdl)
	registry.Add(adminRoleHdl)
	registry.Add(adminUserHdl)
	registry.Add(meHdl)
	registry.Add(adminCMSHdl)
	registry.Add(adminMediaHdl)
	registry.Add(publicContentHdl)
	if serviceTokenHdl != nil {
		registry.Add(serviceTokenHdl)
	}

	if cfg.App.Mode == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(otelgin.Middleware("go-backend-template"))
	r.Use(middleware.Recovery(appLogger))
	r.Use(middleware.RateLimit(
		rateLimiter,
		appLogger,
		cfg.RateLimit.Enabled,
		cfg.RateLimit.Requests,
		time.Duration(cfg.RateLimit.WindowSeconds)*time.Second,
		middleware.DefaultRateLimitPolicies,
	))
	registry.RegisterAll(r)

	if len(cfg.Server.TrustedProxies) == 0 {
		r.SetTrustedProxies(nil)
	} else {
		r.SetTrustedProxies(cfg.Server.TrustedProxies)
	}

	server := &http.Server{
		Addr:         cfg.Server.IP + ":" + strconv.Itoa(cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	return &App{
		server: server,
		tp:     tp,
	}, nil
}
