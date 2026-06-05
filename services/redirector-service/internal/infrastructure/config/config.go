package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPPort string

	PostgresDSN string
	RedisAddr   string
	RedisPass   string
	RedisDB     int
	CachePrefix string
	CacheTTL    int
}

func Load() (*Config, error) {
	redisDB, err := intFromEnv("REDIRECTOR_REDIS_DB", 0)
	if err != nil {
		return nil, err
	}
	cacheTTL, err := intFromEnv("REDIRECTOR_CACHE_TTL_SECONDS", 300)
	if err != nil {
		return nil, err
	}

	return &Config{
		HTTPPort:    envOrDefault("REDIRECTOR_HTTP_PORT", "8081"),
		PostgresDSN: envOrDefault("REDIRECTOR_POSTGRES_DSN", "postgres://postgres:postgres@localhost:5432/url_shortener?sslmode=disable"),
		RedisAddr:   envOrDefault("REDIRECTOR_REDIS_ADDR", "localhost:6379"),
		RedisPass:   envOrDefault("REDIRECTOR_REDIS_PASSWORD", ""),
		RedisDB:     redisDB,
		CachePrefix: envOrDefault("REDIRECTOR_CACHE_PREFIX", "redirector:"),
		CacheTTL:    cacheTTL,
	}, nil
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func intFromEnv(key string, fallback int) (int, error) {
	raw := envOrDefault(key, strconv.Itoa(fallback))
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be integer: %w", key, err)
	}
	return value, nil
}
