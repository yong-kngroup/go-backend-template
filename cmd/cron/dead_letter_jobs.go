package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/config"
	kafkaConfig "github.com/freeDog-wy/go-backend-template/internal/infra/kafka/config"
	"github.com/freeDog-wy/go-backend-template/internal/infra/kafka/dlq"
	platformMessaging "github.com/freeDog-wy/go-backend-template/internal/platform/messaging"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
	"github.com/freeDog-wy/go-backend-template/pkg/scheduler"
)

func registerKafkaDLQJobs(cfg *config.Config, appLogger logger.Logger, runner *scheduler.Runner) error {
	if cfg.Cron.DLQInspectionEnabled {
		if cfg.Cron.DLQInspectionIntervalSeconds <= 0 {
			return fmt.Errorf("cron.dlq_inspection_interval_seconds must be greater than zero")
		}
		if cfg.Cron.DLQInspectionBatchSize <= 0 {
			return fmt.Errorf("cron.dlq_inspection_batch_size must be greater than zero")
		}
		if strings.TrimSpace(cfg.Cron.DLQInspectionGroup) == "" {
			return fmt.Errorf("cron.dlq_inspection_group must not be empty")
		}

		inspector, err := dlq.NewInspector(deadLetterOptions(cfg, cfg.Cron.DLQInspectionGroup), appLogger)
		if err != nil {
			return fmt.Errorf("initialize kafka dead letter inspector: %w", err)
		}
		service := platformMessaging.NewDeadLetterService(inspector, nil, appLogger, cfg.Cron.DLQInspectionBatchSize, 0, "")
		if err := runner.Register(scheduler.Job{
			Name:     "mq.dlq.inspect",
			Interval: time.Duration(cfg.Cron.DLQInspectionIntervalSeconds) * time.Second,
			Run:      service.InspectDeadLetters,
		}); err != nil {
			return fmt.Errorf("register dlq inspection job: %w", err)
		}
	}

	if cfg.Cron.DLQReplayEnabled {
		if cfg.Cron.DLQReplayIntervalSeconds <= 0 {
			return fmt.Errorf("cron.dlq_replay_interval_seconds must be greater than zero")
		}
		if cfg.Cron.DLQReplayBatchSize <= 0 {
			return fmt.Errorf("cron.dlq_replay_batch_size must be greater than zero")
		}
		if strings.TrimSpace(cfg.Cron.DLQReplayGroup) == "" {
			return fmt.Errorf("cron.dlq_replay_group must not be empty")
		}

		replayer, err := dlq.NewReplayer(deadLetterOptions(cfg, cfg.Cron.DLQReplayGroup), appLogger)
		if err != nil {
			return fmt.Errorf("initialize kafka dead letter replayer: %w", err)
		}
		target, err := dlq.ResolveReplayTarget(cfg.MQ.EventsName, cfg.Cron.DLQReplayTarget, retryLevels(cfg))
		if err != nil {
			return err
		}
		service := platformMessaging.NewDeadLetterService(nil, replayer, appLogger, 0, cfg.Cron.DLQReplayBatchSize, target)
		if err := runner.Register(scheduler.Job{
			Name:     "mq.dlq.replay",
			Interval: time.Duration(cfg.Cron.DLQReplayIntervalSeconds) * time.Second,
			Run:      service.ReplayDeadLetters,
		}); err != nil {
			return fmt.Errorf("register dlq replay job: %w", err)
		}
	}
	return nil
}

func deadLetterOptions(cfg *config.Config, groupID string) kafkaConfig.DeadLetter {
	return kafkaConfig.DeadLetter{
		Connection: kafkaConfig.Connection{
			Brokers:  cfg.MQ.Kafka.Brokers,
			Topic:    dlq.ResolveTopic(cfg.MQ.EventsName, cfg.Worker.KafkaDeadLetterTopic),
			ClientID: cfg.MQ.Kafka.ClientID,
		},
		GroupID:     strings.TrimSpace(groupID),
		MinBytes:    cfg.Worker.KafkaReadMinBytes,
		MaxBytes:    cfg.Worker.KafkaReadMaxBytes,
		MaxWait:     time.Duration(cfg.Worker.KafkaMaxWaitSeconds) * time.Second,
		PollTimeout: time.Duration(cfg.Worker.KafkaMaxWaitSeconds) * time.Second,
	}
}

func retryLevels(cfg *config.Config) []kafkaConfig.RetryLevel {
	levels := make([]kafkaConfig.RetryLevel, 0, len(cfg.Worker.KafkaRetryTopics))
	for _, level := range cfg.Worker.KafkaRetryTopics {
		levels = append(levels, kafkaConfig.RetryLevel{Topic: level.Topic, Delay: time.Duration(level.DelaySeconds) * time.Second})
	}
	return levels
}
