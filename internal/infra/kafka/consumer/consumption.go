package consumer

import (
	"context"
	"time"
)

// ConsumptionStatus 表示消息在 consumer group 中的处理状态。
type ConsumptionStatus string

const (
	ConsumptionStatusProcessing ConsumptionStatus = "processing"
	ConsumptionStatusDone       ConsumptionStatus = "done"
	ConsumptionStatusFailed     ConsumptionStatus = "failed"
	ConsumptionStatusDead       ConsumptionStatus = "dead"
)

// ConsumptionDecision 表示领取消息后的执行决策。
type ConsumptionDecision string

const (
	ConsumptionDecisionAcquired ConsumptionDecision = "acquired"
	ConsumptionDecisionDone     ConsumptionDecision = "done"
	ConsumptionDecisionLocked   ConsumptionDecision = "locked"
)

// ConsumptionBegin 是一次消息领取请求。
type ConsumptionBegin struct {
	ConsumerGroup string
	MessageKey    string
	EventName     string
	TraceID       string
	AttemptedAt   time.Time
	LockedUntil   time.Time
}

// ConsumptionBeginResult 是消息领取的结果。
type ConsumptionBeginResult struct {
	Decision     ConsumptionDecision
	AttemptCount int
}

// ConsumptionStore 持久化消费去重、处理锁和重试次数。
//
// Kafka 适配器只依赖该技术契约；具体的 PostgreSQL 实现在 platform/messaging 中。
type ConsumptionStore interface {
	Begin(ctx context.Context, command ConsumptionBegin) (ConsumptionBeginResult, error)
	MarkDone(ctx context.Context, consumerGroup, messageKey string, processedAt time.Time) error
	MarkFailed(ctx context.Context, consumerGroup, messageKey, lastError string, failedAt time.Time) error
	MarkDead(ctx context.Context, consumerGroup, messageKey, lastError string, failedAt time.Time) error
}
