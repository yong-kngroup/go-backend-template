package mq

import (
	"context"
	"testing"
	"time"

	pkgkafka "github.com/freeDog-wy/go-backend-template/pkg/kafka"
)

type fakeConsumptionRepository struct{}

func (fakeConsumptionRepository) Begin(context.Context, ConsumptionBegin) (ConsumptionBeginResult, error) {
	return ConsumptionBeginResult{}, nil
}

func (fakeConsumptionRepository) MarkDone(context.Context, string, string, time.Time) error {
	return nil
}

func (fakeConsumptionRepository) MarkFailed(context.Context, string, string, string, time.Time) error {
	return nil
}

func (fakeConsumptionRepository) MarkDead(context.Context, string, string, string, time.Time) error {
	return nil
}

func TestNewConsumerRejectsInvalidOptions(t *testing.T) {
	t.Parallel()

	_, err := NewConsumer(ConsumerOptions{
		KafkaOptions:      KafkaOptions{Brokers: []string{"localhost:9092"}, Topic: "events"},
		GroupID:           "worker",
		MaxRetries:        1,
		ProcessingLockTTL: time.Minute,
		MinBytes:          1,
		MaxBytes:          1,
		MaxWait:           time.Second,
		RetryLevels:       []pkgkafka.RetryLevel{{Topic: "retry", Delay: time.Second}, {Topic: "retry", Delay: time.Minute}},
	}, fakeConsumptionRepository{}, nil)
	if err == nil {
		t.Fatal("NewConsumer() expected an error for duplicate retry topics")
	}
}

func TestResolveDeadLetterReplayTarget(t *testing.T) {
	t.Parallel()

	if _, err := ResolveDeadLetterReplayTarget("events", "first_retry", nil); err == nil {
		t.Fatal("ResolveDeadLetterReplayTarget() expected an error without retry topics")
	}
}
