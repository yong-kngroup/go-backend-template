package main

import (
	"fmt"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/config"
	kafkaConfig "github.com/freeDog-wy/go-backend-template/internal/infra/kafka/config"
	"github.com/freeDog-wy/go-backend-template/internal/infra/kafka/consumer"
	"github.com/freeDog-wy/go-backend-template/internal/infra/kafka/dlq"
	platformMessaging "github.com/freeDog-wy/go-backend-template/internal/platform/messaging"
	"gorm.io/gorm"
)

type workerPlatform struct {
	consumption *platformMessaging.Repository
}

func newWorkerPlatform(db *gorm.DB) *workerPlatform {
	return &workerPlatform{
		consumption: platformMessaging.New(db),
	}
}

func newWorkerConsumer(cfg *config.Config, infra *workerInfrastructure, platform *workerPlatform) (consumer.Consumer, error) {
	consumer, err := consumer.New(workerConsumerOptions(cfg), platform.consumption, infra.logger)
	if err != nil {
		return nil, fmt.Errorf("initialize kafka consumer: %w", err)
	}
	return consumer, nil
}

func workerConsumerOptions(cfg *config.Config) kafkaConfig.Consumer {
	retryLevels := make([]kafkaConfig.RetryLevel, 0, len(cfg.Worker.KafkaRetryTopics))
	for _, level := range cfg.Worker.KafkaRetryTopics {
		retryLevels = append(retryLevels, kafkaConfig.RetryLevel{
			Topic: level.Topic,
			Delay: time.Duration(level.DelaySeconds) * time.Second,
		})
	}
	return kafkaConfig.Consumer{
		Connection: kafkaConfig.Connection{
			Brokers:  cfg.MQ.Kafka.Brokers,
			Topic:    cfg.MQ.EventsName,
			ClientID: cfg.MQ.Kafka.ClientID,
		},
		GroupID:           cfg.Worker.ConsumerGroup,
		MaxRetries:        cfg.Worker.ConsumerMaxRetries,
		ProcessingLockTTL: time.Duration(cfg.Worker.ConsumerProcessingLockSeconds) * time.Second,
		MinBytes:          cfg.Worker.KafkaReadMinBytes,
		MaxBytes:          cfg.Worker.KafkaReadMaxBytes,
		MaxWait:           time.Duration(cfg.Worker.KafkaMaxWaitSeconds) * time.Second,
		RetryLevels:       retryLevels,
		DeadLetterTopic:   dlq.ResolveTopic(cfg.MQ.EventsName, cfg.Worker.KafkaDeadLetterTopic),
	}
}
