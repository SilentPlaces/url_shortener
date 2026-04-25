package domain

import (
	"net/url"
	"regexp"
	"time"
)

const (
	MinAliasLength = 3
	MaxAliasLength = 50
)

type URL struct {
	ID int64

	OriginalUrl string
	Alias       string

	CreatedAt time.Time
	ExpiresAt *time.Time
	IsActive  bool

	MetaData map[string]string
}

func NewURL(originalUrl, alias string, expiresAt *time.Time, metaData map[string]string) (*URL, error) {
	//Validate URL format
	if originalUrl == "" {
		return nil, ErrInvalidURL
	}

	if !IsValidURL(originalUrl) {
		return nil, ErrInvalidURL
	}

	//Validate alias
	if alias == "" {
		return nil, ErrInvalidAlias
	}

	if !IsValidAlias(alias) {
		return nil, ErrInvalidAlias
	}

	if expiresAt != nil && expiresAt.Before(time.Now()) {
		return nil, ErrExpired
	}

	//set metadata
	return &URL{
		OriginalUrl: originalUrl,
		Alias:       alias,
		CreatedAt:   time.Now(),
		ExpiresAt:   expiresAt,
		IsActive:    true,
		MetaData:    metaData,
	}, nil
}

func (u *URL) IsExpired() bool {
	if u.ExpiresAt == nil {
		return true
	}

	if u.ExpiresAt.IsZero() {
		return false
	}

	return u.ExpiresAt.After(time.Now())
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

	// Only allow alphanumeric characters, hyphens, and underscores
	matched, err := regexp.MatchString("^[a-zA-Z0-9_-]+$", alias)
	if err != nil {
		return false
	}

	return matched
}
