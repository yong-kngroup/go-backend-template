package mq

import "context"

// DeadLetterInspector 提供死信巡检所需的读取能力。
type DeadLetterInspector interface {
	Inspect(ctx context.Context, batchSize int) ([]DeadLetterMessage, error)
}

// DeadLetterReplayer 提供死信回放入口。
type DeadLetterReplayer interface {
	Replay(ctx context.Context, request DeadLetterReplayRequest) (DeadLetterReplayResult, error)
}
