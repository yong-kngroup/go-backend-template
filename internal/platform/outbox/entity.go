package outbox

import (
	"strings"
	"time"
)

// Event is a domain event persisted in the local outbox table until it is published.
type Event struct {
	id           uint
	eventName    string
	payload      string
	traceID      string
	traceContext string
	publishedAt  *time.Time
	createdAt    time.Time
}

func NewEvent(eventName, payload, traceID, traceContext string) (*Event, error) {
	eventName = strings.TrimSpace(eventName)
	if eventName == "" || strings.TrimSpace(payload) == "" {
		return nil, ErrInvalidEvent
	}

	return &Event{
		eventName:    eventName,
		payload:      payload,
		traceID:      strings.TrimSpace(traceID),
		traceContext: strings.TrimSpace(traceContext),
		createdAt:    time.Now(),
	}, nil
}

func ReconstituteEvent(id uint, eventName, payload, traceID, traceContext string, publishedAt *time.Time, createdAt time.Time) *Event {
	return &Event{
		id:           id,
		eventName:    eventName,
		payload:      payload,
		traceID:      traceID,
		traceContext: traceContext,
		publishedAt:  publishedAt,
		createdAt:    createdAt,
	}
}

func (e *Event) MarkPublished(now time.Time) {
	e.publishedAt = &now
}

func (e *Event) GetID() uint {
	return e.id
}

func (e *Event) GetEventName() string {
	return e.eventName
}

func (e *Event) GetPayload() string {
	return e.payload
}

func (e *Event) GetTraceID() string {
	return e.traceID
}

func (e *Event) GetTraceContext() string {
	return e.traceContext
}

func (e *Event) GetPublishedAt() *time.Time {
	return e.publishedAt
}

func (e *Event) GetCreatedAt() time.Time {
	return e.createdAt
}
