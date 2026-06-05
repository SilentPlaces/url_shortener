package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/arminaray/url_shortener/services/redirector-service/internal/application"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) GetByAlias(ctx context.Context, alias string) (*application.URLRecord, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	query := `SELECT alias, original_url, expires_at, is_active FROM urls WHERE alias = $1`

	var record application.URLRecord
	var expiresAt sql.NullTime

	err := r.db.QueryRowContext(ctxTimeout, query, alias).Scan(
		&record.Alias,
		&record.OriginalURL,
		&expiresAt,
		&record.IsActive,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, application.ErrNotFound
		}
		return nil, fmt.Errorf("get alias from postgres: %w", err)
	}

	if expiresAt.Valid {
		record.ExpiresAt = &expiresAt.Time
	}

	return &record, nil
}
