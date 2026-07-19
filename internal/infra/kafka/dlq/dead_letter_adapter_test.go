package dlq

import (
	"testing"

	"github.com/freeDog-wy/go-backend-template/internal/infra/kafka/internal/client"
	kgo "github.com/segmentio/kafka-go"
)

func TestDecodeDeadLetter(t *testing.T) {
	headers := []kgo.Header{
		{Key: "event", Value: []byte("user.registered")}, {Key: "original_topic", Value: []byte("domain.events")},
		{Key: "source_topic", Value: []byte("domain.events.retry.30s")}, {Key: "source_partition", Value: []byte("1")},
		{Key: "source_offset", Value: []byte("42")}, {Key: "consumer_group", Value: []byte("user-worker")},
		{Key: "reason", Value: []byte("handler failed")}, {Key: "retry_count", Value: []byte("3")},
		{Key: "retry_topic", Value: []byte("domain.events.retry.30s")}, {Key: "retry_delay_seconds", Value: []byte("30")},
		{Key: "failed_at", Value: []byte("2026-07-11T10:20:30Z")}, {Key: "trace_id", Value: []byte("trace-1")},
	}
	adapter := &deadLetterAdapter{}
	message, err := adapter.decodeDeadLetter(kgo.Message{Key: []byte("message-1"), Value: []byte(`{"id":1}`), Headers: headers, Topic: "domain.events.dlq", Partition: 2, Offset: 99})
	if err != nil {
		t.Fatalf("decodeDeadLetter() error = %v", err)
	}
	if message.Message.Key != "message-1" || message.Message.Event != "user.registered" || message.OriginalTopic != "domain.events" || message.SourceOffset != 42 || message.RetryCount != 3 || message.DeadLetterOffset != 99 {
		t.Fatalf("decoded dead letter = %+v", message)
	}
	if _, err := adapter.decodeDeadLetter(kgo.Message{Key: []byte("message-1")}); err == nil {
		t.Fatal("missing event header should fail")
	}
}

func TestBuildReplayMessageCreatesNewKeyAndPreservesContext(t *testing.T) {
	adapter := &deadLetterAdapter{}
	record := kgo.Message{Key: []byte("message-1"), Value: []byte(`{"id":1}`), Headers: []kgo.Header{{Key: "event", Value: []byte("user.registered")}, {Key: "retry_count", Value: []byte("0")}, {Key: "retry_delay_seconds", Value: []byte("0")}, {Key: "source_partition", Value: []byte("0")}, {Key: "source_offset", Value: []byte("0")}}}
	message, err := adapter.buildReplayMessage(record)
	if err != nil {
		t.Fatalf("buildReplayMessage() error = %v", err)
	}
	if string(message.Key) == "message-1" || client.HeaderValue(message.Headers, "original_message_key") != "message-1" || client.HeaderValue(message.Headers, "replayed_from_dlq") != "true" {
		t.Fatalf("replay message = %+v", message)
	}
	if _, err := adapter.buildReplayMessage(kgo.Message{Key: []byte("message-1")}); err == nil {
		t.Fatal("invalid DLQ record should fail")
	}
}
