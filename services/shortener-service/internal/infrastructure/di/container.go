package di

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/arminaray/url_shortener/pkg/safeurl"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/adapters/metrics"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/adapters/queue"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/adapters/redis"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/adapters/repository"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/application"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/ports"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/infrastructure/config"

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
	promMetrics := metrics.NewPromMetrics("shortener")
	urlValidator := safeurl.New(safeurl.WithAllowPrivate(cfg.AllowPrivateURLs))

	useCase := application.NewShortenerUseCase(
		urlRepo,
		eventPublisher,
		cache,
		bloom,
		idAllocator,
		urlValidator,
		application.Config{
			BaseURL:        cfg.BaseURL,
			RequestTimeout: cfg.RequestTimeout,
			Metrics:        promMetrics,
		},
	)

	if cfg.BloomRehydrateEnabled {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			rehydrateBloomFilter(ctx, urlRepo, bloom, cfg.BloomRehydrateBatchSize)
		}()
	}

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
	captureErr := func(err error, wrap string) {
		if err != nil && firstErr == nil {
			firstErr = fmt.Errorf("%s: %w", wrap, err)
		}
	}

	if c.UseCase != nil {
		c.UseCase.WaitBackground()
	}
	if c.publisher != nil {
		if closer, ok := c.publisher.(interface{ Close() error }); ok {
			captureErr(closer.Close(), "close publisher")
		}
	}
	if c.allocator != nil {
		if closer, ok := c.allocator.(interface{ Close() error }); ok {
			captureErr(closer.Close(), "close id allocator")
		}
	}
	if c.redisClient != nil {
		captureErr(c.redisClient.Close(), "close redis")
	}
	if c.db != nil {
		captureErr(c.db.Close(), "close postgres")
	}
	if firstErr != nil {
		log.Printf("dependency close error: %v", firstErr)
	}
	return firstErr
}
