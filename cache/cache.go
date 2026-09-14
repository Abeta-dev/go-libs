// SPDX-License-Identifier: MIT

// Package cache provides a thread-safe, singleflight-protected in-memory cache with TTL.
package cache

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/umesh0492/go-libs/clock"
	"github.com/umesh0492/go-libs/metrics"
	"golang.org/x/sync/singleflight"
)

// DefaultCapacity is the default maximum number of entries allowed in the cache (0 means unbounded).
// Capacity enforcement is opt-in via WithCapacity(n).
const DefaultCapacity = 0

const (
	numStripes         = 16
	evictionSampleSize = 16
)

func fnv32(key string) uint32 {
	var hash uint32 = 2166136261
	for i := 0; i < len(key); i++ {
		hash ^= uint32(key[i])
		hash *= 16777619
	}
	return hash
}

// Item represents a cached value with its expiration time and creation timestamp.
type Item[T any] struct {
	Value      T
	Expiration int64
	CreatedAt  int64
}

// Metrics holds optional metrics instrumentation for cache observability.
type Metrics struct {
	Hits      metrics.Counter
	Misses    metrics.Counter
	Evictions metrics.Counter
}

// EvictionPolicy defines the replacement algorithm used when capacity is exceeded.
type EvictionPolicy int

const (
	// EvictionSampledLRU selects candidates from a random sample and evicts the least recently accessed (Redis-style).
	EvictionSampledLRU EvictionPolicy = iota
	// EvictionLRU evicts the strictly least recently used item (O(1) doubly-linked list).
	EvictionLRU
	// EvictionLFU evicts the strictly least frequently used item (O(1) frequency buckets).
	EvictionLFU
	// EvictionFIFO evicts the strictly first inserted item (O(1) insertion order).
	EvictionFIFO
)

// WithEvictionPolicy sets the eviction policy to use when capacity is exceeded.
// Defaults to EvictionSampledLRU.
func WithEvictionPolicy[T any](policy EvictionPolicy) Option[T] {
	return func(c *TypedCache[T]) {
		c.policy = policy
	}
}

// Option configures a TypedCache.
type Option[T any] func(*TypedCache[T])

// WithClock sets an abstract time source for the cache.
// If nil or not provided, clock.NewReal() is used.
func WithClock[T any](c clock.Clock) Option[T] {
	return func(tc *TypedCache[T]) {
		if c != nil {
			tc.clock = c
		}
	}
}

// WithMetrics attaches metrics collectors to the cache.
func WithMetrics[T any](m Metrics) Option[T] {
	return func(c *TypedCache[T]) {
		c.metrics = m
	}
}

// WithEvictionInterval starts a background goroutine that sweeps and evicts
// expired cache entries at the specified interval. Call Close() to stop the sweeper.
func WithEvictionInterval[T any](d time.Duration) Option[T] {
	return func(c *TypedCache[T]) {
		c.evictInterval = d
	}
}

// WithCapacity sets an upper bound on the number of entries stored in the cache.
// When capacity is exceeded, an existing entry (the oldest or an expired entry) is evicted.
// If maxEntries <= 0, capacity enforcement is disabled (unbounded).
func WithCapacity[T any](maxEntries int) Option[T] {
	return func(c *TypedCache[T]) {
		c.capacity = maxEntries
	}
}

// Cache defines a generic type-safe cache interface with stampede protection.
type Cache[T any] interface {
	// Get retrieves an item from the cache. Returns false if not found or expired.
	Get(key string) (T, bool)
	// Set stores an item in the cache with an optional TTL.
	Set(key string, value T, ttl ...time.Duration)
	// GetOrFetch attempts to get an item from cache, or fetches it via fetchFn under singleflight stampede protection.
	GetOrFetch(ctx context.Context, key string, ttl time.Duration, fetchFn func(context.Context) (T, error)) (T, error)
	// Delete removes an item from the cache.
	Delete(key string)
	// Len returns the number of active items stored in the cache in O(1) time.
	Len() int
}

// TypedCache provides type-safe in-memory caching with singleflight stampede protection.
type TypedCache[T any] struct {
	policy        EvictionPolicy
	data          sync.Map
	sf            singleflight.Group
	metrics       Metrics
	stopEvict     chan struct{}
	closeOnce     sync.Once
	capacity      int
	stripes       [numStripes]sync.Mutex
	size          atomic.Int64
	evictOffset   atomic.Uint64
	clock         clock.Clock
	evictInterval time.Duration

	lru      *lruBackend[T]
	lfu      *lfuBackend[T]
	fifo     *fifoBackend[T]
	initOnce sync.Once
}

func (c *TypedCache[T]) ensureInit() {
	c.initOnce.Do(func() {
		if c.clock == nil {
			c.clock = clock.NewReal()
		}
		if c.metrics.Hits == nil {
			c.metrics.Hits = metrics.NoopCounter{}
		}
		if c.metrics.Misses == nil {
			c.metrics.Misses = metrics.NoopCounter{}
		}
		if c.metrics.Evictions == nil {
			c.metrics.Evictions = metrics.NoopCounter{}
		}
		capVal := c.capacity
		if capVal <= 0 {
			capVal = 128
		}
		switch c.policy {
		case EvictionLRU:
			if c.lru == nil {
				c.lru = newLRUBackend[T](capVal, c.metrics, c.clock)
			}
		case EvictionLFU:
			if c.lfu == nil {
				c.lfu = newLFUBackend[T](capVal, c.metrics, c.clock)
			}
		case EvictionFIFO:
			if c.fifo == nil {
				c.fifo = newFIFOBackend[T](capVal, c.metrics, c.clock)
			}
		}
	})
}

func (c *TypedCache[T]) getStripe(key string) *sync.Mutex {
	return &c.stripes[fnv32(key)%numStripes]
}

// NewTypedCache creates a new type-safe in-memory cache for type T with DefaultCapacity (unbounded).
func NewTypedCache[T any](opts ...Option[T]) *TypedCache[T] {
	c := &TypedCache[T]{
		capacity: DefaultCapacity,
		policy:   EvictionSampledLRU,
		clock:    clock.NewReal(),
		metrics: Metrics{
			Hits:      metrics.NoopCounter{},
			Misses:    metrics.NoopCounter{},
			Evictions: metrics.NoopCounter{},
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.metrics.Hits == nil {
		c.metrics.Hits = metrics.NoopCounter{}
	}
	if c.metrics.Misses == nil {
		c.metrics.Misses = metrics.NoopCounter{}
	}
	if c.metrics.Evictions == nil {
		c.metrics.Evictions = metrics.NoopCounter{}
	}

	capVal := c.capacity
	if capVal <= 0 {
		capVal = 128
	}

	if c.clock == nil {
		c.clock = clock.NewReal()
	}

	switch c.policy {
	case EvictionLRU:
		c.lru = newLRUBackend[T](capVal, c.metrics, c.clock)
	case EvictionLFU:
		c.lfu = newLFUBackend[T](capVal, c.metrics, c.clock)
	case EvictionFIFO:
		c.fifo = newFIFOBackend[T](capVal, c.metrics, c.clock)
	}

	if c.evictInterval > 0 {
		c.stopEvict = make(chan struct{})
		go c.startEviction(c.evictInterval)
	}

	return c
}

func (c *TypedCache[T]) startEviction(interval time.Duration) {
	ticker := c.clock.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-c.stopEvict:
			return
		case <-ticker.C():
			now := c.clock.Now().UnixNano()
			c.data.Range(func(k, v any) bool {
				if item, ok := v.(Item[T]); ok && item.Expiration > 0 && item.Expiration <= now {
					keyStr, ok := k.(string)
					if ok {
						stripe := c.getStripe(keyStr)
						stripe.Lock()
						if v2, ok2 := c.data.Load(k); ok2 {
							if item2, ok3 := v2.(Item[T]); ok3 && item2.Expiration > 0 && item2.Expiration <= now {
								if _, loaded := c.data.LoadAndDelete(k); loaded {
									c.size.Add(-1)
									c.metrics.Evictions.Inc()
								}
							}
						}
						stripe.Unlock()
					}
				}
				return true
			})
		}
	}
}

func (c *TypedCache[T]) findFallbackEvictionCandidate(newKey string, now int64) any {
	var (
		oldestKey  any
		oldestTime int64 = 1<<63 - 1
		expiredKey any
		sampled    int
	)
	c.data.Range(func(k, v any) bool {
		if k == newKey {
			return true
		}
		sampled++
		if item, ok := v.(Item[T]); ok {
			if item.Expiration > 0 && item.Expiration <= now {
				expiredKey = k
				return false
			}
			if item.CreatedAt < oldestTime {
				oldestTime = item.CreatedAt
				oldestKey = k
			}
		} else {
			oldestKey = k
		}
		return sampled < evictionSampleSize
	})

	if expiredKey != nil {
		return expiredKey
	}
	return oldestKey
}

func (c *TypedCache[T]) findSampledEvictionCandidate(newKey string, now int64) any {
	var (
		oldestKey  any
		oldestTime int64 = 1<<63 - 1
		expiredKey any
		sampled    int
	)

	sz := c.size.Load()
	stride := 1
	if s := sz / int64(evictionSampleSize); s > 1 {
		//nolint:gosec // G115: bounded by cache capacity
		stride = int(s)
	}
	//nolint:gosec // G115: modulo arithmetic guarantees bounded result
	offset := int(c.evictOffset.Add(1) % uint64(stride))

	var idx int
	c.data.Range(func(k, v any) bool {
		if k == newKey {
			return true
		}
		if idx >= offset && (idx-offset)%stride == 0 {
			sampled++
			if item, ok := v.(Item[T]); ok {
				if item.Expiration > 0 && item.Expiration <= now {
					expiredKey = k
					return false
				}
				if item.CreatedAt < oldestTime {
					oldestTime = item.CreatedAt
					oldestKey = k
				}
			} else {
				oldestKey = k
			}
			if sampled >= evictionSampleSize {
				return false
			}
		}
		idx++
		return true
	})

	if expiredKey != nil {
		return expiredKey
	}
	if oldestKey != nil {
		return oldestKey
	}

	return c.findFallbackEvictionCandidate(newKey, now)
}

func (c *TypedCache[T]) evictIfNeeded(newKey string) {
	if c.capacity <= 0 {
		return
	}

	for c.size.Load() >= int64(c.capacity) {
		now := c.clock.Now().UnixNano()
		targetKey := c.findSampledEvictionCandidate(newKey, now)
		if targetKey == nil {
			break
		}

		if _, loaded := c.data.LoadAndDelete(targetKey); loaded {
			c.size.Add(-1)
			c.metrics.Evictions.Inc()
		} else {
			break
		}
	}
}

// Close stops any background eviction goroutine and releases resources.
func (c *TypedCache[T]) Close() {
	c.closeOnce.Do(func() {
		if c.stopEvict != nil {
			close(c.stopEvict)
		}
		switch c.policy {
		case EvictionLRU:
			if c.lru != nil {
				c.lru.Close()
			}
		case EvictionLFU:
			if c.lfu != nil {
				c.lfu.Close()
			}
		case EvictionFIFO:
			if c.fifo != nil {
				c.fifo.Close()
			}
		}
	})
}

// MemoryCache provides in-memory caching for any values with singleflight stampede protection.
type MemoryCache = TypedCache[any]

// NewMemoryCache creates a new stampede-protected in-memory cache for any value.
func NewMemoryCache(opts ...Option[any]) *MemoryCache {
	return NewTypedCache[any](opts...)
}

func (c *TypedCache[T]) getOrFetchSampledLRU(ctx context.Context, key string, ttl time.Duration, fetchFn func(context.Context) (T, error)) (T, error) {
	if err := ctx.Err(); err != nil {
		var zero T
		return zero, err
	}

	// Fast path: check cache
	if val, ok := c.data.Load(key); ok {
		item := val.(Item[T])
		if item.Expiration == 0 || item.Expiration > c.clock.Now().UnixNano() {
			c.metrics.Hits.Inc()
			return item.Value, nil
		}
	}

	c.metrics.Misses.Inc()

	// Cache miss or expired: use singleflight to fetch
	ch := c.sf.DoChan(key, func() (any, error) {
		bgCtx := context.WithoutCancel(ctx)
		newValue, fetchErr := fetchFn(bgCtx)
		if fetchErr != nil {
			var zero T
			return zero, fetchErr
		}

		var exp int64
		now := c.clock.Now()
		if ttl > 0 {
			exp = now.Add(ttl).UnixNano()
		}
		item := Item[T]{
			Value:      newValue,
			Expiration: exp,
			CreatedAt:  now.UnixNano(),
		}

		if c.capacity > 0 {
			stripe := c.getStripe(key)
			stripe.Lock()
			if _, exists := c.data.Load(key); !exists {
				c.evictIfNeeded(key)
			}
			if _, loaded := c.data.Swap(key, item); !loaded {
				c.size.Add(1)
			}
			if c.size.Load() > int64(c.capacity) {
				c.evictIfNeeded(key)
			}
			stripe.Unlock()
		} else {
			if _, loaded := c.data.Swap(key, item); !loaded {
				c.size.Add(1)
			}
		}

		return newValue, nil
	})

	select {
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	case res := <-ch:
		if res.Err != nil {
			var zero T
			return zero, res.Err
		}
		return res.Val.(T), nil
	}
}

// GetOrFetch attempts to get a value from cache. If it misses or is expired,
// it uses singleflight to ensure fetchFn is only executed once for concurrent requests.
func (c *TypedCache[T]) GetOrFetch(ctx context.Context, key string, ttl time.Duration, fetchFn func(context.Context) (T, error)) (T, error) {
	c.ensureInit()
	switch c.policy {
	case EvictionLRU:
		return c.lru.GetOrFetch(ctx, key, ttl, fetchFn)
	case EvictionLFU:
		return c.lfu.GetOrFetch(ctx, key, ttl, fetchFn)
	case EvictionFIFO:
		return c.fifo.GetOrFetch(ctx, key, ttl, fetchFn)
	default:
		return c.getOrFetchSampledLRU(ctx, key, ttl, fetchFn)
	}
}

// Get retrieves an item from the cache. Returns false if missing or expired.
func (c *TypedCache[T]) Get(key string) (T, bool) {
	c.ensureInit()
	switch c.policy {
	case EvictionLRU:
		return c.lru.Get(key)
	case EvictionLFU:
		return c.lfu.Get(key)
	case EvictionFIFO:
		return c.fifo.Get(key)
	}

	if val, ok := c.data.Load(key); ok {
		item := val.(Item[T])
		if item.Expiration == 0 || item.Expiration > c.clock.Now().UnixNano() {
			c.metrics.Hits.Inc()
			return item.Value, true
		}
		// Expired entry cleanup under stripe lock
		stripe := c.getStripe(key)
		stripe.Lock()
		if val2, ok2 := c.data.Load(key); ok2 {
			item2 := val2.(Item[T])
			if item2.Expiration > 0 && item2.Expiration <= c.clock.Now().UnixNano() {
				if _, loaded := c.data.LoadAndDelete(key); loaded {
					c.size.Add(-1)
					c.metrics.Evictions.Inc()
				}
			}
		}
		stripe.Unlock()
	}
	c.metrics.Misses.Inc()
	var zero T
	return zero, false
}

// Set stores an item in the cache with an optional TTL. If no TTL is passed, it does not expire.
func (c *TypedCache[T]) Set(key string, value T, ttl ...time.Duration) {
	c.ensureInit()
	switch c.policy {
	case EvictionLRU:
		c.lru.Set(key, value, ttl...)
		return
	case EvictionLFU:
		c.lfu.Set(key, value, ttl...)
		return
	case EvictionFIFO:
		c.fifo.Set(key, value, ttl...)
		return
	}

	var exp int64
	now := c.clock.Now()
	if len(ttl) > 0 && ttl[0] > 0 {
		exp = now.Add(ttl[0]).UnixNano()
	}
	item := Item[T]{
		Value:      value,
		Expiration: exp,
		CreatedAt:  now.UnixNano(),
	}

	if c.capacity > 0 {
		stripe := c.getStripe(key)
		stripe.Lock()
		defer stripe.Unlock()

		if _, exists := c.data.Load(key); !exists {
			c.evictIfNeeded(key)
		}
		if _, loaded := c.data.Swap(key, item); !loaded {
			c.size.Add(1)
		}
		if c.size.Load() > int64(c.capacity) {
			c.evictIfNeeded(key)
		}
		return
	}

	if _, loaded := c.data.Swap(key, item); !loaded {
		c.size.Add(1)
	}
}

// Len returns the count of items currently in the cache in O(1) time.
func (c *TypedCache[T]) Len() int {
	c.ensureInit()
	switch c.policy {
	case EvictionLRU:
		return c.lru.Len()
	case EvictionLFU:
		return c.lfu.Len()
	case EvictionFIFO:
		return c.fifo.Len()
	}

	n := c.size.Load()
	if n < 0 {
		return 0
	}
	return int(n)
}

// Delete removes an item from the cache.
func (c *TypedCache[T]) Delete(key string) {
	c.ensureInit()
	switch c.policy {
	case EvictionLRU:
		c.lru.Delete(key)
		return
	case EvictionLFU:
		c.lfu.Delete(key)
		return
	case EvictionFIFO:
		c.fifo.Delete(key)
		return
	}

	if c.capacity > 0 {
		stripe := c.getStripe(key)
		stripe.Lock()
		defer stripe.Unlock()
	}
	if _, loaded := c.data.LoadAndDelete(key); loaded {
		c.size.Add(-1)
	}
}
