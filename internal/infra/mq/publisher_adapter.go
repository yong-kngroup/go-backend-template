package mq

import (
	"context"

	pkgkafka "github.com/freeDog-wy/go-backend-template/pkg/kafka"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
)

// publisherAdapter maps the project message model onto the Kafka publisher.
type publisherAdapter struct {
	publisher *pkgkafka.Publisher
	logger    logger.Logger
}

func newPublisherAdapter(brokers []string, topic, clientID string, log logger.Logger) *publisherAdapter {
	return &publisherAdapter{
		publisher: pkgkafka.NewPublisher(brokers, topic, pkgkafka.WriterConfig{ClientID: clientID}),
		logger:    log,
	}
}

var _ Publisher = (*publisherAdapter)(nil)

func (p *publisherAdapter) Publish(ctx context.Context, message Message) error {
	headers := []pkgkafka.Header{
		{Key: "event", Value: []byte(message.Event)},
	}
	if message.TraceID != "" {
		headers = append(headers, pkgkafka.Header{Key: "trace_id", Value: []byte(message.TraceID)})
	}

	msg := pkgkafka.Message{
		Key:     []byte(message.Key),
		Value:   message.Payload,
		Headers: headers,
	}

	if err := p.publisher.Publish(ctx, msg); err != nil {
		return err
	}

	if p.logger != nil {
		p.logger.Debug("event published", "event", message.Event, "trace_id", message.TraceID)
	}
	return nil
}
