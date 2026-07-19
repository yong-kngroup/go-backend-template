package event

import "time"

type Event struct {
	Key          string
	Event        string
	Payload      []byte
	TraceID      string
	TraceContext string
}

type DeadLetter struct {
	Message           Event
	OriginalMessageID string
	OriginalTopic     string
	Source            string
	SourcePartition   int
	SourceOffset      int64
	ConsumerGroup     string
	Consumer          string
	Reason            string
	RetryCount        int64
	RetryTopic        string
	RetryDelaySeconds int
	FailedAt          time.Time
	DeadLetterTopic   string
	DeadLetterOffset  int64
	DeadLetterPart    int
}
