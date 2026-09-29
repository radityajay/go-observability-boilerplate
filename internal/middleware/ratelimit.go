package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter is a Redis-backed sliding window rate limiter.
type RateLimiter struct {
	client *redis.Client
	max    int
	window time.Duration
}

// NewRateLimiter creates a new Redis-backed rate limiter.
func NewRateLimiter(client *redis.Client, max int, window time.Duration) *RateLimiter {
	return &RateLimiter{client: client, max: max, window: window}
}

// Middleware returns an HTTP middleware that applies rate limiting per client IP.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		key := fmt.Sprintf("ratelimit:%s", ip)
		ctx := r.Context()

		now := time.Now()
		windowStart := now.Add(-rl.window)

		pipe := rl.client.Pipeline()
		// Remove old entries outside the window
		pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart.UnixNano()))
		// Count current entries
		countCmd := pipe.ZCard(ctx, key)
		_, err := pipe.Exec(ctx)
		if err != nil {
			// On Redis failure, allow the request (fail open)
			next.ServeHTTP(w, r)
			return
		}

		count := countCmd.Val()
		remaining := int64(rl.max) - count

		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.max))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(now.Add(rl.window).Unix(), 10))

		if remaining <= 0 {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("Retry-After", strconv.FormatInt(int64(rl.window.Seconds()), 10))
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}

		// Add current request
		rl.client.ZAdd(ctx, key, redis.Z{
			Score:  float64(now.UnixNano()),
			Member: fmt.Sprintf("%d", now.UnixNano()),
		})
		rl.client.Expire(ctx, key, rl.window)

		w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining-1, 10))
		next.ServeHTTP(w, r)
	})
}

// Allow checks if a request from the given IP is allowed (used for testing).
func (rl *RateLimiter) Allow(ctx context.Context, ip string) (bool, error) {
	key := fmt.Sprintf("ratelimit:%s", ip)
	now := time.Now()
	windowStart := now.Add(-rl.window)

	pipe := rl.client.Pipeline()
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart.UnixNano()))
	countCmd := pipe.ZCard(ctx, key)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return true, err
	}
	return countCmd.Val() < int64(rl.max), nil
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
