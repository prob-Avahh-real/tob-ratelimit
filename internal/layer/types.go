package layer

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

// BucketParams defines token bucket capacity and refill rate.
type BucketParams struct {
	Capacity float64
	Rate     float64
	Weight   float64
}

// EndpointLimit applies to a specific route pattern.
type EndpointLimit struct {
	Pattern       string
	Params        BucketParams
	IdentityScope string // ip, device, user
}

// Config holds layered rate limit defaults for the resolver.
type Config struct {
	BlockedIPs         map[string]struct{}
	CrawlerIPs         map[string]struct{}
	Crawler            BucketParams
	Anonymous          BucketParams
	User               BucketParams
	Endpoints          []EndpointLimit
	RedisKeyPrefix     string
	BatchWindow        time.Duration
	BatchThreshold     int
	LocalFallbackScale float64
}
