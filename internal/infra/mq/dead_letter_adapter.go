package mq

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	pkgkafka "github.com/freeDog-wy/go-backend-template/pkg/kafka"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
)

type deadLetterAdapterConfig struct {
	GroupID     string
	ClientID    string
	MinBytes    int
	MaxBytes    int
	MaxWait     time.Duration
	PollTimeout time.Duration
}

// deadLetterAdapter maps the project's dead-letter semantics onto pkg/kafka.
type deadLetterAdapter struct {
	queue  *pkgkafka.DeadLetterQueue
	logger logger.Logger
}

var _ DeadLetterInspector = (*deadLetterAdapter)(nil)
var _ DeadLetterReplayer = (*deadLetterAdapter)(nil)

func newDeadLetterAdapter(
	brokers []string,
	topic string,
	log logger.Logger,
	cfg deadLetterAdapterConfig,
) *deadLetterAdapter {
	return &deadLetterAdapter{
		queue: pkgkafka.NewDeadLetterQueue(brokers, topic, pkgkafka.DeadLetterConfig{
			GroupID:     cfg.GroupID,
			ClientID:    cfg.ClientID,
			MinBytes:    cfg.MinBytes,
			MaxBytes:    cfg.MaxBytes,
			MaxWait:     cfg.MaxWait,
			PollTimeout: cfg.PollTimeout,
		}, log),
		logger: log,
	}
}

func (q *deadLetterAdapter) Inspect(ctx context.Context, batchSize int) ([]DeadLetterMessage, error) {
	records, err := q.queue.Inspect(ctx, batchSize)
	if err != nil {
		return nil, err
	}

	messages := make([]DeadLetterMessage, 0, len(records))
	for _, record := range records {
		message, decodeErr := q.decodeDeadLetter(record)
		if decodeErr != nil {
			if q.logger != nil {
				q.logger.Error(
					"decode kafka dead letter message failed",
					"topic", record.Topic,
					"partition", record.Partition,
					"offset", record.Offset,
					"error", decodeErr,
				)
			}
			continue
		}
		messages = append(messages, message)
	}

	return messages, nil
}

func (q *deadLetterAdapter) Replay(ctx context.Context, request DeadLetterReplayRequest) (DeadLetterReplayResult, error) {
	result, err := q.queue.Replay(ctx, pkgkafka.DeadLetterReplayRequest{
		BatchSize:   request.BatchSize,
		TargetTopic: request.TargetTopic,
		BuildMessage: func(record pkgkafka.DeadLetterRecord) (pkgkafka.Message, error) {
			return q.buildReplayMessage(record)
		},
	})
	if err != nil {
		return DeadLetterReplayResult{}, err
	}

	return DeadLetterReplayResult{
		Fetched:  result.Fetched,
		Replayed: result.Replayed,
		Skipped:  result.Skipped,
	}, nil
}

func (q *deadLetterAdapter) decodeDeadLetter(record pkgkafka.DeadLetterRecord) (DeadLetterMessage, error) {
	eventName := strings.TrimSpace(pkgkafka.HeaderValue(record.Headers, "event"))
	if eventName == "" {
		return DeadLetterMessage{}, errors.New("dead letter message missing event header")
	}

	messageKey := strings.TrimSpace(string(record.Key))
	if messageKey == "" {
		return DeadLetterMessage{}, errors.New("dead letter message missing key")
	}

	failedAt, err := pkgkafka.ParseRFC3339Header(record.Headers, "failed_at")
	if err != nil {
		return DeadLetterMessage{}, err
	}

	retryCount, err := pkgkafka.ParseInt64Header(record.Headers, "retry_count")
	if err != nil {
		return DeadLetterMessage{}, err
	}
	sourcePartition, err := pkgkafka.ParseIntHeader(record.Headers, "source_partition")
	if err != nil {
		return DeadLetterMessage{}, err
	}
	sourceOffset, err := pkgkafka.ParseInt64Header(record.Headers, "source_offset")
	if err != nil {
		return DeadLetterMessage{}, err
	}
	retryDelaySeconds, err := pkgkafka.ParseIntHeader(record.Headers, "retry_delay_seconds")
	if err != nil {
		return DeadLetterMessage{}, err
	}

	return DeadLetterMessage{
		Message: Message{
			Key:     messageKey,
			Event:   eventName,
			Payload: record.Value,
			TraceID: pkgkafka.HeaderValue(record.Headers, "trace_id"),
		},
		OriginalMessageID: pkgkafka.HeaderValue(record.Headers, "original_message_id"),
		OriginalTopic:     originalTopicFromRecord(record),
		Source:            pkgkafka.HeaderValue(record.Headers, "source_topic"),
		SourcePartition:   sourcePartition,
		SourceOffset:      sourceOffset,
		ConsumerGroup:     pkgkafka.HeaderValue(record.Headers, "consumer_group"),
		Consumer:          pkgkafka.HeaderValue(record.Headers, "consumer"),
		Reason:            pkgkafka.HeaderValue(record.Headers, "reason"),
		RetryCount:        retryCount,
		RetryTopic:        pkgkafka.HeaderValue(record.Headers, "retry_topic"),
		RetryDelaySeconds: retryDelaySeconds,
		FailedAt:          failedAt,
		DeadLetterTopic:   record.Topic,
		DeadLetterOffset:  record.Offset,
		DeadLetterPart:    record.Partition,
	}, nil
}

func (q *deadLetterAdapter) buildReplayMessage(record pkgkafka.DeadLetterRecord) (pkgkafka.Message, error) {
	message, err := q.decodeDeadLetter(record)
	if err != nil {
		return pkgkafka.Message{}, err
	}

	replayedAt := time.Now().UTC()
	headers := []pkgkafka.Header{
		{Key: "event", Value: []byte(message.Event)},
		{Key: "replayed_from_dlq", Value: []byte("true")},
		{Key: "dlq_replayed_at", Value: []byte(replayedAt.Format(time.RFC3339))},
		{Key: "original_message_key", Value: []byte(message.Key)},
	}
	if message.TraceID != "" {
		headers = append(headers, pkgkafka.Header{Key: "trace_id", Value: []byte(message.TraceID)})
	}
	if strings.TrimSpace(message.OriginalTopic) != "" {
		headers = append(headers, pkgkafka.Header{Key: "original_topic", Value: []byte(message.OriginalTopic)})
	}
	if strings.TrimSpace(message.DeadLetterTopic) != "" {
		headers = append(headers, pkgkafka.Header{Key: "dlq_topic", Value: []byte(message.DeadLetterTopic)})
	}
	if strings.TrimSpace(message.Reason) != "" {
		headers = append(headers, pkgkafka.Header{Key: "dlq_reason", Value: []byte(strings.TrimSpace(message.Reason))})
	}
	if !message.FailedAt.IsZero() {
		headers = append(headers, pkgkafka.Header{Key: "dlq_failed_at", Value: []byte(message.FailedAt.UTC().Format(time.RFC3339))})
	}
	if strings.TrimSpace(message.Source) != "" {
		headers = append(headers, pkgkafka.Header{Key: "dlq_original_source_topic", Value: []byte(message.Source)})
	}
	if message.DeadLetterPart >= 0 {
		headers = append(headers, pkgkafka.Header{Key: "dlq_source_partition", Value: []byte(strconv.Itoa(message.DeadLetterPart))})
	}
	if message.DeadLetterOffset >= 0 {
		headers = append(headers, pkgkafka.Header{Key: "dlq_source_offset", Value: []byte(strconv.FormatInt(message.DeadLetterOffset, 10))})
	}

	return pkgkafka.Message{
		Key:     []byte(newReplayMessageKey(message.Key, replayedAt)),
		Value:   message.Payload,
		Headers: headers,
	}, nil
}

func originalTopicFromRecord(record pkgkafka.DeadLetterRecord) string {
	if value := pkgkafka.HeaderValue(record.Headers, "original_topic"); strings.TrimSpace(value) != "" {
		return value
	}
	return record.Topic
}

func newReplayMessageKey(originalKey string, replayedAt time.Time) string {
	sum := sha1.Sum([]byte(strings.TrimSpace(originalKey) + "|" + replayedAt.UTC().Format(time.RFC3339Nano)))
	return "dlq-replay-" + hex.EncodeToString(sum[:])
}
