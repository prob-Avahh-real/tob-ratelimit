package bucket

import "context"

// FallbackBucket tries Redis first, then local in-memory on error.
type FallbackBucket struct {
	primary *RedisBucket
	local   *LocalBucket
}

// NewFallback wires primary Redis with conservative local fallback.
func NewFallback(primary *RedisBucket, local *LocalBucket) *FallbackBucket {
	return &FallbackBucket{primary: primary, local: local}
}

func (b *FallbackBucket) Consume(ctx context.Context, key string, params Params, cost float64) (Result, error) {
	res, err := b.primary.Consume(ctx, key, params, cost)
	if err == nil {
		return res, nil
	}
	return b.local.Consume(ctx, key, params, cost)
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
