package mq

import (
	"strings"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/config"
	domainConsumption "github.com/freeDog-wy/go-backend-template/internal/domain/consumption"
	pkgkafka "github.com/freeDog-wy/go-backend-template/pkg/kafka"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
)

func NewPublisherFromConfig(cfg *config.Config, log logger.Logger) Publisher {
	return newPublisherAdapter(cfg.MQ.Kafka.Brokers, cfg.MQ.EventsName, cfg.MQ.Kafka.ClientID, log)
}

func NewConsumerFromConfig(cfg *config.Config, records domainConsumption.Repository, log logger.Logger) Consumer {
	retryLevels := make([]pkgkafka.RetryLevel, 0, len(cfg.Worker.KafkaRetryTopics))
	for _, level := range cfg.Worker.KafkaRetryTopics {
		retryLevels = append(retryLevels, pkgkafka.RetryLevel{
			Topic: level.Topic,
			Delay: time.Duration(level.DelaySeconds) * time.Second,
		})
	}

	return newConsumerAdapter(
		cfg.MQ.Kafka.Brokers,
		cfg.MQ.EventsName,
		log,
		records,
		consumerAdapterConfig{
			GroupID:           cfg.Worker.ConsumerGroup,
			ClientID:          cfg.MQ.Kafka.ClientID,
			MinBytes:          cfg.Worker.KafkaReadMinBytes,
			MaxBytes:          cfg.Worker.KafkaReadMaxBytes,
			MaxWait:           time.Duration(cfg.Worker.KafkaMaxWaitSeconds) * time.Second,
			ProcessingLockTTL: time.Duration(cfg.Worker.ConsumerProcessingLockSeconds) * time.Second,
			MaxRetries:        cfg.Worker.ConsumerMaxRetries,
			RetryLevels:       retryLevels,
			DeadLetterTopic:   ResolveDeadLetterTopicFromConfig(cfg),
		},
	)
}

func NewDeadLetterInspectorFromConfig(cfg *config.Config, groupID string, log logger.Logger) DeadLetterInspector {
	return newDeadLetterAdapter(
		cfg.MQ.Kafka.Brokers,
		ResolveDeadLetterTopicFromConfig(cfg),
		log,
		deadLetterAdapterConfig{
			GroupID:     strings.TrimSpace(groupID),
			ClientID:    cfg.MQ.Kafka.ClientID,
			MinBytes:    cfg.Worker.KafkaReadMinBytes,
			MaxBytes:    cfg.Worker.KafkaReadMaxBytes,
			MaxWait:     time.Duration(cfg.Worker.KafkaMaxWaitSeconds) * time.Second,
			PollTimeout: time.Duration(cfg.Worker.KafkaMaxWaitSeconds) * time.Second,
		},
	)
}

func NewDeadLetterReplayerFromConfig(cfg *config.Config, groupID string, log logger.Logger) DeadLetterReplayer {
	return newDeadLetterAdapter(
		cfg.MQ.Kafka.Brokers,
		ResolveDeadLetterTopicFromConfig(cfg),
		log,
		deadLetterAdapterConfig{
			GroupID:     strings.TrimSpace(groupID),
			ClientID:    cfg.MQ.Kafka.ClientID,
			MinBytes:    cfg.Worker.KafkaReadMinBytes,
			MaxBytes:    cfg.Worker.KafkaReadMaxBytes,
			MaxWait:     time.Duration(cfg.Worker.KafkaMaxWaitSeconds) * time.Second,
			PollTimeout: time.Duration(cfg.Worker.KafkaMaxWaitSeconds) * time.Second,
		},
	)
}

func ResolveDeadLetterTopicFromConfig(cfg *config.Config) string {
	if strings.TrimSpace(cfg.Worker.KafkaDeadLetterTopic) != "" {
		return strings.TrimSpace(cfg.Worker.KafkaDeadLetterTopic)
	}
	return strings.TrimSpace(cfg.MQ.EventsName) + ".dlq"
}

func ResolveDeadLetterReplayTargetFromConfig(cfg *config.Config) string {
	target := strings.ToLower(strings.TrimSpace(cfg.Cron.DLQReplayTarget))
	switch target {
	case "", "main":
		return strings.TrimSpace(cfg.MQ.EventsName)
	case "first_retry":
		if len(cfg.Worker.KafkaRetryTopics) == 0 {
			panic("cron.dlq_replay_target=first_retry requires worker.kafka_retry_topics")
		}
		return strings.TrimSpace(cfg.Worker.KafkaRetryTopics[0].Topic)
	default:
		return strings.TrimSpace(cfg.Cron.DLQReplayTarget)
	}
}
