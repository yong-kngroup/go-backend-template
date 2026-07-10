package mq

import "time"

// Message 是消息中间件适配层的统一消息模型。
type Message struct {
	Key          string
	Event        string
	Payload      []byte
	TraceID      string
	TraceContext string
}

// DeadLetterMessage 描述死信消息保留的上下文信息。
type DeadLetterMessage struct {
	Message
	OriginalMessageID string
	OriginalTopic     string
	Source            string
	SourcePartition   int
	SourceOffset      int64
	ConsumerGroup     string
	Consumer          string
	Reason            string
	RetryCount        int64
	RetryTopic        string
	RetryDelaySeconds int
	FailedAt          time.Time
	DeadLetterTopic   string
	DeadLetterOffset  int64
	DeadLetterPart    int
}

// DeadLetterReplayRequest 描述一次死信回放任务的输入。
type DeadLetterReplayRequest struct {
	BatchSize   int
	TargetTopic string
}

// DeadLetterReplayResult 描述一次死信回放任务的结果。
type DeadLetterReplayResult struct {
	Fetched  int
	Replayed int
	Skipped  int
}
