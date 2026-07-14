package main

import (
	"fmt"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/config"
	"github.com/freeDog-wy/go-backend-template/internal/infra/mq"
	repoAudit "github.com/freeDog-wy/go-backend-template/internal/repository/audit"
	repoConsumption "github.com/freeDog-wy/go-backend-template/internal/repository/consumption"
	"github.com/freeDog-wy/go-backend-template/pkg/kafka"
	"gorm.io/gorm"
)

type workerRepositories struct {
	audit       *repoAudit.Repository
	consumption *repoConsumption.Repository
}

func newWorkerRepositories(db *gorm.DB) *workerRepositories {
	return &workerRepositories{
		audit:       repoAudit.New(db),
		consumption: repoConsumption.New(db),
	}
}

func newWorkerConsumer(cfg *config.Config, infra *workerInfrastructure, repos *workerRepositories) (mq.Consumer, error) {
	consumer, err := mq.NewConsumer(workerConsumerOptions(cfg), repos.consumption, infra.logger)
	if err != nil {
		return nil, fmt.Errorf("initialize kafka consumer: %w", err)
	}
	return consumer, nil
}

func workerConsumerOptions(cfg *config.Config) mq.ConsumerOptions {
	retryLevels := make([]kafka.RetryLevel, 0, len(cfg.Worker.KafkaRetryTopics))
	for _, level := range cfg.Worker.KafkaRetryTopics {
		retryLevels = append(retryLevels, kafka.RetryLevel{
			Topic: level.Topic,
			Delay: time.Duration(level.DelaySeconds) * time.Second,
		})
	}
	return mq.ConsumerOptions{
		KafkaOptions: mq.KafkaOptions{
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
		DeadLetterTopic:   mq.ResolveDeadLetterTopic(cfg.MQ.EventsName, cfg.Worker.KafkaDeadLetterTopic),
	}
}
