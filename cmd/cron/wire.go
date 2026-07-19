package main

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/config"
	hdlHealth "github.com/freeDog-wy/go-backend-template/internal/handler/health"
	kafkaHealth "github.com/freeDog-wy/go-backend-template/internal/infra/kafka/health"
	"github.com/freeDog-wy/go-backend-template/internal/infra/tracing"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
	"github.com/freeDog-wy/go-backend-template/pkg/scheduler"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type CronApp struct {
	enabled     bool
	logger      logger.Logger
	runner      *scheduler.Runner
	probeServer *hdlHealth.Server
	running     atomic.Bool
	tp          *sdktrace.TracerProvider
}

func (a *CronApp) Run(ctx context.Context) error {
	a.running.Store(true)
	defer a.running.Store(false)
	if !a.enabled {
		a.logger.Info("cron is disabled by configuration")
		<-ctx.Done()
		return ctx.Err()
	}

	return a.runner.Run(ctx)
}

func (a *CronApp) ServeProbe() error {
	return a.probeServer.Serve()
}

func (a *CronApp) Shutdown(ctx context.Context) error {
	err := a.probeServer.Shutdown(ctx)
	tracing.Shutdown(ctx, a.tp)
	return err
}

// initCronApp is the cron composition root. Providers own their layer while
// this function keeps the process startup order explicit.
func initCronApp(cfg *config.Config) (*CronApp, error) {
	infra, err := newCronInfrastructure(cfg)
	if err != nil {
		return nil, err
	}
	app := &CronApp{
		enabled: cfg.Cron.Enabled,
		logger:  infra.logger,
		runner:  infra.runner,
		tp:      infra.tracerProvider,
	}
	checks := map[string]hdlHealth.Checker{
		"scheduler": hdlHealth.CheckFunc(func(context.Context) error {
			if !app.running.Load() {
				return errors.New("scheduler loop is not running")
			}
			return nil
		}),
	}

	if cfg.Cron.Enabled {
		if err := validateCronConfig(cfg); err != nil {
			return nil, err
		}
		runtime, err := newCronRuntimeInfrastructure(cfg)
		if err != nil {
			return nil, err
		}
		checks["database"] = hdlHealth.CheckFunc(runtime.sqlDB.PingContext)
		checks["kafka"] = hdlHealth.CheckFunc(func(ctx context.Context) error {
			return kafkaHealth.Ping(ctx, cfg.MQ.Kafka.Brokers)
		})
		if err := registerCronJobs(cfg, infra, runtime); err != nil {
			return nil, err
		}
	}

	app.probeServer = hdlHealth.NewServer(cfg.Cron.Probe.Address(), checks, 2*time.Second)
	return app, nil
}
