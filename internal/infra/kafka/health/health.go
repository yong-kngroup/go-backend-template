package health

import (
	"context"
	"errors"

	"github.com/freeDog-wy/go-backend-template/internal/infra/kafka/internal/client"
	kgo "github.com/segmentio/kafka-go"
)

// Ping verifies that at least one configured Kafka broker is reachable.
func Ping(ctx context.Context, brokers []string) error {
	normalized := client.NormalizeBrokers(brokers)
	if len(normalized) == 0 {
		return errors.New("kafka brokers must not be empty")
	}
	conn, err := kgo.DialContext(ctx, "tcp", normalized[0])
	if err != nil {
		return err
	}
	return conn.Close()
}
