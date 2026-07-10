package kafka

import (
	"context"

	kgo "github.com/segmentio/kafka-go"
)

type Publisher struct {
	writer *kgo.Writer
}

func NewPublisher(brokers []string, topic string, cfg WriterConfig) *Publisher {
	return &Publisher{
		writer: NewWriter(brokers, topic, cfg),
	}
}

func (p *Publisher) Publish(ctx context.Context, message Message) error {
	return p.writer.WriteMessages(ctx, kafkaMessageFromMessage(message))
}
