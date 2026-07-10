package kafka

import "time"

type WriterConfig struct {
	ClientID string
}

type ReaderConfig struct {
	GroupID     string
	ClientID    string
	MinBytes    int
	MaxBytes    int
	MaxWait     time.Duration
	StartOffset int64
}

type DeadLetterConfig struct {
	GroupID     string
	ClientID    string
	MinBytes    int
	MaxBytes    int
	MaxWait     time.Duration
	PollTimeout time.Duration
}
