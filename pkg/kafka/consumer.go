package kafka

import (
	"context"
	"time"

	kgo "github.com/segmentio/kafka-go"
	"golang.org/x/sync/errgroup"
)

type ReaderLoop struct {
	Topic  string
	Delay  time.Duration
	Reader *kgo.Reader
}

type LoopHandler func(ctx context.Context, loop ReaderLoop, msg Record) error

type ConsumerRunner struct {
	readers []ReaderLoop
}

func NewReaderLoop(brokers []string, topic string, delay time.Duration, cfg ReaderConfig) ReaderLoop {
	return ReaderLoop{
		Topic:  topic,
		Delay:  delay,
		Reader: NewReader(brokers, topic, cfg),
	}
}

func NewConsumerRunner(readers []ReaderLoop) *ConsumerRunner {
	return &ConsumerRunner{readers: readers}
}

func (l ReaderLoop) FetchMessage(ctx context.Context) (Record, error) {
	message, err := l.Reader.FetchMessage(ctx)
	if err != nil {
		return Record{}, err
	}
	return recordFromKafkaMessage(message), nil
}

func (l ReaderLoop) CommitMessages(ctx context.Context, msg Record) error {
	return l.Reader.CommitMessages(ctx, kafkaMessageFromRecord(msg))
}

func (r *ConsumerRunner) Run(ctx context.Context, handler LoopHandler) error {
	group, runCtx := errgroup.WithContext(ctx)
	for _, loop := range r.readers {
		loop := loop
		group.Go(func() error {
			return runReaderLoop(runCtx, loop, handler)
		})
	}
	return group.Wait()
}

func runReaderLoop(ctx context.Context, loop ReaderLoop, handler LoopHandler) error {
	for {
		msg, err := loop.FetchMessage(ctx)
		if err != nil {
			return err
		}
		if err := handler(ctx, loop, msg); err != nil {
			return err
		}
	}
}
