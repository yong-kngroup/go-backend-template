package mq

import (
	"context"
	"testing"
)

func TestPingKafkaRequiresBroker(t *testing.T) {
	if err := PingKafka(context.Background(), nil); err == nil {
		t.Fatal("PingKafka() error = nil, want an error for empty brokers")
	}
}
