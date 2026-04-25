package domain

import (
	"context"
	"time"
)

type Event struct {
	Type      string
	Timestamp time.Time
	Payload   interface{}
}

type EventPublisher interface {
	Publish(ctx context.Context, event *Event) error
}
