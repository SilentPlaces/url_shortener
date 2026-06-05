package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPPort string
	BaseURL  string

	PostgresDSN string
	RedisAddr   string
	RedisPass   string
	RedisDB     int
	KafkaBrokers []string
	KafkaTopicPrefix string

	CachePrefix   string
	CacheTTL      int
	BloomKey      string
	BloomExpected int64
	BloomErrorRate float64

	IDAllocatorKey       string
	IDAllocatorBatchSize int64
	IDAllocatorBuffer    int

	RequestTimeout time.Duration
}

func Load() (*Config, error) {
	redisDB, err := intFromEnv("SHORTENER_REDIS_DB", 0)
	if err != nil {
		return nil, err
	}
	cacheTTL, err := intFromEnv("SHORTENER_CACHE_TTL_SECONDS", 300)
	if err != nil {
		return nil, err
	}
	bloomExpected, err := int64FromEnv("SHORTENER_BLOOM_EXPECTED_ITEMS", 1000000)
	if err != nil {
		return nil, err
	}
	bloomRate, err := float64FromEnv("SHORTENER_BLOOM_FALSE_POSITIVE_RATE", 0.01)
	if err != nil {
		return nil, err
	}
	allocBatch, err := int64FromEnv("SHORTENER_ID_ALLOCATOR_BATCH_SIZE", 1024)
	if err != nil {
		return nil, err
	}
	allocBuffer, err := intFromEnv("SHORTENER_ID_ALLOCATOR_BUFFER_SIZE", 2048)
	if err != nil {
		return nil, err
	}
	timeoutSeconds, err := intFromEnv("SHORTENER_REQUEST_TIMEOUT_SECONDS", 10)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		HTTPPort:           envOrDefault("SHORTENER_HTTP_PORT", "8080"),
		BaseURL:            envOrDefault("SHORTENER_BASE_URL", "http://localhost:8081"),
		PostgresDSN:        envOrDefault("SHORTENER_POSTGRES_DSN", "postgres://postgres:postgres@localhost:5432/url_shortener?sslmode=disable"),
		RedisAddr:          envOrDefault("SHORTENER_REDIS_ADDR", "localhost:6379"),
		RedisPass:          envOrDefault("SHORTENER_REDIS_PASSWORD", ""),
		RedisDB:            redisDB,
		KafkaBrokers:       strings.Split(envOrDefault("SHORTENER_KAFKA_BROKERS", "localhost:9092"), ","),
		KafkaTopicPrefix:   envOrDefault("SHORTENER_KAFKA_TOPIC_PREFIX", "url_shortener"),
		CachePrefix:        envOrDefault("SHORTENER_CACHE_PREFIX", "shortener:"),
		CacheTTL:           cacheTTL,
		BloomKey:           envOrDefault("SHORTENER_BLOOM_KEY", "shortener:aliases"),
		BloomExpected:      bloomExpected,
		BloomErrorRate:     bloomRate,
		IDAllocatorKey:     envOrDefault("SHORTENER_ID_ALLOCATOR_KEY", "shortener:id"),
		IDAllocatorBatchSize: allocBatch,
		IDAllocatorBuffer:  allocBuffer,
		RequestTimeout:     time.Duration(timeoutSeconds) * time.Second,
	}

	return cfg, nil
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

func int64FromEnv(key string, fallback int64) (int64, error) {
	raw := envOrDefault(key, strconv.FormatInt(fallback, 10))
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be int64: %w", key, err)
	}
	return value, nil
}

func float64FromEnv(key string, fallback float64) (float64, error) {
	raw := envOrDefault(key, strconv.FormatFloat(fallback, 'f', -1, 64))
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be float64: %w", key, err)
	}
	return value, nil
}
