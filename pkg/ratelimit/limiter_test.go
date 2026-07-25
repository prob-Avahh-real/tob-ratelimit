package ratelimit

import (
	"context"
	"testing"
)

func TestLimiterBlockedIP(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BlockedIPs["10.0.0.1"] = struct{}{}
	l := New(cfg, nil)

	res, err := l.Allow(context.Background(), Identity{
		IP:       "10.0.0.1",
		UserID:   "u1",
		Endpoint: "/api/data",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Allowed {
		t.Fatal("blocked IP should be denied")
	}
	if res.Tier != TierBlock {
		t.Fatalf("expected block tier, got %s", res.Tier)
	}
}

func TestLimiterUserBurst(t *testing.T) {
	cfg := DefaultConfig()
	cfg.User = BucketParams{Capacity: 3, Rate: 1000.0 / 60.0, Weight: 1.5}
	l := New(cfg, nil)

	id := Identity{UserID: "user-1", Endpoint: "/api/x"}
	for i := 0; i < 3; i++ {
		res, err := l.Allow(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if !res.Allowed {
			t.Fatalf("burst request %d denied", i+1)
		}
	}

	res, err := l.Allow(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if res.Allowed {
		t.Fatal("4th immediate request should be rate limited")
	}
}

func TestLimiterBatchPenalty(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BatchWindow = 0 // disable batch detector for predictable bucket test
	cfg.User = BucketParams{Capacity: 5, Rate: 1000.0 / 60.0, Weight: 1}
	l := New(cfg, nil)

	id := Identity{UserID: "batch-user", Endpoint: "/api/items"}
	for i := 0; i < 5; i++ {
		res, _ := l.Allow(context.Background(), id)
		if !res.Allowed {
			t.Fatalf("request %d should pass", i+1)
		}
	}
	res, _ := l.Allow(context.Background(), id)
	if res.Allowed {
		t.Fatal("expected rate limit after burst")
	}
}
