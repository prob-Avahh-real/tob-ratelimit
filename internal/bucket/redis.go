package bucket

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisBucket implements a distributed token bucket via Redis + Lua.
type RedisBucket struct {
	client *redis.Client
	script *redis.Script
}

// NewRedis creates a Redis-backed token bucket.
func NewRedis(client *redis.Client) *RedisBucket {
	return &RedisBucket{
		client: client,
		script: redis.NewScript(tokenBucketLua),
	}
}

func (b *RedisBucket) Consume(ctx context.Context, key string, params Params, cost float64) (Result, error) {
	nowMs := time.Now().UnixMilli()
	raw, err := b.script.Run(ctx, b.client, []string{key},
		params.Capacity, params.Rate, params.Weight, cost, nowMs,
	).Int64Slice()
	if err != nil {
		return Result{}, fmt.Errorf("redis token bucket: %w", err)
	}
	if len(raw) < 3 {
		return Result{}, fmt.Errorf("unexpected lua response: %v", raw)
	}
	return Result{
		Allowed:    raw[0] == 1,
		Limit:      params.Capacity,
		Remaining:  float64(raw[1]),
		RetryAfter: time.Duration(raw[2]) * time.Millisecond,
	}, nil
}
