package shared

import "context"

// Event 领域事件接口——每个事件都有一个唯一的事件名。
type Event interface {
	EventName() string
}

// EventBus 事件总线接口——领域层只定契约，实现在 platform/outbox。
type EventBus interface {
	Publish(ctx context.Context, events ...Event) error
}
