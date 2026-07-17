package outbox

import (
	"context"
	"encoding/json"

	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// EventBus 将领域事件持久化到本地 Outbox 表，而不在事务内直接发送到外部消息系统。
type EventBus struct {
	repo Store
}

func NewEventBus(repo Store) *EventBus {
	return &EventBus{repo: repo}
}

var _ shared.EventBus = (*EventBus)(nil)

// Publish 序列化事件及其追踪上下文，并通过 context 中的事务连接写入 Outbox。
// 调用方应在业务状态变更的同一事务内调用它，保证提交后才可能对外投递。
func (b *EventBus) Publish(ctx context.Context, events ...shared.Event) error {
	if len(events) == 0 {
		return nil
	}

	traceID := extractTraceID(ctx)
	traceContext := extractTraceContext(ctx)
	outboxEvents := make([]*Event, 0, len(events))
	for _, event := range events {
		payload, err := json.Marshal(event)
		if err != nil {
			return err
		}

		outboxEvent, err := NewEvent(event.EventName(), string(payload), traceID, traceContext)
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
