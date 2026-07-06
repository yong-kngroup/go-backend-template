package main

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/config"
	"github.com/freeDog-wy/go-backend-template/internal/handler"
	HdlCaptcha "github.com/freeDog-wy/go-backend-template/internal/handler/captcha"
	HdlUser "github.com/freeDog-wy/go-backend-template/internal/handler/user"
	"github.com/freeDog-wy/go-backend-template/internal/infra/cache"
	"github.com/freeDog-wy/go-backend-template/internal/infra/crypto"
	"github.com/freeDog-wy/go-backend-template/internal/infra/database"
	"github.com/freeDog-wy/go-backend-template/internal/infra/logging"
	"github.com/freeDog-wy/go-backend-template/internal/infra/mq"
	"github.com/freeDog-wy/go-backend-template/internal/infra/tracing"
	RepoUser "github.com/freeDog-wy/go-backend-template/internal/repository/user"
	SvcUser "github.com/freeDog-wy/go-backend-template/internal/service/user"
	"github.com/freeDog-wy/go-backend-template/pkg/captcha"
	"github.com/freeDog-wy/go-backend-template/pkg/email"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// App 应用顶层结构，包含运行和优雅关闭所需的所有资源。
type App struct {
	server *http.Server
	tp     *sdktrace.TracerProvider
}

// Run 启动 HTTP 服务。
func (a *App) Run() error {
	return a.server.ListenAndServe()
}

// Shutdown 优雅关闭——先停 HTTP 服务，再 flush 所有未发送的 trace。
func (a *App) Shutdown(ctx context.Context) error {
	if err := a.server.Shutdown(ctx); err != nil {
		return err
	}
	tracing.Shutdown(ctx, a.tp)
	return nil
}

func initApp(cfg *config.Config) *App {
	// —————————— 基础设施初始化（注意顺序）——————————
	tp, err := tracing.Init(cfg.App.Mode, cfg.Tracing.Endpoint)
	if err != nil {
		panic("failed to init tracing: " + err.Error())
	}

	appLogger := logging.Init(cfg.App.Mode)

	// —————————— 缓存层 ——————————
	rdb, err := cache.NewRedis(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		panic("failed to init redis: " + err.Error())
	}
	_ = rdb

	// —————————— 外部服务适配器 ——————————
	emailSender := email.New(email.Config{
		SmtpHost:     cfg.Email.SmtpHost,
		SmtpPort:     cfg.Email.SmtpPort,
		SmtpUser:     cfg.Email.SmtpUser,
		SmtpPassword: cfg.Email.SmtpPassword,
		FromAddress:  cfg.Email.FromAddress,
	})

	_ = emailSender

	captchaGenerator := captcha.NewWithStore(captcha.Config{
		Width:  cfg.Captcha.Width,
		Height: cfg.Captcha.Height,
		Length: cfg.Captcha.Length,
	}, captcha.NewRedisStore(rdb, "captcha:", 5*time.Minute))

	// —————————— 持久层 ——————————
	db := database.NewPostgresDB(cfg.Database.DSN)
	database.RunAutoMigrate(db, cfg.App.Mode)
	txManager := database.NewTxManager(db)

	// —————————— 仓储层 ——————————
	repoUser := RepoUser.New(db)

	// —————————— 应用层 ——————————
	pwdHasher := crypto.NewBcryptHasher(0)
	eventBus  := mq.NewRedisEventBus(rdb, "domain.events", appLogger)

	userSvc := SvcUser.New(txManager, repoUser, pwdHasher, captchaGenerator, emailSender, appLogger, eventBus)
	_ = userSvc

	// —————————— 接口层 ——————————
	captchaHdl := HdlCaptcha.New(captchaGenerator)
	userHdl := HdlUser.New(userSvc)

	registry := handler.NewRegistry()
	registry.Add(captchaHdl)
	registry.Add(userHdl)

	// —————————— Gin 路由 ——————————
	if cfg.App.Mode == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// OTel 中间件——自动为每个请求创建 root span
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
