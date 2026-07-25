package ratelimit

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"

	"github.com/skat/tob-ratelimit/internal/bucket"
	"github.com/skat/tob-ratelimit/internal/detector"
	"github.com/skat/tob-ratelimit/internal/layer"
)

// Limiter enforces layered token-bucket limits with batch detection.
type Limiter struct {
	cfg      Config
	resolver *layer.Resolver
	bucket   bucket.Bucket
	detector detector.Detector
}

// New creates a limiter backed by Redis with local fallback.
func New(cfg Config, client *goredis.Client) *Limiter {
	if cfg.RedisKeyPrefix == "" {
		cfg.RedisKeyPrefix = "rl"
	}
	if cfg.BlockedIPs == nil {
		cfg.BlockedIPs = make(map[string]struct{})
	}
	if cfg.CrawlerIPs == nil {
		cfg.CrawlerIPs = make(map[string]struct{})
	}
	scale := cfg.LocalFallbackScale
	if scale <= 0 || scale > 1 {
		scale = 0.5
	}

	var store bucket.Bucket
	fallbackLocal := bucket.NewLocal(scale)
	if client != nil {
		store = bucket.NewFallback(bucket.NewRedis(client), fallbackLocal)
	} else {
		store = bucket.NewLocal(1) // full quota when Redis is not configured
	}

	batchCfg := detector.BatchConfig{
		Window:    cfg.BatchWindow,
		Threshold: cfg.BatchThreshold,
		MaxFactor: 10,
	}
	var det detector.Detector
	localDet := detector.NewLocal(batchCfg)
	if client != nil {
		det = detector.NewFallback(
			detector.NewRedis(client, cfg.RedisKeyPrefix+":", batchCfg),
			localDet,
		)
	} else {
		det = localDet
	}

	return &Limiter{
		cfg:      cfg,
		resolver: layer.NewResolver(toLayerConfig(cfg)),
		bucket:   store,
		detector: det,
	}
}

// Allow checks all resolved layers; first denial wins.
func (l *Limiter) Allow(ctx context.Context, id Identity) (Result, error) {
	rules := l.resolver.Resolve(toLayerIdentity(id))
	if len(rules) == 0 {
		return Result{Allowed: true, Tier: TierUser}, nil
	}

	batchKey := batchIdentity(id)
	var batchRes detector.Result
	if l.cfg.BatchThreshold > 0 && l.cfg.BatchWindow > 0 {
		var err error
		batchRes, err = l.detector.Record(ctx, batchKey+":"+id.Endpoint)
		if err != nil {
			return Result{}, fmt.Errorf("batch detector: %w", err)
		}
	}

	for _, rule := range rules {
		cost := 1.0
		if batchRes.Abusive && rule.Tier != layer.TierBlock && rule.Tier != layer.TierCrawler {
			cost *= batchRes.CostFactor
		}

		res, err := l.bucket.Consume(ctx, rule.Key, rule.Params, cost)
		if err != nil {
			return Result{}, err
		}

		if !res.Allowed {
			reason := string(rule.Tier) + "_rate_exceeded"
			if rule.Tier == layer.TierBlock {
				reason = "ip_blocked"
			}
			return Result{
				Allowed:    false,
				Tier:       fromLayerTier(rule.Tier),
				Limit:      rule.Params.Capacity,
				Remaining:  res.Remaining,
				RetryAfter: res.RetryAfter,
				Key:        rule.Key,
				Reason:     reason,
			}, nil
		}
	}

	last := rules[len(rules)-1]
	return Result{
		Allowed: true,
		Tier:    fromLayerTier(last.Tier),
		Limit:   last.Params.Capacity,
		Key:     last.Key,
	}, nil
}

func batchIdentity(id Identity) string {
	if id.UserID != "" {
		return "usr:" + id.UserID
	}
	if id.DeviceID != "" {
		return "dev:" + id.DeviceID
	}
	return "ip:" + id.IP
}
