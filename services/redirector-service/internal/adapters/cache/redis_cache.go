package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/arminaray/url_shortener/services/redirector-service/internal/application"
)

var ErrCacheMiss = errors.New("cache miss")

type RedisCache struct {
	client *redis.Client
	prefix string
	ttl    int
}

func NewRedisCache(client *redis.Client, prefix string, ttl int) *RedisCache {
	return &RedisCache{
		client: client,
		prefix: prefix,
		ttl:    ttl,
	}
}

func (r *RedisCache) Get(ctx context.Context, alias string) (*application.URLRecord, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	data, err := r.client.Get(ctx, r.prefix+alias).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrCacheMiss
		}
		return nil, fmt.Errorf("get alias from redis: %w", err)
	}

	var record application.URLRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("unmarshal cached record: %w", err)
	}
	return &record, nil
}

func (r *RedisCache) Set(ctx context.Context, alias string, record *application.URLRecord) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal record: %w", err)
	}

	if err := r.client.Set(ctx, r.prefix+alias, data, time.Duration(r.ttl)*time.Second).Err(); err != nil {
		return fmt.Errorf("set alias in redis: %w", err)
	}
	return nil
}

func (r *RedisCache) Delete(ctx context.Context, alias string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := r.client.Del(ctx, r.prefix+alias).Err(); err != nil {
		return fmt.Errorf("delete alias in redis: %w", err)
	}
	return nil
}
