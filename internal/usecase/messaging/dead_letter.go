package messaging

import (
	"context"
	"strings"

	"github.com/freeDog-wy/go-backend-template/internal/infra/mq"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
)

type DeadLetterUsecase struct {
	inspector           mq.DeadLetterInspector
	replayer            mq.DeadLetterReplayer
	logger              logger.Logger
	inspectionBatchSize int
	replayBatchSize     int
	replayTargetTopic   string
}

func NewDeadLetterUsecase(
	inspector mq.DeadLetterInspector,
	replayer mq.DeadLetterReplayer,
	appLogger logger.Logger,
	inspectionBatchSize int,
	replayBatchSize int,
	replayTargetTopic string,
) *DeadLetterUsecase {
	if inspectionBatchSize <= 0 {
		inspectionBatchSize = 20
	}
	if replayBatchSize <= 0 {
		replayBatchSize = 20
	}

	return &DeadLetterUsecase{
		inspector:           inspector,
		replayer:            replayer,
		logger:              appLogger,
		inspectionBatchSize: inspectionBatchSize,
		replayBatchSize:     replayBatchSize,
		replayTargetTopic:   strings.TrimSpace(replayTargetTopic),
	}
}

// InspectDeadLetters 聚合一批 DLQ 消息并输出巡检日志。
func (u *DeadLetterUsecase) InspectDeadLetters(ctx context.Context) error {
	if u.inspector == nil {
		return nil
	}

	messages, err := u.inspector.Inspect(ctx, u.inspectionBatchSize)
	if err != nil {
		return err
	}
	if len(messages) == 0 {
		return nil
	}

	aggregates := make(map[string]int, len(messages))
	for _, message := range messages {
		key := deadLetterAggregateKey(message)
		aggregates[key]++
	}

	if u.logger != nil {
		u.logger.Info("dead letter inspection completed", "count", len(messages))
		for key, count := range aggregates {
			eventName, consumerGroup, reason := parseDeadLetterAggregateKey(key)
			u.logger.Info(
				"dead letter inspection summary",
				"event", eventName,
				"consumer_group", consumerGroup,
				"reason", reason,
				"count", count,
			)
		}
	}

	return nil
}

// ReplayDeadLetters 从 DLQ 拉取一批消息并回放到指定目标 topic。
func (u *DeadLetterUsecase) ReplayDeadLetters(ctx context.Context) error {
	if u.replayer == nil {
		return nil
	}

	result, err := u.replayer.Replay(ctx, mq.DeadLetterReplayRequest{
		BatchSize:   u.replayBatchSize,
		TargetTopic: u.replayTargetTopic,
	})
	if err != nil {
		return err
	}
	if u.logger != nil && (result.Fetched > 0 || result.Replayed > 0 || result.Skipped > 0) {
		u.logger.Info(
			"dead letter replay completed",
			"target_topic", u.replayTargetTopic,
			"fetched", result.Fetched,
			"replayed", result.Replayed,
			"skipped", result.Skipped,
		)
	}

	return nil
}

func deadLetterAggregateKey(message mq.DeadLetterMessage) string {
	return strings.Join([]string{
		strings.TrimSpace(message.Event),
		strings.TrimSpace(message.ConsumerGroup),
		strings.TrimSpace(message.Reason),
	}, "\x00")
}

func parseDeadLetterAggregateKey(key string) (string, string, string) {
	parts := strings.SplitN(key, "\x00", 3)
	for len(parts) < 3 {
		parts = append(parts, "")
	}
	return parts[0], parts[1], parts[2]
}
