package ports

import (
	"context"
	"time"
)

type Event struct {
	Timestamp time.Time
	Payload   interface{} // Can be ANY event type
}

func NewEvent(payload interface{}) *Event {
	return &Event{
		Timestamp: time.Now(),
		Payload:   payload,
	}
}

type EventPublisher interface {
	Publish(ctx context.Context, event *Event) error
}
