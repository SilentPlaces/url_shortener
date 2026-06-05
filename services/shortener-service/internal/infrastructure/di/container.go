package di

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/arminaray/url_shortener/services/shortener-service/internal/adapters/queue"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/adapters/redis"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/adapters/repository"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/application"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/ports"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/infrastructure/config"
	goredis "github.com/redis/go-redis/v9"

	_ "github.com/lib/pq"
)

type Container struct {
	Config  *config.Config
	UseCase *application.ShortenerUseCase

	db          *sql.DB
	redisClient *goredis.Client
	allocator   ports.IDAllocator
	publisher   ports.EventPublisher
}

func NewContainer(cfg *config.Config) (*Container, error) {
	db, err := sql.Open("postgres", cfg.PostgresDSN)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetMaxIdleConns(10)
	db.SetMaxOpenConns(30)

	pingCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	redisClient := goredis.NewClient(&goredis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       cfg.RedisDB,
	})
	if err := redisClient.Ping(pingCtx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	urlRepo := repository.NewPostgresRepository(db)
	cache := redis.NewRedisCache(redisClient, cfg.CachePrefix, cfg.CacheTTL)
	bloom, err := redis.NewRedisBloomFilter(redisClient, cfg.BloomKey, cfg.BloomExpected, cfg.BloomErrorRate)
	if err != nil {
		return nil, fmt.Errorf("create bloom filter: %w", err)
	}
	idAllocator := redis.NewRedisIDAllocator(redisClient, cfg.IDAllocatorKey, cfg.IDAllocatorBatchSize, cfg.IDAllocatorBuffer)
	eventPublisher := queue.NewKafkaProducer(cfg.KafkaBrokers, cfg.KafkaTopicPrefix)

	useCase := application.NewShortenerUseCase(
		urlRepo,
		eventPublisher,
		cache,
		bloom,
		idAllocator,
		cfg.BaseURL,
		cfg.RequestTimeout,
	)

	return &Container{
		Config:      cfg,
		UseCase:     useCase,
		db:          db,
		redisClient: redisClient,
		allocator:   idAllocator,
		publisher:   eventPublisher,
	}, nil
}

func (c *Container) Close() error {
	var firstErr error

	if c.UseCase != nil {
		c.UseCase.WaitBackground()
	}
	if c.publisher != nil {
		if closer, ok := c.publisher.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil && firstErr == nil {
				firstErr = fmt.Errorf("close publisher: %w", err)
			}
		}
	}
	if c.allocator != nil {
		if closer, ok := c.allocator.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil && firstErr == nil {
				firstErr = fmt.Errorf("close id allocator: %w", err)
			}
		}
	}
	if c.redisClient != nil {
		if err := c.redisClient.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close redis: %w", err)
		}
	}
	if c.db != nil {
		if err := c.db.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close postgres: %w", err)
		}
	}
	return firstErr
}
