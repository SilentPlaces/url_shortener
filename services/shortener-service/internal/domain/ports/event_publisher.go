package ports

import (
	"context"
	"time"
)

type Event struct {
	Type      string // Event type: "URLCreated", "URLClicked", etc.
	Timestamp time.Time
	Payload   interface{} // Can be ANY event type
}

func NewEvent(eventType string, payload interface{}) *Event {
	return &Event{
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Payload:   payload,
	}
}

type EventPublisher interface {
	Publish(ctx context.Context, event *Event) error
}
