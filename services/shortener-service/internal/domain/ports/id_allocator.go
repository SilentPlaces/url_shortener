package ports

import "context"

type IDAllocator interface {
	NextID(ctx context.Context) (int64, error)
}
