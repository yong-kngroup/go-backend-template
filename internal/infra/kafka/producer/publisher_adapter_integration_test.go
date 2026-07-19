//go:build integration

package producer

import (
	"context"
	"testing"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/infra/kafka/config"
	"github.com/freeDog-wy/go-backend-template/internal/infra/kafka/event"
	"github.com/freeDog-wy/go-backend-template/internal/infra/kafka/internal/client"
	"github.com/freeDog-wy/go-backend-template/internal/testsupport"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
	kgo "github.com/segmentio/kafka-go"
)

func TestPublisherAdapterIntegrationPublish(t *testing.T) {
	broker := testsupport.OpenKafka(t)
	topic := broker.CreateTopic(t, "integration.mq.publisher")
	publisher, err := New(config.Connection{Brokers: broker.Brokers, Topic: topic, ClientID: "integration-test"}, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	message := event.Event{
		Key:     "message-key",
		Event:   "user.registered",
		Payload: []byte(`{"user_id":42}`),
		TraceID: "trace-integration-42",
	}
	if err := publisher.Publish(context.Background(), message); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	reader := kgo.NewReader(kgo.ReaderConfig{Brokers: broker.Brokers, Topic: topic, MinBytes: 1, MaxBytes: 1024, MaxWait: 100 * time.Millisecond, StartOffset: kgo.FirstOffset})
	t.Cleanup(func() { _ = reader.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	record, err := reader.FetchMessage(ctx)
	if err != nil {
		t.Fatalf("FetchMessage() error = %v", err)
	}
	if string(record.Key) != message.Key {
		t.Fatalf("message key = %q, want %q", record.Key, message.Key)
	}
	if string(record.Value) != string(message.Payload) {
		t.Fatalf("payload = %q, want %q", record.Value, message.Payload)
	}
	if event := client.HeaderValue(record.Headers, "event"); event != message.Event {
		t.Fatalf("event header = %q, want %q", event, message.Event)
	}
	if traceID := client.HeaderValue(record.Headers, "trace_id"); traceID != message.TraceID {
		t.Fatalf("trace header = %q, want %q", traceID, message.TraceID)
	}
}
