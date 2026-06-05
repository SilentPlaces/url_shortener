package domain

import "fmt"

type Error struct {
	Code    string // Error code for programmatic handling
	Message string // Human-readable message
	Err     error  // Wrapped underlying error (if any)
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Err
}

func NewAliasTakenError(alias string) error {
	return &Error{
		Code:    "ALIAS_TAKEN",
		Message: fmt.Sprintf("alias '%s' is already taken", alias),
	}
}

func NewURLNotFoundError(alias string) error {
	return &Error{
		Code:    "URL_NOT_FOUND",
		Message: fmt.Sprintf("URL with alias '%s' not found", alias),
	}
}

func NewInvalidAliasError(alias string, reason string) error {
	return &Error{
		Code:    "INVALID_ALIAS",
		Message: fmt.Sprintf("invalid alias '%s': %s", alias, reason),
	}
}

var (
	ErrInvalidURL          = &Error{Code: "INVALID_URL", Message: "invalid URL format"}
	ErrAliasRequired       = &Error{Code: "ALIAS_REQUIRED", Message: "alias is required"}
	ErrOriginalURLRequired = &Error{Code: "URL_REQUIRED", Message: "original URL is required"}
	ErrExpired             = &Error{Code: "EXPIRED", Message: "URL has expired"}
	ErrReservedAlias       = &Error{Code: "RESERVED_ALIAS", Message: "alias is reserved"}
	ErrURLInactive         = &Error{Code: "URL_INACTIVE", Message: "URL is inactive"}
	ErrInvalidAlias        = &Error{Code: "INVALID_ALIAS", Message: "invalid alias format"}
)
