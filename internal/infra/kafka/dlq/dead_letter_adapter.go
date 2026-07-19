package dlq

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/infra/kafka/event"
	"github.com/freeDog-wy/go-backend-template/internal/infra/kafka/internal/client"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
	kgo "github.com/segmentio/kafka-go"
)

type deadLetterAdapter struct {
	queue  *deadLetterQueue
	logger logger.Logger
}

var _ Inspector = (*deadLetterAdapter)(nil)
var _ Replayer = (*deadLetterAdapter)(nil)

type deadLetterAdapterConfig struct {
	GroupID, ClientID    string
	MinBytes, MaxBytes   int
	MaxWait, PollTimeout time.Duration
}

type deadLetterQueue struct {
	reader      *kgo.Reader
	brokers     []string
	clientID    string
	pollTimeout time.Duration
	logger      logger.Logger
	writerMu    sync.Mutex
	writers     map[string]*kgo.Writer
}

func newDeadLetterAdapter(brokers []string, topic string, log logger.Logger, cfg deadLetterAdapterConfig) *deadLetterAdapter {
	return &deadLetterAdapter{queue: &deadLetterQueue{reader: client.NewReader(brokers, topic, cfg.GroupID, cfg.ClientID, cfg.MinBytes, cfg.MaxBytes, cfg.MaxWait, kgo.FirstOffset), brokers: client.NormalizeBrokers(brokers), clientID: cfg.ClientID, pollTimeout: cfg.PollTimeout, logger: log, writers: make(map[string]*kgo.Writer)}, logger: log}
}
func (q *deadLetterQueue) fetch(ctx context.Context) (kgo.Message, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, q.pollTimeout)
	defer cancel()
	return q.reader.FetchMessage(fetchCtx)
}
func (q *deadLetterQueue) writer(topic string) *kgo.Writer {
	q.writerMu.Lock()
	defer q.writerMu.Unlock()
	if w := q.writers[topic]; w != nil {
		return w
	}
	w := client.NewWriter(q.brokers, topic, q.clientID)
	q.writers[topic] = w
	return w
}
func stopPolling(ctx context.Context, err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || (ctx.Err() == nil && errors.Is(err, context.Canceled))
}

func (q *deadLetterQueue) inspect(ctx context.Context, batchSize int) ([]kgo.Message, error) {
	if batchSize <= 0 {
		batchSize = 20
	}
	messages := make([]kgo.Message, 0, batchSize)
	for len(messages) < batchSize {
		message, err := q.fetch(ctx)
		if err != nil {
			if stopPolling(ctx, err) {
				break
			}
			return nil, err
		}
		messages = append(messages, message)
		if err := q.reader.CommitMessages(ctx, message); err != nil {
			return nil, err
		}
	}
	return messages, nil
}
func (q *deadLetterQueue) replay(ctx context.Context, batchSize int, target string, build func(kgo.Message) (kgo.Message, error)) (ReplayResult, error) {
	if batchSize <= 0 {
		batchSize = 20
	}
	result := ReplayResult{}
	writer := q.writer(target)
	for result.Fetched < batchSize {
		message, err := q.fetch(ctx)
		if err != nil {
			if stopPolling(ctx, err) {
				break
			}
			return result, err
		}
		result.Fetched++
		replay, err := build(message)
		if err != nil {
			result.Skipped++
			if err := q.reader.CommitMessages(ctx, message); err != nil {
				return result, err
			}
			continue
		}
		if err := writer.WriteMessages(ctx, replay); err != nil {
			return result, err
		}
		if err := q.reader.CommitMessages(ctx, message); err != nil {
			return result, err
		}
		result.Replayed++
	}
	return result, nil
}

func (q *deadLetterAdapter) Inspect(ctx context.Context, batchSize int) ([]event.DeadLetter, error) {
	records, err := q.queue.inspect(ctx, batchSize)
	if err != nil {
		return nil, err
	}
	messages := make([]event.DeadLetter, 0, len(records))
	for _, record := range records {
		message, err := q.decodeDeadLetter(record)
		if err != nil {
			if q.logger != nil {
				q.logger.Error("decode kafka dead letter message failed", "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "error", err)
			}
			continue
		}
		messages = append(messages, message)
	}
	return messages, nil
}
func (q *deadLetterAdapter) Replay(ctx context.Context, request ReplayRequest) (ReplayResult, error) {
	return q.queue.replay(ctx, request.BatchSize, request.TargetTopic, q.buildReplayMessage)
}

func (q *deadLetterAdapter) decodeDeadLetter(record kgo.Message) (event.DeadLetter, error) {
	eventName := strings.TrimSpace(client.HeaderValue(record.Headers, "event"))
	if eventName == "" {
		return event.DeadLetter{}, errors.New("dead letter message missing event header")
	}
	key := strings.TrimSpace(string(record.Key))
	if key == "" {
		return event.DeadLetter{}, errors.New("dead letter message missing key")
	}
	failedAt, err := client.ParseRFC3339Header(record.Headers, "failed_at")
	if err != nil {
		return event.DeadLetter{}, err
	}
	retryCount, err := client.ParseInt64Header(record.Headers, "retry_count")
	if err != nil {
		return event.DeadLetter{}, err
	}
	partition, err := client.ParseIntHeader(record.Headers, "source_partition")
	if err != nil {
		return event.DeadLetter{}, err
	}
	offset, err := client.ParseInt64Header(record.Headers, "source_offset")
	if err != nil {
		return event.DeadLetter{}, err
	}
	delay, err := client.ParseIntHeader(record.Headers, "retry_delay_seconds")
	if err != nil {
		return event.DeadLetter{}, err
	}
	return event.DeadLetter{Message: event.Event{Key: key, Event: eventName, Payload: record.Value, TraceID: client.TraceIDFromHeaders(record.Headers), TraceContext: client.SerializeHeadersTraceContext(record.Headers)}, OriginalMessageID: client.HeaderValue(record.Headers, "original_message_id"), OriginalTopic: originalTopic(record), Source: client.HeaderValue(record.Headers, "source_topic"), SourcePartition: partition, SourceOffset: offset, ConsumerGroup: client.HeaderValue(record.Headers, "consumer_group"), Consumer: client.HeaderValue(record.Headers, "consumer"), Reason: client.HeaderValue(record.Headers, "reason"), RetryCount: retryCount, RetryTopic: client.HeaderValue(record.Headers, "retry_topic"), RetryDelaySeconds: delay, FailedAt: failedAt, DeadLetterTopic: record.Topic, DeadLetterOffset: record.Offset, DeadLetterPart: record.Partition}, nil
}
func (q *deadLetterAdapter) buildReplayMessage(record kgo.Message) (kgo.Message, error) {
	message, err := q.decodeDeadLetter(record)
	if err != nil {
		return kgo.Message{}, err
	}
	replayedAt := time.Now().UTC()
	headers := []kgo.Header{{Key: "event", Value: []byte(message.Message.Event)}, {Key: "replayed_from_dlq", Value: []byte("true")}, {Key: "dlq_replayed_at", Value: []byte(replayedAt.Format(time.RFC3339))}, {Key: "original_message_key", Value: []byte(message.Message.Key)}}
	if message.Message.TraceID != "" {
		headers = append(headers, kgo.Header{Key: "trace_id", Value: []byte(message.Message.TraceID)})
	}
	if message.OriginalTopic != "" {
		headers = append(headers, kgo.Header{Key: "original_topic", Value: []byte(message.OriginalTopic)})
	}
	if message.DeadLetterTopic != "" {
		headers = append(headers, kgo.Header{Key: "dlq_topic", Value: []byte(message.DeadLetterTopic)})
	}
	if message.Reason != "" {
		headers = append(headers, kgo.Header{Key: "dlq_reason", Value: []byte(message.Reason)})
	}
	if !message.FailedAt.IsZero() {
		headers = append(headers, kgo.Header{Key: "dlq_failed_at", Value: []byte(message.FailedAt.UTC().Format(time.RFC3339))})
	}
	if message.Source != "" {
		headers = append(headers, kgo.Header{Key: "dlq_original_source_topic", Value: []byte(message.Source)})
	}
	if message.DeadLetterPart >= 0 {
		headers = append(headers, kgo.Header{Key: "dlq_source_partition", Value: []byte(strconv.Itoa(message.DeadLetterPart))})
	}
	if message.DeadLetterOffset >= 0 {
		headers = append(headers, kgo.Header{Key: "dlq_source_offset", Value: []byte(strconv.FormatInt(message.DeadLetterOffset, 10))})
	}
	if message.Message.TraceContext != "" {
		headers = client.InjectTraceContext(client.ContextWithSerializedTraceContext(context.Background(), message.Message.TraceContext), headers)
	}
	return kgo.Message{Key: []byte(newReplayMessageKey(message.Message.Key, replayedAt)), Value: message.Message.Payload, Headers: headers}, nil
}
func newReplayMessageKey(key string, at time.Time) string {
	sum := sha1.Sum([]byte(strings.TrimSpace(key) + "|" + at.UTC().Format(time.RFC3339Nano)))
	return "dlq-replay-" + hex.EncodeToString(sum[:])
}

func originalTopic(record kgo.Message) string {
	if value := strings.TrimSpace(client.HeaderValue(record.Headers, "original_topic")); value != "" {
		return value
	}
	return record.Topic
}
