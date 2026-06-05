package events

import "time"

type URLCreatedEvent struct {
	URLID       string
	OriginalURL string
	Alias       string
	IsCustom    bool
	CreatedAt   time.Time
	ExpiresAt   *time.Time
}

type URLClickedEvent struct {
	URLID     string
	Alias     string
	ClickedAt time.Time
	UserAgent string
	IPAddress string
	Referrer  string
}

type URLExpiredEvent struct {
	URLID     string
	Alias     string
	ExpiredAt time.Time
}

type URLDeactivatedEvent struct {
	URLID         string
	Alias         string
	DeactivatedAt time.Time
	Reason        string
}
