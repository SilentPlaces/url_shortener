package di

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/arminaray/url_shortener/pkg/httpx"
	"github.com/arminaray/url_shortener/services/redirector-service/internal/adapters/cache"
	httpadapter "github.com/arminaray/url_shortener/services/redirector-service/internal/adapters/http"
	"github.com/arminaray/url_shortener/services/redirector-service/internal/adapters/metrics"
	"github.com/arminaray/url_shortener/services/redirector-service/internal/adapters/repository"
	"github.com/arminaray/url_shortener/services/redirector-service/internal/application"
	"github.com/arminaray/url_shortener/services/redirector-service/internal/infrastructure/config"

	_ "github.com/lib/pq"
)

type Container struct {
	Config *config.Config
	Router *httpadapter.Handler

	db          *sql.DB
	redisClient *goredis.Client
}

func NewContainer(cfg *config.Config) (*Container, error) {
	db, err := sql.Open("postgres", cfg.PostgresDSN)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetMaxIdleConns(10)
	db.SetMaxOpenConns(30)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	redisClient := goredis.NewClient(&goredis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       cfg.RedisDB,
	})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	repo := repository.NewPostgresRepository(db)
	cacheAdapter := cache.NewRedisCache(redisClient, cfg.CachePrefix, cfg.CacheTTL)
	promMetrics := metrics.NewPromMetrics("redirector")
	service := application.NewRedirectService(repo, cacheAdapter, application.Config{Metrics: promMetrics})

	httpMetrics := httpx.NewMetrics("redirector")
	handler := httpadapter.NewHandlerWithMetrics(service, httpMetrics)

	return &Container{
		Config:      cfg,
		Router:      handler,
		db:          db,
		redisClient: redisClient,
	}, nil
}

func (c *Container) Close() error {
	var firstErr error
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
