package detector

import (
	"context"
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

// Result reports batch abuse and extra token cost.
type Result struct {
	Abusive    bool
	CostFactor float64
}

func (c BatchConfig) costFactor(count int) Result {
	if c.Threshold <= 0 || count <= c.Threshold {
		return Result{CostFactor: 1}
	}
	max := c.MaxFactor
	if max <= 0 {
		max = 10
	}
	over := float64(count - c.Threshold)
	factor := 1 + min(over*0.5, max-1)
	return Result{Abusive: true, CostFactor: factor}
}

// Detector tracks rapid sequential calls from the same identity.
type Detector interface {
	Record(ctx context.Context, key string) (Result, error)
}

type localDetector struct {
	cfg BatchConfig
	mu  sync.Mutex
	evt map[string][]time.Time
}

// NewLocal creates an in-memory batch detector.
func NewLocal(cfg BatchConfig) Detector {
	if cfg.Threshold <= 0 {
		cfg.Threshold = 20
	}
	if cfg.Window <= 0 {
		cfg.Window = 10 * time.Second
	}
	if cfg.MaxFactor <= 0 {
		cfg.MaxFactor = 10
	}
	return &localDetector{cfg: cfg, evt: make(map[string][]time.Time)}
}

func (d *localDetector) Record(_ context.Context, key string) (Result, error) {
	now := time.Now()
	cutoff := now.Add(-d.cfg.Window)

	d.mu.Lock()
	defer d.mu.Unlock()

	events := d.evt[key]
	fresh := events[:0]
	for _, t := range events {
		if t.After(cutoff) {
			fresh = append(fresh, t)
		}
	}
	fresh = append(fresh, now)
	d.evt[key] = fresh

	return d.cfg.costFactor(len(fresh)), nil
}

type redisDetector struct {
	cfg    BatchConfig
	prefix string
	client *redis.Client
	script *redis.Script
}

// NewRedis creates a Redis-backed batch detector.
func NewRedis(client *redis.Client, prefix string, cfg BatchConfig) Detector {
	if cfg.Threshold <= 0 {
		cfg.Threshold = 20
	}
	if cfg.Window <= 0 {
		cfg.Window = 10 * time.Second
	}
	if cfg.MaxFactor <= 0 {
		cfg.MaxFactor = 10
	}
	return &redisDetector{
		cfg:    cfg,
		prefix: prefix,
		client: client,
		script: redis.NewScript(slidingWindowLua),
	}
}

func (d *redisDetector) Record(ctx context.Context, key string) (Result, error) {
	nowMs := time.Now().UnixMilli()
	windowMs := d.cfg.Window.Milliseconds()
	raw, err := d.script.Run(ctx, d.client, []string{d.prefix + "batch:" + key}, nowMs, windowMs, windowMs*2).Int64()
	if err != nil {
		return Result{}, err
	}
	return d.cfg.costFactor(int(raw)), nil
}

type fallbackDetector struct {
	primary Detector
	local   Detector
}

// NewFallback tries Redis first, then falls back to local detection.
func NewFallback(primary, local Detector) Detector {
	return &fallbackDetector{primary: primary, local: local}
}

func (f *fallbackDetector) Record(ctx context.Context, key string) (Result, error) {
	res, err := f.primary.Record(ctx, key)
	if err != nil {
		return f.local.Record(ctx, key)
	}
	return res, nil
}
