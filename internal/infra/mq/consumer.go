package mq

import "context"

// EventHandler 使用项目统一消息模型处理事件，不暴露 Kafka 原生类型。
// 返回错误会触发重试或死信决策；需要直接进入死信队列时应使用 MarkNonRetryable 包装错误。
type EventHandler func(ctx context.Context, message Message) error

// Consumer 定义项目级消息消费契约。
// 实现负责至少一次投递、消费记录、重试、死信和 offset 提交顺序；Handler 负责幂等业务副作用。
type Consumer interface {
	Handle(eventName string, fn EventHandler)
	Run(ctx context.Context) error
}
