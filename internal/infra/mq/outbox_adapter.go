package mq

import "context"

// OutboxPublisherAdapter maps the outbox publisher contract onto mq.Publisher.
type OutboxPublisherAdapter struct {
	publisher Publisher
}

func NewOutboxPublisherAdapter(publisher Publisher) *OutboxPublisherAdapter {
	return &OutboxPublisherAdapter{publisher: publisher}
}

func (a *OutboxPublisherAdapter) Publish(ctx context.Context, messageKey, eventName string, payload []byte, traceID, traceContext string) error {
	return a.publisher.Publish(ctx, Message{
		Key:          messageKey,
		Event:        eventName,
		Payload:      payload,
		TraceID:      traceID,
		TraceContext: traceContext,
	})
}
