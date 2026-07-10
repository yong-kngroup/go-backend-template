package mq

import (
	"context"
	"strings"

	pkgkafka "github.com/freeDog-wy/go-backend-template/pkg/kafka"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// publisherAdapter maps the project message model onto the Kafka publisher.
type publisherAdapter struct {
	publisher *pkgkafka.Publisher
	logger    logger.Logger
	topic     string
}

func newPublisherAdapter(brokers []string, topic, clientID string, log logger.Logger) *publisherAdapter {
	return &publisherAdapter{
		publisher: pkgkafka.NewPublisher(brokers, topic, pkgkafka.WriterConfig{ClientID: clientID}),
		logger:    log,
		topic:     strings.TrimSpace(topic),
	}
}

var _ Publisher = (*publisherAdapter)(nil)

func (p *publisherAdapter) Publish(ctx context.Context, message Message) (err error) {
	propagationCtx := ctx
	if strings.TrimSpace(message.TraceContext) != "" {
		propagationCtx = ContextWithSerializedTraceContext(context.WithoutCancel(ctx), message.TraceContext)
	}

	traceID := strings.TrimSpace(message.TraceID)
	if traceID == "" {
		traceID = TraceIDFromContext(propagationCtx)
	}

	propagationCtx, span := mqTracer.Start(propagationCtx, "mq.publish",
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination.name", p.topic),
			attribute.String("messaging.operation", "publish"),
			attribute.String("app.event.name", message.Event),
			attribute.String("messaging.message.id", message.Key),
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

	headers := []pkgkafka.Header{
		{Key: "event", Value: []byte(message.Event)},
	}
	if traceID != "" {
		headers = append(headers, pkgkafka.Header{Key: traceIDHeader, Value: []byte(traceID)})
	}
	headers = InjectTraceContext(propagationCtx, headers)

	msg := pkgkafka.Message{
		Key:     []byte(message.Key),
		Value:   message.Payload,
		Headers: headers,
	}

	if err = p.publisher.Publish(propagationCtx, msg); err != nil {
		return err
	}

	if p.logger != nil {
		p.logger.Debug("event published", "event", message.Event, "trace_id", traceID)
	}
	return nil
}
