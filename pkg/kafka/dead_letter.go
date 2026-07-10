package kafka

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/freeDog-wy/go-backend-template/pkg/logger"
	kgo "github.com/segmentio/kafka-go"
)

type DeadLetterRecord struct {
	Key       []byte
	Value     []byte
	Headers   []Header
	Topic     string
	Partition int
	Offset    int64
}

type DeadLetterReplayRequest struct {
	BatchSize    int
	TargetTopic  string
	BuildMessage func(record DeadLetterRecord) (Message, error)
}

type DeadLetterReplayResult struct {
	Fetched  int
	Replayed int
	Skipped  int
}

type DeadLetterQueue struct {
	brokers     []string
	topic       string
	clientID    string
	reader      *kgo.Reader
	pollTimeout time.Duration
	logger      logger.Logger

	writerMu sync.Mutex
	writers  map[string]*Publisher
}

func NewDeadLetterQueue(brokers []string, topic string, cfg DeadLetterConfig, log logger.Logger) *DeadLetterQueue {
	normalizedBrokers := NormalizeBrokers(brokers)
	if len(normalizedBrokers) == 0 {
		panic("kafka brokers must not be empty")
	}
	if strings.TrimSpace(topic) == "" {
		panic("kafka dead letter topic must not be empty")
	}
	if strings.TrimSpace(cfg.GroupID) == "" {
		panic("kafka dead letter group id must not be empty")
	}
	if cfg.MinBytes <= 0 {
		cfg.MinBytes = 1024
	}
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = 10 * 1024 * 1024
	}
	if cfg.MaxWait <= 0 {
		cfg.MaxWait = time.Second
	}
	if cfg.PollTimeout <= 0 {
		cfg.PollTimeout = cfg.MaxWait
	}

	return &DeadLetterQueue{
		brokers:     normalizedBrokers,
		topic:       strings.TrimSpace(topic),
		clientID:    strings.TrimSpace(cfg.ClientID),
		reader:      NewReader(normalizedBrokers, topic, ReaderConfig{GroupID: cfg.GroupID, ClientID: cfg.ClientID, MinBytes: cfg.MinBytes, MaxBytes: cfg.MaxBytes, MaxWait: cfg.MaxWait, StartOffset: kgo.FirstOffset}),
		pollTimeout: cfg.PollTimeout,
		logger:      log,
		writers:     make(map[string]*Publisher),
	}
}

func (q *DeadLetterQueue) Inspect(ctx context.Context, batchSize int) ([]DeadLetterRecord, error) {
	if batchSize <= 0 {
		batchSize = 20
	}

	records := make([]DeadLetterRecord, 0, batchSize)
	for len(records) < batchSize {
		msg, err := q.fetchMessage(ctx)
		if err != nil {
			if shouldStopDeadLetterPolling(ctx, err) {
				break
			}
			return nil, err
		}

		records = append(records, deadLetterRecordFromKafkaMessage(msg))
		if err := q.reader.CommitMessages(ctx, msg); err != nil {
			return nil, err
		}
	}

	return records, nil
}

func (q *DeadLetterQueue) Replay(ctx context.Context, request DeadLetterReplayRequest) (DeadLetterReplayResult, error) {
	if request.BatchSize <= 0 {
		request.BatchSize = 20
	}
	if request.BuildMessage == nil {
		return DeadLetterReplayResult{}, errors.New("dead letter replay builder must not be nil")
	}

	targetTopic := strings.TrimSpace(request.TargetTopic)
	if targetTopic == "" {
		return DeadLetterReplayResult{}, errors.New("dead letter replay target topic must not be empty")
	}

	publisher := q.publisherFor(targetTopic)
	result := DeadLetterReplayResult{}
	for result.Fetched < request.BatchSize {
		msg, err := q.fetchMessage(ctx)
		if err != nil {
			if shouldStopDeadLetterPolling(ctx, err) {
				break
			}
			return result, err
		}
		result.Fetched++

		record := deadLetterRecordFromKafkaMessage(msg)
		replayMessage, buildErr := request.BuildMessage(record)
		if buildErr != nil {
			result.Skipped++
			if q.logger != nil {
				q.logger.Error("build kafka dead letter replay message failed", "topic", msg.Topic, "partition", msg.Partition, "offset", msg.Offset, "error", buildErr)
			}
			if err := q.reader.CommitMessages(ctx, msg); err != nil {
				return result, err
			}
			continue
		}

		if err := publisher.Publish(ctx, replayMessage); err != nil {
			return result, err
		}
		if err := q.reader.CommitMessages(ctx, msg); err != nil {
			return result, err
		}
		result.Replayed++
	}

	return result, nil
}

func (q *DeadLetterQueue) fetchMessage(ctx context.Context) (kgo.Message, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, q.pollTimeout)
	defer cancel()

	return q.reader.FetchMessage(fetchCtx)
}

func (q *DeadLetterQueue) publisherFor(topic string) *Publisher {
	normalizedTopic := strings.TrimSpace(topic)

	q.writerMu.Lock()
	defer q.writerMu.Unlock()

	if publisher, ok := q.writers[normalizedTopic]; ok {
		return publisher
	}

	publisher := NewPublisher(q.brokers, normalizedTopic, WriterConfig{ClientID: q.clientID})
	q.writers[normalizedTopic] = publisher
	return publisher
}

func deadLetterRecordFromKafkaMessage(msg kgo.Message) DeadLetterRecord {
	return DeadLetterRecord{
		Key:       msg.Key,
		Value:     msg.Value,
		Headers:   msg.Headers,
		Topic:     msg.Topic,
		Partition: msg.Partition,
		Offset:    msg.Offset,
	}
}

func shouldStopDeadLetterPolling(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return ctx.Err() == nil && errors.Is(err, context.Canceled)
}
