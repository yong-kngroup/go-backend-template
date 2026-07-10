package main

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/config"
	"github.com/freeDog-wy/go-backend-template/internal/handler"
	HdlAdminRole "github.com/freeDog-wy/go-backend-template/internal/handler/admin_role"
	HdlAdminUser "github.com/freeDog-wy/go-backend-template/internal/handler/admin_user"
	HdlAuth "github.com/freeDog-wy/go-backend-template/internal/handler/auth"
	HdlCaptcha "github.com/freeDog-wy/go-backend-template/internal/handler/captcha"
	HdlMe "github.com/freeDog-wy/go-backend-template/internal/handler/me"
	"github.com/freeDog-wy/go-backend-template/internal/infra/cache"
	"github.com/freeDog-wy/go-backend-template/internal/infra/crypto"
	"github.com/freeDog-wy/go-backend-template/internal/infra/database"
	"github.com/freeDog-wy/go-backend-template/internal/infra/logging"
	InfraOutbox "github.com/freeDog-wy/go-backend-template/internal/infra/outbox"
	infraToken "github.com/freeDog-wy/go-backend-template/internal/infra/token"
	"github.com/freeDog-wy/go-backend-template/internal/infra/tracing"
	RepoAuth "github.com/freeDog-wy/go-backend-template/internal/repository/auth"
	RepoAuthorization "github.com/freeDog-wy/go-backend-template/internal/repository/authorization"
	RepoIdentity "github.com/freeDog-wy/go-backend-template/internal/repository/identity"
	RepoOutbox "github.com/freeDog-wy/go-backend-template/internal/repository/outbox"
	RepoVerification "github.com/freeDog-wy/go-backend-template/internal/repository/verification"
	svcAuth "github.com/freeDog-wy/go-backend-template/internal/usecase/auth"
	SvcAuthorization "github.com/freeDog-wy/go-backend-template/internal/usecase/authorization"
	SvcBootstrap "github.com/freeDog-wy/go-backend-template/internal/usecase/bootstrap"
	SvcIdentity "github.com/freeDog-wy/go-backend-template/internal/usecase/identity"
	SvcVerification "github.com/freeDog-wy/go-backend-template/internal/usecase/verification"
	"github.com/freeDog-wy/go-backend-template/pkg/captcha"

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

func initApp(cfg *config.Config) *App {
	tp, err := tracing.Init(cfg.App.Mode, cfg.Tracing.Endpoint, "go-backend-template-server")
	if err != nil {
		panic("failed to init tracing: " + err.Error())
	}

	appLogger := logging.Init(cfg.App.Mode)

	rdb, err := cache.NewRedis(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		panic("failed to init redis: " + err.Error())
	}

	captchaGenerator := captcha.NewWithStore(captcha.Config{
		Width:  cfg.Captcha.Width,
		Height: cfg.Captcha.Height,
		Length: cfg.Captcha.Length,
	}, captcha.NewRedisStore(rdb, "captcha:", 5*time.Minute))

	db := database.NewPostgresDB(cfg.Database.DSN)
	database.RunAutoMigrate(db, cfg.App.Mode)
	txManager := database.NewTxManager(db)

	credentialRepo := RepoAuth.New(db)
	authorizationRepo := RepoAuthorization.New(db)
	userRepo := RepoIdentity.New(db)
	outboxRepo := RepoOutbox.New(db)
	verifyRepo := RepoVerification.New(db)

	pwdHasher := crypto.NewBcryptHasher(0)
	eventBus := InfraOutbox.NewEventBus(outboxRepo)
	sessionStore := cache.NewRefreshSessionStore(rdb)
	tokenManager := infraToken.NewJWTManager(cfg.Auth.JWTIssuer, cfg.Auth.JWTAudience, cfg.Auth.JWTSecret)

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
	if err := bootstrapSvc.BootstrapAdmin(context.Background(), SvcBootstrap.BootstrapAdminCmd{
		Enabled:  cfg.BootstrapAdmin.Enabled,
		Name:     cfg.BootstrapAdmin.Name,
		Email:    cfg.BootstrapAdmin.Email,
		Password: cfg.BootstrapAdmin.Password,
	}); err != nil {
		panic("failed to bootstrap admin: " + err.Error())
	}

	captchaHdl := HdlCaptcha.New(captchaGenerator)
	authHdl := HdlAuth.New(authSvc, authorizationSvc, identitySvc, verificationSvc)
	adminRoleHdl := HdlAdminRole.New(authSvc, authorizationSvc)
	adminUserHdl := HdlAdminUser.New(authSvc, authorizationSvc, identitySvc)
	meHdl := HdlMe.New(authSvc, identitySvc)

	registry := handler.NewRegistry()
	registry.Add(captchaHdl)
	registry.Add(authHdl)
	registry.Add(adminRoleHdl)
	registry.Add(adminUserHdl)
	registry.Add(meHdl)

	if cfg.App.Mode == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(otelgin.Middleware("go-backend-template"))
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
	}
}
