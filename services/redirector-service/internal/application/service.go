package application

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound = errors.New("alias not found")
	ErrExpired  = errors.New("alias expired")
	ErrInactive = errors.New("alias inactive")
)

type URLRecord struct {
	Alias       string     `json:"alias"`
	OriginalURL string     `json:"original_url"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	IsActive    bool       `json:"is_active"`
}

func (r *URLRecord) IsExpired() bool {
	if r.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*r.ExpiresAt)
}

type URLRepository interface {
	GetByAlias(ctx context.Context, alias string) (*URLRecord, error)
}

type Cache interface {
	Get(ctx context.Context, alias string) (*URLRecord, error)
	Set(ctx context.Context, alias string, record *URLRecord) error
	Delete(ctx context.Context, alias string) error
}

type RedirectService struct {
	repository URLRepository
	cache      Cache
}

func NewRedirectService(repository URLRepository, cache Cache) *RedirectService {
	return &RedirectService{
		repository: repository,
		cache:      cache,
	}
}

func (s *RedirectService) Resolve(ctx context.Context, alias string) (string, error) {
	if cached, err := s.cache.Get(ctx, alias); err == nil && cached != nil {
		if !cached.IsActive {
			_ = s.cache.Delete(ctx, alias)
			return "", ErrInactive
		}
		if cached.IsExpired() {
			_ = s.cache.Delete(ctx, alias)
			return "", ErrExpired
		}
		return cached.OriginalURL, nil
	}

	record, err := s.repository.GetByAlias(ctx, alias)
	if err != nil {
		return "", err
	}

	if !record.IsActive {
		return "", ErrInactive
	}
	if record.IsExpired() {
		return "", ErrExpired
	}

	if err := s.cache.Set(ctx, alias, record); err != nil {
		// best-effort cache write; do not fail the redirect
		_ = err
	}
	return record.OriginalURL, nil
}
