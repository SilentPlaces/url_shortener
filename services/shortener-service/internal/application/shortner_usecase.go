package application

import (
	"context"
	"fmt"
	"time"

	"github.com/arminaray/url_shortener/services/shortener-service/internal/adapters/mapper"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/entities"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/events"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/ports"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/valueobjects"
)

type ShortnerUseCase struct {
	urlRepository  ports.URLRepository
	eventPublisher ports.EventPublisher
	cache          ports.CacheRepository
	bloomFilter    ports.BloomFilter
	idAllocator    ports.IDAllocator
	baseURL        string
}

func NewShortnerUseCase(urlRepository ports.URLRepository,
	eventPublisher ports.EventPublisher,
	cache ports.CacheRepository,
	bloomFilter ports.BloomFilter,
	idAllocator ports.IDAllocator,
	baseURL string) *ShortnerUseCase {
	return &ShortnerUseCase{
		urlRepository:  urlRepository,
		eventPublisher: eventPublisher,
		cache:          cache,
		bloomFilter:    bloomFilter,
		idAllocator:    idAllocator,
		baseURL:        baseURL,
	}
}

// ShortenURL creates a short URL
func (s *ShortnerUseCase) ShortenURL(ctx context.Context, request *ShortenURLRequesty) (*ShortenURLResponse, error) {
	//TODO read from config
	context, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if request.CustomAlias != "" {
		return s.handleCustomAlias(context, request)
	}
	return s.handleRandomAlias(context, request)
}

func (s *ShortnerUseCase) handleCustomAlias(ctx context.Context, request *ShortenURLRequesty) (*ShortenURLResponse, error) {
	// Validate
	alisVO, err := valueobjects.NewAlias(request.CustomAlias, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create alias: %w", err)
	}
	alias := alisVO.Value
	// Create Entity
	urlEntity, err := entities.NewURL(request.OriginalURL, alias, request.ExpiresAt, request.Metadata, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create URL entity: %w", err)
	}

	// Insert into repository
	err = s.urlRepository.Insert(ctx, urlEntity)
	if err != nil {
		return nil, domain.NewAliasTakenError(alias)
	}

	// Add to bloom filter
	go s.bloomFilter.Add(ctx, alias)

	// Set in cache
	s.cache.Set(ctx, alias, urlEntity)

	// Publish event
	go s.publishURLCreatedEvent(ctx, urlEntity)

	return buildShortenURLResponse(urlEntity, s.baseURL), nil
}

func (s *ShortnerUseCase) handleRandomAlias(ctx context.Context, request *ShortenURLRequesty) (*ShortenURLResponse, error) {
	maxAttemps := 10
	for i := 0; i < maxAttemps; i++ {
		id, err := s.idAllocator.NextID(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to allocate ID: %w", err)
		}

		//Convert to Base62
		alias := mapper.EncodeToBase62(id)

		//CHeck for bloom filter collision
		mightExists, err := s.bloomFilter.MightContain(ctx, alias)
		if err != nil {
			return nil, fmt.Errorf("failed to check for bloom filter collision: %w", err)
		}
		if mightExists {
			continue
		}

		// Create Entity
		urlEntity, err := entities.NewURL(request.OriginalURL, alias, request.ExpiresAt, request.Metadata, false)
		if err != nil {
			return nil, fmt.Errorf("failed to create URL entity: %w", err)
		}

		urlEntity.ID = id

		err = s.urlRepository.Insert(ctx, urlEntity)
		if err != nil {
			continue
		}

		// Add to bloom filter
		go s.bloomFilter.Add(ctx, alias)

		// Set in cache
		s.cache.Set(ctx, alias, urlEntity)

		go s.publishURLCreatedEvent(ctx, urlEntity)

		return buildShortenURLResponse(urlEntity, s.baseURL), nil
	}

	return nil, domain.NewAliasTakenError(request.CustomAlias)
}

func (s *ShortnerUseCase) getUniqueAlias(ctx context.Context, alias string) (*GetURLResponse, error) {
	// try to get from cache first
	url, err := s.cache.Get(ctx, alias)
	if err == nil {
		return buildGetResponse(url), nil
	}

	urlEntity, err := s.urlRepository.GetByAlias(ctx, alias)
	if err != nil {
		return nil, domain.NewURLNotFoundError(alias)
	}

	if urlEntity.IsExpired() {
		return nil, domain.ErrExpired
	}

	if urlEntity.IsActive {
		return nil, domain.ErrURLInactive
	}

	//cache for next times
	s.cache.Set(ctx, alias, urlEntity)

	return buildGetResponse(urlEntity), nil
}

func (s *ShortnerUseCase) publishURLCreatedEvent(ctx context.Context, urlEntity *entities.URL) error {
	event := events.URLCreatedEvent{
		URLID:       urlEntity.ID,
		OriginalURL: urlEntity.OriginalUrl,
		Alias:       urlEntity.Alias,
		IsCustom:    urlEntity.IsCustom,
		CreatedAt:   urlEntity.CreatedAt,
		ExpiresAt:   urlEntity.ExpiresAt,
	}
	return s.eventPublisher.Publish(ctx, ports.NewEvent(event))
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
