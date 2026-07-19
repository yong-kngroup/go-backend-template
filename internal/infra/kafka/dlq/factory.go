package dlq

import (
	"fmt"
	"strings"

	"github.com/freeDog-wy/go-backend-template/internal/infra/kafka/config"
	"github.com/freeDog-wy/go-backend-template/internal/infra/kafka/internal/client"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
)

func NewInspector(options config.DeadLetter, log logger.Logger) (Inspector, error) {
	adapter, err := newAdapter(options, log)
	if err != nil {
		return nil, err
	}
	return adapter, nil
}

func NewReplayer(options config.DeadLetter, log logger.Logger) (Replayer, error) {
	adapter, err := newAdapter(options, log)
	if err != nil {
		return nil, err
	}
	return adapter, nil
}

func ResolveTopic(topic, configuredTopic string) string {
	if configuredTopic = strings.TrimSpace(configuredTopic); configuredTopic != "" {
		return configuredTopic
	}
	return strings.TrimSpace(topic) + ".dlq"
}

func ResolveReplayTarget(mainTopic, configuredTarget string, retryLevels []config.RetryLevel) (string, error) {
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

func newAdapter(options config.DeadLetter, log logger.Logger) (*deadLetterAdapter, error) {
	if len(client.NormalizeBrokers(options.Brokers)) == 0 || strings.TrimSpace(options.Topic) == "" {
		return nil, fmt.Errorf("kafka dead letter brokers and topic are required")
	}
	if strings.TrimSpace(options.GroupID) == "" || options.MinBytes <= 0 || options.MaxBytes <= 0 || options.MaxWait <= 0 || options.PollTimeout <= 0 {
		return nil, fmt.Errorf("kafka dead letter group and reader settings must be configured")
	}
	return newDeadLetterAdapter(options.Brokers, options.Topic, log, deadLetterAdapterConfig{GroupID: options.GroupID, ClientID: options.ClientID, MinBytes: options.MinBytes, MaxBytes: options.MaxBytes, MaxWait: options.MaxWait, PollTimeout: options.PollTimeout}), nil
}
