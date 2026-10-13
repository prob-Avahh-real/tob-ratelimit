package detector

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// BatchConfig configures sliding-window batch abuse detection.
type BatchConfig struct {
	Window    time.Duration
	Threshold int
	MaxFactor float64
}

func (c BatchConfig) normalized() BatchConfig {
	out := c
	if out.Threshold <= 0 {
		out.Threshold = 20
	}
	if out.Window <= 0 {
		out.Window = 10 * time.Second
	}
	if out.MaxFactor <= 0 {
		out.MaxFactor = 10
	}
	return out
}

// Result reports batch abuse status for a request identity.
type Result struct {
	Count      int
	Abusive    bool
	CostFactor float64
}

func (c BatchConfig) costFactor(count int) Result {
	cfg := c.normalized()
	if count <= cfg.Threshold {
		return Result{Count: count, Abusive: false, CostFactor: 1}
	}
	over := float64(count - cfg.Threshold)
	factor := 1 + min(over*0.5, cfg.MaxFactor-1)
	return Result{Count: count, Abusive: true, CostFactor: factor}
}

// Detector records request events and flags batch abuse.
type Detector interface {
	Record(ctx context.Context, key string) (Result, error)
}

type localDetector struct {
	cfg BatchConfig
	mu  sync.Mutex
	// key -> event timestamps
	events map[string][]time.Time
}

// NewLocal creates an in-memory batch detector.
func NewLocal(cfg BatchConfig) Detector {
	return &localDetector{
		cfg:    cfg.normalized(),
		events: make(map[string][]time.Time),
	}
}

func (d *localDetector) Record(_ context.Context, key string) (Result, error) {
	now := time.Now()
	cutoff := now.Add(-d.cfg.Window)

	d.mu.Lock()
	defer d.mu.Unlock()

	events := d.events[key]
	fresh := events[:0]
	for _, t := range events {
		if t.After(cutoff) {
			fresh = append(fresh, t)
		}
	}
	fresh = append(fresh, now)
	d.events[key] = fresh

	return d.cfg.costFactor(len(fresh)), nil
}

type redisDetector struct {
	cfg    BatchConfig
	client *redis.Client
	prefix string
	script *redis.Script
}

// NewRedis creates a Redis-backed sliding-window batch detector.
func NewRedis(client *redis.Client, prefix string, cfg BatchConfig) Detector {
	return &redisDetector{
		cfg:    cfg.normalized(),
		client: client,
		prefix: prefix,
		script: redis.NewScript(slidingWindowLua),
	}
}

func (d *redisDetector) Record(ctx context.Context, key string) (Result, error) {
	fullKey := d.prefix + "batch:" + key
	nowMs := time.Now().UnixMilli()
	windowMs := d.cfg.Window.Milliseconds()
	raw, err := d.script.Run(ctx, d.client, []string{fullKey}, nowMs, windowMs, windowMs*2).Int64()
	if err != nil {
		return Result{}, fmt.Errorf("batch detector: %w", err)
	}
	return d.cfg.costFactor(int(raw)), nil
}

type fallbackDetector struct {
	primary Detector
	fallback Detector
}

// NewFallback tries primary (Redis) and falls back to local on error.
func NewFallback(primary, fallback Detector) Detector {
	return &fallbackDetector{primary: primary, fallback: fallback}
}

func (d *fallbackDetector) Record(ctx context.Context, key string) (Result, error) {
	res, err := d.primary.Record(ctx, key)
	if err == nil {
		return res, nil
	}
	return d.fallback.Record(ctx, key)
}
