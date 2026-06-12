package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"

	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/entities"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/ports"
)

const pgUniqueViolation = "23505"

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Insert(ctx context.Context, url *entities.URL) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
		INSERT INTO urls (alias, id, original_url, is_custom, created_at, expires_at, is_active, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	metadataJSON, err := json.Marshal(url.MetaData)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	_, err = r.db.ExecContext(ctx,
		query,
		url.Alias,
		url.ID,
		url.OriginalUrl,
		url.IsCustom,
		url.CreatedAt,
		url.ExpiresAt,
		url.IsActive,
		metadataJSON)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.NewAliasTakenError(url.Alias)
		}
		return fmt.Errorf("failed to insert URL: %w", err)
	}
	return nil
}

func (r *PostgresRepository) GetByAlias(ctx context.Context, alias string) (*entities.URL, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `SELECT alias, id, original_url, is_custom, created_at, expires_at, is_active, metadata FROM urls WHERE alias = $1`

	row := r.db.QueryRowContext(ctx, query, alias)

	var expiresAt sql.NullTime
	var metadataJSON []byte
	var url entities.URL

	err := row.Scan(
		&url.Alias,
		&url.ID,
		&url.OriginalUrl,
		&url.IsCustom,
		&url.CreatedAt,
		&expiresAt,
		&url.IsActive,
		&metadataJSON,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.NewURLNotFoundError(alias)
		}
		return nil, fmt.Errorf("failed to get URL by alias: %w", err)
	}

	if expiresAt.Valid {
		url.ExpiresAt = &expiresAt.Time
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &url.MetaData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &url, nil
}

func (r *PostgresRepository) ExistsByAlias(ctx context.Context, alias string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `SELECT EXISTS(SELECT 1 FROM urls WHERE alias = $1)`

	var exists bool
	if err := r.db.QueryRowContext(ctx, query, alias).Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check if alias exists: %w", err)
	}
	return exists, nil
}

func (r *PostgresRepository) IterateAliases(ctx context.Context, batchSize int, visit ports.AliasVisitor) error {
	if batchSize <= 0 {
		batchSize = 1000
	}

	const query = `
		SELECT alias FROM urls
		WHERE alias > $1
		ORDER BY alias ASC
		LIMIT $2
	`

	cursor := ""
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		batch, err := r.fetchAliasBatch(ctx, query, cursor, batchSize)
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			return nil
		}
		if err := visit(batch); err != nil {
			return err
		}
		cursor = batch[len(batch)-1]
		if len(batch) < batchSize {
			return nil
		}
	}
}

func (r *PostgresRepository) fetchAliasBatch(ctx context.Context, query, cursor string, limit int) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, query, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("query aliases: %w", err)
	}
	defer rows.Close()

	batch := make([]string, 0, limit)
	for rows.Next() {
		var alias string
		if err := rows.Scan(&alias); err != nil {
			return nil, fmt.Errorf("scan alias: %w", err)
		}
		batch = append(batch, alias)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate aliases: %w", err)
	}
	return batch, nil
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == pgUniqueViolation
	}
	return false
}
