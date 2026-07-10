package kafka

import kgo "github.com/segmentio/kafka-go"

type Header = kgo.Header

type Message struct {
	Key     []byte
	Value   []byte
	Headers []Header
}

type Record struct {
	Message
	Topic     string
	Partition int
	Offset    int64
}

func recordFromKafkaMessage(message kgo.Message) Record {
	return Record{
		Message: Message{
			Key:     message.Key,
			Value:   message.Value,
			Headers: message.Headers,
		},
		Topic:     message.Topic,
		Partition: message.Partition,
		Offset:    message.Offset,
	}
}

func kafkaMessageFromMessage(message Message) kgo.Message {
	return kgo.Message{
		Key:     message.Key,
		Value:   message.Value,
		Headers: message.Headers,
	}
}

func kafkaMessageFromRecord(record Record) kgo.Message {
	message := kafkaMessageFromMessage(record.Message)
	message.Topic = record.Topic
	message.Partition = record.Partition
	message.Offset = record.Offset
	return message
}
