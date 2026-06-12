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

func (e URLCreatedEvent) EventKey() string { return e.URLID }

type URLClickedEvent struct {
	URLID     string
	Alias     string
	ClickedAt time.Time
	UserAgent string
	IPAddress string
	Referrer  string
}

func (e URLClickedEvent) EventKey() string { return e.URLID }

type URLExpiredEvent struct {
	URLID     string
	Alias     string
	ExpiredAt time.Time
}

func (e URLExpiredEvent) EventKey() string { return e.URLID }

type URLDeactivatedEvent struct {
	URLID         string
	Alias         string
	DeactivatedAt time.Time
	Reason        string
}

func (e URLDeactivatedEvent) EventKey() string { return e.URLID }
