package application

import (
	"context"
	"errors"
	"time"

	"golang.org/x/sync/singleflight"
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

type Metrics interface {
	CacheHit(alias string)
	CacheMiss(alias string)
}

type NoopMetrics struct{}

func (NoopMetrics) CacheHit(string)  {}
func (NoopMetrics) CacheMiss(string) {}

type RedirectService struct {
	repository URLRepository
	cache      Cache
	metrics    Metrics
	group      singleflight.Group
}

type Config struct {
	Metrics Metrics
}

func NewRedirectService(repository URLRepository, cache Cache, cfg ...Config) *RedirectService {
	c := Config{}
	if len(cfg) > 0 {
		c = cfg[0]
	}
	if c.Metrics == nil {
		c.Metrics = NoopMetrics{}
	}
	return &RedirectService{
		repository: repository,
		cache:      cache,
		metrics:    c.Metrics,
	}
}

func (s *RedirectService) Resolve(ctx context.Context, alias string) (string, error) {
	if cached, err := s.cache.Get(ctx, alias); err == nil && cached != nil {
		s.metrics.CacheHit(alias)
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
	s.metrics.CacheMiss(alias)

	record, err := s.loadRecord(ctx, alias)
	if err != nil {
		return "", err
	}

	if !record.IsActive {
		return "", ErrInactive
	}
	if record.IsExpired() {
		return "", ErrExpired
	}

	_ = s.cache.Set(ctx, alias, record)
	return record.OriginalURL, nil
}

func (s *RedirectService) loadRecord(ctx context.Context, alias string) (*URLRecord, error) {
	v, err, _ := s.group.Do(alias, func() (any, error) {
		return s.repository.GetByAlias(ctx, alias)
	})
	if err != nil {
		return nil, err
	}
	return v.(*URLRecord), nil
}
