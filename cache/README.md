# `cache` Package

The `cache` package provides high-performance, in-memory caching with **4 industry-standard eviction algorithms** and built-in **Singleflight Cache Stampede Protection** with zero external dependencies (pure Go standard library).

## When to Use
- **High-Read Microservice Entities**: Caching hot database records, user permissions, or tenant configurations in RAM.
- **Cache Stampede / Thundering Herd Prevention**: Preventing thousands of concurrent requests from overwhelming downstream databases when an item expires via `GetOrFetch`.
- **Working Set Sizing**: Enforcing strict capacity boundaries via LRU, LFU, or FIFO when working memory must remain deterministic under high load.
- **Microsecond Response Latency**: Serving requests directly from memory in 65–115 ns/op without network roundtrips.

## Why It Is Written Like That
- **Generic Type-Safe `Cache[T]` Interface**: Eliminates runtime interface reflection and type assertions (`cache.NewTypedCache[UserProfile](1000, cache.WithEvictionPolicy(cache.EvictionLRU))`).
- **Singleflight Stampede Protection**: On cache misses, concurrent requests collapse into a single database fetch execution via internal `singleflight.Group`. All callers receive the resolved value simultaneously, eliminating thundering-herd crashes.
- **Strictly Zero Allocations**: Doubly linked list node reuse and hash map indexing achieve zero heap allocations on steady-state reads and cache hits.
- **Dual Expiration & Capacity Bounds**: Combines capacity limits (LRU, LFU, FIFO) with optional per-item time-to-live (TTL) expiry.

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/cache` |
|---|---|---|---|
| **`patrickmn/go-cache`** | Simple time-based expiration | TTL-only; no capacity limits (unbounded memory growth); no singleflight stampede defense; archived/unmaintained | Provides 4 eviction algorithms, strict capacity limits, and automatic stampede protection |
| **`dgraph-io/ristretto`** | High throughput via TinyLFU | Complex external dependency; eventual consistency delay on writes; heavy memory footprint for metadata | Pure Go standard library with strictly zero dependencies and deterministic synchronous operations |
| **Distributed Redis** | Shared cache across instances | 1-5ms network roundtrip latency; external network failure risk; serialized payloads | Zero-dependency in-process memory cache with sub-100ns access time |

---

## 🎯 4 Industry-Standard Eviction Algorithms

Every cache algorithm implements the unified `Cache[T]` interface:
```go
type Cache[T any] interface {
    Get(key string) (T, bool)
    Set(key string, value T, ttl ...time.Duration)
    GetOrFetch(ctx context.Context, key string, ttl time.Duration, fetchFn func(context.Context) (T, error)) (T, error)
    Delete(key string)
    Len() int
}
```

| Algorithm | Policy Option | Best For | Time Complexity | Latency (ns/op) | Allocations |
|:---|:---|:---|:---|:---|:---|
| **Sampled LRU (Default)** | `WithEvictionPolicy(EvictionSampledLRU)` | Highly concurrent striped in-memory caching | $O(1)$ | 94.7 ns/op | 0 allocs/op |
| **LRU (Least Recently Used)** | `WithEvictionPolicy(EvictionLRU)` | Working sets where recently accessed data is most likely to be requested again | $O(1)$ | 69.4 ns/op | 0 allocs/op |
| **LFU (Least Frequently Used)** | `WithEvictionPolicy(EvictionLFU)` | High-frequency catalog items, top sellers, steady-state access patterns | $O(1)$ | 113.1 ns/op | 0 allocs/op |
| **FIFO (First-In, First-Out)** | `WithEvictionPolicy(EvictionFIFO)` | Queue-like streams, sequential processing where recency has no bearing | $O(1)$ | 65.9 ns/op | 0 allocs/op |

---

## 🚀 Key Architectural Highlights

- **Singleflight Stampede Protection**: On cache misses, concurrent requests collapse into a single database/fetch execution via `singleflight.Group`. All callers receive the resolved value simultaneously, eliminating thundering-herd crashes.
- **Strictly Zero Allocations**: All eviction data structures (doubly linked list nodes, frequency buckets) achieve zero allocations on steady-state reads and cache hits.
- **Optional Per-Item TTL on Eviction Caches**: `LRU`, `LFU`, and `FIFO` support per-item expiration in addition to capacity limits.
- **Thread-Safe**: Fully protected against data races under high-concurrency multi-threaded workloads.

---

## 💻 Quickstart

### 1. LRU Cache (Capacity-Bounded)
```go
import (
    "context"
    "time"
    "github.com/umesh0492/go-libs/cache"
)

// LRU cache with capacity 1,000 items
lru := cache.NewTypedCache[UserProfile](
    cache.WithCapacity[UserProfile](1000),
    cache.WithEvictionPolicy[UserProfile](cache.EvictionLRU),
)

// Fast Get/Set
lru.Set("user:123", profile)
if user, ok := lru.Get("user:123"); ok {
    // Cache hit
}

// Singleflight GetOrFetch
user, err := lru.GetOrFetch(ctx, "user:123", time.Hour, func(ctx context.Context) (UserProfile, error) {
    return db.LoadUserProfile(ctx, "user:123")
})
```

### 2. LFU Cache (Frequency-Bounded)
```go
// LFU cache tracking access frequency with O(1) recency tie-breaking
lfu := cache.NewTypedCache[ProductDetails](
    cache.WithCapacity[ProductDetails](500),
    cache.WithEvictionPolicy[ProductDetails](cache.EvictionLFU),
)
details, err := lfu.GetOrFetch(ctx, "sku:456", 24*time.Hour, fetchProductFromDB)
```

### 3. FIFO Cache (Strict Insertion-Order Queue)
```go
// FIFO cache evicting oldest inserted item
fifo := cache.NewTypedCache[AuditEvent](
    cache.WithCapacity[AuditEvent](2000),
    cache.WithEvictionPolicy[AuditEvent](cache.EvictionFIFO),
)
fifo.Set("event:1", event)
```

### 4. TTL Cache with Stampede Protection
```go
var memCache = cache.NewMemoryCache()

func GetPlatformConfig(ctx context.Context, key string) (string, error) {
    val, err := memCache.GetOrFetch(ctx, "config:"+key, 5*time.Minute, func(ctx context.Context) (interface{}, error) {
        // This query executes ONLY ONCE even if 5,000 users ask simultaneously!
        return queryDBForConfig(ctx, key)
    })
    if err != nil {
        return "", err
    }
    return val.(string), nil
}
```

---

## 🛡️ Concurrency & Correctness Guarantees

- **`-race` Certified**: Stress-tested under 100+ concurrent goroutines performing simultaneous `Get`, `Set`, `Delete`, and adversarial cache evictions with 0 data races.
- **Singleflight De-Duplication**: Built-in `singleflight.Group` collapses thousands of concurrent misses on the same key into a single execution, eliminating database cache stampedes.
- **Zero-Allocation Steady-State Reads**: Cache hits run in 65–115 ns/op with 0 heap allocations, avoiding GC overhead under high-QPS workloads.
- **Memory Safety Under Eviction**: Linked lists and frequency buckets synchronize internal node pointers under mutex perimeters, guaranteeing zero broken pointer chains or memory leaks during rapid eviction cycles.

