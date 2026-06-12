package application

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeRepo struct {
	record *URLRecord
	err    error
	calls  int32
	delay  time.Duration
}

func (f *fakeRepo) GetByAlias(_ context.Context, _ string) (*URLRecord, error) {
	atomic.AddInt32(&f.calls, 1)
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.record, nil
}

type fakeCache struct{ missErr error }

func (f *fakeCache) Get(_ context.Context, _ string) (*URLRecord, error) {
	return nil, f.missErr
}
func (f *fakeCache) Set(_ context.Context, _ string, _ *URLRecord) error { return nil }
func (f *fakeCache) Delete(_ context.Context, _ string) error            { return nil }

func TestResolve_SingleflightDeduplicatesConcurrentLoads(t *testing.T) {
	repo := &fakeRepo{
		record: &URLRecord{Alias: "abc", OriginalURL: "https://example.com", IsActive: true},
		delay:  40 * time.Millisecond,
	}
	svc := NewRedirectService(repo, &fakeCache{missErr: errors.New("miss")})

	var wg sync.WaitGroup
	const concurrency = 20
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			if _, err := svc.Resolve(context.Background(), "abc"); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(&repo.calls); got > 5 {
		t.Fatalf("expected singleflight to coalesce %d concurrent loads; got %d DB calls",
			concurrency, got)
	}
}

func TestResolve_CacheHitRejectsInactive(t *testing.T) {
	cache := &recordedCache{record: &URLRecord{IsActive: false, OriginalURL: "x"}}
	svc := NewRedirectService(&fakeRepo{}, cache)
	_, err := svc.Resolve(context.Background(), "abc")
	if !errors.Is(err, ErrInactive) {
		t.Fatalf("expected ErrInactive, got %v", err)
	}
	if !cache.deleted {
		t.Fatalf("expected stale cache entry to be deleted")
	}
}

func TestResolve_CacheHitRejectsExpired(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	cache := &recordedCache{record: &URLRecord{IsActive: true, OriginalURL: "x", ExpiresAt: &past}}
	svc := NewRedirectService(&fakeRepo{}, cache)
	_, err := svc.Resolve(context.Background(), "abc")
	if !errors.Is(err, ErrExpired) {
		t.Fatalf("expected ErrExpired, got %v", err)
	}
	if !cache.deleted {
		t.Fatalf("expected stale cache entry to be deleted")
	}
}

type recordedCache struct {
	record  *URLRecord
	deleted bool
}

func (r *recordedCache) Get(_ context.Context, _ string) (*URLRecord, error) {
	return r.record, nil
}
func (r *recordedCache) Set(_ context.Context, _ string, _ *URLRecord) error { return nil }
func (r *recordedCache) Delete(_ context.Context, _ string) error {
	r.deleted = true
	return nil
}
