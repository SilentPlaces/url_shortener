package cache

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
	err := client.BFReserve(ctx, key, falsePositiveRate, expectedItems)
	if err != nil {
		// If filter already exists, that's okay
		if strings.Contains(err.Err().Error(), "exists") {
			return bf, nil
		}
	}

	return bf, nil
}

func (bf *RedisBloomFilter) Add(ctx context.Context, key string) error {
	// BF.ADD adds an item to the bloom filter
	err := bf.client.BFAdd(ctx, bf.key, key).Err()
	if err != nil {
		return fmt.Errorf("failed to add key to bloom filter: %w", err)
	}
	return nil
}

func (bf *RedisBloomFilter) MightContain(ctx context.Context, key string) (bool, error) {
	// BF.EXISTS checks if an item might exist in the bloom filter
	//result, err := bf.client.Do(ctx, "BF.EXISTS", bf.key, key).Result()
	result, err := bf.client.BFExists(ctx, bf.key, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check bloom filter: %w", err)
	}

	return result, nil
}
