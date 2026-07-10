package mq

import "context"

// Publisher defines the project's message-publish contract.
type Publisher interface {
	Publish(ctx context.Context, message Message) error
}
