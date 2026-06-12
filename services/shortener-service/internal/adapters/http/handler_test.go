package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arminaray/url_shortener/services/shortener-service/internal/application"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/entities"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/ports"
)

type handlerRepo struct {
	record *entities.URL
}

func (h *handlerRepo) Insert(_ context.Context, _ *entities.URL) error { return nil }
func (h *handlerRepo) GetByAlias(_ context.Context, _ string) (*entities.URL, error) {
	return h.record, nil
}
func (h *handlerRepo) ExistsByAlias(_ context.Context, _ string) (bool, error) { return false, nil }
func (h *handlerRepo) IterateAliases(_ context.Context, _ int, _ ports.AliasVisitor) error {
	return nil
}

type handlerPublisher struct{}

func (h *handlerPublisher) Publish(_ context.Context, _ *ports.Event) error { return nil }

type handlerCache struct{}

func (h *handlerCache) Get(_ context.Context, _ string) (*entities.URL, error) {
	return nil, http.ErrNoCookie
}
func (h *handlerCache) Set(_ context.Context, _ string, _ *entities.URL) error { return nil }
func (h *handlerCache) Delete(_ context.Context, _ string) error                { return nil }
func (h *handlerCache) Exists(_ context.Context, _ string) (bool, error)        { return false, nil }
func (h *handlerCache) Expire(_ context.Context, _ string, _ int) error         { return nil }

type handlerBloom struct{}

func (h *handlerBloom) Add(_ context.Context, _ string) error                  { return nil }
func (h *handlerBloom) AddMany(_ context.Context, _ []string) error            { return nil }
func (h *handlerBloom) MightContain(_ context.Context, _ string) (bool, error) { return false, nil }

type handlerAllocator struct{}

func (h *handlerAllocator) NextID(_ context.Context) (int64, error) { return 238328, nil }

type handlerValidator struct{}

func (h *handlerValidator) Validate(_ string) error { return nil }

func newHandlerUseCase(repo *handlerRepo) *application.ShortenerUseCase {
	return application.NewShortenerUseCase(
		repo,
		&handlerPublisher{},
		&handlerCache{},
		&handlerBloom{},
		&handlerAllocator{},
		&handlerValidator{},
		application.Config{BaseURL: "http://localhost:8081", RequestTimeout: 5 * time.Second},
	)
}

func TestShortenURLEndpoint(t *testing.T) {
	useCase := newHandlerUseCase(&handlerRepo{})
	handler := NewHandler(useCase)

	body, _ := json.Marshal(map[string]string{
		"original_url": "https://example.com",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/urls", bytes.NewReader(body))
	res := httptest.NewRecorder()
	handler.Router().ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", res.Code)
	}
	useCase.WaitBackground()
}

func TestGetURLEndpoint(t *testing.T) {
	record, _ := entities.NewURL("https://example.com", "abc123", nil, nil, false)
	record.ID = 1
	useCase := newHandlerUseCase(&handlerRepo{record: record})
	handler := NewHandler(useCase)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/urls/abc123", nil)
	res := httptest.NewRecorder()
	handler.Router().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
}
