package application

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/entities"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/ports"
)

type fakeURLRepository struct {
	mu               sync.Mutex
	inserted         *entities.URL
	insertCalls      int32
	insertErr        error
	getByAliasResult *entities.URL
	getByAliasErr    error
	getByAliasCalls  int32
	getByAliasDelay  time.Duration
}

func (f *fakeURLRepository) Insert(_ context.Context, url *entities.URL) error {
	atomic.AddInt32(&f.insertCalls, 1)
	f.mu.Lock()
	f.inserted = url
	f.mu.Unlock()
	return f.insertErr
}

func (f *fakeURLRepository) GetByAlias(_ context.Context, _ string) (*entities.URL, error) {
	atomic.AddInt32(&f.getByAliasCalls, 1)
	if f.getByAliasDelay > 0 {
		time.Sleep(f.getByAliasDelay)
	}
	if f.getByAliasErr != nil {
		return nil, f.getByAliasErr
	}
	return f.getByAliasResult, nil
}

func (f *fakeURLRepository) ExistsByAlias(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (f *fakeURLRepository) IterateAliases(_ context.Context, _ int, _ ports.AliasVisitor) error {
	return nil
}

type fakePublisher struct{}

func (f *fakePublisher) Publish(_ context.Context, _ *ports.Event) error { return nil }

type fakeCache struct {
	getValue *entities.URL
	getErr   error
	setErr   error
	setCalls int32
	deletes  int32
}

func (f *fakeCache) Get(_ context.Context, _ string) (*entities.URL, error) {
	return f.getValue, f.getErr
}
func (f *fakeCache) Set(_ context.Context, _ string, _ *entities.URL) error {
	atomic.AddInt32(&f.setCalls, 1)
	return f.setErr
}
func (f *fakeCache) Delete(_ context.Context, _ string) error {
	atomic.AddInt32(&f.deletes, 1)
	return nil
}
func (f *fakeCache) Exists(_ context.Context, _ string) (bool, error) { return false, nil }
func (f *fakeCache) Expire(_ context.Context, _ string, _ int) error  { return nil }

type fakeBloomFilter struct{}

func (f *fakeBloomFilter) Add(_ context.Context, _ string) error                  { return nil }
func (f *fakeBloomFilter) AddMany(_ context.Context, _ []string) error            { return nil }
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

type fakeURLValidator struct{ err error }

func (f *fakeURLValidator) Validate(_ string) error { return f.err }

func newUseCase(repo *fakeURLRepository, cache *fakeCache, alloc *fakeIDAllocator) *ShortenerUseCase {
	return NewShortenerUseCase(
		repo,
		&fakePublisher{},
		cache,
		&fakeBloomFilter{},
		alloc,
		&fakeURLValidator{},
		Config{BaseURL: "http://localhost:8081", RequestTimeout: 5 * time.Second},
	)
}

func TestShortenURL_CustomAliasAssignsIDBeforeInsert(t *testing.T) {
	repo := &fakeURLRepository{}
	useCase := newUseCase(repo, &fakeCache{}, &fakeIDAllocator{nextID: 42})

	_, err := useCase.ShortenURL(context.Background(), &ShortenURLRequest{
		OriginalURL: "https://example.com",
		CustomAlias: "abc123",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.inserted == nil {
		t.Fatalf("expected insert to be called")
	}
	if repo.inserted.ID != 42 {
		t.Fatalf("expected ID to be assigned before insert, got %d", repo.inserted.ID)
	}
	useCase.WaitBackground()
}

func TestShortenURL_RejectsInvalidURLFromValidator(t *testing.T) {
	useCase := NewShortenerUseCase(
		&fakeURLRepository{},
		&fakePublisher{},
		&fakeCache{},
		&fakeBloomFilter{},
		&fakeIDAllocator{nextID: 1},
		&fakeURLValidator{err: errors.New("ssrf blocked")},
		Config{BaseURL: "http://localhost:8081", RequestTimeout: time.Second},
	)

	_, err := useCase.ShortenURL(context.Background(), &ShortenURLRequest{
		OriginalURL: "http://10.0.0.1",
	})
	var domainErr *domain.Error
	if !errors.As(err, &domainErr) || domainErr.Code != "INVALID_URL" {
		t.Fatalf("expected INVALID_URL domain error, got %v", err)
	}
}

func TestGetURL_PropagatesInfrastructureError(t *testing.T) {
	repo := &fakeURLRepository{getByAliasErr: errors.New("db unavailable")}
	useCase := newUseCase(repo, &fakeCache{getErr: errors.New("cache miss")}, &fakeIDAllocator{nextID: 7})

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
	useCase := newUseCase(repo, &fakeCache{getErr: errors.New("cache miss")}, &fakeIDAllocator{nextID: 7})

	_, err := useCase.GetURL(context.Background(), "miss")
	if err == nil {
		t.Fatalf("expected error")
	}
	var domainErr *domain.Error
	if !errors.As(err, &domainErr) || domainErr.Code != "URL_NOT_FOUND" {
		t.Fatalf("expected URL_NOT_FOUND, got %v", err)
	}
}

func TestGetURL_CacheHit_RejectsInactive(t *testing.T) {
	cached, _ := entities.NewURL("https://example.com", "abc123", nil, nil, false)
	cached.Deactivate()
	useCase := newUseCase(&fakeURLRepository{}, &fakeCache{getValue: cached}, &fakeIDAllocator{nextID: 1})

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
	useCase := newUseCase(&fakeURLRepository{}, &fakeCache{getValue: cached}, &fakeIDAllocator{nextID: 1})

	_, err := useCase.GetURL(context.Background(), "abc123")
	var domainErr *domain.Error
	if !errors.As(err, &domainErr) || domainErr.Code != "EXPIRED" {
		t.Fatalf("expected EXPIRED, got %v", err)
	}
}

func TestShortenURL_RandomAlias_DoesNotRetryAfterInsertWhenCacheFails(t *testing.T) {
	repo := &fakeURLRepository{}
	useCase := newUseCase(repo,
		&fakeCache{setErr: errors.New("redis down"), getErr: errors.New("miss")},
		&fakeIDAllocator{nextID: 999_999})

	_, err := useCase.ShortenURL(context.Background(), &ShortenURLRequest{OriginalURL: "https://example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := atomic.LoadInt32(&repo.insertCalls); got != 1 {
		t.Fatalf("expected exactly one insert, got %d", got)
	}
	useCase.WaitBackground()
}

func TestShortenURL_CustomAlias_ReservedRejected(t *testing.T) {
	useCase := newUseCase(&fakeURLRepository{}, &fakeCache{}, &fakeIDAllocator{nextID: 1})

	_, err := useCase.ShortenURL(context.Background(), &ShortenURLRequest{
		OriginalURL: "https://example.com",
		CustomAlias: "admin",
	})
	var domainErr *domain.Error
	if !errors.As(err, &domainErr) || domainErr.Code != "RESERVED_ALIAS" {
		t.Fatalf("expected RESERVED_ALIAS, got %v", err)
	}
}

func TestGetURL_SingleflightDeduplicatesConcurrentLoads(t *testing.T) {
	record, _ := entities.NewURL("https://example.com", "abc123", nil, nil, false)
	record.ID = 1
	repo := &fakeURLRepository{getByAliasResult: record, getByAliasDelay: 50 * time.Millisecond}
	useCase := newUseCase(repo, &fakeCache{getErr: errors.New("miss")}, &fakeIDAllocator{nextID: 1})

	var wg sync.WaitGroup
	const concurrency = 25
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			if _, err := useCase.GetURL(context.Background(), "abc123"); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(&repo.getByAliasCalls); got > 5 {
		t.Fatalf("expected singleflight to coalesce calls; got %d DB calls for %d goroutines",
			got, concurrency)
	}
}
