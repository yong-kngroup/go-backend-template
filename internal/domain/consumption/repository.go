package consumption

import (
	"context"
	"time"
)

// Repository 定义消费记录的持久化契约。
//
// Begin 必须原子地判断消息是否可由当前 worker 执行。完成或已死信的消息返回 Done；
// 未过期的 processing 锁返回 Locked；其余情况领取消息并递增尝试次数。状态更新只应
// 在相应的业务结果或下一跳消息已可靠写入后调用。
type Repository interface {
	// Begin 领取消息或返回其既有消费决策。
	Begin(ctx context.Context, command BeginCommand) (BeginResult, error)
	// MarkDone 在业务副作用成功后清除锁并阻止后续重复执行。
	MarkDone(ctx context.Context, consumerGroup, messageKey string, processedAt time.Time) error
	// MarkFailed 在重试消息已写入后记录可恢复失败并清除处理锁。
	MarkFailed(ctx context.Context, consumerGroup, messageKey, lastError string, failedAt time.Time) error
	// MarkDead 在死信消息已写入后记录终态并阻止后续重复执行。
	MarkDead(ctx context.Context, consumerGroup, messageKey, lastError string, failedAt time.Time) error
}
