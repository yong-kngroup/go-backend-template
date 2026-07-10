package kafka

import (
	"strings"

	kgo "github.com/segmentio/kafka-go"
)

func NewReader(brokers []string, topic string, cfg ReaderConfig) *kgo.Reader {
	normalizedBrokers := NormalizeBrokers(brokers)
	if len(normalizedBrokers) == 0 {
		panic("kafka brokers must not be empty")
	}
	if strings.TrimSpace(topic) == "" {
		panic("kafka topic must not be empty")
	}

	return kgo.NewReader(kgo.ReaderConfig{
		Brokers:        normalizedBrokers,
		Topic:          strings.TrimSpace(topic),
		GroupID:        strings.TrimSpace(cfg.GroupID),
		MinBytes:       cfg.MinBytes,
		MaxBytes:       cfg.MaxBytes,
		MaxWait:        cfg.MaxWait,
		CommitInterval: 0,
		StartOffset:    cfg.StartOffset,
		Dialer: &kgo.Dialer{
			ClientID: strings.TrimSpace(cfg.ClientID),
		},
	})
}
