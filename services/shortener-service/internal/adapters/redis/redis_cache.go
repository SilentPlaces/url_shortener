package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/entities"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/ports"
	"github.com/redis/go-redis/v9"
)

var ErrCacheMiss = errors.New("cache miss")

type RedisCache struct {
	client *redis.Client
	prefix string
	ttl    int
}

func NewRedisCache(client *redis.Client, prefix string, ttl int) ports.CacheRepository {
	return &RedisCache{client: client, prefix: prefix, ttl: ttl}
}

func (r *RedisCache) Expire(ctx context.Context, key string, ttl int) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := r.client.Expire(ctx, r.prefix+key, time.Duration(ttl)*time.Second).Err(); err != nil {
		return fmt.Errorf("failed to set expiration for key %s: %w", key, err)
	}
	return nil
}

func (r *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	count, err := r.client.Exists(ctx, r.prefix+key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check if key %s exists: %w", key, err)
	}
	return count == 1, nil
}

func (r *RedisCache) Delete(ctx context.Context, key string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := r.client.Del(ctx, r.prefix+key).Err(); err != nil {
		return fmt.Errorf("failed to delete key %s: %w", key, err)
	}
	return nil
}

func (r *RedisCache) Set(ctx context.Context, key string, url *entities.URL) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	data, err := json.Marshal(url)
	if err != nil {
		return fmt.Errorf("failed to marshal URL: %w", err)
	}

	if err := r.client.Set(ctx, r.prefix+key, data, time.Duration(r.ttl)*time.Second).Err(); err != nil {
		return fmt.Errorf("failed to set key %s: %w", key, err)
	}
	return nil
}

func (r *RedisCache) Get(ctx context.Context, key string) (*entities.URL, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	data, err := r.client.Get(ctx, r.prefix+key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrCacheMiss
		}
		return nil, fmt.Errorf("failed to get key %s: %w", key, err)
	}

	url := &entities.URL{}
	if err := json.Unmarshal(data, url); err != nil {
		return nil, fmt.Errorf("failed to unmarshal URL: %w", err)
	}
	return url, nil
}
