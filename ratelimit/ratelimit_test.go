// SPDX-License-Identifier: MIT

package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTrimSpace(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"  hello  ", "hello"},
		{"\thello\t", "hello"},
		{"hello", "hello"},
		{"  ", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := trimSpace(tt.input); got != tt.want {
			t.Errorf("trimSpace(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRealIP_XFF(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", " 203.0.113.195 , 198.51.100.100")
	req.RemoteAddr = "127.0.0.1:1234"

	// 1. Without trusted proxies: safe by default, ignores spoofed header
	if got := RealIP(req); got != "127.0.0.1" {
		t.Errorf("RealIP(XFF without trusted proxy) = %v, want 127.0.0.1", got)
	}

	// 2. With 127.0.0.1 trusted: walks right-to-left, stops at 198.51.100.100
	trustedLocal := ParseTrustedProxies("127.0.0.1/32")
	if got := RealIP(req, trustedLocal...); got != "198.51.100.100" {
		t.Errorf("RealIP(XFF trusted 127.0.0.1) = %v, want 198.51.100.100", got)
	}

	// 3. With both 127.0.0.1 and 198.51.100.100 trusted: stops at 203.0.113.195
	trustedChain := ParseTrustedProxies("127.0.0.1/32", "198.51.100.100/32")
	if got := RealIP(req, trustedChain...); got != "203.0.113.195" {
		t.Errorf("RealIP(XFF trusted chain) = %v, want 203.0.113.195", got)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("X-Forwarded-For", "\t198.51.100.1\t")
	req2.RemoteAddr = "127.0.0.1:5678"
	if got := RealIP(req2, trustedLocal...); got != "198.51.100.1" {
		t.Errorf("RealIP(XFF trim) = %v, want 198.51.100.1", got)
	}
}

func TestNewGlobal(t *testing.T) {
	mw := NewGlobal()
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("NewGlobal() = %v, want 200", w.Code)
	}
	if w.Header().Get("RateLimit-Limit") != "200" {
		t.Errorf("RateLimit-Limit = %v, want 200", w.Header().Get("RateLimit-Limit"))
	}
}

func TestNewAuth(t *testing.T) {
	mw := NewAuth()
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("NewAuth() = %v, want 200", w.Code)
	}
	if w.Header().Get("RateLimit-Limit") != "10" {
		t.Errorf("RateLimit-Limit = %v, want 10", w.Header().Get("RateLimit-Limit"))
	}
}

func TestRateLimit_Exceeded(t *testing.T) {
	mw := New(1, time.Minute)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:1234"

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("First request = %v, want 200", w.Code)
	}
	// After first consume, remaining should be 0
	if w.Header().Get("RateLimit-Remaining") != "0" {
		t.Errorf("RateLimit-Remaining = %v, want 0", w.Header().Get("RateLimit-Remaining"))
	}

	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, req)
	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("Second request = %v, want 429", w2.Code)
	}
	if !strings.Contains(w2.Body.String(), "too many requests") {
		t.Errorf("Body = %v, want 'too many requests'", w2.Body.String())
	}
	// Verify rate limit headers on rejection
	if w2.Header().Get("RateLimit-Limit") != "1" {
		t.Errorf("RateLimit-Limit on reject = %v, want 1", w2.Header().Get("RateLimit-Limit"))
	}
	if w2.Header().Get("RateLimit-Remaining") != "0" {
		t.Errorf("RateLimit-Remaining on reject = %v, want 0", w2.Header().Get("RateLimit-Remaining"))
	}
	if w2.Header().Get("RateLimit-Reset") == "" {
		t.Error("RateLimit-Reset header should be set on rejection")
	}
}

func TestRealIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "10.10.10.10")
	req.RemoteAddr = "127.0.0.1:1234"

	// Without trusted proxies: safe by default, ignores spoofed header
	if got := RealIP(req); got != "127.0.0.1" {
		t.Errorf("RealIP(X-Real-IP without trusted proxy) = %v, want 127.0.0.1", got)
	}

	// With trusted proxy: accepts X-Real-IP
	trusted := ParseTrustedProxies("127.0.0.1/32")
	if got := RealIP(req, trusted...); got != "10.10.10.10" {
		t.Errorf("RealIP(X-Real-IP with trusted proxy) = %v, want 10.10.10.10", got)
	}

	// With untrusted direct connection: ignores X-Real-IP
	untrustedReq := httptest.NewRequest(http.MethodGet, "/", nil)
	untrustedReq.Header.Set("X-Real-IP", "10.10.10.10")
	untrustedReq.RemoteAddr = "203.0.113.5:1234"
	if got := RealIP(untrustedReq, trusted...); got != "203.0.113.5" {
		t.Errorf("RealIP(untrusted remote with X-Real-IP) = %v, want 203.0.113.5", got)
	}
}

func TestRealIP_FallbackToRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"

	if got := RealIP(req); got != "127.0.0.1" {
		t.Errorf("RealIP(RemoteAddr) = %v, want 127.0.0.1", got)
	}
}

func TestRealIP_InvalidXFF(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "invalid-ip")
	req.RemoteAddr = "127.0.0.1:1234"

	trusted := ParseTrustedProxies("127.0.0.1/32")
	if got := RealIP(req, trusted...); got != "127.0.0.1" {
		t.Errorf("RealIP(invalid XFF) = %v, want 127.0.0.1", got)
	}
}

func TestRealIP_InvalidXRealIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "invalid-ip")
	req.RemoteAddr = "127.0.0.1:1234"

	trusted := ParseTrustedProxies("127.0.0.1/32")
	if got := RealIP(req, trusted...); got != "127.0.0.1" {
		t.Errorf("RealIP(invalid X-Real-IP) = %v, want 127.0.0.1", got)
	}
}

func TestRealIP_AllTrustedInXFF(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.2, 10.0.0.1")
	req.RemoteAddr = "127.0.0.1:1234"

	// Both proxies and remote are trusted; all XFF entries are trusted:
	// should return leftmost valid IP.
	trusted := ParseTrustedProxies("127.0.0.1/32", "10.0.0.0/8")
	if got := RealIP(req, trusted...); got != "10.0.0.2" {
		t.Errorf("RealIP(all trusted XFF) = %v, want 10.0.0.2", got)
	}
}

func TestRealIP_UntrustedProxy_IgnoresSpoofedHeaders(t *testing.T) {
	trusted := ParseTrustedProxies("10.0.0.0/8")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "1.1.1.1, 8.8.8.8")
	req.Header.Set("X-Real-IP", "9.9.9.9")
	req.RemoteAddr = "192.168.1.50:4321" // Not in 10.0.0.0/8

	// Untrusted direct connection must ignore all spoofed headers
	if got := RealIP(req, trusted...); got != "192.168.1.50" {
		t.Errorf("RealIP(untrusted direct connection) = %v, want 192.168.1.50", got)
	}
}

func TestWithTrustedProxies_Parsing(t *testing.T) {
	tests := []struct {
		cidrs []string
		count int
	}{
		{[]string{"10.0.0.0/8", "192.168.1.1"}, 2},
		{[]string{"127.0.0.1, 10.0.0.1/24"}, 2},
		{[]string{"::1", "2001:db8::/32"}, 2},
		{[]string{"invalid-cidr", ""}, 0},
	}

	for _, tt := range tests {
		proxies := ParseTrustedProxies(tt.cidrs...)
		if len(proxies) != tt.count {
			t.Errorf("ParseTrustedProxies(%v) count = %d, want %d", tt.cidrs, len(proxies), tt.count)
		}
	}
}

func TestWithTrustedProxies_Middleware(t *testing.T) {
	// 1 req/min per IP, trusting proxy 10.0.0.1
	mw := New(1, time.Minute, WithTrustedProxies("10.0.0.1/32"))
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 1. Through trusted proxy 10.0.0.1 for client 198.51.100.1 -> Allowed
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = "10.0.0.1:1234"
	req1.Header.Set("X-Forwarded-For", "198.51.100.1")
	w1 := httptest.NewRecorder()
	h.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w1.Code)
	}

	// 2. Same client 198.51.100.1 through proxy again -> Rate limited (429)
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "10.0.0.1:5678"
	req2.Header.Set("X-Forwarded-For", "198.51.100.1")
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for same client IP, got %d", w2.Code)
	}

	// 3. Different client 198.51.100.2 through same proxy -> Allowed (isolated bucket)
	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	req3.RemoteAddr = "10.0.0.1:9012"
	req3.Header.Set("X-Forwarded-For", "198.51.100.2")
	w3 := httptest.NewRecorder()
	h.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200 for different client IP, got %d", w3.Code)
	}

	// 4. Untrusted direct connection attempting to spoof 198.51.100.99
	// Should be rate-limited by its direct connection IP (172.16.0.5)
	req4 := httptest.NewRequest(http.MethodGet, "/", nil)
	req4.RemoteAddr = "172.16.0.5:1234"
	req4.Header.Set("X-Forwarded-For", "198.51.100.99")
	w4 := httptest.NewRecorder()
	h.ServeHTTP(w4, req4)
	if w4.Code != http.StatusOK {
		t.Fatalf("expected 200 for first request from 172.16.0.5, got %d", w4.Code)
	}

	// Second request from 172.16.0.5 with DIFFERENT spoofed header is blocked
	// because direct IP was consumed!
	req5 := httptest.NewRequest(http.MethodGet, "/", nil)
	req5.RemoteAddr = "172.16.0.5:1234"
	req5.Header.Set("X-Forwarded-For", "198.51.100.100")
	w5 := httptest.NewRecorder()
	h.ServeHTTP(w5, req5)
	if w5.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 because spoofed header is ignored and direct IP is rate limited, got %d", w5.Code)
	}
}

func TestRealIP_NoPortRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1" // No port

	if got := RealIP(req); got != "192.168.1.1" {
		t.Errorf("RealIP(No Port RemoteAddr) = %v, want 192.168.1.1", got)
	}
}

func TestRateLimit_AllowedHeaders(t *testing.T) {
	mw := New(5, time.Minute)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.1.2.3:5678"

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("allowed request = %v, want 200", w.Code)
	}
	if w.Header().Get("RateLimit-Limit") != "5" {
		t.Errorf("RateLimit-Limit = %v, want 5", w.Header().Get("RateLimit-Limit"))
	}
	if w.Header().Get("RateLimit-Remaining") == "" {
		t.Error("RateLimit-Remaining should be set on allowed requests")
	}
}

type mockCounter struct {
	mu sync.Mutex
	n  float64
}

func (c *mockCounter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
}

func (c *mockCounter) Add(d float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n += d
}

func (c *mockCounter) Get() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

func TestNew_Metrics(t *testing.T) {
	allowed := &mockCounter{}
	blocked := &mockCounter{}

	mw := New(1, time.Minute, WithMetrics(Metrics{
		Allowed: allowed,
		Blocked: blocked,
	}))

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 1. Allowed request
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if allowed.Get() != 1.0 {
		t.Errorf("allowed count = %v, want 1", allowed.Get())
	}

	// 2. Blocked request
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec2.Code)
	}
	if blocked.Get() != 1.0 {
		t.Errorf("blocked count = %v, want 1", blocked.Get())
	}
}

func TestTokenBucketLimiter(t *testing.T) {
	limiter := NewTokenBucket(2, 50*time.Millisecond)

	// Test boundary / defaults
	defLimiter := NewTokenBucket(0, 0)
	if defLimiter.capacity != 1 || defLimiter.refillRate != time.Second {
		t.Errorf("expected defaults (1, 1s), got (%d, %v)", defLimiter.capacity, defLimiter.refillRate)
	}

	key := "user-tb-1"
	if !limiter.Allow(key) {
		t.Error("expected first request allowed")
	}
	if !limiter.AllowN(key, 0) {
		t.Error("AllowN(0) should be true")
	}
	if !limiter.Allow(key) {
		t.Error("expected second request allowed")
	}
	if limiter.Allow(key) {
		t.Error("expected third request denied")
	}
	if rem := limiter.Remaining(key); rem != 0 {
		t.Errorf("expected remaining 0, got %d", rem)
	}

	time.Sleep(60 * time.Millisecond)
	if !limiter.Allow(key) {
		t.Error("expected refilled request allowed")
	}

	limiter.Reset(key)
	if rem := limiter.Remaining(key); rem != 2 {
		t.Errorf("after reset, expected capacity 2, got %d", rem)
	}

	// Test GC
	limiter.Allow("old-key")
	limiter.getBucket("old-key").last = time.Now().Add(-1 * time.Hour)
	limiter.lastGC = time.Now().Add(-10 * time.Minute)
	limiter.idleTimeout = 10 * time.Millisecond
	limiter.Allow("user-tb-gc-trigger")
}

func TestSlidingWindowLimiter(t *testing.T) {
	limiter := NewSlidingWindow(2, 100*time.Millisecond)

	defLimiter := NewSlidingWindow(0, 0)
	if defLimiter.limit != 1 || defLimiter.window != time.Minute {
		t.Errorf("expected defaults (1, 1m), got (%d, %v)", defLimiter.limit, defLimiter.window)
	}

	key := "user-sw-1"
	if !limiter.Allow(key) {
		t.Error("expected 1st request allowed")
	}
	if !limiter.AllowN(key, 0) {
		t.Error("AllowN(0) should be true")
	}
	if !limiter.Allow(key) {
		t.Error("expected 2nd request allowed")
	}
	if limiter.Allow(key) {
		t.Error("expected 3rd request denied")
	}

	time.Sleep(120 * time.Millisecond)
	if !limiter.Allow(key) {
		t.Error("expected request allowed after window rollover")
	}

	limiter.Reset(key)
	if rem := limiter.Remaining(key); rem != 2 {
		t.Errorf("after reset, expected remaining 2, got %d", rem)
	}

	// Test GC
	limiter.Allow("old-key")
	limiter.getEntry("old-key").currWindowStart = time.Now().Add(-1 * time.Hour)
	limiter.lastGC = time.Now().Add(-10 * time.Minute)
	limiter.idleTimeout = 10 * time.Millisecond
	limiter.Allow("user-sw-gc-trigger")
}

func TestNewWithLimiter_Middleware(t *testing.T) {
	limiter := NewTokenBucket(1, time.Minute)
	mw := NewWithLimiter(limiter)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.RemoteAddr = "192.0.2.1:1234"

	// 1. First request succeeds
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request code = %d, want 200", rec1.Code)
	}
	if rec1.Header().Get("X-RateLimit-Remaining") == "" {
		t.Error("expected X-RateLimit-Remaining header")
	}

	// 2. Second request fails (rate limited)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request code = %d, want 429", rec2.Code)
	}
	if rec2.Header().Get("Retry-After") != "1" {
		t.Errorf("Retry-After = %q, want 1", rec2.Header().Get("Retry-After"))
	}

	// 3. Custom keyFunc
	customMw := NewWithLimiter(limiter, func(r *http.Request) string {
		return r.Header.Get("X-API-Key")
	})
	customHandler := customMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	reqCustom := httptest.NewRequest(http.MethodGet, "/api/custom", nil)
	reqCustom.Header.Set("X-API-Key", "client-token-abc")
	rec3 := httptest.NewRecorder()
	customHandler.ServeHTTP(rec3, reqCustom)
	if rec3.Code != http.StatusOK {
		t.Fatalf("custom key request code = %d, want 200", rec3.Code)
	}
}

func TestAlgorithm_EdgeCases(t *testing.T) {
	// 1. Sliding Window multiple window skip
	sw := NewSlidingWindow(5, 50*time.Millisecond)
	sw.Allow("skip-test")
	time.Sleep(150 * time.Millisecond) // skip >2 windows
	if !sw.Allow("skip-test") {
		t.Error("expected allow after skipping >2 windows")
	}
	if rem := sw.Remaining("skip-test"); rem < 4 {
		t.Errorf("expected remaining >= 4, got %d", rem)
	}

	// 2. Token Bucket remaining when full
	tb := NewTokenBucket(5, 10*time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	if rem := tb.Remaining("tb-full"); rem != 5 {
		t.Errorf("expected remaining capped at 5, got %d", rem)
	}
}

func TestTokenBucket_Adversarial_HighFrequencyRefill(t *testing.T) {
	// Refill rate: 1 token every 20ms
	tb := NewTokenBucket(2, 20*time.Millisecond)

	// Consume all 2 tokens
	if !tb.AllowN("user", 2) {
		t.Fatal("expected 2 initial tokens to be allowed")
	}
	if tb.Allow("user") {
		t.Fatal("expected 3rd token to be rejected")
	}

	// Poll every 3ms for 50ms (total ~2.5 tokens refilled).
	// With naive b.last = now, fractional times are wiped and 0 tokens are gained.
	// With proper remainder tracking, at least 2 tokens must be allowed!
	allowed := 0
	deadline := time.Now().Add(50 * time.Millisecond)
	for time.Now().Before(deadline) {
		if tb.Allow("user") {
			allowed++
		}
		time.Sleep(3 * time.Millisecond)
	}

	if allowed < 2 {
		t.Fatalf("expected at least 2 tokens refilled over 50ms, got %d (remainder truncation bug)", allowed)
	}
}

func TestRateLimiter_Adversarial_ConcurrentAllow(t *testing.T) {
	limiters := map[string]Limiter{
		"token_bucket":   NewTokenBucket(20, 5*time.Millisecond),
		"sliding_window": NewSlidingWindow(20, 100*time.Millisecond),
	}

	for name, lim := range limiters {
		t.Run(name, func(t *testing.T) {
			var wg sync.WaitGroup
			concurrency := 50
			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					key := "shared-key"
					for j := 0; j < 30; j++ {
						_ = lim.Allow(key)
						_ = lim.Remaining(key)
					}
				}(i)
			}
			wg.Wait()
		})
	}
}

func TestRateLimiter_Adversarial_ConcurrentAllowAndReset(t *testing.T) {
	limiters := map[string]Limiter{
		"token_bucket":   NewTokenBucket(100, 10*time.Millisecond),
		"sliding_window": NewSlidingWindow(100, 100*time.Millisecond),
	}

	for name, lim := range limiters {
		t.Run(name, func(t *testing.T) {
			var wg sync.WaitGroup
			workers := 100

			for i := 0; i < workers; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					key := "stress-key"

					for j := 0; j < 50; j++ {
						if j%10 == 0 {
							lim.Reset(key)
						} else if j%2 == 0 {
							_ = lim.Allow(key)
						} else {
							_ = lim.AllowN(key, 2)
						}
						_ = lim.Remaining(key)
					}
				}(i)
			}

			wg.Wait()
		})
	}
}

func TestTokenBucketLimiter_ZeroValue(t *testing.T) {
	var lim TokenBucketLimiter
	if !lim.Allow("user-1") {
		t.Fatal("expected first Allow to succeed")
	}
	if lim.Allow("user-1") {
		t.Fatal("expected second Allow to fail with capacity exhausted")
	}
	if rem := lim.Remaining("user-1"); rem != 0 {
		t.Fatalf("expected 0 remaining tokens, got %d", rem)
	}
	lim.Reset("user-1")
	if !lim.Allow("user-1") {
		t.Fatal("expected Allow to succeed after reset")
	}
}

func TestSlidingWindowLimiter_ZeroValue(t *testing.T) {
	var lim SlidingWindowLimiter
	if !lim.Allow("user-1") {
		t.Fatal("expected first Allow to succeed")
	}
	if lim.Allow("user-1") {
		t.Fatal("expected second Allow to fail with limit exhausted")
	}
	if rem := lim.Remaining("user-1"); rem != 0 {
		t.Fatalf("expected 0 remaining requests, got %d", rem)
	}
	lim.Reset("user-1")
	if !lim.Allow("user-1") {
		t.Fatal("expected Allow to succeed after reset")
	}
}
