package bucket

import (
	"context"
	"time"
)

// Params configures a token bucket instance.
type Params struct {
	Capacity float64 // max burst (tokens)
	Rate     float64 // sustained refill (tokens/sec)
	Weight   float64 // priority multiplier for authenticated users
}

// Result holds consume outcome and header values.
type Result struct {
	Allowed    bool
	Limit      float64
	Remaining  float64
	RetryAfter time.Duration
}

// Bucket consumes tokens from a distributed or local store.
type Bucket interface {
	Consume(ctx context.Context, key string, params Params, cost float64) (Result, error)
}
