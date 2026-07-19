//go:build integration

package consumer_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/infra/kafka/config"
	"github.com/freeDog-wy/go-backend-template/internal/infra/kafka/consumer"
	"github.com/freeDog-wy/go-backend-template/internal/infra/kafka/event"
	"github.com/freeDog-wy/go-backend-template/internal/infra/kafka/producer"
	platformMessaging "github.com/freeDog-wy/go-backend-template/internal/platform/messaging"
	"github.com/freeDog-wy/go-backend-template/internal/testsupport"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
	kgo "github.com/segmentio/kafka-go"
	"gorm.io/gorm"
)

func TestConsumerAdapterIntegrationConsumesAndMarksDone(t *testing.T) {
	db := testsupport.OpenPostgres(t)
	if err := db.AutoMigrate(&platformMessaging.RecordModel{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	broker := testsupport.OpenKafka(t)
	topic := broker.CreateTopic(t, "integration.mq.consumer")
	seed := time.Now().UnixNano()
	groupID := fmt.Sprintf("integration-consumer-%d", seed)
	messageKey := fmt.Sprintf("message-%d", seed)
	startOffset := int64(kgo.FirstOffset)
	records := platformMessaging.New(db)
	consumer, err := consumer.New(config.Consumer{
		Connection:        config.Connection{Brokers: broker.Brokers, Topic: topic, ClientID: "integration-consumer"},
		StartOffset:       &startOffset,
		GroupID:           groupID,
		MinBytes:          1,
		MaxBytes:          1024,
		MaxWait:           100 * time.Millisecond,
		ProcessingLockTTL: time.Minute,
		MaxRetries:        1,
		DeadLetterTopic:   topic + ".dlq",
	}, records, logger.Noop())
	if err != nil {
		t.Fatalf("NewConsumer() error = %v", err)
	}

	handled := make(chan event.Event, 1)
	consumer.Handle("integration.event", func(_ context.Context, message event.Event) error {
		handled <- message
		return nil
	})
	publisher, err := producer.New(config.Connection{Brokers: broker.Brokers, Topic: topic, ClientID: "integration-producer"}, logger.Noop())
	if err != nil {
		t.Fatalf("NewPublisher() error = %v", err)
	}
	if err := publisher.Publish(context.Background(), event.Event{
		Key:     messageKey,
		Event:   "integration.event",
		Payload: []byte(`{"id":42}`),
	}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- consumer.Run(ctx) }()
	select {
	case message := <-handled:
		if message.Key != messageKey || message.Event != "integration.event" || string(message.Payload) != `{"id":42}` {
			t.Fatalf("handled message = %+v", message)
		}
	case <-time.After(15 * time.Second):
		cancel()
		t.Fatal("consumer did not handle the Kafka message")
	}

	assertConsumptionDone(t, db, groupID, messageKey)
	cancel()
	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("consumer Run() error = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("consumer did not stop after context cancellation")
	}
}

func assertConsumptionDone(t *testing.T, db *gorm.DB, groupID, messageKey string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var record platformMessaging.RecordModel
		if err := db.Where("consumer_group = ? AND message_key = ?", groupID, messageKey).First(&record).Error; err == nil {
			if record.Status != string(consumer.ConsumptionStatusDone) || record.ProcessedAt == nil {
				t.Fatalf("consumption record = %+v", record)
			}
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("message consumption record not found for group=%s key=%s", groupID, messageKey)
}
