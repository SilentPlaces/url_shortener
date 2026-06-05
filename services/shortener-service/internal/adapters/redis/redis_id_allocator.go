package redis

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/ports"
	"github.com/redis/go-redis/v9"
)

// ErrAllocatorClosed is returned by NextID after Close has been called.
var ErrAllocatorClosed = errors.New("id allocator is closed")

type RedisIDAllocator struct {
	client    *redis.Client
	key       string
	batchSize int64
	buffer    chan int64
	stopCh    chan struct{}
	wg        sync.WaitGroup
	closeOnce sync.Once
}

func NewRedisIDAllocator(client *redis.Client, key string, batchSize int64, bufferSize int) ports.IDAllocator {
	if batchSize <= 0 {
		batchSize = 1024
	}
	if bufferSize <= 0 {
		bufferSize = 2048
	}

	a := &RedisIDAllocator{
		client:    client,
		key:       key,
		batchSize: batchSize,
		buffer:    make(chan int64, bufferSize),
		stopCh:    make(chan struct{}),
	}

	a.wg.Add(1)
	go a.runBufferFiller()

	return a
}

func (a *RedisIDAllocator) runBufferFiller() {
	defer a.wg.Done()

	refillThreshold := cap(a.buffer) / 4
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		if len(a.buffer) <= refillThreshold {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := a.fillBuffer(ctx)
			cancel()
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, errStopped) {
					return
				}
				log.Printf("failed to fill id buffer: %v; retrying", err)
				select {
				case <-a.stopCh:
					return
				case <-time.After(time.Second):
				}
				continue
			}
		}

		select {
		case <-a.stopCh:
			return
		case <-ticker.C:
		}
	}
}

var errStopped = errors.New("allocator stopped")

func (a *RedisIDAllocator) NextID(ctx context.Context) (int64, error) {
	select {
	case <-a.stopCh:
		return 0, ErrAllocatorClosed
	default:
	}

	select {
	case id, ok := <-a.buffer:
		if !ok {
			return 0, ErrAllocatorClosed
		}
		return id, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-a.stopCh:
		return 0, ErrAllocatorClosed
	}
}

func (a *RedisIDAllocator) fillBuffer(ctx context.Context) error {
	end, err := a.client.IncrBy(ctx, a.key, a.batchSize).Result()
	if err != nil {
		return err
	}

	for i := end - a.batchSize + 1; i <= end; i++ {
		select {
		case a.buffer <- i:
		case <-a.stopCh:
			return errStopped
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (a *RedisIDAllocator) Close() error {
	a.closeOnce.Do(func() {
		close(a.stopCh)
		a.wg.Wait()
		close(a.buffer)
	})
	return nil
}
