package outbox

import "time"

// eventModel 是 outbox_events 表的组件内部持久化模型。
type eventModel struct {
	ID           uint       `gorm:"primaryKey"`
	EventName    string     `gorm:"size:191;index;not null"`
	Payload      string     `gorm:"type:text;not null"`
	TraceID      string     `gorm:"size:64;index"`
	TraceContext string     `gorm:"type:text"`
	PublishedAt  *time.Time `gorm:"index"`
	CreatedAt    time.Time  `gorm:"index;not null"`
}

func (eventModel) TableName() string {
	return "outbox_events"
}

func (e *eventModel) toEvent() *Event {
	return ReconstituteEvent(
		e.ID,
		e.EventName,
		e.Payload,
		e.TraceID,
		e.TraceContext,
		e.PublishedAt,
		e.CreatedAt,
	)
}

func eventModelFromEvent(event *Event) *eventModel {
	return &eventModel{
		ID:           event.GetID(),
		EventName:    event.GetEventName(),
		Payload:      event.GetPayload(),
		TraceID:      event.GetTraceID(),
		TraceContext: event.GetTraceContext(),
		PublishedAt:  event.GetPublishedAt(),
		CreatedAt:    event.GetCreatedAt(),
	}
}
