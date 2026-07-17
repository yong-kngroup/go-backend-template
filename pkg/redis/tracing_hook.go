package redis

import (
	"context"
	"errors"
	"strings"

	redisv9 "github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type tracingHook struct {
	tracer trace.Tracer
	addr   string
	db     int
}

func newTracingHook(addr string, db int) redisv9.Hook {
	return &tracingHook{
		tracer: otel.Tracer("github.com/freeDog-wy/go-backend-template/pkg/redis"),
		addr:   strings.TrimSpace(addr),
		db:     db,
	}
}

func (h *tracingHook) DialHook(next redisv9.DialHook) redisv9.DialHook {
	return next
}

func (h *tracingHook) ProcessHook(next redisv9.ProcessHook) redisv9.ProcessHook {
	return func(ctx context.Context, cmd redisv9.Cmder) error {
		if ctx == nil {
			ctx = context.Background()
		}

		op := strings.ToUpper(strings.TrimSpace(cmd.FullName()))
		ctx, span := h.tracer.Start(ctx, "redis "+strings.ToLower(op))
		defer span.End()

		span.SetAttributes(
			attribute.String("db.system", "redis"),
			attribute.String("db.operation", op),
			attribute.String("server.address", h.addr),
			attribute.Int("db.redis.database_index", h.db),
		)

		err := next(ctx, cmd)
		if err != nil && !errors.Is(err, redisv9.Nil) {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return err
		}

		span.SetStatus(codes.Ok, "")
		return err
	}
}

func (h *tracingHook) ProcessPipelineHook(next redisv9.ProcessPipelineHook) redisv9.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redisv9.Cmder) error {
		if ctx == nil {
			ctx = context.Background()
		}

		names := make([]string, 0, len(cmds))
		for _, cmd := range cmds {
			names = append(names, strings.ToUpper(strings.TrimSpace(cmd.FullName())))
		}

		ctx, span := h.tracer.Start(ctx, "redis pipeline")
		defer span.End()

		span.SetAttributes(
			attribute.String("db.system", "redis"),
			attribute.String("server.address", h.addr),
			attribute.Int("db.redis.database_index", h.db),
			attribute.Int("db.redis.pipeline.length", len(cmds)),
		)
		if len(names) > 0 {
			span.SetAttributes(attribute.StringSlice("db.redis.pipeline.commands", names))
		}

		err := next(ctx, cmds)
		if err != nil && !errors.Is(err, redisv9.Nil) {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return err
		}

		span.SetStatus(codes.Ok, "")
		return err
	}
}
