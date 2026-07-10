package outbox

import (
	"context"
	"time"
)

type Repository interface {
	Create(ctx context.Context, events ...*Event) error
	ListUnpublished(ctx context.Context, limit int) ([]*Event, error)
	MarkPublished(ctx context.Context, ids []uint, publishedAt time.Time) error
}

type Publisher interface {
	Publish(ctx context.Context, messageKey, eventName string, payload []byte, traceID, traceContext string) error
}
