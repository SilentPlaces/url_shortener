package application

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/arminaray/url_shortener/services/shortener-service/internal/adapters/mapper"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/entities"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/events"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/ports"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/valueobjects"
)

const maxRandomAliasAttempts = 10

type ShortenerUseCase struct {
	urlRepository  ports.URLRepository
	eventPublisher ports.EventPublisher
	cache          ports.CacheRepository
	bloomFilter    ports.BloomFilter
	idAllocator    ports.IDAllocator
	baseURL        string
	requestTimeout time.Duration

	bgWG sync.WaitGroup
}

func NewShortenerUseCase(urlRepository ports.URLRepository,
	eventPublisher ports.EventPublisher,
	cache ports.CacheRepository,
	bloomFilter ports.BloomFilter,
	idAllocator ports.IDAllocator,
	baseURL string,
	requestTimeout time.Duration) *ShortenerUseCase {
	if requestTimeout <= 0 {
		requestTimeout = 10 * time.Second
	}

	return &ShortenerUseCase{
		urlRepository:  urlRepository,
		eventPublisher: eventPublisher,
		cache:          cache,
		bloomFilter:    bloomFilter,
		idAllocator:    idAllocator,
		baseURL:        baseURL,
		requestTimeout: requestTimeout,
	}
}

// WaitBackground blocks until in-flight async tasks (event publish, bloom updates) finish.
func (s *ShortenerUseCase) WaitBackground() {
	s.bgWG.Wait()
}

func (s *ShortenerUseCase) GetURL(ctx context.Context, alias string) (*GetURLResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, s.requestTimeout)
	defer cancel()

	alias = valueobjects.NormalizeAlias(alias)

	if cached, err := s.cache.Get(ctx, alias); err == nil && cached != nil {
		if cached.IsExpired() {
			_ = s.cache.Delete(ctx, alias)
			return nil, domain.ErrExpired
		}
		if !cached.IsActive {
			_ = s.cache.Delete(ctx, alias)
			return nil, domain.ErrURLInactive
		}
		return buildGetResponse(cached), nil
	}

	urlEntity, err := s.urlRepository.GetByAlias(ctx, alias)
	if err != nil {
		if isDomainErrorCode(err, "URL_NOT_FOUND") {
			return nil, domain.NewURLNotFoundError(alias)
		}
		return nil, fmt.Errorf("failed to load URL by alias: %w", err)
	}

	if urlEntity.IsExpired() {
		return nil, domain.ErrExpired
	}

	if !urlEntity.IsActive {
		return nil, domain.ErrURLInactive
	}

	if err := s.cache.Set(ctx, alias, urlEntity); err != nil {
		log.Printf("failed to cache URL: %v", err)
	}

	return buildGetResponse(urlEntity), nil
}

func (s *ShortenerUseCase) ShortenURL(ctx context.Context, request *ShortenURLRequest) (*ShortenURLResponse, error) {
	ctxTimeOut, cancel := context.WithTimeout(ctx, s.requestTimeout)
	defer cancel()

	if strings.TrimSpace(request.CustomAlias) != "" {
		return s.handleCustomAlias(ctxTimeOut, request)
	}
	return s.handleRandomAlias(ctxTimeOut, request)
}

func (s *ShortenerUseCase) handleCustomAlias(ctx context.Context, request *ShortenURLRequest) (*ShortenURLResponse, error) {
	aliasVO, err := valueobjects.NewAlias(request.CustomAlias, true)
	if err != nil {
		return nil, err
	}
	alias := aliasVO.Value

	id, err := s.idAllocator.NextID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate ID: %w", err)
	}

	urlEntity, err := entities.NewURL(request.OriginalURL, alias, request.ExpiresAt, request.Metadata, true)
	if err != nil {
		return nil, err
	}
	urlEntity.ID = id

	if err := s.urlRepository.Insert(ctx, urlEntity); err != nil {
		if isDomainErrorCode(err, "ALIAS_TAKEN") {
			return nil, domain.NewAliasTakenError(alias)
		}
		return nil, fmt.Errorf("failed to persist custom alias URL: %w", err)
	}

	s.afterInsert(urlEntity)
	return buildShortenURLResponse(urlEntity, s.baseURL), nil
}

func (s *ShortenerUseCase) handleRandomAlias(ctx context.Context, request *ShortenURLRequest) (*ShortenURLResponse, error) {
	for i := 0; i < maxRandomAliasAttempts; i++ {
		id, err := s.idAllocator.NextID(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to allocate ID: %w", err)
		}

		alias := valueobjects.NormalizeAlias(mapper.EncodeToBase62(id))

		if !entities.IsValidAlias(alias) {
			continue
		}
		if valueobjects.IsReservedAlias(alias) {
			continue
		}

		mightExist, err := s.bloomFilter.MightContain(ctx, alias)
		if err != nil {
			return nil, fmt.Errorf("failed to check bloom filter: %w", err)
		}
		if mightExist {
			continue
		}

		urlEntity, err := entities.NewURL(request.OriginalURL, alias, request.ExpiresAt, request.Metadata, false)
		if err != nil {
			return nil, err
		}
		urlEntity.ID = id

		if err := s.urlRepository.Insert(ctx, urlEntity); err != nil {
			if isDomainErrorCode(err, "ALIAS_TAKEN") {
				continue
			}
			return nil, fmt.Errorf("failed to persist generated alias URL: %w", err)
		}

		s.afterInsert(urlEntity)
		return buildShortenURLResponse(urlEntity, s.baseURL), nil
	}

	return nil, fmt.Errorf("failed to generate unique alias after %d attempts", maxRandomAliasAttempts)
}

// afterInsert handles best-effort post-insert side effects: cache warm-up,
// bloom filter update, and event publication. Failures are logged and do not
// affect the request outcome.
func (s *ShortenerUseCase) afterInsert(urlEntity *entities.URL) {
	cacheCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.cache.Set(cacheCtx, urlEntity.Alias, urlEntity); err != nil {
		log.Printf("failed to cache URL: %v", err)
	}

	s.bgWG.Add(2)
	go func() {
		defer s.bgWG.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.bloomFilter.Add(ctx, urlEntity.Alias); err != nil {
			log.Printf("failed to update bloom filter: %v", err)
		}
	}()
	go func() {
		defer s.bgWG.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.publishURLCreatedEvent(ctx, urlEntity)
	}()
}

func (s *ShortenerUseCase) publishURLCreatedEvent(ctx context.Context, urlEntity *entities.URL) {
	event := events.URLCreatedEvent{
		URLID:       fmt.Sprintf("%d", urlEntity.ID),
		OriginalURL: urlEntity.OriginalUrl,
		Alias:       urlEntity.Alias,
		IsCustom:    urlEntity.IsCustom,
		CreatedAt:   urlEntity.CreatedAt,
		ExpiresAt:   urlEntity.ExpiresAt,
	}
	if err := s.eventPublisher.Publish(ctx, ports.NewEvent("URLCreated", event)); err != nil {
		log.Printf("failed to publish event: %v", err)
	}
}

func buildShortenURLResponse(urlEntity *entities.URL, baseURL string) *ShortenURLResponse {
	return &ShortenURLResponse{
		OriginalURL: urlEntity.OriginalUrl,
		ShortURL:    fmt.Sprintf("%s/%s", baseURL, urlEntity.Alias),
		Alias:       urlEntity.Alias,
		CreatedAt:   urlEntity.CreatedAt,
		ExpiresAt:   urlEntity.ExpiresAt,
		Metadata:    urlEntity.MetaData,
	}
}

func buildGetResponse(url *entities.URL) *GetURLResponse {
	return &GetURLResponse{
		ID:          url.ID,
		OriginalURL: url.OriginalUrl,
		Alias:       url.Alias,
		CreatedAt:   url.CreatedAt,
		ExpiresAt:   url.ExpiresAt,
		IsActive:    url.IsActive,
		Metadata:    url.MetaData,
	}
}

func isDomainErrorCode(err error, code string) bool {
	var domainErr *domain.Error
	if !errors.As(err, &domainErr) {
		return false
	}
	return domainErr.Code == code
}
