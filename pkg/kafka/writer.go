package kafka

import (
	"strings"

	kgo "github.com/segmentio/kafka-go"
)

func NormalizeBrokers(brokers []string) []string {
	normalizedBrokers := make([]string, 0, len(brokers))
	for _, broker := range brokers {
		if trimmed := strings.TrimSpace(broker); trimmed != "" {
			normalizedBrokers = append(normalizedBrokers, trimmed)
		}
	}
	return normalizedBrokers
}

func NewWriter(brokers []string, topic string, cfg WriterConfig) *kgo.Writer {
	normalizedBrokers := NormalizeBrokers(brokers)
	if len(normalizedBrokers) == 0 {
		panic("kafka brokers must not be empty")
	}
	if strings.TrimSpace(topic) == "" {
		panic("kafka topic must not be empty")
	}

	return &kgo.Writer{
		Addr:         kgo.TCP(normalizedBrokers...),
		Topic:        topic,
		Balancer:     &kgo.Hash{},
		RequiredAcks: kgo.RequireAll,
		Async:        false,
		Transport: &kgo.Transport{
			ClientID: strings.TrimSpace(cfg.ClientID),
		},
	}
}
