package outbox

import (
	"context"
	"time"
)

// Repository 持久化本地事务消息。
// Create 必须使用调用 context 中的事务连接；ListUnpublished 与 MarkPublished 由发布
// 任务使用，允许发布成功但标记失败后再次投递。
type Repository interface {
	// Create 在业务事务内写入待发布事件。
	Create(ctx context.Context, events ...*Event) error
	// ListUnpublished 返回一批尚未确认发布的事件。
	ListUnpublished(ctx context.Context, limit int) ([]*Event, error)
	// MarkPublished 仅确认已经成功发送到外部系统的事件。
	MarkPublished(ctx context.Context, ids []uint, publishedAt time.Time) error
}

// Publisher 将已提交的 Outbox 事件投递到外部消息系统。
// 调用方必须把稳定消息 key 传给下游消费者作为幂等依据。
type Publisher interface {
	Publish(ctx context.Context, messageKey, eventName string, payload []byte, traceID, traceContext string) error
}
