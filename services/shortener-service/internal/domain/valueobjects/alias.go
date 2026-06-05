package valueobjects

import (
	"strings"

	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/entities"
)

type Alias struct {
	Value      string
	IsCustom   bool
	IsReserved bool
}

func NewAlias(value string, isCustom bool) (*Alias, error) {
	normalized := NormalizeAlias(value)

	if !entities.IsValidAlias(normalized) {
		return nil, domain.ErrInvalidAlias
	}

	if IsReservedAlias(normalized) {
		return nil, domain.ErrReservedAlias
	}

	return &Alias{
		Value:      normalized,
		IsCustom:   isCustom,
		IsReserved: false,
	}, nil
}

func NormalizeAlias(alias string) string {
	return strings.ToLower(strings.TrimSpace(alias))
}

func (a *Alias) String() string {
	return a.Value
}

func (a *Alias) Equals(other *Alias) bool {
	if other == nil {
		return false
	}
	return a.Value == other.Value
}

func IsReservedAlias(alias string) bool {
	reservedAliases := []string{
		"api",
		"admin",
		"dashboard",
		"health",
		"metrics",
		"shorten",
		"redirect",
		"stats",
		"analytics",
		"docs",
		"swagger",
		"v1",
		"v2",
		"auth",
		"login",
		"logout",
		"register",
		"about",
		"help",
		"terms",
		"privacy",
		"contact",
	}

	normalizedAlias := NormalizeAlias(alias)

	for _, reserved := range reservedAliases {
		if normalizedAlias == reserved {
			return true
		}
	}

	return false
}
