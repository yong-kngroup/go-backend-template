package main

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/config"
	"github.com/freeDog-wy/go-backend-template/internal/handler"
	hdlAdminCMS "github.com/freeDog-wy/go-backend-template/internal/handler/admin_cms"
	hdlAdminMedia "github.com/freeDog-wy/go-backend-template/internal/handler/admin_media"
	hdlAdminRole "github.com/freeDog-wy/go-backend-template/internal/handler/admin_role"
	hdlAdminUser "github.com/freeDog-wy/go-backend-template/internal/handler/admin_user"
	hdlAuth "github.com/freeDog-wy/go-backend-template/internal/handler/auth"
	hdlCaptcha "github.com/freeDog-wy/go-backend-template/internal/handler/captcha"
	hdlHealth "github.com/freeDog-wy/go-backend-template/internal/handler/health"
	hdlMe "github.com/freeDog-wy/go-backend-template/internal/handler/me"
	"github.com/freeDog-wy/go-backend-template/internal/handler/middleware"
	hdlPublicContent "github.com/freeDog-wy/go-backend-template/internal/handler/public_content"
	hdlServiceToken "github.com/freeDog-wy/go-backend-template/internal/handler/service_token"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func newServerRegistry(infra *serverInfrastructure, repos *serverRepositories, services *serverServices, serviceTokenHandler *hdlServiceToken.Handler) *handler.Registry {
	registry := handler.NewRegistry()
	registry.Add(hdlHealth.New(map[string]hdlHealth.Checker{
		"database": hdlHealth.CheckFunc(infra.sqlDB.PingContext),
		"redis": hdlHealth.CheckFunc(func(ctx context.Context) error {
			return infra.redis.Ping(ctx).Err()
		}),
	}, 2*time.Second))
	registry.Add(hdlCaptcha.New(infra.captcha))
	registry.Add(hdlAuth.New(services.auth, services.authorization, services.identity, services.verification))
	registry.Add(hdlAdminRole.New(services.auth, services.authorization, services.authorization))
	registry.Add(hdlAdminUser.New(services.auth, services.authorization, services.authorization, services.identity))
	registry.Add(hdlMe.New(services.auth, services.auth, services.identity))

	adminCMS := hdlAdminCMS.New(services.auth, services.authorization, services.cms)
	adminCMS.SetIdempotency(middleware.Idempotency(repos.idempotency))
	registry.Add(adminCMS)
	registry.Add(hdlAdminMedia.New(services.auth, services.authorization, services.media))
	registry.Add(hdlPublicContent.New(services.cms))
	if serviceTokenHandler != nil {
		registry.Add(serviceTokenHandler)
	}
	return registry
}

func newRouter(cfg *config.Config, infra *serverInfrastructure, registry *handler.Registry) *gin.Engine {
	if cfg.App.Mode == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(otelgin.Middleware("go-backend-template"))
	r.Use(middleware.Recovery(infra.logger))
	r.Use(middleware.RateLimit(
		infra.rateLimiter,
		infra.logger,
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
	return r
}

func newHTTPServer(cfg *config.Config, router http.Handler) *http.Server {
	return &http.Server{
		Addr:         cfg.Server.IP + ":" + strconv.Itoa(cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}
}
