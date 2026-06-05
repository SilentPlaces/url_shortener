package entities

import (
	"net/url"
	"regexp"
	"time"

	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain"
)

const (
	MinAliasLength = 3
	MaxAliasLength = 16
)

var aliasPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type URL struct {
	ID int64 `json:"id"`

	OriginalUrl string `json:"original_url"`
	Alias       string `json:"alias"`
	IsCustom    bool   `json:"is_custom"`

	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	IsActive  bool       `json:"is_active"`

	MetaData map[string]string `json:"metadata,omitempty"`
}

func NewURL(originalUrl, alias string, expiresAt *time.Time, metaData map[string]string, isCustom bool) (*URL, error) {
	if originalUrl == "" {
		return nil, domain.ErrInvalidURL
	}

	if !IsValidURL(originalUrl) {
		return nil, domain.ErrInvalidURL
	}

	if alias == "" {
		return nil, domain.ErrInvalidAlias
	}

	if !IsValidAlias(alias) {
		return nil, domain.ErrInvalidAlias
	}

	now := time.Now().UTC()
	if expiresAt != nil && expiresAt.Before(now) {
		return nil, domain.ErrExpired
	}

	return &URL{
		OriginalUrl: originalUrl,
		Alias:       alias,
		CreatedAt:   now,
		ExpiresAt:   expiresAt,
		IsActive:    true,
		IsCustom:    isCustom,
		MetaData:    metaData,
	}, nil
}

func (u *URL) IsExpired() bool {
	if u.ExpiresAt == nil {
		return false
	}

	if u.ExpiresAt.IsZero() {
		return false
	}

	return time.Now().After(*u.ExpiresAt)
}

func (u *URL) Deactivate() {
	u.IsActive = false
}

func (u *URL) Activate() {
	u.IsActive = true
}

func (u *URL) CanBeAccessed() bool {
	return u.IsActive && !u.IsExpired()
}

func (u *URL) UpdateExpiration(expiresAt *time.Time) {
	u.ExpiresAt = expiresAt
}

func IsValidURL(str string) bool {
	if str == "" {
		return false
	}

	parsed, err := url.Parse(str)
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}

	if parsed.Host == "" {
		return false
	}

	return true
}

func IsValidAlias(alias string) bool {
	if len(alias) < MinAliasLength || len(alias) > MaxAliasLength {
		return false
	}
	return aliasPattern.MatchString(alias)
}
