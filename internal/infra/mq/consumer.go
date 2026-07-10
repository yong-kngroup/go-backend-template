package mq

import "context"

// EventHandler unifies message handling so upper layers do not depend on broker-native payloads.
type EventHandler func(ctx context.Context, message Message) error

// Consumer defines the project's message-consume contract.
type Consumer interface {
	Handle(eventName string, fn EventHandler)
	Run(ctx context.Context) error
}
