// Package mq 提供基于 Redis Streams 的事件总线实现。
// 消息自动携带 OTel trace_id，确保 server → worker 的链路追踪连续性。
package mq

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/trace"
)

// context key，用于在 handler 上下文中传递 trace_id。
type ctxKey struct{}

// TraceIDFromContext 从 ctx 提取由 mq 注入的 trace_id。
func TraceIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKey{}).(string); ok {
		return v
	}
	return ""
}

// —————————— event bus (producer) ——————————

// RedisEventBus 基于 Redis Streams 实现 shared.EventBus。
type RedisEventBus struct {
	rdb    *redis.Client
	stream string
	logger logger.Logger
}

// NewRedisEventBus 创建 Redis 事件总线（生产者端）。
func NewRedisEventBus(rdb *redis.Client, stream string, log logger.Logger) *RedisEventBus {
	return &RedisEventBus{rdb: rdb, stream: stream, logger: log}
}

var _ shared.EventBus = (*RedisEventBus)(nil)

func (b *RedisEventBus) Publish(ctx context.Context, events ...shared.Event) error {
	// 从 OpenTelemetry span 提取 trace_id
	traceID := extractTraceID(ctx)

	for _, evt := range events {
		payload, err := json.Marshal(evt)
		if err != nil {
			return err
		}
		vals := map[string]any{
			"event": evt.EventName(),
			"data":  string(payload),
		}
		if traceID != "" {
			vals["trace_id"] = traceID
		}

		if err := b.rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: b.stream,
			Values: vals,
		}).Err(); err != nil {
			return err
		}
	}

	b.logger.Debug("event published", "count", len(events), "trace_id", traceID)
	return nil
}

// —————————— consumer ——————————

// EventHandler 事件处理函数签名。ctx 中携带 trace_id。
type EventHandler func(ctx context.Context, data []byte) error

// RedisConsumer 基于 Redis Streams + 消费者组的事件消费者。
type RedisConsumer struct {
	rdb      *redis.Client
	stream   string
	group    string
	consumer string
	handlers map[string]EventHandler
	logger   logger.Logger
}

// NewRedisConsumer 创建消费者。consumer 为消费者实例名（如 "worker-1"）。
func NewRedisConsumer(rdb *redis.Client, stream, group, consumer string, log logger.Logger) *RedisConsumer {
	return &RedisConsumer{
		rdb:      rdb,
		stream:   stream,
		group:    group,
		consumer: consumer,
		handlers: make(map[string]EventHandler),
		logger:   log,
	}
}

// Handle 注册事件处理器。
func (c *RedisConsumer) Handle(eventName string, fn EventHandler) {
	c.handlers[eventName] = fn
}

// Run 阻塞运行，从 Stream 消费事件并分发给注册的 handler。
// ctx 取消时退出循环。
func (c *RedisConsumer) Run(ctx context.Context) error {
	if err := c.createGroup(ctx); err != nil {
		return err
	}

	c.logger.Info("consumer started", "group", c.group, "consumer", c.consumer, "stream", c.stream)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		msgs, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    c.group,
			Consumer: c.consumer,
			Streams:  []string{c.stream, ">"},
			Count:    10,
			Block:    1 * time.Second,
		}).Result()

		if err != nil {
			if errors.Is(err, redis.Nil) || errors.Is(err, context.Canceled) {
				continue
			}
			c.logger.Error("xreadgroup error", "error", err)
			time.Sleep(time.Second)
			continue
		}

		for _, stream := range msgs {
			for _, msg := range stream.Messages {
				c.dispatch(ctx, msg)
			}
		}
	}
}

func (c *RedisConsumer) createGroup(ctx context.Context) error {
	err := c.rdb.XGroupCreateMkStream(ctx, c.stream, c.group, "$").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}

func (c *RedisConsumer) dispatch(ctx context.Context, msg redis.XMessage) {
	eventName, ok := msg.Values["event"].(string)
	if !ok {
		c.logger.Error("message missing event field", "msg_id", msg.ID)
		c.ack(ctx, msg.ID)
		return
	}

	handler, ok := c.handlers[eventName]
	if !ok {
		c.logger.Debug("no handler for event", "event", eventName, "msg_id", msg.ID)
		c.ack(ctx, msg.ID)
		return
	}

	// 从消息中还原 trace_id，注入 handler 的上下文
	traceID, _ := msg.Values["trace_id"].(string)
	handlerCtx := ctx
	if traceID != "" {
		handlerCtx = context.WithValue(ctx, ctxKey{}, traceID)
	}

	data, _ := msg.Values["data"].(string)
	if err := handler(handlerCtx, []byte(data)); err != nil {
		c.logger.Error("handler error", "event", eventName, "error", err, "trace_id", traceID)
	}

	c.ack(ctx, msg.ID)
}

func (c *RedisConsumer) ack(ctx context.Context, id string) {
	if err := c.rdb.XAck(ctx, c.stream, c.group, id).Err(); err != nil {
		c.logger.Error("xack error", "error", err)
	}
}

// —————————— util ——————————

// extractTraceID 从 ctx 的 OTel span 中提取 trace_id。
func extractTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}
