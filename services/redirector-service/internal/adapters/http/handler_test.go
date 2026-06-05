package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arminaray/url_shortener/services/redirector-service/internal/application"
)

func TestRedirectSuccess(t *testing.T) {
	repo := &redirectRepo{
		record: &application.URLRecord{
			Alias:       "abc",
			OriginalURL: "https://example.com",
			IsActive:    true,
		},
	}
	service := application.NewRedirectService(repo, &redirectCache{})
	handler := NewHandler(service)

	req := httptest.NewRequest(http.MethodGet, "/abc", nil)
	res := httptest.NewRecorder()
	handler.Router().ServeHTTP(res, req)

	if res.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307, got %d", res.Code)
	}
}

func TestRedirectNotFound(t *testing.T) {
	repo := &redirectRepo{err: application.ErrNotFound}
	service := application.NewRedirectService(repo, &redirectCache{})
	handler := NewHandler(service)

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	res := httptest.NewRecorder()
	handler.Router().ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", res.Code)
	}
}

type redirectRepo struct {
	record *application.URLRecord
	err    error
}

func (r *redirectRepo) GetByAlias(_ context.Context, _ string) (*application.URLRecord, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.record, nil
}

type redirectCache struct{}

func (r *redirectCache) Get(_ context.Context, _ string) (*application.URLRecord, error) {
	return nil, errors.New("miss")
}
func (r *redirectCache) Set(_ context.Context, _ string, _ *application.URLRecord) error { return nil }
func (r *redirectCache) Delete(_ context.Context, _ string) error                        { return nil }
