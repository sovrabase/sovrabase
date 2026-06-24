package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

// rateLimiter implements a token bucket rate limiter.
type rateLimiter struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

func newRateLimiter(refillRate float64, burst int) *rateLimiter {
	return &rateLimiter{
		tokens:     float64(burst),
		maxTokens:  float64(burst),
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

func (rl *rateLimiter) allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Refill tokens based on elapsed time.
	elapsed := time.Since(rl.lastRefill).Seconds()
	rl.tokens = math.Min(rl.maxTokens, rl.tokens+elapsed*rl.refillRate)
	rl.lastRefill = time.Now()

	if rl.tokens >= 1 {
		rl.tokens--
		return true
	}
	return false
}

func (rl *rateLimiter) remaining() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return int(math.Floor(rl.tokens))
}

func (rl *rateLimiter) resetIn() time.Duration {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if rl.tokens >= 1 {
		return 0
	}
	// Time until one token is refilled.
	needed := 1 - rl.tokens
	sec := needed / rl.refillRate
	return time.Duration(sec * float64(time.Second))
}

// tenantRateLimiter manages per-key token buckets.
type tenantRateLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*rateLimiter
	rate     float64
	burst    int
}

func newTenantRateLimiter(ratePerSec float64, burst int) *tenantRateLimiter {
	return &tenantRateLimiter{
		limiters: make(map[string]*rateLimiter),
		rate:     ratePerSec,
		burst:    burst,
	}
}

func (t *tenantRateLimiter) get(key string) *rateLimiter {
	t.mu.RLock()
	rl, exists := t.limiters[key]
	t.mu.RUnlock()
	if exists {
		return rl
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	// Double-check.
	if rl, exists := t.limiters[key]; exists {
		return rl
	}
	rl = newRateLimiter(t.rate, t.burst)
	t.limiters[key] = rl
	return rl
}

func (t *tenantRateLimiter) cleanup(maxAge time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	for key, rl := range t.limiters {
		rl.mu.Lock()
		age := now.Sub(rl.lastRefill)
		rl.mu.Unlock()
		if age > maxAge {
			delete(t.limiters, key)
		}
	}
}

// startCleanup launches a periodic cleanup goroutine for stale limiters.
func (t *tenantRateLimiter) startCleanup(interval, maxAge time.Duration, stop <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				t.cleanup(maxAge)
			case <-stop:
				return
			}
		}
	}()
}

// rateLimitMiddleware enforces per-tenant rate limits.
// It identifies the caller by project key or client IP.
func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Determine tenant key: X-Project-Key or client IP.
		key := r.Header.Get("X-Project-Key")
		if key == "" {
			// Fall back to client IP (strip port).
			addr := r.RemoteAddr
			if idx := strings.LastIndex(addr, ":"); idx != -1 {
				addr = addr[:idx]
			}
			key = "ip:" + addr
		} else {
			key = "project:" + key
		}

		limiter := s.rateLimiters.get(key)

		if !limiter.allow() {
			resetIn := limiter.resetIn()
			retryAfter := int(math.Ceil(resetIn.Seconds()))
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", s.config.RateLimitPerMinute))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(resetIn).Unix()))
			w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":       "rate limit exceeded",
				"retry_after": retryAfter,
			})
			return
		}

		rem := limiter.remaining()
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", s.config.RateLimitPerMinute))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", rem))

		next.ServeHTTP(w, r)
	})
}
