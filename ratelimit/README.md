# `ratelimit` Package

The `ratelimit` package provides in-memory, thread-safe rate limiting with **Token Bucket and Sliding Window algorithms**, a unified `Limiter` interface, standard `net/http` and `ginmw` middleware integrations, RFC-compliant headers (`RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset`, `Retry-After`), and automatic idle GC to prevent memory leaks under DDoS attacks with strictly zero external dependencies.

## When to Use
- **Inbound API Protection**: Throttling traffic per client IP, tenant ID, or API key at the ingress boundary to prevent service degradation.
- **Bursty Web APIs**: Allowing transient bursts up to bucket capacity while enforcing steady-state throughput (Token Bucket).
- **Abuse & Scraping Prevention**: Enforcing smooth rolling window request quotas without boundary spike vulnerabilities (Sliding Window).

## Why It Is Written Like That
- **Unified `Limiter` Interface**: Standardizes `Allow(key string) bool` across high-performance algorithms (`TokenBucket`, `SlidingWindow`), allowing developers to switch algorithms based on traffic shape without rewriting middleware.
- **Zero External Dependencies**: Pure Go standard library (`sync.RWMutex`, `time.Time`), operating at sub-microsecond in-memory latencies without external network roundtrips.
- **Automatic TTL & Idle Eviction**: Background sweeper routinely purges inactive client state, preventing memory exhaustion attacks from randomized IP spoofing under volumetric DDoS.
- **RFC-Compliant Header Injection**: Exposes standard `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset`, and `Retry-After` headers to API consumers.

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/ratelimit` |
|---|---|---|---|
| **`golang.org/x/time/rate`** | Official Go subrepository token bucket | Single algorithm only (Token Bucket); no per-key registry, no HTTP header injection, no automatic cleanup of stale keys | Multi-algorithm suite, built-in key management, automatic cleanup, and standard middleware |
| **Distributed Redis Limiter** | Global limit across multiple pods | Network roundtrip latency (1-5ms); Redis downtime halts API traffic or bypasses limits; introduces external infrastructure dependency | Zero-dependency local microsecond in-memory speed; provides sub-100ns execution per request |

---

## 🎯 Supported Algorithms

| Algorithm | Constructor | Best For | Memory Complexity | Burst Behavior |
|:---|:---|:---|:---|:---|
| **Token Bucket** | `NewTokenBucket(capacity, refillRate)` | Bursty web APIs, gateways allowing burst then steady rate | $O(1)$ per client | Permits bursts up to capacity |
| **Sliding Window** | `NewSlidingWindow(limit, window)` | General API protection (Cloudflare algorithm) eliminating boundary bursts | $O(1)$ per client | Smooth rolling approximation |

---

## 💻 How to Use It?

### 1. Unified `Limiter` Interface
```go
import (
    "time"
    "github.com/umesh0492/go-libs/ratelimit"
)

// Choose the algorithm that fits your workload:
var limiter ratelimit.Limiter

// Token Bucket: 100 burst capacity, refills 1 token every 100ms
limiter = ratelimit.NewTokenBucket(100, 100*time.Millisecond)

// Sliding Window Counter: 1000 requests per minute (Cloudflare weighted)
limiter = ratelimit.NewSlidingWindow(1000, time.Minute)

// Check if request is allowed
if limiter.Allow("user_123") {
    // Process request
} else {
    // Return 429 Too Many Requests
}
```

### 2. Standard `net/http` Middleware
```go
// Plug any Limiter implementation into net/http
httpLimiter := ratelimit.NewSlidingWindow(500, time.Minute)
router.Use(ratelimit.NewWithLimiter(httpLimiter))

// Custom Key Extraction (e.g. API Key or Tenant ID)
router.Use(ratelimit.NewWithLimiter(httpLimiter, func(r *http.Request) string {
    return r.Header.Get("X-API-Key")
}))
```

### 3. Pre-Built Conveniences
```go
// Generous Global Protection: 200 req/min per IP
r.Use(ratelimit.NewGlobal())

// Strict Auth Endpoint Protection: 10 req/min per IP to stop brute force
r.POST("/login", ratelimit.NewAuth(), authHandler)
```

### 4. Gin Web Framework (`ginmw`)
```go
import "github.com/umesh0492/go-libs/ginmw"

// Attach any algorithm to Gin:
limiter := ratelimit.NewTokenBucket(50, 100*time.Millisecond)
router.Use(ginmw.RateLimitWithLimiter(limiter))
```

---

## 🛡️ Edge Cases Handled
- **IP Spoofing Abusers**: Clients can fake `X-Forwarded-For` metadata headers pretending to jump IPs over NATs. Our parser defensively scans right-to-left checking valid bounds and falls back to socket `RemoteAddr` seamlessly.
- **Memory Leak Prevention (DDoS Protection)**: Under a volumetric DDoS attack against millions of randomly rotating socket proxies, reserving buckets dynamically eats RAM. The internal background Garbage Collector safely purges untouched IP caches idling over five minutes to immediately free RAM natively.
- **Zero External Dependencies**: Implemented strictly with standard library `sync.Mutex`, `sync.RWMutex`, and `time.Time`.

---

## 🛡️ Concurrency & Correctness Guarantees

- **`-race` Certified**: All limiter algorithms (`TokenBucket` and `SlidingWindow`) are verified under 100+ concurrent goroutines hammering `Allow`, `AllowN`, `Remaining`, and `Reset` operations simultaneously with 0 data races.
- **Per-Key Lock Granularity**: Map registry lookups use read-write lock stripping (`sync.RWMutex`) while individual bucket state mutations use fine-grained per-bucket `sync.Mutex`, preventing cross-client contention bottlenecks.
- **Clock Drift & Monotonic Safety**: Window calculations and refill rates use `time.Time` and `time.Duration` arithmetic with floor clamping, guaranteeing safety against clock step adjustments or negative time elapsed.
- **Zero Heap Allocations**: Core `Allow()` calls for `TokenBucket` and `SlidingWindow` run in under 200ns with **0 B/op** and **0 allocs/op**.

