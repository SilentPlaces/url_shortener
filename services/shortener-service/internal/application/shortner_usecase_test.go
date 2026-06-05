package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/entities"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/ports"
)

type fakeURLRepository struct {
	inserted         *entities.URL
	insertCalls      int
	insertErr        error
	getByAliasResult *entities.URL
	getByAliasErr    error
}

func (f *fakeURLRepository) Insert(_ context.Context, url *entities.URL) error {
	f.insertCalls++
	f.inserted = url
	return f.insertErr
}

func (f *fakeURLRepository) GetByAlias(_ context.Context, _ string) (*entities.URL, error) {
	if f.getByAliasErr != nil {
		return nil, f.getByAliasErr
	}
	return f.getByAliasResult, nil
}

func (f *fakeURLRepository) ExistsByAlias(_ context.Context, _ string) (bool, error) {
	return false, nil
}

type fakePublisher struct{}

func (f *fakePublisher) Publish(_ context.Context, _ *ports.Event) error { return nil }

type fakeCache struct {
	getValue *entities.URL
	getErr   error
	setErr   error
	setCalls int
	deletes  int
}

func (f *fakeCache) Get(_ context.Context, _ string) (*entities.URL, error) {
	return f.getValue, f.getErr
}
func (f *fakeCache) Set(_ context.Context, _ string, _ *entities.URL) error {
	f.setCalls++
	return f.setErr
}
func (f *fakeCache) Delete(_ context.Context, _ string) error {
	f.deletes++
	return nil
}
func (f *fakeCache) Exists(_ context.Context, _ string) (bool, error) { return false, nil }
func (f *fakeCache) Expire(_ context.Context, _ string, _ int) error  { return nil }

type fakeBloomFilter struct{}

func (f *fakeBloomFilter) Add(_ context.Context, _ string) error               { return nil }
func (f *fakeBloomFilter) MightContain(_ context.Context, _ string) (bool, error) { return false, nil }

type fakeIDAllocator struct {
	nextID int64
	err    error
}

func (f *fakeIDAllocator) NextID(_ context.Context) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	return f.nextID, nil
}

func TestShortenURL_CustomAliasAssignsIDBeforeInsert(t *testing.T) {
	repo := &fakeURLRepository{}
	useCase := NewShortenerUseCase(
		repo,
		&fakePublisher{},
		&fakeCache{},
		&fakeBloomFilter{},
		&fakeIDAllocator{nextID: 42},
		"http://localhost:8081",
		5*time.Second,
	)

	_, err := useCase.ShortenURL(context.Background(), &ShortenURLRequest{
		OriginalURL: "https://example.com",
		CustomAlias: "abc123",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.inserted == nil {
		t.Fatalf("expected insert to be called")
	}
	if repo.inserted.ID != 42 {
		t.Fatalf("expected ID to be assigned before insert, got %d", repo.inserted.ID)
	}
}

func TestGetURL_PropagatesInfrastructureError(t *testing.T) {
	repo := &fakeURLRepository{getByAliasErr: errors.New("db unavailable")}
	useCase := NewShortenerUseCase(
		repo,
		&fakePublisher{},
		&fakeCache{getErr: errors.New("cache miss")},
		&fakeBloomFilter{},
		&fakeIDAllocator{nextID: 7},
		"http://localhost:8081",
		5*time.Second,
	)

	_, err := useCase.GetURL(context.Background(), "abc")
	if err == nil {
		t.Fatalf("expected error")
	}
	var domainErr *domain.Error
	if errors.As(err, &domainErr) && domainErr.Code == "URL_NOT_FOUND" {
		t.Fatalf("expected infra error, got domain not found: %v", err)
	}
}

func TestGetURL_NotFoundReturnsDomainError(t *testing.T) {
	repo := &fakeURLRepository{getByAliasErr: domain.NewURLNotFoundError("miss")}
	useCase := NewShortenerUseCase(
		repo,
		&fakePublisher{},
		&fakeCache{getErr: errors.New("cache miss")},
		&fakeBloomFilter{},
		&fakeIDAllocator{nextID: 7},
		"http://localhost:8081",
		5*time.Second,
	)

	_, err := useCase.GetURL(context.Background(), "miss")
	if err == nil {
		t.Fatalf("expected error")
	}
	var domainErr *domain.Error
	if !errors.As(err, &domainErr) || domainErr.Code != "URL_NOT_FOUND" {
		t.Fatalf("expected URL_NOT_FOUND, got %v", err)
	}
}

// Regression: cache hit must re-check IsActive / IsExpired.
func TestGetURL_CacheHit_RejectsInactive(t *testing.T) {
	cached, _ := entities.NewURL("https://example.com", "abc123", nil, nil, false)
	cached.Deactivate()
	useCase := NewShortenerUseCase(
		&fakeURLRepository{},
		&fakePublisher{},
		&fakeCache{getValue: cached},
		&fakeBloomFilter{},
		&fakeIDAllocator{nextID: 1},
		"http://localhost:8081",
		5*time.Second,
	)

	_, err := useCase.GetURL(context.Background(), "abc123")
	var domainErr *domain.Error
	if !errors.As(err, &domainErr) || domainErr.Code != "URL_INACTIVE" {
		t.Fatalf("expected URL_INACTIVE, got %v", err)
	}
}

func TestGetURL_CacheHit_RejectsExpired(t *testing.T) {
	cached, _ := entities.NewURL("https://example.com", "abc123", nil, nil, false)
	past := time.Now().Add(-time.Hour)
	cached.ExpiresAt = &past
	useCase := NewShortenerUseCase(
		&fakeURLRepository{},
		&fakePublisher{},
		&fakeCache{getValue: cached},
		&fakeBloomFilter{},
		&fakeIDAllocator{nextID: 1},
		"http://localhost:8081",
		5*time.Second,
	)

	_, err := useCase.GetURL(context.Background(), "abc123")
	var domainErr *domain.Error
	if !errors.As(err, &domainErr) || domainErr.Code != "EXPIRED" {
		t.Fatalf("expected EXPIRED, got %v", err)
	}
}

// Regression: a successful Insert must not be retried when cache.Set fails.
func TestShortenURL_RandomAlias_DoesNotRetryAfterInsertWhenCacheFails(t *testing.T) {
	repo := &fakeURLRepository{}
	useCase := NewShortenerUseCase(
		repo,
		&fakePublisher{},
		&fakeCache{setErr: errors.New("redis down"), getErr: errors.New("miss")},
		&fakeBloomFilter{},
		&fakeIDAllocator{nextID: 999_999},
		"http://localhost:8081",
		5*time.Second,
	)

	_, err := useCase.ShortenURL(context.Background(), &ShortenURLRequest{
		OriginalURL: "https://example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.insertCalls != 1 {
		t.Fatalf("expected exactly one insert, got %d", repo.insertCalls)
	}
	useCase.WaitBackground()
}

func TestShortenURL_CustomAlias_ReservedRejected(t *testing.T) {
	useCase := NewShortenerUseCase(
		&fakeURLRepository{},
		&fakePublisher{},
		&fakeCache{},
		&fakeBloomFilter{},
		&fakeIDAllocator{nextID: 1},
		"http://localhost:8081",
		5*time.Second,
	)

	_, err := useCase.ShortenURL(context.Background(), &ShortenURLRequest{
		OriginalURL: "https://example.com",
		CustomAlias: "admin",
	})
	var domainErr *domain.Error
	if !errors.As(err, &domainErr) || domainErr.Code != "RESERVED_ALIAS" {
		t.Fatalf("expected RESERVED_ALIAS, got %v", err)
	}
}
