package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/config"
	domainMedia "github.com/freeDog-wy/go-backend-template/internal/domain/media"
	"github.com/freeDog-wy/go-backend-template/internal/infra/crypto"
	"github.com/freeDog-wy/go-backend-template/internal/infra/logging"
	"github.com/freeDog-wy/go-backend-template/internal/infra/postgres"
	redisClient "github.com/freeDog-wy/go-backend-template/internal/infra/redis"
	"github.com/freeDog-wy/go-backend-template/internal/infra/storage"
	infraToken "github.com/freeDog-wy/go-backend-template/internal/infra/token"
	"github.com/freeDog-wy/go-backend-template/internal/infra/tracing"
	platformOutbox "github.com/freeDog-wy/go-backend-template/internal/platform/outbox"
	baseRepository "github.com/freeDog-wy/go-backend-template/internal/repository"
	repoAuth "github.com/freeDog-wy/go-backend-template/internal/repository/auth"
	"github.com/freeDog-wy/go-backend-template/pkg/captcha"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
	"github.com/freeDog-wy/go-backend-template/pkg/ratelimit"
	"github.com/redis/go-redis/v9"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"gorm.io/gorm"
)

type serverInfrastructure struct {
	tracerProvider *sdktrace.TracerProvider
	logger         logger.Logger
	redis          *redis.Client
	db             *gorm.DB
	sqlDB          *sql.DB
	txManager      *baseRepository.TxManager
	captcha        captcha.Generator
	passwordHasher *crypto.BcryptHasher
	sessionStore   *repoAuth.RefreshSessionStore
	rateLimiter    *ratelimit.RateLimiter
	tokenManager   *infraToken.JWTManager
	mediaStorage   domainMedia.Storage
}

func newServerInfrastructure(cfg *config.Config) (*serverInfrastructure, error) {
	tp, err := tracing.Init(cfg.App.Mode, cfg.Tracing.Endpoint, "go-backend-template-server")
	if err != nil {
		return nil, fmt.Errorf("initialize tracing: %w", err)
	}
	appLogger := logging.Init(cfg.App.Mode)

	rdb, err := redisClient.Open(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		return nil, fmt.Errorf("initialize redis: %w", err)
	}
	db, err := postgres.Open(cfg.Database.DSN)
	if err != nil {
		return nil, fmt.Errorf("initialize postgres: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get postgres health check handle: %w", err)
	}

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
	mediaStorage, err := newMediaStorage(cfg)
	if err != nil {
		return nil, err
	}

	return &serverInfrastructure{
		tracerProvider: tp,
		logger:         appLogger,
		redis:          rdb,
		db:             db,
		sqlDB:          sqlDB,
		txManager:      baseRepository.NewTxManager(db),
		captcha: captcha.NewWithStore(captcha.Config{
			Width:  cfg.Captcha.Width,
			Height: cfg.Captcha.Height,
			Length: cfg.Captcha.Length,
		}, captcha.NewRedisStore(rdb, "captcha:", 5*time.Minute)),
		passwordHasher: crypto.NewBcryptHasher(0),
		sessionStore:   repoAuth.NewRefreshSessionStore(rdb),
		rateLimiter:    ratelimit.NewRateLimiter(rdb, "rate_limit"),
		tokenManager:   tokenManager,
		mediaStorage:   mediaStorage,
	}, nil
}

func newServerEventBus(repo *platformOutbox.Repository) *platformOutbox.EventBus {
	return platformOutbox.NewEventBus(repo)
}

func newMediaStorage(cfg *config.Config) (domainMedia.Storage, error) {
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
	if err != nil {
		return nil, nil
	}
	return s3Storage, nil
}
