package mq

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	domainConsumption "github.com/freeDog-wy/go-backend-template/internal/domain/consumption"
	pkgkafka "github.com/freeDog-wy/go-backend-template/pkg/kafka"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type consumerAdapterConfig struct {
	GroupID           string
	ClientID          string
	MinBytes          int
	MaxBytes          int
	MaxWait           time.Duration
	StartOffset       *int64
	ProcessingLockTTL time.Duration
	MaxRetries        int
	RetryLevels       []pkgkafka.RetryLevel
	DeadLetterTopic   string
}

// consumerAdapter keeps project-level consume semantics while delegating Kafka topology to pkg/kafka.
type consumerAdapter struct {
	topology           *pkgkafka.ConsumerTopology
	groupID            string
	processingLockTTL  time.Duration
	maxRetries         int
	consumptionRecords domainConsumption.Repository
	handlers           map[string]EventHandler
	logger             logger.Logger
}

var _ Consumer = (*consumerAdapter)(nil)

func newConsumerAdapter(
	brokers []string,
	topic string,
	log logger.Logger,
	records domainConsumption.Repository,
	cfg consumerAdapterConfig,
) *consumerAdapter {
	if strings.TrimSpace(cfg.GroupID) == "" {
		panic("kafka consumer group id must not be empty")
	}
	if cfg.ProcessingLockTTL <= 0 {
		cfg.ProcessingLockTTL = 5 * time.Minute
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 10
	}
	if records == nil {
		panic("kafka consumption repository must not be nil")
	}

	topology := pkgkafka.NewConsumerTopology(brokers, topic, pkgkafka.ConsumerTopologyConfig{
		GroupID:         cfg.GroupID,
		ClientID:        cfg.ClientID,
		MinBytes:        cfg.MinBytes,
		MaxBytes:        cfg.MaxBytes,
		MaxWait:         cfg.MaxWait,
		StartOffset:     cfg.StartOffset,
		RetryLevels:     cfg.RetryLevels,
		DeadLetterTopic: cfg.DeadLetterTopic,
	})

	return &consumerAdapter{
		topology:           topology,
		groupID:            cfg.GroupID,
		processingLockTTL:  cfg.ProcessingLockTTL,
		maxRetries:         cfg.MaxRetries,
		consumptionRecords: records,
		handlers:           make(map[string]EventHandler),
		logger:             log,
	}
}

func (c *consumerAdapter) Handle(eventName string, fn EventHandler) {
	c.handlers[eventName] = fn
}

func (c *consumerAdapter) Run(ctx context.Context) error {
	c.logger.Info(
		"consumer started",
		"group", c.groupID,
		"topics", strings.Join(c.topology.Topics(), ","),
		"retry_topics", strings.Join(c.topology.RetryTopics(), ","),
		"dead_letter_topic", c.topology.DeadLetterTopic,
		"provider", "kafka",
	)

	return c.topology.Run(ctx, c.handleLoopMessage)
}

// handleLoopMessage 维持“结果持久化后才提交 offset”的消费顺序。
//
// 格式错误必须先写入 DLQ；业务失败必须先写入 retry/DLQ 并更新消费记录。任一步失败
// 都返回错误以保留原 offset，等待 Kafka 重新投递。消费记录只提供去重和处理锁，不能
// 将该流程误解为恰好一次投递。
func (c *consumerAdapter) handleLoopMessage(ctx context.Context, loop pkgkafka.ReaderLoop, record pkgkafka.Record) (err error) {
	eventMessage, err := c.decodeMessage(record)
	if err != nil {
		if routeErr := c.routeMalformedToDeadLetter(ctx, record, err); routeErr != nil {
			return routeErr
		}
		if err := loop.CommitMessages(ctx, record); err != nil {
			c.logger.Error("commit malformed kafka message failed", "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "error", err)
			return err
		}
		return nil
	}

	if err := c.waitRetryDelay(ctx, loop, eventMessage); err != nil {
		return err
	}

	handlerCtx := ExtractTraceContext(ctx, record.Headers)
	if eventMessage.TraceID == "" {
		eventMessage.TraceID = TraceIDFromContext(handlerCtx)
	}

	handlerCtx, span := mqTracer.Start(handlerCtx, "mq.consume",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination.name", record.Topic),
			attribute.String("messaging.operation", "consume"),
			attribute.String("app.event.name", eventMessage.Event),
			attribute.String("messaging.message.id", eventMessage.Key),
			attribute.Int("messaging.kafka.partition", record.Partition),
			attribute.Int64("messaging.kafka.offset", record.Offset),
			attribute.String("messaging.consumer.group.name", c.groupID),
		),
	)
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "")
		}
		span.End()
	}()

	beginAt := time.Now()
	beginResult, err := c.beginConsumption(handlerCtx, eventMessage, beginAt)
	if err != nil {
		c.logger.Error("begin kafka consumption failed", "event", eventMessage.Event, "message_key", eventMessage.Key, "error", err)
		return err
	}

	switch beginResult.Decision {
	case domainConsumption.BeginDecisionDone:
		if err := loop.CommitMessages(handlerCtx, record); err != nil {
			c.logger.Error("commit already processed kafka message failed", "event", eventMessage.Event, "message_key", eventMessage.Key, "offset", record.Offset, "error", err)
			return err
		}
		return nil
	case domainConsumption.BeginDecisionLocked:
		lockErr := errors.New("message is being processed by another worker")
		c.logger.Error("kafka message processing lock is active", "event", eventMessage.Event, "message_key", eventMessage.Key, "offset", record.Offset, "topic", loop.Topic)
		return lockErr
	}

	handler, ok := c.handlers[eventMessage.Event]
	if !ok {
		handlerErr := MarkNonRetryable(errors.New("no handler for event: " + eventMessage.Event))
		return c.handleFailure(handlerCtx, loop, record, eventMessage, beginResult.AttemptCount, handlerErr)
	}

	if err := handler(handlerCtx, eventMessage); err != nil {
		return c.handleFailure(handlerCtx, loop, record, eventMessage, beginResult.AttemptCount, err)
	}

	if err := c.markDone(handlerCtx, eventMessage.Key, time.Now()); err != nil {
		c.logger.Error("mark kafka message done state error", "event", eventMessage.Event, "message_key", eventMessage.Key, "error", err)
		return err
	}

	if err := loop.CommitMessages(handlerCtx, record); err != nil {
		c.logger.Error("commit kafka message failed", "event", eventMessage.Event, "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "error", err)
		return err
	}

	return nil
}

func (c *consumerAdapter) decodeMessage(record pkgkafka.Record) (Message, error) {
	eventName := pkgkafka.HeaderValue(record.Headers, "event")
	if strings.TrimSpace(eventName) == "" {
		return Message{}, errors.New("message missing event header")
	}

	messageKey := strings.TrimSpace(string(record.Key))
	if messageKey == "" {
		return Message{}, errors.New("message missing key")
	}

	return Message{
		Key:          messageKey,
		Event:        eventName,
		Payload:      record.Value,
		TraceID:      TraceIDFromHeaders(record.Headers),
		TraceContext: SerializeHeadersTraceContext(record.Headers),
	}, nil
}

func (c *consumerAdapter) waitRetryDelay(ctx context.Context, loop pkgkafka.ReaderLoop, message Message) error {
	if loop.Delay <= 0 {
		return nil
	}

	if c.logger != nil {
		c.logger.Info(
			"waiting for layered kafka retry topic",
			"event", message.Event,
			"message_key", message.Key,
			"topic", loop.Topic,
			"delay_seconds", int(loop.Delay.Seconds()),
		)
	}

	timer := time.NewTimer(loop.Delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// handleFailure 在提交原消息前确保失败结果已进入可恢复的 retry 或终态 DLQ。
func (c *consumerAdapter) handleFailure(ctx context.Context, loop pkgkafka.ReaderLoop, record pkgkafka.Record, message Message, attemptCount int, handlerErr error) error {
	now := time.Now()
	nonRetryable := IsNonRetryable(handlerErr)
	if nonRetryable || attemptCount >= c.maxRetries || len(c.topology.RetryPublishers) == 0 {
		if err := c.publishDeadLetter(ctx, record, message, attemptCount, handlerErr); err != nil {
			return err
		}
		if err := c.markDead(ctx, message.Key, handlerErr, now); err != nil {
			c.logger.Error("mark kafka message dead state error", "event", message.Event, "message_key", message.Key, "error", err)
			return err
		}
		if err := loop.CommitMessages(ctx, record); err != nil {
			c.logger.Error("commit kafka dead letter message failed", "event", message.Event, "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "error", err)
			return err
		}
		c.logger.Error("kafka message moved to dead letter topic", "event", message.Event, "message_key", message.Key, "attempt_count", attemptCount, "error", handlerErr)
		return nil
	}

	route := c.retryRoute(attemptCount)
	if err := c.publishRetry(ctx, route, record, message, attemptCount, handlerErr); err != nil {
		return err
	}
	if err := c.markFailed(ctx, message.Key, handlerErr, now); err != nil {
		c.logger.Error("mark kafka message failed state error", "event", message.Event, "message_key", message.Key, "error", err)
		return err
	}
	if err := loop.CommitMessages(ctx, record); err != nil {
		c.logger.Error("commit kafka retried message failed", "event", message.Event, "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "error", err)
		return err
	}
	c.logger.Error("kafka message moved to layered retry topic", "event", message.Event, "message_key", message.Key, "attempt_count", attemptCount, "retry_topic", route.Topic, "error", handlerErr)
	return nil
}

func (c *consumerAdapter) retryRoute(attemptCount int) pkgkafka.RetryPublisher {
	index := attemptCount - 1
	if index < 0 {
		index = 0
	}
	if index >= len(c.topology.RetryPublishers) {
		index = len(c.topology.RetryPublishers) - 1
	}
	return c.topology.RetryPublishers[index]
}

func (c *consumerAdapter) routeMalformedToDeadLetter(ctx context.Context, record pkgkafka.Record, decodeErr error) error {
	routeCtx := ExtractTraceContext(ctx, record.Headers)
	headers := pkgkafka.AppendHeaders(
		record.Headers,
		pkgkafka.Header{Key: "reason", Value: []byte(strings.TrimSpace(decodeErr.Error()))},
		pkgkafka.Header{Key: "failed_at", Value: []byte(time.Now().UTC().Format(time.RFC3339))},
		pkgkafka.Header{Key: "consumer_group", Value: []byte(c.groupID)},
		pkgkafka.Header{Key: "source_topic", Value: []byte(record.Topic)},
		pkgkafka.Header{Key: "source_partition", Value: []byte(strconv.Itoa(record.Partition))},
		pkgkafka.Header{Key: "source_offset", Value: []byte(strconv.FormatInt(record.Offset, 10))},
	)
	headers = InjectTraceContext(routeCtx, headers)

	err := c.topology.DeadLetterPublisher.Publish(ctx, pkgkafka.Message{
		Key:     record.Key,
		Value:   record.Value,
		Headers: headers,
	})
	if err != nil {
		c.logger.Error("write malformed kafka dead letter failed", "topic", record.Topic, "offset", record.Offset, "error", err)
		return err
	}
	c.logger.Error("malformed kafka message moved to dead letter topic", "topic", record.Topic, "offset", record.Offset, "error", decodeErr)
	return nil
}

func (c *consumerAdapter) publishRetry(ctx context.Context, route pkgkafka.RetryPublisher, record pkgkafka.Record, message Message, attemptCount int, handlerErr error) error {
	traceID := strings.TrimSpace(message.TraceID)
	if traceID == "" {
		traceID = TraceIDFromContext(ctx)
	}

	headers := []pkgkafka.Header{
		{Key: "event", Value: []byte(message.Event)},
		{Key: traceIDHeader, Value: []byte(traceID)},
		{Key: "retry_count", Value: []byte(strconv.Itoa(attemptCount))},
		{Key: "retry_topic", Value: []byte(route.Topic)},
		{Key: "retry_delay_seconds", Value: []byte(strconv.Itoa(int(route.Delay.Seconds())))},
		{Key: "original_topic", Value: []byte(originalTopic(record))},
		{Key: "source_topic", Value: []byte(record.Topic)},
		{Key: "source_partition", Value: []byte(strconv.Itoa(record.Partition))},
		{Key: "source_offset", Value: []byte(strconv.FormatInt(record.Offset, 10))},
		{Key: "last_error", Value: []byte(strings.TrimSpace(handlerErr.Error()))},
	}
	headers = InjectTraceContext(ctx, headers)

	retryMessage := pkgkafka.Message{
		Key:     []byte(message.Key),
		Value:   message.Payload,
		Headers: headers,
	}
	if err := route.Publisher.Publish(ctx, retryMessage); err != nil {
		c.logger.Error("write layered kafka retry message failed", "event", message.Event, "message_key", message.Key, "retry_topic", route.Topic, "error", err)
		return err
	}
	return nil
}

func (c *consumerAdapter) publishDeadLetter(ctx context.Context, record pkgkafka.Record, message Message, attemptCount int, handlerErr error) error {
	traceID := strings.TrimSpace(message.TraceID)
	if traceID == "" {
		traceID = TraceIDFromContext(ctx)
	}

	headers := []pkgkafka.Header{
		{Key: "event", Value: []byte(message.Event)},
		{Key: traceIDHeader, Value: []byte(traceID)},
		{Key: "retry_count", Value: []byte(strconv.Itoa(attemptCount))},
		{Key: "retry_topic", Value: []byte(strings.TrimSpace(pkgkafka.HeaderValue(record.Headers, "retry_topic")))},
		{Key: "retry_delay_seconds", Value: []byte(strings.TrimSpace(pkgkafka.HeaderValue(record.Headers, "retry_delay_seconds")))},
		{Key: "original_topic", Value: []byte(originalTopic(record))},
		{Key: "source_topic", Value: []byte(record.Topic)},
		{Key: "source_partition", Value: []byte(strconv.Itoa(record.Partition))},
		{Key: "source_offset", Value: []byte(strconv.FormatInt(record.Offset, 10))},
		{Key: "reason", Value: []byte(strings.TrimSpace(handlerErr.Error()))},
		{Key: "failed_at", Value: []byte(time.Now().UTC().Format(time.RFC3339))},
		{Key: "consumer_group", Value: []byte(c.groupID)},
	}
	headers = InjectTraceContext(ctx, headers)

	deadLetterMessage := pkgkafka.Message{
		Key:     []byte(message.Key),
		Value:   message.Payload,
		Headers: headers,
	}
	if err := c.topology.DeadLetterPublisher.Publish(ctx, deadLetterMessage); err != nil {
		c.logger.Error("write kafka dead letter message failed", "event", message.Event, "message_key", message.Key, "error", err)
		return err
	}
	return nil
}

func (c *consumerAdapter) beginConsumption(ctx context.Context, message Message, attemptedAt time.Time) (domainConsumption.BeginResult, error) {
	return c.consumptionRecords.Begin(ctx, domainConsumption.BeginCommand{
		ConsumerGroup: c.groupID,
		MessageKey:    message.Key,
		EventName:     message.Event,
		TraceID:       message.TraceID,
		AttemptedAt:   attemptedAt,
		LockedUntil:   attemptedAt.Add(c.processingLockTTL),
	})
}

func (c *consumerAdapter) markDone(ctx context.Context, messageKey string, processedAt time.Time) error {
	return c.consumptionRecords.MarkDone(ctx, c.groupID, messageKey, processedAt)
}

func (c *consumerAdapter) markFailed(ctx context.Context, messageKey string, handlerErr error, failedAt time.Time) error {
	if handlerErr == nil {
		return nil
	}
	return c.consumptionRecords.MarkFailed(ctx, c.groupID, messageKey, handlerErr.Error(), failedAt)
}

func (c *consumerAdapter) markDead(ctx context.Context, messageKey string, handlerErr error, failedAt time.Time) error {
	if handlerErr == nil {
		return nil
	}
	return c.consumptionRecords.MarkDead(ctx, c.groupID, messageKey, handlerErr.Error(), failedAt)
}

func originalTopic(record pkgkafka.Record) string {
	if value := pkgkafka.HeaderValue(record.Headers, "original_topic"); strings.TrimSpace(value) != "" {
		return value
	}
	return record.Topic
}
