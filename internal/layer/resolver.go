package layer

import (
	"strings"

	"github.com/skat/tob-ratelimit/internal/bucket"
)

// Rule is one rate-limit layer to enforce for a request.
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

// Resolve returns ordered rules: block/crawler → endpoint → anonymous or user.
func (r *Resolver) Resolve(id Identity) []Rule {
	endpoint := normalizeEndpoint(id.Endpoint)
	prefix := r.cfg.RedisKeyPrefix
	if prefix == "" {
		prefix = "rl"
	}

	if _, blocked := r.cfg.BlockedIPs[id.IP]; blocked {
		return []Rule{{
			Tier:   TierBlock,
			Key:    buildKey(prefix, TierBlock, "ip:"+id.IP, endpoint),
			Params: bucket.Params{Capacity: 0, Rate: 0, Weight: 1},
		}}
	}

	var rules []Rule

	if _, crawler := r.cfg.CrawlerIPs[id.IP]; crawler {
		rules = append(rules, Rule{
			Tier:   TierCrawler,
			Key:    buildKey(prefix, TierCrawler, "ip:"+id.IP, endpoint),
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
			Key:    buildKey(prefix, TierEndpoint, scopeID, ep.Pattern),
			Params: toParams(ep.Params),
		})
	}

	if id.IsAuthenticated() {
		rules = append(rules, Rule{
			Tier:   TierUser,
			Key:    buildKey(prefix, TierUser, id.UserID, endpoint),
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
		Key:    buildKey(prefix, TierAnonymous, identity, endpoint),
		Params: toParams(r.cfg.Anonymous),
	})
	return rules
}

func buildKey(prefix string, tier Tier, identity, endpoint string) string {
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

func normalizeEndpoint(path string) string {
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
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
			return "dev:" + id.DeviceID
		}
		return "ip:" + id.IP
	case "user":
		if id.UserID != "" {
			return "usr:" + id.UserID
		}
		if id.DeviceID != "" {
			return "dev:" + id.DeviceID
		}
		return "ip:" + id.IP
	default:
		return "ip:" + id.IP
	}
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
