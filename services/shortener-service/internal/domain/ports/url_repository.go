package ports

import (
	"context"

	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/entities"
)

type AliasVisitor func(aliases []string) error

type URLRepository interface {
	Insert(ctx context.Context, url *entities.URL) error
	GetByAlias(ctx context.Context, alias string) (*entities.URL, error)
	ExistsByAlias(ctx context.Context, alias string) (bool, error)
	IterateAliases(ctx context.Context, batchSize int, visit AliasVisitor) error
}
