package outbox

import (
	"context"
	"encoding/json"

	domainOutbox "github.com/freeDog-wy/go-backend-template/internal/domain/outbox"
	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// EventBus persists domain events into the local outbox table.
type EventBus struct {
	repo domainOutbox.Repository
}

func NewEventBus(repo domainOutbox.Repository) *EventBus {
	return &EventBus{repo: repo}
}

var _ shared.EventBus = (*EventBus)(nil)

func (b *EventBus) Publish(ctx context.Context, events ...shared.Event) error {
	if len(events) == 0 {
		return nil
	}

	traceID := extractTraceID(ctx)
	traceContext := extractTraceContext(ctx)
	outboxEvents := make([]*domainOutbox.Event, 0, len(events))
	for _, event := range events {
		payload, err := json.Marshal(event)
		if err != nil {
			return err
		}

		outboxEvent, err := domainOutbox.NewEvent(event.EventName(), string(payload), traceID, traceContext)
		if err != nil {
			return err
		}
		outboxEvents = append(outboxEvents, outboxEvent)
	}

	return b.repo.Create(ctx, outboxEvents...)
}

func extractTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

func extractTraceContext(ctx context.Context) string {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	if len(carrier) == 0 {
		return ""
	}

	payload, err := json.Marshal(map[string]string(carrier))
	if err != nil {
		return ""
	}
	return string(payload)
}
