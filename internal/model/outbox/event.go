package outbox

import (
	"time"

	domainOutbox "github.com/freeDog-wy/go-backend-template/internal/domain/outbox"
)

type Event struct {
	ID           uint       `gorm:"primaryKey"`
	EventName    string     `gorm:"size:191;index;not null"`
	Payload      string     `gorm:"type:text;not null"`
	TraceID      string     `gorm:"size:64;index"`
	TraceContext string     `gorm:"type:text"`
	PublishedAt  *time.Time `gorm:"index"`
	CreatedAt    time.Time  `gorm:"index;not null"`
}

func (Event) TableName() string {
	return "outbox_events"
}

func (e *Event) ToEntity() *domainOutbox.Event {
	return domainOutbox.ReconstituteEvent(
		e.ID,
		e.EventName,
		e.Payload,
		e.TraceID,
		e.TraceContext,
		e.PublishedAt,
		e.CreatedAt,
	)
}

func FromEntity(event *domainOutbox.Event) *Event {
	return &Event{
		ID:           event.GetID(),
		EventName:    event.GetEventName(),
		Payload:      event.GetPayload(),
		TraceID:      event.GetTraceID(),
		TraceContext: event.GetTraceContext(),
		PublishedAt:  event.GetPublishedAt(),
		CreatedAt:    event.GetCreatedAt(),
	}
}
