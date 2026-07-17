package mq

import "context"

// EventHandler 使用统一消息模型处理事件，不暴露 Kafka 原生类型。
// 返回错误会触发重试或死信决策；需要直接进入死信队列时应使用 MarkNonRetryable 包装错误。
type EventHandler func(ctx context.Context, message Message) error

// Consumer 定义 Kafka 消费契约。
// 实现负责重试、死信和 offset 提交顺序；消费状态由注入的 ConsumptionStore 持久化。
type Consumer interface {
	Handle(eventName string, fn EventHandler)
	Run(ctx context.Context) error
}
