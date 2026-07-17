package mq

import (
	"fmt"
	"strings"
	"time"

	pkgkafka "github.com/freeDog-wy/go-backend-template/pkg/kafka"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
)

type KafkaOptions struct {
	Brokers  []string
	Topic    string
	ClientID string
}

type ConsumerOptions struct {
	KafkaOptions
	StartOffset       *int64
	GroupID           string
	MaxRetries        int
	ProcessingLockTTL time.Duration
	MinBytes          int
	MaxBytes          int
	MaxWait           time.Duration
	RetryLevels       []pkgkafka.RetryLevel
	DeadLetterTopic   string
}

type DeadLetterOptions struct {
	KafkaOptions
	GroupID     string
	MinBytes    int
	MaxBytes    int
	MaxWait     time.Duration
	PollTimeout time.Duration
}

func NewPublisher(options KafkaOptions, log logger.Logger) (Publisher, error) {
	if err := options.validate("kafka publisher"); err != nil {
		return nil, err
	}
	return newPublisherAdapter(options.Brokers, options.Topic, options.ClientID, log), nil
}

func NewConsumer(options ConsumerOptions, records ConsumptionStore, log logger.Logger) (Consumer, error) {
	if err := options.KafkaOptions.validate("kafka consumer"); err != nil {
		return nil, err
	}
	if strings.TrimSpace(options.GroupID) == "" {
		return nil, fmt.Errorf("kafka consumer group id is required")
	}
	if records == nil {
		return nil, fmt.Errorf("kafka consumption repository is required")
	}
	if options.ProcessingLockTTL <= 0 || options.MaxRetries <= 0 || options.MinBytes <= 0 || options.MaxBytes <= 0 || options.MaxWait <= 0 {
		return nil, fmt.Errorf("kafka consumer retry, lock and reader settings must be greater than zero")
	}
	if err := validateRetryLevels(options.RetryLevels); err != nil {
		return nil, err
	}

	return newConsumerAdapter(options.Brokers, options.Topic, log, records, consumerAdapterConfig{
		GroupID:           options.GroupID,
		ClientID:          options.ClientID,
		MinBytes:          options.MinBytes,
		MaxBytes:          options.MaxBytes,
		MaxWait:           options.MaxWait,
		StartOffset:       options.StartOffset,
		ProcessingLockTTL: options.ProcessingLockTTL,
		MaxRetries:        options.MaxRetries,
		RetryLevels:       options.RetryLevels,
		DeadLetterTopic:   options.DeadLetterTopic,
	}), nil
}

func NewDeadLetterInspector(options DeadLetterOptions, log logger.Logger) (DeadLetterInspector, error) {
	adapter, err := newDeadLetterAdapterFromOptions(options, log)
	if err != nil {
		return nil, err
	}
	return adapter, nil
}

func NewDeadLetterReplayer(options DeadLetterOptions, log logger.Logger) (DeadLetterReplayer, error) {
	adapter, err := newDeadLetterAdapterFromOptions(options, log)
	if err != nil {
		return nil, err
	}
	return adapter, nil
}

func ResolveDeadLetterTopic(topic, configuredTopic string) string {
	if configuredTopic = strings.TrimSpace(configuredTopic); configuredTopic != "" {
		return configuredTopic
	}
	return strings.TrimSpace(topic) + ".dlq"
}

func ResolveDeadLetterReplayTarget(mainTopic, configuredTarget string, retryLevels []pkgkafka.RetryLevel) (string, error) {
	switch strings.ToLower(strings.TrimSpace(configuredTarget)) {
	case "", "main":
		return strings.TrimSpace(mainTopic), nil
	case "first_retry":
		if len(retryLevels) == 0 {
			return "", fmt.Errorf("cron.dlq_replay_target=first_retry requires at least one retry topic")
		}
		return retryLevels[0].Topic, nil
	default:
		return strings.TrimSpace(configuredTarget), nil
	}
}

func newDeadLetterAdapterFromOptions(options DeadLetterOptions, log logger.Logger) (*deadLetterAdapter, error) {
	if err := options.KafkaOptions.validate("kafka dead letter"); err != nil {
		return nil, err
	}
	if strings.TrimSpace(options.GroupID) == "" || options.MinBytes <= 0 || options.MaxBytes <= 0 || options.MaxWait <= 0 || options.PollTimeout <= 0 {
		return nil, fmt.Errorf("kafka dead letter group and reader settings must be configured")
	}
	return newDeadLetterAdapter(options.Brokers, options.Topic, log, deadLetterAdapterConfig{
		GroupID:     options.GroupID,
		ClientID:    options.ClientID,
		MinBytes:    options.MinBytes,
		MaxBytes:    options.MaxBytes,
		MaxWait:     options.MaxWait,
		PollTimeout: options.PollTimeout,
	}), nil
}

func (options KafkaOptions) validate(component string) error {
	if len(pkgkafka.NormalizeBrokers(options.Brokers)) == 0 {
		return fmt.Errorf("%s brokers are required", component)
	}
	if strings.TrimSpace(options.Topic) == "" {
		return fmt.Errorf("%s topic is required", component)
	}
	return nil
}

func validateRetryLevels(levels []pkgkafka.RetryLevel) error {
	seen := make(map[string]struct{}, len(levels))
	for _, level := range levels {
		topic := strings.TrimSpace(level.Topic)
		if topic == "" || level.Delay <= 0 {
			return fmt.Errorf("kafka retry topic and delay must be configured")
		}
		if _, exists := seen[topic]; exists {
			return fmt.Errorf("kafka retry topic %q is duplicated", topic)
		}
		seen[topic] = struct{}{}
	}
	return nil
}
