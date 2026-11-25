// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/abeta-dev/go-libs/clock"
	"github.com/abeta-dev/go-libs/metrics"
	"golang.org/x/sync/singleflight"
)

type lruNode[T any] struct {
	key        string
	value      T
	expiration int64
	prev       *lruNode[T]
	next       *lruNode[T]
}

// lruBackend provides an in-memory, thread-safe cache with Least-Recently-Used eviction.
type lruBackend[T any] struct {
	mu       sync.Mutex
	capacity int
	items    map[string]*lruNode[T]
	head     *lruNode[T] // MRU
	tail     *lruNode[T] // LRU
	metrics  Metrics
	sf       singleflight.Group
	clock    clock.Clock
	onEvict  func(key string)
}

func newLRUBackend[T any](capacity int, m Metrics, clk clock.Clock) *lruBackend[T] {
	if capacity <= 0 {
		capacity = 128
	}
	if clk == nil {
		clk = clock.NewReal()
	}
	c := &lruBackend[T]{
		capacity: capacity,
		items:    make(map[string]*lruNode[T], capacity),
		metrics:  m,
		clock:    clk,
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
	return c
}

// Get retrieves an item from the cache and marks it as most recently used.
func (c *lruBackend[T]) Get(key string) (T, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	node, exists := c.items[key]
	if !exists {
		c.metrics.Misses.Inc()
		var zero T
		return zero, false
	}

	if node.expiration > 0 && node.expiration <= c.clock.Now().UnixNano() {
		c.removeNode(node)
		delete(c.items, key)
		c.metrics.Evictions.Inc()
		c.metrics.Misses.Inc()
		if c.onEvict != nil {
			c.onEvict(key)
		}
		var zero T
		return zero, false
	}

	c.moveToFront(node)
	c.metrics.Hits.Inc()
	return node.value, true
}

// Set stores an item in the cache, evicting the least recently used item if capacity is reached.
func (c *lruBackend[T]) Set(key string, value T, ttl ...time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var exp int64
	if len(ttl) > 0 && ttl[0] > 0 {
		exp = c.clock.Now().Add(ttl[0]).UnixNano()
	}

	if node, exists := c.items[key]; exists {
		node.value = value
		node.expiration = exp
		c.moveToFront(node)
		return
	}

	if len(c.items) >= c.capacity {
		c.evictOldest()
	}

	newNode := &lruNode[T]{
		key:        key,
		value:      value,
		expiration: exp,
	}
	c.addToFront(newNode)
	c.items[key] = newNode
}

// GetOrFetch retrieves an item or executes fetchFn under singleflight stampede protection.
func (c *lruBackend[T]) GetOrFetch(ctx context.Context, key string, ttl time.Duration, fetchFn func(context.Context) (T, error)) (T, error) {
	v, _, err := c.GetOrFetchWithStatus(ctx, key, ttl, nil, fetchFn)
	return v, err
}

// GetOrFetchWithStatus retrieves an item or executes fetchFn under singleflight stampede protection,
// reporting whether the result was a HIT, MISS, or COALESCED fetch.
func (c *lruBackend[T]) GetOrFetchWithStatus(ctx context.Context, key string, ttl time.Duration, onStore func(), fetchFn func(context.Context) (T, error)) (T, FetchStatus, error) {
	if err := ctx.Err(); err != nil {
		var zero T
		return zero, "", err
	}

	if val, ok := c.Get(key); ok {
		return val, FetchStatusHit, nil
	}

	var executed atomic.Bool
	ch := c.sf.DoChan(key, func() (any, error) {
		executed.Store(true)
		bgCtx := context.WithoutCancel(ctx)
		newValue, fetchErr := fetchFn(bgCtx)
		if fetchErr != nil {
			var zero T
			return zero, fetchErr
		}
		c.Set(key, newValue, ttl)
		if onStore != nil {
			onStore()
		}
		return newValue, nil
	})

	select {
	case <-ctx.Done():
		var zero T
		return zero, "", ctx.Err()
	case res := <-ch:
		if res.Err != nil {
			var zero T
			return zero, "", res.Err
		}
		if executed.Load() {
			return res.Val.(T), FetchStatusMiss, nil
		}
		return res.Val.(T), FetchStatusCoalesced, nil
	}
}

// Delete removes an item from the cache and reports whether it existed.
func (c *lruBackend[T]) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if node, exists := c.items[key]; exists {
		c.removeNode(node)
		delete(c.items, key)
		return true
	}
	return false
}

// DeletePrefix removes all items whose keys start with prefix and returns the deleted keys.
func (c *lruBackend[T]) DeletePrefix(prefix string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	var deleted []string
	for k, node := range c.items {
		if strings.HasPrefix(k, prefix) {
			c.removeNode(node)
			delete(c.items, k)
			deleted = append(deleted, k)
		}
	}
	return deleted
}

// Len returns the current number of items stored in the cache.
func (c *lruBackend[T]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

func (c *lruBackend[T]) addToFront(node *lruNode[T]) {
	node.prev = nil
	node.next = c.head
	if c.head != nil {
		c.head.prev = node
	}
	c.head = node
	if c.tail == nil {
		c.tail = node
	}
}

func (c *lruBackend[T]) removeNode(node *lruNode[T]) {
	if node.prev != nil {
		node.prev.next = node.next
	} else {
		c.head = node.next
	}

	if node.next != nil {
		node.next.prev = node.prev
	} else {
		c.tail = node.prev
	}

	node.prev = nil
	node.next = nil
}

func (c *lruBackend[T]) moveToFront(node *lruNode[T]) {
	if c.head == node {
		return
	}
	c.removeNode(node)
	c.addToFront(node)
}

func (c *lruBackend[T]) evictOldest() {
	if c.tail == nil {
		return
	}
	oldest := c.tail
	c.removeNode(oldest)
	delete(c.items, oldest.key)
	c.metrics.Evictions.Inc()
	if c.onEvict != nil {
		c.onEvict(oldest.key)
	}
}
