package ports

import "context"

type IDAllocator interface {
	NextID(ctx context.Context) (int64, error)
	NextBatch(ctx context.Context, batchSize int) ([]int64, error)
}
