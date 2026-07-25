package layer

import (
	"testing"
)

func defaultTestConfig() Config {
	return Config{
		RedisKeyPrefix: "rl",
		BlockedIPs:     make(map[string]struct{}),
		CrawlerIPs:     make(map[string]struct{}),
		Crawler: BucketParams{
			Capacity: 1,
			Rate:     1.0 / 60.0,
			Weight:   1,
		},
		Anonymous: BucketParams{
			Capacity: 5,
			Rate:     10.0 / 60.0,
			Weight:   1,
		},
		User: BucketParams{
			Capacity: 200,
			Rate:     1000.0 / 60.0,
			Weight:   1.5,
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
	}
}

func TestResolveBlockedIP(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.BlockedIPs["10.0.0.1"] = struct{}{}
	r := NewResolver(cfg)

	rules := r.Resolve(Identity{
		IP:       "10.0.0.1",
		UserID:   "u1",
		Endpoint: "/api/data",
	})
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Tier != TierBlock {
		t.Fatalf("expected block tier, got %s", rules[0].Tier)
	}
	if rules[0].Params.Capacity != 0 {
		t.Fatal("block tier should have zero capacity")
	}
}

func TestResolveAuthenticatedUser(t *testing.T) {
	cfg := defaultTestConfig()
	r := NewResolver(cfg)

	rules := r.Resolve(Identity{
		IP:       "1.2.3.4",
		UserID:   "user-42",
		Endpoint: "/api/profile",
	})
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Tier != TierUser {
		t.Fatalf("expected user tier, got %s", rules[0].Tier)
	}
	if rules[0].Params.Capacity != cfg.User.Capacity {
		t.Fatalf("capacity mismatch: got %v want %v", rules[0].Params.Capacity, cfg.User.Capacity)
	}
}

func TestResolveAnonymousDevice(t *testing.T) {
	cfg := defaultTestConfig()
	r := NewResolver(cfg)

	rules := r.Resolve(Identity{
		IP:       "1.2.3.4",
		DeviceID: "dev-abc",
		Endpoint: "/api/public",
	})
	if len(rules) != 1 || rules[0].Tier != TierAnonymous {
		t.Fatalf("expected anonymous tier, got %+v", rules)
	}
	if rules[0].Key != "rl:anonymous:dev-abc:/api/public" {
		t.Fatalf("unexpected key: %s", rules[0].Key)
	}
}

func TestResolveLoginEndpointPerIP(t *testing.T) {
	cfg := defaultTestConfig()
	r := NewResolver(cfg)

	rules := r.Resolve(Identity{
		IP:       "5.6.7.8",
		UserID:   "user-1",
		Endpoint: "/api/login",
	})
	if len(rules) != 2 {
		t.Fatalf("expected endpoint + user rules, got %d", len(rules))
	}
	if rules[0].Tier != TierEndpoint {
		t.Fatalf("first rule should be endpoint, got %s", rules[0].Tier)
	}
	if rules[0].Params.Rate*60 != 5 {
		t.Fatalf("login rate should be 5/min, got %v/min", rules[0].Params.Rate*60)
	}
}

func TestResolveCrawlerIP(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.CrawlerIPs["bad.bot"] = struct{}{}
	r := NewResolver(cfg)

	rules := r.Resolve(Identity{IP: "bad.bot", Endpoint: "/api/x"})
	if len(rules) != 1 || rules[0].Tier != TierCrawler {
		t.Fatalf("expected crawler tier, got %+v", rules)
	}
}

func TestMatchEndpointWildcard(t *testing.T) {
	if !matchEndpoint("/api/v1/items", "/api/v1/*") {
		t.Fatal("wildcard should match prefix")
	}
	if matchEndpoint("/other", "/api/v1/*") {
		t.Fatal("wildcard should not match unrelated path")
	}
}
