package config

import "time"

type Connection struct {
	Brokers  []string
	Topic    string
	ClientID string
}

type RetryLevel struct {
	Topic string
	Delay time.Duration
}

type Consumer struct {
	Connection
	StartOffset       *int64
	GroupID           string
	MaxRetries        int
	ProcessingLockTTL time.Duration
	MinBytes          int
	MaxBytes          int
	MaxWait           time.Duration
	RetryLevels       []RetryLevel
	DeadLetterTopic   string
}

type DeadLetter struct {
	Connection
	GroupID     string
	MinBytes    int
	MaxBytes    int
	MaxWait     time.Duration
	PollTimeout time.Duration
}
