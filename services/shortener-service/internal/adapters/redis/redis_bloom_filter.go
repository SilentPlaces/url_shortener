package redis

import (
	"context"
	"fmt"
	"strings"

	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/ports"
	"github.com/redis/go-redis/v9"
)

type RedisBloomFilter struct {
	client *redis.Client
	key    string
}

func NewRedisBloomFilter(client *redis.Client, key string, expectedItems int64, falsePositiveRate float64) (ports.BloomFilter, error) {
	bf := &RedisBloomFilter{
		client: client,
		key:    key,
	}

	ctx := context.Background()
	err := client.BFReserve(ctx, key, falsePositiveRate, expectedItems).Err()
	if err != nil {
		if strings.Contains(err.Error(), "exists") {
			return bf, nil
		}
		return nil, fmt.Errorf("failed to initialize bloom filter: %w", err)
	}

	return bf, nil
}

func (bf *RedisBloomFilter) Add(ctx context.Context, key string) error {
	err := bf.client.BFAdd(ctx, bf.key, key).Err()
	if err != nil {
		return fmt.Errorf("failed to add key to bloom filter: %w", err)
	}
	return nil
}

func (bf *RedisBloomFilter) AddMany(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	args := make([]any, 0, len(keys))
	for _, k := range keys {
		args = append(args, k)
	}
	if err := bf.client.BFMAdd(ctx, bf.key, args...).Err(); err != nil {
		return fmt.Errorf("failed to bulk-add keys to bloom filter: %w", err)
	}
	return nil
}

func (bf *RedisBloomFilter) MightContain(ctx context.Context, key string) (bool, error) {
	result, err := bf.client.BFExists(ctx, bf.key, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check bloom filter: %w", err)
	}

	return result, nil
}
