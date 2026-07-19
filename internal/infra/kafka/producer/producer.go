package producer

import (
	"context"
	"fmt"
	"strings"

	"github.com/freeDog-wy/go-backend-template/internal/infra/kafka/config"
	"github.com/freeDog-wy/go-backend-template/internal/infra/kafka/event"
	"github.com/freeDog-wy/go-backend-template/internal/infra/kafka/internal/client"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
	kgo "github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Publisher interface {
	Publish(context.Context, event.Event) error
}

type kafkaPublisher struct {
	writer *kgo.Writer
	logger logger.Logger
	topic  string
}

func New(options config.Connection, log logger.Logger) (Publisher, error) {
	if len(client.NormalizeBrokers(options.Brokers)) == 0 || strings.TrimSpace(options.Topic) == "" {
		return nil, fmt.Errorf("kafka publisher brokers and topic are required")
	}
	return &kafkaPublisher{writer: client.NewWriter(options.Brokers, options.Topic, options.ClientID), logger: log, topic: strings.TrimSpace(options.Topic)}, nil
}

func (p *kafkaPublisher) Publish(ctx context.Context, message event.Event) (err error) {
	propagationCtx := ctx
	if strings.TrimSpace(message.TraceContext) != "" {
		propagationCtx = client.ContextWithSerializedTraceContext(context.WithoutCancel(ctx), message.TraceContext)
	}
	traceID := strings.TrimSpace(message.TraceID)
	if traceID == "" {
		traceID = client.TraceIDFromContext(propagationCtx)
	}
	propagationCtx, span := client.Tracer.Start(propagationCtx, "kafka.publish", trace.WithSpanKind(trace.SpanKindProducer), trace.WithAttributes(attribute.String("messaging.system", "kafka"), attribute.String("messaging.destination.name", p.topic), attribute.String("app.event.name", message.Event), attribute.String("messaging.message.id", message.Key)))
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "")
		}
		span.End()
	}()
	headers := []kgo.Header{{Key: "event", Value: []byte(message.Event)}}
	if traceID != "" {
		headers = append(headers, kgo.Header{Key: "trace_id", Value: []byte(traceID)})
	}
	headers = client.InjectTraceContext(propagationCtx, headers)
	if err = p.writer.WriteMessages(propagationCtx, kgo.Message{Key: []byte(message.Key), Value: message.Payload, Headers: headers}); err != nil {
		return err
	}
	if p.logger != nil {
		p.logger.Debug("event published", "event", message.Event, "trace_id", traceID)
	}
	return nil
}

type OutboxAdapter struct{ publisher Publisher }

func NewOutboxAdapter(p Publisher) *OutboxAdapter { return &OutboxAdapter{publisher: p} }
func (a *OutboxAdapter) Publish(ctx context.Context, key, name string, payload []byte, traceID, traceContext string) error {
	return a.publisher.Publish(ctx, event.Event{Key: key, Event: name, Payload: payload, TraceID: traceID, TraceContext: traceContext})
}
