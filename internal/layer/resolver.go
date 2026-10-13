package layer

import (
	"strings"

	"github.com/skat/tob-ratelimit/internal/bucket"
)

// Rule is a resolved limit layer for one request.
type Rule struct {
	Tier   Tier
	Key    string
	Params bucket.Params
}

// Resolver maps identity to layered rules (first failing layer wins at enforcement).
type Resolver struct {
	cfg Config
}

// NewResolver creates a layer resolver from config.
func NewResolver(cfg Config) *Resolver {
	return &Resolver{cfg: cfg}
}

// Resolve returns ordered rules from strictest applicable layers.
// Order: block/crawler IP → endpoint → anonymous device → authenticated user.
func (r *Resolver) Resolve(id Identity) []Rule {
	var rules []Rule
	endpoint := normalizeEndpoint(id.Endpoint)
	prefix := r.cfg.RedisKeyPrefix
	if prefix == "" {
		prefix = "rl"
	}

	if _, blocked := r.cfg.BlockedIPs[id.IP]; blocked {
		return []Rule{{
			Tier:   TierBlock,
			Key:    key(prefix, TierBlock, id.IP, endpoint),
			Params: bucket.Params{Capacity: 0, Rate: 0, Weight: 1},
		}}
	}

	if _, crawler := r.cfg.CrawlerIPs[id.IP]; crawler {
		rules = append(rules, Rule{
			Tier:   TierCrawler,
			Key:    key(prefix, TierCrawler, id.IP, endpoint),
			Params: toParams(r.cfg.Crawler),
		})
		if !id.IsAuthenticated() {
			return rules
		}
	}

	for _, ep := range r.cfg.Endpoints {
		if !matchEndpoint(endpoint, ep.Pattern) {
			continue
		}
		scopeID := scopeIdentity(ep.IdentityScope, id)
		rules = append(rules, Rule{
			Tier:   TierEndpoint,
			Key:    key(prefix, TierEndpoint, scopeID, ep.Pattern),
			Params: toParams(ep.Params),
		})
	}

	if id.IsAuthenticated() {
		rules = append(rules, Rule{
			Tier:   TierUser,
			Key:    key(prefix, TierUser, id.UserID, endpoint),
			Params: toParams(r.cfg.User),
		})
		return rules
	}

	identity := id.IP
	if id.DeviceID != "" {
		identity = id.DeviceID
	}
	rules = append(rules, Rule{
		Tier:   TierAnonymous,
		Key:    key(prefix, TierAnonymous, identity, endpoint),
		Params: toParams(r.cfg.Anonymous),
	})
	return rules
}

func key(prefix string, tier Tier, identity, endpoint string) string {
	ep := sanitize(endpoint)
	if ep == "" {
		ep = "global"
	}
	return prefix + ":" + string(tier) + ":" + identity + ":" + ep
}

func sanitize(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

func matchEndpoint(path, pattern string) bool {
	if pattern == "" {
		return false
	}
	if pattern == path {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(path, prefix)
	}
	return strings.HasPrefix(path, pattern)
}

func scopeIdentity(scope string, id Identity) string {
	switch scope {
	case "device":
		if id.DeviceID != "" {
			return id.DeviceID
		}
		return id.IP
	case "user":
		if id.UserID != "" {
			return id.UserID
		}
		if id.DeviceID != "" {
			return id.DeviceID
		}
		return id.IP
	default:
		return id.IP
	}
}

func normalizeEndpoint(path string) string {
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

func toParams(p BucketParams) bucket.Params {
	weight := p.Weight
	if weight <= 0 {
		weight = 1
	}
	return bucket.Params{
		Capacity: p.Capacity,
		Rate:     p.Rate,
		Weight:   weight,
	}
}
