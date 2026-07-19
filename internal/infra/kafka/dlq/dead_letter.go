package dlq

import (
	"context"

	"github.com/freeDog-wy/go-backend-template/internal/infra/kafka/event"
)

// DeadLetterInspector 提供死信巡检所需的读取能力。
type Inspector interface {
	Inspect(ctx context.Context, batchSize int) ([]event.DeadLetter, error)
}

// DeadLetterReplayer 提供死信回放入口。
type Replayer interface {
	Replay(ctx context.Context, request ReplayRequest) (ReplayResult, error)
}

type ReplayRequest struct {
	BatchSize   int
	TargetTopic string
}
type ReplayResult struct {
	Fetched  int
	Replayed int
	Skipped  int
}
