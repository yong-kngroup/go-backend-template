package main

import (
	"context"
	"net/http"

	"github.com/freeDog-wy/go-backend-template/internal/config"
	"github.com/freeDog-wy/go-backend-template/internal/infra/tracing"
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

// initApp is the server composition root. Each provider below owns one layer
// of the dependency graph, keeping wiring explicit and locally reviewable.
func initApp(cfg *config.Config) (*App, error) {
	infra, err := newServerInfrastructure(cfg)
	if err != nil {
		return nil, err
	}
	repos := newServerRepositories(infra.db)
	services, err := newServerServices(cfg, infra, repos)
	if err != nil {
		return nil, err
	}
	if err := bootstrapServer(context.Background(), cfg, services); err != nil {
		return nil, err
	}

	serviceTokenHandler, err := newMCPServiceTokenHandler(context.Background(), cfg, infra, repos, services.eventBus)
	if err != nil {
		return nil, err
	}
	registry := newServerRegistry(infra, repos, services, serviceTokenHandler)

	return &App{
		server: newHTTPServer(cfg, newRouter(cfg, infra, registry)),
		tp:     infra.tracerProvider,
	}, nil
}
