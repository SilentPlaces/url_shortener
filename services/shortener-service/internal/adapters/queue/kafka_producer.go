package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/ports"
)

type KafkaProducer struct {
	brokers  []string
	topicMap map[string]string
}

func NewKafkaProducer(brokers []string, topicPrefix string) ports.EventPublisher {
	topicMap := map[string]string{
		"URLCreated":     topicPrefix + ".url.created",
		"URLClicked":     topicPrefix + ".url.clicked",
		"URLExpired":     topicPrefix + ".url.expired",
		"URLDeactivated": topicPrefix + ".url.deactivated",
	}

	return &KafkaProducer{
		brokers:  brokers,
		topicMap: topicMap,
	}
}

func (k *KafkaProducer) Publish(ctx context.Context, event *ports.Event) error {
	topic := k.getTopic(event.Type)
	if topic == "" {
		return fmt.Errorf("unknown event type: %s", event.Type)
	}

	payloadJSON, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal event payload: %w", err)
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	log.Printf("event_published brokers=%s topic=%s type=%s payload=%s event=%s",
		strings.Join(k.brokers, ","),
		topic,
		event.Type,
		string(payloadJSON),
		string(eventJSON),
	)

	return nil
}

func (k *KafkaProducer) getTopic(eventType string) string {
	topic, exists := k.topicMap[eventType]
	if exists {
		return topic
	}

	return strings.ToLower(eventType)
}

func (k *KafkaProducer) Close() error {
	return nil
}
