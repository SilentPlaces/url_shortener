package ports

import (
	"context"

	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/entities"
)

type URLRepository interface {
	Insert(ctx context.Context, url *entities.URL) error

	GetByAlias(ctx context.Context, alias string) (*entities.URL, error)

	ExistsByAlias(ctx context.Context, alias string) (bool, error)
}
