package consumer

import (
	"fmt"
	"strings"

	"github.com/freeDog-wy/go-backend-template/internal/infra/kafka/config"
	"github.com/freeDog-wy/go-backend-template/internal/infra/kafka/internal/client"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
)

func New(options config.Consumer, records ConsumptionStore, log logger.Logger) (Consumer, error) {
	if len(client.NormalizeBrokers(options.Brokers)) == 0 || strings.TrimSpace(options.Topic) == "" {
		return nil, fmt.Errorf("kafka consumer brokers and topic are required")
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
		GroupID: options.GroupID, ClientID: options.ClientID, MinBytes: options.MinBytes, MaxBytes: options.MaxBytes,
		MaxWait: options.MaxWait, StartOffset: options.StartOffset, ProcessingLockTTL: options.ProcessingLockTTL,
		MaxRetries: options.MaxRetries, RetryLevels: options.RetryLevels, DeadLetterTopic: options.DeadLetterTopic,
	}), nil
}

func validateRetryLevels(levels []config.RetryLevel) error {
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
