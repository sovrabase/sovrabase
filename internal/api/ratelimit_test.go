package api

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter_Burst(t *testing.T) {
	rl := newRateLimiter(10, 5) // 10 tokens/sec, burst 5

	// First 5 calls should all be allowed (burst).
	for i := 0; i < 5; i++ {
		if !rl.allow() {
			t.Fatalf("expected allow at call %d (burst)", i)
		}
	}

	// 6th call should be blocked (burst exhausted).
	if rl.allow() {
		t.Fatal("expected block after burst exhausted")
	}
}

func TestRateLimiter_Refill(t *testing.T) {
	rl := newRateLimiter(100, 10) // 100 tokens/sec, burst 10

	// Exhaust burst.
	for i := 0; i < 10; i++ {
		rl.allow()
	}

	// Wait for refill of ~1 token.
	time.Sleep(15 * time.Millisecond)

	if !rl.allow() {
		t.Fatal("expected allow after refill")
	}
}

func TestTenantRateLimiter_Isolation(t *testing.T) {
	trl := newTenantRateLimiter(100, 5)

	// Exhaust tenant-a.
	limA := trl.get("tenant-a")
	for i := 0; i < 5; i++ {
		limA.allow()
	}

	// tenant-b should still have its own bucket.
	limB := trl.get("tenant-b")
	if !limB.allow() {
		t.Fatal("tenant-b should have own bucket, not affected by tenant-a")
	}
}

func TestTenantRateLimiter_Cleanup(t *testing.T) {
	trl := newTenantRateLimiter(100, 5)

	_ = trl.get("stale")
	_ = trl.get("fresh")

	// Cleanup with negative maxAge — both removed (lastRefill is in the future relative to cleanup? No, lastRefill is in the past).
	// maxAge=0 means anything that's been around for >0ns gets removed.
	// The limiters were created <1ms ago, so age > 0 should be true.
	time.Sleep(time.Millisecond)
	trl.cleanup(time.Nanosecond)

	if len(trl.limiters) != 0 {
		t.Fatalf("expected 0 limiters after cleanup, got %d", len(trl.limiters))
	}
}

func TestRateLimitMiddleware_Returns429(t *testing.T) {
	// Create a minimal server with tight rate limit: 1 req/min, burst 1.
	cfg := &Config{
		ListenAddr:         ":0",
		AllowOrigins:       "*",
		JWTSecret:          "test",
		RateLimitPerMinute: 1,
		RateLimitBurst:     1,
	}

	s := &Server{
		config:       cfg,
		rateLimiters: newTenantRateLimiter(1.0/60.0, 1),
		logger:       slog.Default(),
	}

	handler := s.rateLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	// First request should pass.
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.Header.Set("X-Project-Key", "test-project")
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w1.Code)
	}

	// Second request should be 429 (burst exhausted).
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("X-Project-Key", "test-project")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w2.Code)
	}

	// Different tenant should pass.
	req3 := httptest.NewRequest("GET", "/test", nil)
	req3.Header.Set("X-Project-Key", "other-project")
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200 for different tenant, got %d", w3.Code)
	}
}
