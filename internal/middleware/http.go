package middleware

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/skat/tob-ratelimit/pkg/ratelimit"
)

type ctxKey int

const identityKey ctxKey = iota

// IdentityFromContext returns identity attached by Middleware.
func IdentityFromContext(ctx context.Context) (ratelimit.Identity, bool) {
	id, ok := ctx.Value(identityKey).(ratelimit.Identity)
	return id, ok
}

// Options configures HTTP middleware behavior.
type Options struct {
	// HeaderDeviceID reads anonymous device fingerprint (e.g. X-Device-ID).
	HeaderDeviceID string
	// HeaderUserID reads authenticated user id (e.g. X-User-ID or from your auth layer).
	HeaderUserID string
	// TrustedProxyCIDRs when set, X-Forwarded-For first hop is used for client IP.
	TrustedProxyCIDRs []*net.IPNet
	// OnLimit is called when a request is denied (optional observability hook).
	OnLimit func(r *http.Request, res ratelimit.Result)
}

func defaultOptions() Options {
	return Options{
		HeaderDeviceID: "X-Device-ID",
		HeaderUserID:   "X-User-ID",
	}
}

// Middleware returns net/http middleware that enforces layered rate limits.
func Middleware(l *ratelimit.Limiter, opts ...Options) func(http.Handler) http.Handler {
	o := defaultOptions()
	if len(opts) > 0 {
		o = opts[0]
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := extractIdentity(r, o)
			res, err := l.Allow(r.Context(), id)
			if err != nil {
				http.Error(w, "rate limiter unavailable", http.StatusServiceUnavailable)
				return
			}

			setRateLimitHeaders(w, res)

			if !res.Allowed {
				if o.OnLimit != nil {
					o.OnLimit(r, res)
				}
				if res.RetryAfter > 0 {
					w.Header().Set("Retry-After", strconv.Itoa(int(res.RetryAfter.Seconds()+0.999)))
				}
				status := http.StatusTooManyRequests
				if res.Tier == ratelimit.TierBlock {
					status = http.StatusForbidden
				}
				http.Error(w, res.Reason, status)
				return
			}

			ctx := context.WithValue(r.Context(), identityKey, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func setRateLimitHeaders(w http.ResponseWriter, res ratelimit.Result) {
	if res.Limit > 0 {
		w.Header().Set("X-RateLimit-Limit", formatFloat(res.Limit))
	}
	if res.Remaining >= 0 {
		w.Header().Set("X-RateLimit-Remaining", formatFloat(res.Remaining))
	}
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func extractIdentity(r *http.Request, o Options) ratelimit.Identity {
	id := ratelimit.Identity{
		IP:       clientIP(r, o.TrustedProxyCIDRs),
		DeviceID: strings.TrimSpace(r.Header.Get(o.HeaderDeviceID)),
		UserID:   strings.TrimSpace(r.Header.Get(o.HeaderUserID)),
		Endpoint: r.URL.Path,
	}
	return id
}

func clientIP(r *http.Request, trusted []*net.IPNet) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	if len(trusted) > 0 && isTrustedProxy(host, trusted) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if ip := strings.TrimSpace(parts[0]); ip != "" {
				return ip
			}
		}
	}
	return host
}

func isTrustedProxy(ip string, nets []*net.IPNet) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, n := range nets {
		if n.Contains(parsed) {
			return true
		}
	}
	return false
}
