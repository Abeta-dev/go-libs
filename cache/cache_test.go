// SPDX-License-Identifier: MIT

package cache_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/umesh0492/go-libs/cache"
)

func TestMemoryCache_GetOrFetch(t *testing.T) {
	c := cache.NewMemoryCache()
	ctx := context.Background()

	var fetchCount int32
	fetchFn := func(ctx context.Context) (interface{}, error) {
		atomic.AddInt32(&fetchCount, 1)
		time.Sleep(50 * time.Millisecond) // Simulate slow DB query
		return "result", nil
	}

	// Test stampede protection
	var wg sync.WaitGroup
	results := make([]interface{}, 10)
	errs := make([]error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			res, err := c.GetOrFetch(ctx, "test-key", 1*time.Minute, fetchFn)
			results[idx] = res
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	// All should have the same result without errors
	for i := 0; i < 10; i++ {
		assert.NoError(t, errs[i])
		assert.Equal(t, "result", results[i])
	}

	// Fetch function should only have been called exactly once!
	assert.Equal(t, int32(1), atomic.LoadInt32(&fetchCount))

	// Fast path test (should hit cache immediately)
	res, err := c.GetOrFetch(ctx, "test-key", 1*time.Minute, fetchFn)
	assert.NoError(t, err)
	assert.Equal(t, "result", res)
	assert.Equal(t, int32(1), atomic.LoadInt32(&fetchCount)) // Still 1

	// Delete test
	c.Delete("test-key")
	res, err = c.GetOrFetch(ctx, "test-key", 1*time.Minute, fetchFn)
	assert.NoError(t, err)
	assert.Equal(t, "result", res)
	assert.Equal(t, int32(2), atomic.LoadInt32(&fetchCount)) // Now 2
}

func TestMemoryCache_FetchError(t *testing.T) {
	c := cache.NewMemoryCache()
	ctx := context.Background()

	expectedErr := errors.New("db error")
	res, err := c.GetOrFetch(ctx, "err-key", 1*time.Minute, func(ctx context.Context) (interface{}, error) {
		return nil, expectedErr
	})

	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, res)
}

func TestMemoryCache_Expiration(t *testing.T) {
	c := cache.NewMemoryCache()
	ctx := context.Background()

	var fetchCount int32
	fetchFn := func(ctx context.Context) (interface{}, error) {
		atomic.AddInt32(&fetchCount, 1)
		return "result", nil
	}

	// Very short TTL
	_, err := c.GetOrFetch(ctx, "exp-key", 1*time.Millisecond, fetchFn)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&fetchCount))

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	_, err = c.GetOrFetch(ctx, "exp-key", 1*time.Millisecond, fetchFn)
	assert.NoError(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&fetchCount))
}

func TestTypedCache_Generics(t *testing.T) {
	type User struct {
		ID   int
		Name string
	}

	c := cache.NewTypedCache[User]()
	ctx := context.Background()

	user, err := c.GetOrFetch(ctx, "user:1", time.Minute, func(ctx context.Context) (User, error) {
		return User{ID: 1, Name: "Alice"}, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, user.ID)
	assert.Equal(t, "Alice", user.Name)

	// Fetch error test
	expectedErr := errors.New("not found")
	_, err = c.GetOrFetch(ctx, "user:2", time.Minute, func(ctx context.Context) (User, error) {
		return User{}, expectedErr
	})
	assert.ErrorIs(t, err, expectedErr)
}

type safeCounter struct {
	mu sync.Mutex
	n  float64
}

func (c *safeCounter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
}

func (c *safeCounter) Add(d float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n += d
}

func (c *safeCounter) Get() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

func TestCache_MetricsAndEviction(t *testing.T) {
	hits := &safeCounter{}
	misses := &safeCounter{}
	evictions := &safeCounter{}

	c := cache.NewTypedCache[string](
		cache.WithMetrics[string](cache.Metrics{
			Hits:      hits,
			Misses:    misses,
			Evictions: evictions,
		}),
		cache.WithEvictionInterval[string](10*time.Millisecond),
	)
	defer c.Close()

	ctx := context.Background()

	// 1. Initial fetch (miss)
	val, err := c.GetOrFetch(ctx, "k1", 20*time.Millisecond, func(_ context.Context) (string, error) {
		return "v1", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "v1", val)
	assert.Equal(t, 1.0, misses.Get())
	assert.Equal(t, 0.0, hits.Get())

	// 2. Second fetch before expiry (hit)
	val, err = c.GetOrFetch(ctx, "k1", 20*time.Millisecond, func(_ context.Context) (string, error) {
		return "v2", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "v1", val)
	assert.Equal(t, 1.0, hits.Get())

	// 3. Wait for background sweeper to evict
	time.Sleep(50 * time.Millisecond)
	assert.GreaterOrEqual(t, evictions.Get(), 1.0)
}

func TestTypedCache_GetSetLen(t *testing.T) {
	c := cache.NewTypedCache[string]()
	assert.Equal(t, 0, c.Len())

	// Miss on empty
	val, ok := c.Get("missing")
	assert.False(t, ok)
	assert.Empty(t, val)

	// Set and Get
	c.Set("k1", "v1")
	assert.Equal(t, 1, c.Len())
	val, ok = c.Get("k1")
	assert.True(t, ok)
	assert.Equal(t, "v1", val)

	// Set with TTL and expire
	c.Set("k2", "v2", 10*time.Millisecond)
	time.Sleep(25 * time.Millisecond)
	val, ok = c.Get("k2")
	assert.False(t, ok)
	assert.Empty(t, val)
}

func TestLRUCache(t *testing.T) {
	hits := &safeCounter{}
	misses := &safeCounter{}
	evictions := &safeCounter{}

	lru := cache.NewTypedCache[int](
		cache.WithCapacity[int](2),
		cache.WithEvictionPolicy[int](cache.EvictionLRU),
		cache.WithMetrics[int](cache.Metrics{
			Hits:      hits,
			Misses:    misses,
			Evictions: evictions,
		}),
	)

	// Defaults check
	defLRU := cache.NewTypedCache[int](
		cache.WithCapacity[int](0),
		cache.WithEvictionPolicy[int](cache.EvictionLRU),
	)
	assert.Equal(t, 0, defLRU.Len())

	// 1. Set 2 items
	lru.Set("a", 1)
	lru.Set("b", 2)
	assert.Equal(t, 2, lru.Len())

	// 2. Access "a" so "b" becomes the least recently used
	val, ok := lru.Get("a")
	assert.True(t, ok)
	assert.Equal(t, 1, val)

	// 3. Insert "c" -> evicts "b"
	lru.Set("c", 3)
	assert.Equal(t, 2, lru.Len())

	_, ok = lru.Get("b")
	assert.False(t, ok) // "b" was evicted

	val, ok = lru.Get("a")
	assert.True(t, ok)
	assert.Equal(t, 1, val)

	// 4. Overwrite existing key
	lru.Set("a", 100)
	val, ok = lru.Get("a")
	assert.True(t, ok)
	assert.Equal(t, 100, val)

	// 5. Expiration in LRU
	lru.Set("exp", 999, 10*time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	_, ok = lru.Get("exp")
	assert.False(t, ok)

	// 6. Delete
	lru.Delete("a")
	assert.Equal(t, 0, lru.Len())

	// 7. GetOrFetch
	ctx := context.Background()
	fetchVal, err := lru.GetOrFetch(ctx, "fetched", time.Minute, func(ctx context.Context) (int, error) {
		return 42, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 42, fetchVal)

	// Fetch error
	expErr := errors.New("fail")
	_, err = lru.GetOrFetch(ctx, "err_key", time.Minute, func(ctx context.Context) (int, error) {
		return 0, expErr
	})
	assert.ErrorIs(t, err, expErr)
}

func TestLFUCache(t *testing.T) {
	hits := &safeCounter{}
	misses := &safeCounter{}
	evictions := &safeCounter{}

	lfu := cache.NewTypedCache[string](
		cache.WithCapacity[string](2),
		cache.WithEvictionPolicy[string](cache.EvictionLFU),
		cache.WithMetrics[string](cache.Metrics{
			Hits:      hits,
			Misses:    misses,
			Evictions: evictions,
		}),
	)

	// Defaults check
	defLFU := cache.NewTypedCache[string](
		cache.WithCapacity[string](0),
		cache.WithEvictionPolicy[string](cache.EvictionLFU),
	)
	assert.Equal(t, 0, defLFU.Len())

	// 1. Insert "a" and "b"
	lfu.Set("a", "alpha")
	lfu.Set("b", "beta")

	// 2. Access "a" twice, "b" once
	lfu.Get("a")
	lfu.Get("a")
	lfu.Get("b")

	// Frequency: "a" = 3, "b" = 2.
	// 3. Insert "c" -> should evict "b" (lowest freq)
	lfu.Set("c", "gamma")
	assert.Equal(t, 2, lfu.Len())

	_, ok := lfu.Get("b")
	assert.False(t, ok) // "b" was evicted

	val, ok := lfu.Get("a")
	assert.True(t, ok)
	assert.Equal(t, "alpha", val)

	// 4. Update existing key
	lfu.Set("a", "alpha2")
	val, ok = lfu.Get("a")
	assert.True(t, ok)
	assert.Equal(t, "alpha2", val)

	// 5. Expiration in LFU
	lfu.Set("exp", "exp_val", 10*time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	_, ok = lfu.Get("exp")
	assert.False(t, ok)

	// 6. Delete
	lfu.Delete("a")
	assert.Equal(t, 0, lfu.Len())

	// 7. GetOrFetch
	ctx := context.Background()
	fetchVal, err := lfu.GetOrFetch(ctx, "lfu_fetched", time.Minute, func(ctx context.Context) (string, error) {
		return "hello", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "hello", fetchVal)

	// Fetch error
	expErr := errors.New("lfu fail")
	_, err = lfu.GetOrFetch(ctx, "err_key", time.Minute, func(ctx context.Context) (string, error) {
		return "", expErr
	})
	assert.ErrorIs(t, err, expErr)
}

func TestFIFOCache(t *testing.T) {
	hits := &safeCounter{}
	misses := &safeCounter{}
	evictions := &safeCounter{}

	fifo := cache.NewTypedCache[int](
		cache.WithCapacity[int](2),
		cache.WithEvictionPolicy[int](cache.EvictionFIFO),
		cache.WithMetrics[int](cache.Metrics{
			Hits:      hits,
			Misses:    misses,
			Evictions: evictions,
		}),
	)

	// Defaults check
	defFIFO := cache.NewTypedCache[int](
		cache.WithCapacity[int](0),
		cache.WithEvictionPolicy[int](cache.EvictionFIFO),
	)
	assert.Equal(t, 0, defFIFO.Len())

	// 1. Insert "1" then "2"
	fifo.Set("k1", 10)
	fifo.Set("k2", 20)
	assert.Equal(t, 2, fifo.Len())

	// Access "k1" - in FIFO, accessing does NOT change eviction order!
	val, ok := fifo.Get("k1")
	assert.True(t, ok)
	assert.Equal(t, 10, val)

	// 2. Insert "k3" -> evicts oldest ("k1")
	fifo.Set("k3", 30)
	assert.Equal(t, 2, fifo.Len())

	_, ok = fifo.Get("k1")
	assert.False(t, ok) // "k1" evicted

	val, ok = fifo.Get("k2")
	assert.True(t, ok)
	assert.Equal(t, 20, val)

	// 3. Update existing
	fifo.Set("k2", 200)
	val, ok = fifo.Get("k2")
	assert.True(t, ok)
	assert.Equal(t, 200, val)

	// 4. Expiration in FIFO
	fifo.Set("exp", 555, 10*time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	_, ok = fifo.Get("exp")
	assert.False(t, ok)

	// 5. Delete
	fifo.Delete("k2")
	assert.Equal(t, 1, fifo.Len())

	// 6. GetOrFetch
	ctx := context.Background()
	fetchVal, err := fifo.GetOrFetch(ctx, "fifo_fetched", time.Minute, func(ctx context.Context) (int, error) {
		return 999, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 999, fetchVal)

	// Fetch error
	expErr := errors.New("fifo fail")
	_, err = fifo.GetOrFetch(ctx, "err_key", time.Minute, func(ctx context.Context) (int, error) {
		return 0, expErr
	})
	assert.ErrorIs(t, err, expErr)
}

func TestTypedCache_Adversarial_ZeroTTLExpiration(t *testing.T) {
	c := cache.NewTypedCache[string]()
	ctx := context.Background()

	// Set key without expiration (Expiration == 0)
	c.Set("infinite-key", "permanent-value")

	var fetchCalled atomic.Bool
	val, err := c.GetOrFetch(ctx, "infinite-key", 0, func(ctx context.Context) (string, error) {
		fetchCalled.Store(true)
		return "unexpected-new-value", nil
	})

	assert.NoError(t, err)
	assert.Equal(t, "permanent-value", val)
	assert.False(t, fetchCalled.Load(), "GetOrFetch must hit cache for keys with zero TTL")
}

func TestTypedCache_Adversarial_LeaderCancelDoesNotPoisonFollowers(t *testing.T) {
	c := cache.NewTypedCache[string]()

	leaderStarted := make(chan struct{})
	var fetchCount atomic.Int32

	fetchFn := func(ctx context.Context) (string, error) {
		fetchCount.Add(1)
		close(leaderStarted)
		time.Sleep(50 * time.Millisecond) // slow fetch
		return "shared-value", nil
	}

	// Leader with quick cancellation
	leaderCtx, leaderCancel := context.WithCancel(context.Background())
	var leaderErr error
	var leaderVal string

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		leaderVal, leaderErr = c.GetOrFetch(leaderCtx, "leader-key", time.Minute, fetchFn)
	}()

	// Wait until leader initiates fetch
	<-leaderStarted

	// Follower with long timeout
	followerCtx, followerCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer followerCancel()

	var followerErr error
	var followerVal string
	wg.Add(1)
	go func() {
		defer wg.Done()
		followerVal, followerErr = c.GetOrFetch(followerCtx, "leader-key", time.Minute, fetchFn)
	}()

	// Cancel leader context
	leaderCancel()

	wg.Wait()

	// Leader must have received cancellation error
	assert.ErrorIs(t, leaderErr, context.Canceled)
	assert.Empty(t, leaderVal)

	// Follower must succeed and NOT be poisoned by leader's cancellation
	assert.NoError(t, followerErr)
	assert.Equal(t, "shared-value", followerVal)
	assert.Equal(t, int32(1), fetchCount.Load())
}

func TestTypedCache_Adversarial_HighConcurrencyStampede(t *testing.T) {
	c := cache.NewTypedCache[int]()
	ctx := context.Background()

	var fetchCount atomic.Int32
	var wg sync.WaitGroup
	workers := 100

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			val, err := c.GetOrFetch(ctx, "stampede-key", time.Minute, func(ctx context.Context) (int, error) {
				fetchCount.Add(1)
				time.Sleep(10 * time.Millisecond)
				return 42, nil
			})
			assert.NoError(t, err)
			assert.Equal(t, 42, val)
		}()
	}

	wg.Wait()
	assert.Equal(t, int32(1), fetchCount.Load())
}

func TestTypedCache_NoTTLEntriesSurviveEvictionSweep(t *testing.T) {
	evictions := &safeCounter{}
	c := cache.NewTypedCache[string](
		cache.WithEvictionInterval[string](10*time.Millisecond),
		cache.WithMetrics[string](cache.Metrics{
			Evictions: evictions,
		}),
	)
	defer c.Close()

	// Store an entry without TTL (Expiration == 0) and one with 0 TTL explicitly
	c.Set("permanent-no-ttl", "still-here")
	c.Set("permanent-zero-ttl", "also-here", 0)

	// Store an entry with a short TTL
	c.Set("ephemeral", "gone-soon", 15*time.Millisecond)

	// Wait across multiple eviction sweeps
	time.Sleep(50 * time.Millisecond)

	// Ephemeral entry must have been evicted
	_, ok := c.Get("ephemeral")
	assert.False(t, ok, "Ephemeral entry should be evicted after TTL expires")

	// Permanent entries MUST survive eviction sweeps
	val1, ok1 := c.Get("permanent-no-ttl")
	assert.True(t, ok1, "No-TTL entry must survive background eviction sweeps")
	assert.Equal(t, "still-here", val1)

	val2, ok2 := c.Get("permanent-zero-ttl")
	assert.True(t, ok2, "Zero-TTL entry must survive background eviction sweeps")
	assert.Equal(t, "also-here", val2)

	assert.Equal(t, 1.0, evictions.Get(), "Only the expired item should have been evicted")
}

func TestTypedCache_WithCapacity(t *testing.T) {
	evictions := &safeCounter{}
	c := cache.NewTypedCache[string](
		cache.WithCapacity[string](3),
		cache.WithMetrics[string](cache.Metrics{
			Evictions: evictions,
		}),
	)

	// 1. Insert 3 items (within capacity)
	c.Set("k1", "v1")
	time.Sleep(2 * time.Millisecond)
	c.Set("k2", "v2")
	time.Sleep(2 * time.Millisecond)
	c.Set("k3", "v3")

	assert.Equal(t, 3, c.Len())
	assert.Equal(t, 0.0, evictions.Get())

	// 2. Insert 4th item -> must evict oldest item ("k1")
	time.Sleep(2 * time.Millisecond)
	c.Set("k4", "v4")

	assert.Equal(t, 3, c.Len(), "Capacity upper bound must be enforced")
	assert.Equal(t, 1.0, evictions.Get(), "One item must have been evicted")

	_, ok := c.Get("k1")
	assert.False(t, ok, "Oldest item k1 must have been evicted")

	val4, ok := c.Get("k4")
	assert.True(t, ok, "New item k4 must be present")
	assert.Equal(t, "v4", val4)

	// 3. Update existing item -> should not evict anything
	c.Set("k2", "v2-updated")
	assert.Equal(t, 3, c.Len())
	assert.Equal(t, 1.0, evictions.Get())
	val2, _ := c.Get("k2")
	assert.Equal(t, "v2-updated", val2)

	// 4. Test GetOrFetch respects capacity
	ctx := context.Background()
	_, err := c.GetOrFetch(ctx, "k5", time.Minute, func(ctx context.Context) (string, error) {
		return "v5", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 3, c.Len(), "Capacity upper bound must be enforced on GetOrFetch")
	assert.Equal(t, 2.0, evictions.Get(), "Eviction count must increment on GetOrFetch eviction")
}

func TestTypedCache_WithCapacity_Concurrent(t *testing.T) {
	capacity := 10
	c := cache.NewTypedCache[int](
		cache.WithCapacity[int](capacity),
	)

	var wg sync.WaitGroup
	workers := 100

	for i := 0; i < workers; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			c.Set(string(rune('A'+idx)), idx)
		}()
	}

	wg.Wait()
	assert.LessOrEqual(t, c.Len(), capacity, "Concurrent writes must not exceed cache capacity")
}

func TestTypedCache_DefaultCapacity_And_O1Len(t *testing.T) {
	c := cache.NewTypedCache[string]()
	assert.Equal(t, 0, c.Len())

	// Add items and verify atomic size tracking
	c.Set("k1", "v1")
	c.Set("k2", "v2")
	assert.Equal(t, 2, c.Len())

	// Overwrite existing key - size should remain 2
	c.Set("k2", "v2-updated")
	assert.Equal(t, 2, c.Len())

	// Delete key - size should decrement
	c.Delete("k1")
	assert.Equal(t, 1, c.Len())

	// Delete non-existent key - size should remain 1
	c.Delete("non-existent")
	assert.Equal(t, 1, c.Len())
}

func TestTypedCache_DefaultCapacityUnbounded(t *testing.T) {
	assert.Equal(t, 0, cache.DefaultCapacity)

	c := cache.NewTypedCache[int]()
	const total = 12000
	for i := 0; i < total; i++ {
		c.Set(strconv.Itoa(i), i)
	}

	assert.Equal(t, total, c.Len(), "Default cache should be unbounded (capacity=0)")
	for i := 0; i < total; i += 1000 {
		val, ok := c.Get(strconv.Itoa(i))
		assert.True(t, ok)
		assert.Equal(t, i, val)
	}
}

func TestTypedCache_Len_ConcurrentSetDelete_NoDrift(t *testing.T) {
	runTest := func(name string, opts ...cache.Option[int]) {
		t.Run(name, func(t *testing.T) {
			c := cache.NewTypedCache[int](opts...)
			var wg sync.WaitGroup
			workers := 20
			opsPerWorker := 300

			for w := 0; w < workers; w++ {
				wg.Add(1)
				workerID := w
				go func() {
					defer wg.Done()
					for i := 0; i < opsPerWorker; i++ {
						key := strconv.Itoa((workerID*17 + i) % 40)
						if i%3 == 0 {
							c.Delete(key)
						} else {
							c.Set(key, i)
						}
					}
				}()
			}
			wg.Wait()

			assert.GreaterOrEqual(t, c.Len(), 0)

			// Drain all keys
			for i := 0; i < 40; i++ {
				c.Delete(strconv.Itoa(i))
			}
			assert.Equal(t, 0, c.Len(), "Counter must not drift and should reach 0 when all keys are deleted")
		})
	}

	runTest("Unbounded")
	runTest("BoundedCapacity", cache.WithCapacity[int](15))
}

func TestTypedCache_UniformEvictionSampling(t *testing.T) {
	evictions := &safeCounter{}
	capacity := 30
	c := cache.NewTypedCache[int](
		cache.WithCapacity[int](capacity),
		cache.WithMetrics[int](cache.Metrics{
			Evictions: evictions,
		}),
	)

	// Populate capacity entries
	for i := 0; i < capacity; i++ {
		c.Set(fmt.Sprintf("init-%03d", i), i)
		time.Sleep(50 * time.Microsecond)
	}
	assert.Equal(t, capacity, c.Len())

	// Insert more entries to trigger evictions
	for i := 0; i < 20; i++ {
		c.Set(fmt.Sprintf("new-%03d", i), i)
		time.Sleep(50 * time.Microsecond)
	}

	assert.Equal(t, capacity, c.Len())
	assert.Equal(t, 20.0, evictions.Get())
}

func TestTypedCache_ZeroValue(t *testing.T) {
	var c cache.TypedCache[string]

	assert.Equal(t, 0, c.Len())
	val, ok := c.Get("missing")
	assert.False(t, ok)
	assert.Empty(t, val)

	c.Set("foo", "bar")
	val, ok = c.Get("foo")
	assert.True(t, ok)
	assert.Equal(t, "bar", val)
	assert.Equal(t, 1, c.Len())

	fetched, err := c.GetOrFetch(context.Background(), "baz", time.Minute, func(ctx context.Context) (string, error) {
		return "qux", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "qux", fetched)

	c.Delete("foo")
	assert.Equal(t, 1, c.Len())

	assert.NotPanics(t, func() {
		c.Close()
	})
}
