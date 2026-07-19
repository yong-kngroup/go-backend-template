package consumer

import (
	"context"
	"strings"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/infra/kafka/internal/client"
	kgo "github.com/segmentio/kafka-go"
	"golang.org/x/sync/errgroup"
)

type readerLoop struct {
	Topic  string
	Delay  time.Duration
	reader *kgo.Reader
}

func (l readerLoop) commit(ctx context.Context, message kgo.Message) error {
	return l.reader.CommitMessages(ctx, message)
}

type retryPublisher struct {
	Topic     string
	Delay     time.Duration
	Publisher *kgo.Writer
}

type consumerTopology struct {
	readers             []readerLoop
	retryPublishers     []retryPublisher
	deadLetterPublisher *kgo.Writer
	deadLetterTopic     string
}

func newConsumerTopology(brokers []string, topic string, cfg consumerAdapterConfig) *consumerTopology {
	mainTopic := strings.TrimSpace(topic)
	startOffset := int64(kgo.LastOffset)
	if cfg.StartOffset != nil {
		startOffset = *cfg.StartOffset
	}
	readers := []readerLoop{{Topic: mainTopic, reader: client.NewReader(brokers, mainTopic, cfg.GroupID, cfg.ClientID, cfg.MinBytes, cfg.MaxBytes, cfg.MaxWait, startOffset)}}
	retryPublishers := make([]retryPublisher, 0, len(cfg.RetryLevels))
	for _, level := range cfg.RetryLevels {
		readers = append(readers, readerLoop{Topic: level.Topic, Delay: level.Delay, reader: client.NewReader(brokers, level.Topic, cfg.GroupID, cfg.ClientID, cfg.MinBytes, cfg.MaxBytes, cfg.MaxWait, startOffset)})
		retryPublishers = append(retryPublishers, retryPublisher{Topic: level.Topic, Delay: level.Delay, Publisher: client.NewWriter(brokers, level.Topic, cfg.ClientID)})
	}
	deadLetterTopic := strings.TrimSpace(cfg.DeadLetterTopic)
	if deadLetterTopic == "" {
		deadLetterTopic = mainTopic + ".dlq"
	}
	return &consumerTopology{readers: readers, retryPublishers: retryPublishers, deadLetterPublisher: client.NewWriter(brokers, deadLetterTopic, cfg.ClientID), deadLetterTopic: deadLetterTopic}
}

func (t *consumerTopology) run(ctx context.Context, handler func(context.Context, readerLoop, kgo.Message) error) error {
	group, runCtx := errgroup.WithContext(ctx)
	for _, loop := range t.readers {
		loop := loop
		group.Go(func() error {
			for {
				message, err := loop.reader.FetchMessage(runCtx)
				if err != nil {
					return err
				}
				if err := handler(runCtx, loop, message); err != nil {
					return err
				}
			}
		})
	}
	return group.Wait()
}

func (t *consumerTopology) topics() []string {
	topics := make([]string, 0, len(t.readers))
	for _, reader := range t.readers {
		topics = append(topics, reader.Topic)
	}
	return topics
}

func (t *consumerTopology) retryTopics() []string {
	topics := make([]string, 0, len(t.retryPublishers))
	for _, publisher := range t.retryPublishers {
		topics = append(topics, publisher.Topic)
	}
	return topics
}
