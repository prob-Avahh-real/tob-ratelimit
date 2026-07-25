package ratelimit

import "github.com/skat/tob-ratelimit/internal/layer"

func toLayerConfig(cfg Config) layer.Config {
	endpoints := make([]layer.EndpointLimit, len(cfg.Endpoints))
	for i, ep := range cfg.Endpoints {
		endpoints[i] = layer.EndpointLimit{
			Pattern:       ep.Pattern,
			Params:        toLayerParams(ep.Params),
			IdentityScope: ep.IdentityScope,
		}
	}
	return layer.Config{
		BlockedIPs:         cfg.BlockedIPs,
		CrawlerIPs:         cfg.CrawlerIPs,
		Crawler:            toLayerParams(cfg.Crawler),
		Anonymous:          toLayerParams(cfg.Anonymous),
		User:               toLayerParams(cfg.User),
		Endpoints:          endpoints,
		RedisKeyPrefix:     cfg.RedisKeyPrefix,
		BatchWindow:        cfg.BatchWindow,
		BatchThreshold:     cfg.BatchThreshold,
		LocalFallbackScale: cfg.LocalFallbackScale,
	}
}

func toLayerIdentity(id Identity) layer.Identity {
	return layer.Identity{
		IP:       id.IP,
		DeviceID: id.DeviceID,
		UserID:   id.UserID,
		Endpoint: id.Endpoint,
	}
}

func toLayerParams(p BucketParams) layer.BucketParams {
	return layer.BucketParams{
		Capacity: p.Capacity,
		Rate:     p.Rate,
		Weight:   p.Weight,
	}
}

func fromLayerTier(t layer.Tier) Tier {
	return Tier(t)
}
