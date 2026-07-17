package main

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/config"
	hdlHealth "github.com/freeDog-wy/go-backend-template/internal/handler/health"
	"github.com/freeDog-wy/go-backend-template/internal/infra/mq"
	"github.com/freeDog-wy/go-backend-template/internal/infra/tracing"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type Worker struct {
	consumer    mq.Consumer
	probeServer *hdlHealth.Server
	running     atomic.Bool
	tp          *sdktrace.TracerProvider
}

func (w *Worker) Run(ctx context.Context) error {
	w.running.Store(true)
	defer w.running.Store(false)
	return w.consumer.Run(ctx)
}

func (w *Worker) ServeProbe() error {
	return w.probeServer.Serve()
}

func (w *Worker) Shutdown(ctx context.Context) error {
	err := w.probeServer.Shutdown(ctx)
	tracing.Shutdown(ctx, w.tp)
	return err
}

// initWorker is the worker composition root. Providers own their layer while
// this function keeps the process startup order explicit.
func initWorker(cfg *config.Config) (*Worker, error) {
	infra, err := newWorkerInfrastructure(cfg)
	if err != nil {
		return nil, err
	}
	repos := newWorkerRepositories(infra.db)
	platform := newWorkerPlatform(infra.db)
	handlers := newWorkerEventConsumers(cfg, infra, repos)
	consumer, err := newWorkerConsumer(cfg, infra, platform)
	if err != nil {
		return nil, err
	}

	worker := &Worker{consumer: consumer, tp: infra.tracerProvider}
	worker.probeServer = hdlHealth.NewServer(cfg.Worker.Probe.Address(), map[string]hdlHealth.Checker{
		"consumer": hdlHealth.CheckFunc(func(context.Context) error {
			if !worker.running.Load() {
				return errors.New("consumer loop is not running")
			}
			return nil
		}),
		"database": hdlHealth.CheckFunc(infra.sqlDB.PingContext),
		"kafka": hdlHealth.CheckFunc(func(ctx context.Context) error {
			return mq.PingKafka(ctx, cfg.MQ.Kafka.Brokers)
		}),
	}, 2*time.Second)
	registerWorkerHandlers(consumer, handlers)

	return worker, nil
}
