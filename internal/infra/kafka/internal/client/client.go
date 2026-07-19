package client

import (
	"strings"
	"time"

	kgo "github.com/segmentio/kafka-go"
)

func NormalizeBrokers(brokers []string) []string {
	normalized := make([]string, 0, len(brokers))
	for _, broker := range brokers {
		if broker = strings.TrimSpace(broker); broker != "" {
			normalized = append(normalized, broker)
		}
	}
	return normalized
}

func NewReader(brokers []string, topic, groupID, clientID string, minBytes, maxBytes int, maxWaitDuration time.Duration, startOffset int64) *kgo.Reader {
	return kgo.NewReader(kgo.ReaderConfig{
		Brokers:        NormalizeBrokers(brokers),
		Topic:          strings.TrimSpace(topic),
		GroupID:        strings.TrimSpace(groupID),
		MinBytes:       minBytes,
		MaxBytes:       maxBytes,
		MaxWait:        maxWaitDuration,
		CommitInterval: 0,
		StartOffset:    startOffset,
		Dialer:         &kgo.Dialer{ClientID: strings.TrimSpace(clientID)},
	})
}

func NewWriter(brokers []string, topic, clientID string) *kgo.Writer {
	return &kgo.Writer{
		Addr:         kgo.TCP(NormalizeBrokers(brokers)...),
		Topic:        strings.TrimSpace(topic),
		Balancer:     &kgo.Hash{},
		RequiredAcks: kgo.RequireAll,
		Async:        false,
		Transport:    &kgo.Transport{ClientID: strings.TrimSpace(clientID)},
	}
}
