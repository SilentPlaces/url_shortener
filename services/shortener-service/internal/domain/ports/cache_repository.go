package ports

import (
	"context"

	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/entities"
)

type CacheRepository interface {
	Get(ctx context.Context, key string) (*entities.URL, error)
	Set(ctx context.Context, key string, url *entities.URL) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	Expire(ctx context.Context, key string, ttl int) error
}
