package cache

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/ports"
	"github.com/redis/go-redis/v9"
)

type RedisIDAllocator struct {
	client    *redis.Client
	key       string
	batchSize int64
	buffer    chan int64
	stopCh    chan struct{}
	wg        sync.WaitGroup
	once      sync.Once
}

func NewRedisIDAllocator(client *redis.Client, key string, batchSize int64, bufferSize int) ports.IDAllocator {
	idAllocator := &RedisIDAllocator{
		client:    client,
		key:       key,
		batchSize: batchSize,
		buffer:    make(chan int64, bufferSize),
		stopCh:    make(chan struct{}),
	}

	idAllocator.wg.Add(1)
	go idAllocator.runBufferFiller()

	return idAllocator
}

func (a *RedisIDAllocator) runBufferFiller() {
	defer a.wg.Done()

	// Calculate when to refill (e.g., when buffer is 75% empty)
	refillThreshold := cap(a.buffer) * 25 / 100

	for {
		select {
		case <-a.stopCh:
			return
		default:
			// Check if buffer needs refilling
			if len(a.buffer) <= refillThreshold {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				err := a.fillBuffer(ctx)
				cancel()

				if err != nil {
					// Log error and retry after delay instead of panicking
					log.Printf("Failed to fill buffer: %v. Retrying...\n", err)
					time.Sleep(1 * time.Second)
					continue
				}
			}

			// Small sleep to prevent busy-waiting
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (a *RedisIDAllocator) NextID(ctx context.Context) (int64, error) {
	select {
	case id := <-a.buffer:
		return id, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-a.stopCh:
		return 0, fmt.Errorf("allocator is shutting down")
	}
}

func (a *RedisIDAllocator) fillBuffer(ctx context.Context) error {
	start, err := a.client.IncrBy(ctx, a.key, a.batchSize).Result()
	if err != nil {
		return fmt.Errorf("failed to increment ID: %w", err)
	}

	for i := start - a.batchSize + 1; i <= start; i++ {
		select {
		case a.buffer <- i:
		case <-a.stopCh:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// Close gracefully shuts down the allocator
func (a *RedisIDAllocator) Close() error {
	a.once.Do(func() {
		close(a.stopCh)
	})
	a.wg.Wait()
	close(a.buffer)
	return nil
}
