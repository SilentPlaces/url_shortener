package application

import "time"

type ShortenURLRequest struct {
	OriginalURL string            `json:"original_url"`
	CustomAlias string            `json:"custom_alias,omitempty"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type ShortenURLResponse struct {
	OriginalURL string            `json:"original_url"`
	ShortURL    string            `json:"short_url"`
	Alias       string            `json:"alias,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type GetURLResponse struct {
	ID          int64             `json:"id"`
	OriginalURL string            `json:"original_url"`
	Alias       string            `json:"alias"`
	CreatedAt   time.Time         `json:"created_at"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
	IsActive    bool              `json:"is_active"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}
