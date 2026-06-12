package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/ports"
)

const (
	defaultBatchTimeout = 100 * time.Millisecond
	defaultWriteTimeout = 5 * time.Second
)

type KafkaProducer struct {
	writer   *kafka.Writer
	topicMap map[string]string
}

func NewKafkaProducer(brokers []string, topicPrefix string) ports.EventPublisher {
	if len(brokers) == 0 {
		log.Print("kafka brokers not configured; falling back to log event publisher")
		return NewLogEventPublisher(topicPrefix)
	}

	w := &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Balancer:               &kafka.Hash{},
		AllowAutoTopicCreation: true,
		BatchTimeout:           defaultBatchTimeout,
		WriteTimeout:           defaultWriteTimeout,
		RequiredAcks:           kafka.RequireAll,
		Async:                  false,
	}

	return &KafkaProducer{
		writer:   w,
		topicMap: buildTopicMap(topicPrefix),
	}
}

func (k *KafkaProducer) Publish(ctx context.Context, event *ports.Event) error {
	topic := topicFor(k.topicMap, event.Type)
	if topic == "" {
		return fmt.Errorf("unknown event type: %s", event.Type)
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	msg := kafka.Message{
		Topic: topic,
		Key:   []byte(extractKey(event)),
		Value: body,
		Time:  event.Timestamp,
		Headers: []kafka.Header{
			{Key: "event-type", Value: []byte(event.Type)},
		},
	}

	if err := k.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("publish %s to %s: %w", event.Type, topic, err)
	}
	return nil
}

func (k *KafkaProducer) Close() error {
	if k.writer == nil {
		return nil
	}
	if err := k.writer.Close(); err != nil && !errors.Is(err, kafka.ErrGroupClosed) {
		return fmt.Errorf("close kafka writer: %w", err)
	}
	return nil
}

type LogEventPublisher struct {
	topicMap map[string]string
}

func NewLogEventPublisher(topicPrefix string) ports.EventPublisher {
	return &LogEventPublisher{topicMap: buildTopicMap(topicPrefix)}
}

func (l *LogEventPublisher) Publish(ctx context.Context, event *ports.Event) error {
	topic := topicFor(l.topicMap, event.Type)
	if topic == "" {
		return fmt.Errorf("unknown event type: %s", event.Type)
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	log.Printf("event_logged topic=%s type=%s body=%s", topic, event.Type, string(body))
	return nil
}

func (l *LogEventPublisher) Close() error { return nil }

func buildTopicMap(prefix string) map[string]string {
	return map[string]string{
		"URLCreated":     prefix + ".url.created",
		"URLClicked":     prefix + ".url.clicked",
		"URLExpired":     prefix + ".url.expired",
		"URLDeactivated": prefix + ".url.deactivated",
	}
}

func topicFor(m map[string]string, eventType string) string {
	if topic, ok := m[eventType]; ok {
		return topic
	}
	return strings.ToLower(eventType)
}

func extractKey(event *ports.Event) string {
	if event == nil || event.Payload == nil {
		return ""
	}
	switch v := event.Payload.(type) {
	case map[string]any:
		if id, ok := v["URLID"].(string); ok {
			return id
		}
	default:
		if h, ok := event.Payload.(interface{ EventKey() string }); ok {
			return h.EventKey()
		}
	}
	return ""
}
