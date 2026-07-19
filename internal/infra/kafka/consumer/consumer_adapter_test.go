package consumer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/infra/kafka/event"
	kgo "github.com/segmentio/kafka-go"
)

func TestDecodeMessage(t *testing.T) {
	adapter := &consumerAdapter{}
	headers := []kgo.Header{{Key: "event", Value: []byte("user.registered")}, {Key: "trace_id", Value: []byte("legacy-trace-id")}}
	message, err := adapter.decodeMessage(kgo.Message{Key: []byte("message-1"), Value: []byte(`{"id":1}`), Headers: headers})
	if err != nil {
		t.Fatalf("decodeMessage() error = %v", err)
	}
	if message.Key != "message-1" || message.Event != "user.registered" || message.TraceID != "legacy-trace-id" {
		t.Fatalf("decoded message = %+v", message)
	}
	if _, err := adapter.decodeMessage(kgo.Message{Key: []byte("message-1")}); err == nil {
		t.Fatal("missing event header should fail")
	}
	if _, err := adapter.decodeMessage(kgo.Message{Headers: []kgo.Header{{Key: "event", Value: []byte("user.registered")}}}); err == nil {
		t.Fatal("missing key should fail")
	}
}

func TestWaitRetryDelay(t *testing.T) {
	adapter := &consumerAdapter{}
	if err := adapter.waitRetryDelay(context.Background(), readerLoop{}, event.Event{}); err != nil {
		t.Fatalf("waitRetryDelay() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := adapter.waitRetryDelay(ctx, readerLoop{Delay: time.Millisecond}, event.Event{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("waitRetryDelay() error = %v", err)
	}
}

func TestRetryRouteAndOriginalTopic(t *testing.T) {
	adapter := &consumerAdapter{topology: &consumerTopology{retryPublishers: []retryPublisher{{Topic: "retry.30s"}, {Topic: "retry.5m"}}}}
	if got := adapter.retryRoute(99).Topic; got != "retry.5m" {
		t.Fatalf("retryRoute() = %q", got)
	}
	if got := originalTopic(kgo.Message{Topic: "retry", Headers: []kgo.Header{{Key: "original_topic", Value: []byte("main")}}}); got != "main" {
		t.Fatalf("originalTopic() = %q", got)
	}
}
