package queue

import (
	"context"
	"testing"
	"time"

	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/events"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/ports"
)

func TestTopicFor_KnownEventUsesPrefix(t *testing.T) {
	m := buildTopicMap("url_shortener")
	if got := topicFor(m, "URLCreated"); got != "url_shortener.url.created" {
		t.Fatalf("unexpected topic: %s", got)
	}
}

func TestTopicFor_UnknownEventFallsBackToLowercase(t *testing.T) {
	m := buildTopicMap("url_shortener")
	if got := topicFor(m, "SomethingElse"); got != "somethingelse" {
		t.Fatalf("unexpected fallback topic: %s", got)
	}
}

func TestExtractKey_UsesEventKeyMethod(t *testing.T) {
	ev := ports.NewEvent("URLCreated", events.URLCreatedEvent{URLID: "42"})
	if got := extractKey(ev); got != "42" {
		t.Fatalf("expected key=42, got %q", got)
	}
}

func TestExtractKey_ReturnsEmptyForUnknownPayload(t *testing.T) {
	ev := ports.NewEvent("URLCreated", struct{ X int }{X: 1})
	if got := extractKey(ev); got != "" {
		t.Fatalf("expected empty key, got %q", got)
	}
}

func TestLogPublisher_UnknownEventFallsBackToLowercaseTopic(t *testing.T) {
	pub := NewLogEventPublisher("p")
	if err := pub.Publish(context.Background(), ports.NewEvent("Mystery", nil)); err != nil {
		t.Fatalf("expected fallback topic to be derived from event type, got error: %v", err)
	}
}

func TestLogPublisher_EmptyEventTypeIsRejected(t *testing.T) {
	pub := NewLogEventPublisher("p")
	if err := pub.Publish(context.Background(), ports.NewEvent("", nil)); err == nil {
		t.Fatalf("expected error for empty event type")
	}
}

func TestLogPublisher_RespectsContextCancellation(t *testing.T) {
	pub := NewLogEventPublisher("p")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ev := ports.NewEvent("URLCreated", events.URLCreatedEvent{URLID: "1", CreatedAt: time.Now()})
	if err := pub.Publish(ctx, ev); err == nil {
		t.Fatalf("expected context error")
	}
}
