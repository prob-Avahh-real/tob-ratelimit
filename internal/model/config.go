package model

import "time"

// BucketParams defines token bucket capacity and refill rate.
type BucketParams struct {
	// Capacity is the maximum burst size (tokens).
	Capacity float64
	// Rate is tokens added per second.
	Rate float64
	// Weight scales refill for priority tiers (>1 = faster refill).
	Weight float64
}

// EndpointLimit applies to a specific route pattern.
type EndpointLimit struct {
	Pattern string
	Params  BucketParams
	// IdentityScope controls key suffix: ip, device, user.
	IdentityScope string
}

// Config holds layered rate limit defaults.
type Config struct {
	// BlockedIPs are hard-blocked (0 tokens).
	BlockedIPs map[string]struct{}
	// CrawlerIPs get minimal quota.
	CrawlerIPs map[string]struct{}
	Crawler    BucketParams
	Anonymous  BucketParams
	User       BucketParams
	Endpoints  []EndpointLimit
	// Batch detection thresholds.
	BatchWindow    time.Duration
	BatchThreshold int
	// RedisKeyPrefix prefixes all Redis keys.
	RedisKeyPrefix string
	// LocalFallbackScale shrinks limits when Redis is unavailable.
	LocalFallbackScale float64
}

// DefaultConfig returns production-oriented defaults per requirements.
func DefaultConfig() Config {
	return Config{
		BlockedIPs: make(map[string]struct{}),
		CrawlerIPs: make(map[string]struct{}),
		Crawler: BucketParams{
			Capacity: 1,
			Rate:     1.0 / 60.0, // 1 req/min
			Weight:   1,
		},
		Anonymous: BucketParams{
			Capacity: 5,
			Rate:     10.0 / 60.0, // 10 req/min
			Weight:   1,
		},
		User: BucketParams{
			Capacity: 200,
			Rate:     1000.0 / 60.0, // 1000 req/min
			Weight:   1.5,           // priority refill
		},
		Endpoints: []EndpointLimit{
			{
				Pattern:       "/api/login",
				Params:        BucketParams{Capacity: 5, Rate: 5.0 / 60.0, Weight: 1},
				IdentityScope: "ip",
			},
			{
				Pattern:       "/api/auth/login",
				Params:        BucketParams{Capacity: 5, Rate: 5.0 / 60.0, Weight: 1},
				IdentityScope: "ip",
			},
		},
		BatchWindow:        10 * time.Second,
		BatchThreshold:     20,
		RedisKeyPrefix:     "rl",
		LocalFallbackScale: 0.5,
	}
}
