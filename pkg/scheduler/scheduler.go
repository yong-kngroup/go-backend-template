package scheduler

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/freeDog-wy/go-backend-template/pkg/logger"
)

var ErrInvalidJob = errors.New("invalid scheduler job")

type Job struct {
	Name     string
	Interval time.Duration
	Run      func(context.Context) error
}

type Runner struct {
	logger logger.Logger
	jobs   []Job
}

func New(appLogger logger.Logger) *Runner {
	if appLogger == nil {
		appLogger = logger.Noop()
	}

	return &Runner{logger: appLogger}
}

func (r *Runner) Register(job Job) error {
	if job.Name == "" || job.Interval <= 0 || job.Run == nil {
		return ErrInvalidJob
	}

	r.jobs = append(r.jobs, job)
	return nil
}

func (r *Runner) Run(ctx context.Context) error {
	if len(r.jobs) == 0 {
		r.logger.Info("scheduler started without jobs")
		<-ctx.Done()
		return ctx.Err()
	}

	var wg sync.WaitGroup
	for _, job := range r.jobs {
		wg.Add(1)
		go func(job Job) {
			defer wg.Done()
			r.runJobLoop(ctx, job)
		}(job)
	}

	<-ctx.Done()
	wg.Wait()
	return ctx.Err()
}

func (r *Runner) runJobLoop(ctx context.Context, job Job) {
	ticker := time.NewTicker(job.Interval)
	defer ticker.Stop()

	var running atomic.Bool

	runOnce := func() {
		if !running.CompareAndSwap(false, true) {
			r.logger.Info("cron job skipped because previous run is still in progress", "job", job.Name)
			return
		}
		defer running.Store(false)

		startedAt := time.Now()
		r.logger.Info("cron job started", "job", job.Name)
		if err := job.Run(ctx); err != nil {
			r.logger.Error("cron job failed", "job", job.Name, "error", err, "duration", time.Since(startedAt))
			return
		}
		r.logger.Info("cron job completed", "job", job.Name, "duration", time.Since(startedAt))
	}

	runOnce()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runOnce()
		}
	}
}
