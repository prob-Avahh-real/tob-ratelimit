package bucket

import (
	"context"
	"sync"
	"time"
)

type localEntry struct {
	tokens float64
	ts     time.Time
}

// LocalBucket is an in-memory fallback with conservative scaled limits.
type LocalBucket struct {
	mu      sync.Mutex
	entries map[string]*localEntry
	scale   float64
}

// NewLocal creates a local bucket. scale in (0,1] shrinks limits when Redis is down.
func NewLocal(scale float64) *LocalBucket {
	if scale <= 0 || scale > 1 {
		scale = 0.5
	}
	return &LocalBucket{
		entries: make(map[string]*localEntry),
		scale:   scale,
	}
}

func (b *LocalBucket) scaled(p Params) Params {
	return Params{
		Capacity: p.Capacity * b.scale,
		Rate:     p.Rate * b.scale,
		Weight:   p.Weight,
	}
}

func (b *LocalBucket) Consume(_ context.Context, key string, params Params, cost float64) (Result, error) {
	p := b.scaled(params)
	now := time.Now()

	b.mu.Lock()
	defer b.mu.Unlock()

	e, ok := b.entries[key]
	if !ok {
		e = &localEntry{tokens: p.Capacity, ts: now}
		b.entries[key] = e
	}

	elapsed := now.Sub(e.ts).Seconds()
	if elapsed > 0 {
		e.tokens = min(p.Capacity, e.tokens+elapsed*p.Rate*p.Weight)
		e.ts = now
	}

	if e.tokens >= cost {
		e.tokens -= cost
		return Result{Allowed: true, Limit: p.Capacity, Remaining: e.tokens}, nil
	}

	deficit := cost - e.tokens
	var retry time.Duration
	if p.Rate*p.Weight > 0 {
		retry = time.Duration(deficit/(p.Rate*p.Weight)*1000) * time.Millisecond
	} else {
		retry = time.Minute
	}
	return Result{
		Allowed:    false,
		Limit:      p.Capacity,
		Remaining:  e.tokens,
		RetryAfter: retry,
	}, nil
}
