package mq

import (
	"context"
	"errors"

	pkgkafka "github.com/freeDog-wy/go-backend-template/pkg/kafka"
	kgo "github.com/segmentio/kafka-go"
)

// PingKafka verifies that at least one configured Kafka broker is reachable.
func PingKafka(ctx context.Context, brokers []string) error {
	normalized := pkgkafka.NormalizeBrokers(brokers)
	if len(normalized) == 0 {
		return errors.New("kafka brokers must not be empty")
	}
	conn, err := kgo.DialContext(ctx, "tcp", normalized[0])
	if err != nil {
		return err
	}
	return conn.Close()
}
