package kafka

import (
	"context"
	"strings"
	"time"

	kgo "github.com/segmentio/kafka-go"
)

type RetryLevel struct {
	Topic string
	Delay time.Duration
}

type RetryPublisher struct {
	Topic     string
	Delay     time.Duration
	Publisher *Publisher
}

type ConsumerTopologyConfig struct {
	GroupID         string
	ClientID        string
	MinBytes        int
	MaxBytes        int
	MaxWait         time.Duration
	RetryLevels     []RetryLevel
	DeadLetterTopic string
}

// ConsumerTopology 描述一个主 topic 加多层 retry topic 的消费拓扑。
type ConsumerTopology struct {
	Readers             []ReaderLoop
	RetryPublishers     []RetryPublisher
	DeadLetterPublisher *Publisher
	DeadLetterTopic     string

	runner *ConsumerRunner
}

func NewConsumerTopology(brokers []string, topic string, cfg ConsumerTopologyConfig) *ConsumerTopology {
	normalizedBrokers := NormalizeBrokers(brokers)
	if len(normalizedBrokers) == 0 {
		panic("kafka brokers must not be empty")
	}

	mainTopic := strings.TrimSpace(topic)
	if mainTopic == "" {
		panic("kafka topic must not be empty")
	}
	if strings.TrimSpace(cfg.GroupID) == "" {
		panic("kafka consumer group id must not be empty")
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

	deadLetterTopic := strings.TrimSpace(cfg.DeadLetterTopic)
	if deadLetterTopic == "" {
		deadLetterTopic = mainTopic + ".dlq"
	}

	readerCfg := ReaderConfig{
		GroupID:     cfg.GroupID,
		ClientID:    cfg.ClientID,
		MinBytes:    cfg.MinBytes,
		MaxBytes:    cfg.MaxBytes,
		MaxWait:     cfg.MaxWait,
		StartOffset: kgo.LastOffset,
	}

	retryLevels := normalizeRetryLevels(cfg.RetryLevels)
	readers := make([]ReaderLoop, 0, 1+len(retryLevels))
	readers = append(readers, NewReaderLoop(normalizedBrokers, mainTopic, 0, readerCfg))

	retryPublishers := make([]RetryPublisher, 0, len(retryLevels))
	for _, level := range retryLevels {
		readers = append(readers, NewReaderLoop(normalizedBrokers, level.Topic, level.Delay, readerCfg))
		retryPublishers = append(retryPublishers, RetryPublisher{
			Topic:     level.Topic,
			Delay:     level.Delay,
			Publisher: NewPublisher(normalizedBrokers, level.Topic, WriterConfig{ClientID: cfg.ClientID}),
		})
	}

	return &ConsumerTopology{
		Readers:             readers,
		RetryPublishers:     retryPublishers,
		DeadLetterPublisher: NewPublisher(normalizedBrokers, deadLetterTopic, WriterConfig{ClientID: cfg.ClientID}),
		DeadLetterTopic:     deadLetterTopic,
		runner:              NewConsumerRunner(readers),
	}
}

func (t *ConsumerTopology) Run(ctx context.Context, handler LoopHandler) error {
	return t.runner.Run(ctx, handler)
}

func (t *ConsumerTopology) Topics() []string {
	topics := make([]string, 0, len(t.Readers))
	for _, reader := range t.Readers {
		topics = append(topics, reader.Topic)
	}
	return topics
}

func (t *ConsumerTopology) RetryTopics() []string {
	topics := make([]string, 0, len(t.RetryPublishers))
	for _, publisher := range t.RetryPublishers {
		topics = append(topics, publisher.Topic)
	}
	return topics
}

func normalizeRetryLevels(levels []RetryLevel) []RetryLevel {
	result := make([]RetryLevel, 0, len(levels))
	seen := make(map[string]struct{}, len(levels))
	for _, level := range levels {
		topic := strings.TrimSpace(level.Topic)
		if topic == "" || level.Delay <= 0 {
			continue
		}
		if _, ok := seen[topic]; ok {
			continue
		}
		seen[topic] = struct{}{}
		result = append(result, RetryLevel{
			Topic: topic,
			Delay: level.Delay,
		})
	}
	return result
}
