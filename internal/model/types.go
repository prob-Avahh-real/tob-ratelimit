package model

import "time"

// Tier identifies a rate-limit layer.
type Tier string

const (
	TierBlock     Tier = "block"
	TierCrawler   Tier = "crawler"
	TierAnonymous Tier = "anonymous"
	TierUser      Tier = "user"
	TierEndpoint  Tier = "endpoint"
)

// Identity captures request actors for layered limiting.
type Identity struct {
	IP       string
	DeviceID string
	UserID   string
	Endpoint string
}

// IsAuthenticated reports whether the caller is a logged-in user.
func (id Identity) IsAuthenticated() bool {
	return id.UserID != ""
}

// Result is returned after a limit check.
type Result struct {
	Allowed    bool
	Tier       Tier
	Limit      float64
	Remaining  float64
	RetryAfter time.Duration
	Key        string
	Reason     string
}
