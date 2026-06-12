package ports

import "context"

type BloomFilter interface {
	Add(ctx context.Context, key string) error
	AddMany(ctx context.Context, keys []string) error
	MightContain(ctx context.Context, key string) (bool, error)
}
