package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/Bessmack/hardware-store-api/pkg/response"
	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
)

// ── Rate limiter setup ────────────────────────────────────────────────────────
//
// Implementation: go-redis/redis_rate (GCRA algorithm)
// GCRA (Generic Cell Rate Algorithm) is superior to a simple counter reset:
//   - Bursts are smoothed, not hard-cut at the window boundary
//   - A client cannot get 60 requests at 00:59 and another 60 at 01:00
//   - Unused capacity carries over as partial credit
//
// Redis is used so limits are shared correctly across multiple API instances.
// The rate limiter fails open on Redis errors — a cache hiccup will not
// take down the API.
//
// Payment callback routes (/payments/mpesa/callback, /payments/airtel/callback)
// are NOT rate-limited here — they use the IP allowlist in ip_allowlist.go instead.

// RateLimiter wraps the Redis-backed GCRA rate limiter.
type RateLimiter struct {
	limiter *redis_rate.Limiter
}

// NewRateLimiter creates a RateLimiter backed by the existing Redis client.
func NewRateLimiter(redisClient *redis.Client) *RateLimiter {
	return &RateLimiter{
		limiter: redis_rate.NewLimiter(redisClient),
	}
}

// ── Preset limits ─────────────────────────────────────────────────────────────
// Named methods so server/routes.go never hardcodes numbers.
// Adjust the values below to tune without changing caller code.

// ForLogin — 5 attempts/min per IP.
// Tight enough to block brute-force; loose enough for a user who mistyped twice.
func (rl *RateLimiter) ForLogin() func(http.Handler) http.Handler {
	return rl.limit("login", 5, time.Minute)
}

// ForRegister — 3 registrations/min per IP.
// Prevents automated spam account creation.
func (rl *RateLimiter) ForRegister() func(http.Handler) http.Handler {
	return rl.limit("register", 3, time.Minute)
}

// ForRefresh — 10 refreshes/min per IP.
// More generous than login; a mobile app legitimately calls this on foreground.
func (rl *RateLimiter) ForRefresh() func(http.Handler) http.Handler {
	return rl.limit("refresh", 10, time.Minute)
}

// ForAPI — 60 requests/min per IP for all general routes.
// Comfortable for normal hardware store browsing and checkout flows.
func (rl *RateLimiter) ForAPI() func(http.Handler) http.Handler {
	return rl.limit("api", 60, time.Minute)
}

// ForGeo — 30 requests/min per IP for geocoding/autocomplete endpoints.
// Keeps the system well within Nominatim's 1 req/sec usage policy.
func (rl *RateLimiter) ForGeo() func(http.Handler) http.Handler {
	return rl.limit("geo", 30, time.Minute)
}

// ── Core middleware ───────────────────────────────────────────────────────────

// limit builds a chi-compatible middleware that enforces `max` requests
// per `period` per client IP for the given named bucket.
//
// Redis key format: rate:{bucket}:{ip}
// On limit exceeded: 429 Too Many Requests with a Retry-After header.
func (rl *RateLimiter) limit(bucket string, max int, period time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			key := fmt.Sprintf("rate:%s:%s", bucket, ip)

			res, err := rl.limiter.Allow(context.Background(), key, redis_rate.Limit{
				Rate:   max,
				Burst:  max,      // allow a burst equal to the window limit
				Period: period,
			})
			if err != nil {
				// Redis failure — fail open (allow the request) and log
				// rather than taking the site down due to a cache hiccup
				next.ServeHTTP(w, r)
				return
			}

			// Set standard rate limit headers so clients can adapt
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", max))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", res.Remaining))
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(res.ResetAfter).Unix()))

			if res.Allowed == 0 {
				w.Header().Set("Retry-After", fmt.Sprintf("%.0f", res.RetryAfter.Seconds()))
				response.Error(w, http.StatusTooManyRequests,
					fmt.Sprintf("too many requests — please wait %.0f seconds before trying again",
						res.RetryAfter.Seconds()),
				)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ── IP extraction ─────────────────────────────────────────────────────────────

// clientIP extracts the real client IP from the request.
// Respects X-Forwarded-For and X-Real-IP headers set by a reverse proxy
// (nginx, Cloudflare, etc.) when running behind one in production.
// Falls back to RemoteAddr when no proxy headers are present.
func clientIP(r *http.Request) string {
	// X-Forwarded-For may contain a comma-separated list: "client, proxy1, proxy2"
	// The leftmost value is the original client.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if host, _, err := net.SplitHostPort(xff); err == nil {
			return host
		}
		// No port in XFF — use as-is (take first entry if comma-separated)
		for i, ch := range xff {
			if ch == ',' {
				return xff[:i]
			}
		}
		return xff
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Direct connection — strip port from RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}