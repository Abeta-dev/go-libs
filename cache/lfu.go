// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"sync"
	"time"

	"github.com/umesh0492/go-libs/clock"
	"github.com/umesh0492/go-libs/metrics"
	"golang.org/x/sync/singleflight"
)

type lfuNode[T any] struct {
	key        string
	value      T
	freq       int
	expiration int64
	prev       *lfuNode[T]
	next       *lfuNode[T]
}

type lfuList[T any] struct {
	head *lfuNode[T]
	tail *lfuNode[T]
	len  int
}

func (l *lfuList[T]) pushFront(node *lfuNode[T]) {
	node.prev = nil
	node.next = l.head
	if l.head != nil {
		l.head.prev = node
	}
	l.head = node
	if l.tail == nil {
		l.tail = node
	}
	l.len++
}

func (l *lfuList[T]) remove(node *lfuNode[T]) {
	if node.prev != nil {
		node.prev.next = node.next
	} else {
		l.head = node.next
	}

	if node.next != nil {
		node.next.prev = node.prev
	} else {
		l.tail = node.prev
	}

	node.prev = nil
	node.next = nil
	l.len--
}

func (l *lfuList[T]) popTail() *lfuNode[T] {
	if l.tail == nil {
		return nil
	}
	t := l.tail
	l.remove(t)
	return t
}

// lfuBackend provides an in-memory, thread-safe cache with Least-Frequently-Used eviction.
// Access frequencies are tracked in O(1) time complexity with recency tie-breaking.
type lfuBackend[T any] struct {
	mu          sync.Mutex
	capacity    int
	minFreq     int
	items       map[string]*lfuNode[T]
	freqBuckets map[int]*lfuList[T]
	metrics     Metrics
	sf          singleflight.Group
	clock       clock.Clock
}

func newLFUBackend[T any](capacity int, m Metrics, clk clock.Clock) *lfuBackend[T] {
	if capacity <= 0 {
		capacity = 128
	}
	if clk == nil {
		clk = clock.NewReal()
	}
	c := &lfuBackend[T]{
		capacity:    capacity,
		minFreq:     0,
		items:       make(map[string]*lfuNode[T], capacity),
		freqBuckets: make(map[int]*lfuList[T]),
		metrics:     m,
		clock:       clk,
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

// Get retrieves an item, increments its access frequency, and returns true if found.
func (c *lfuBackend[T]) Get(key string) (T, bool) {
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
		var zero T
		return zero, false
	}

	c.incrementFreq(node)
	c.metrics.Hits.Inc()
	return node.value, true
}

// Set inserts or updates an item in the cache, evicting the least frequently used item if capacity is reached.
func (c *lfuBackend[T]) Set(key string, value T, ttl ...time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var exp int64
	if len(ttl) > 0 && ttl[0] > 0 {
		exp = c.clock.Now().Add(ttl[0]).UnixNano()
	}

	if node, exists := c.items[key]; exists {
		node.value = value
		node.expiration = exp
		c.incrementFreq(node)
		return
	}

	if len(c.items) >= c.capacity {
		c.evictMinFreq()
	}

	newNode := &lfuNode[T]{
		key:        key,
		value:      value,
		freq:       1,
		expiration: exp,
	}
	c.items[key] = newNode
	c.getBucket(1).pushFront(newNode)
	c.minFreq = 1
}

// GetOrFetch retrieves an item or executes fetchFn under singleflight stampede protection.
func (c *lfuBackend[T]) GetOrFetch(ctx context.Context, key string, ttl time.Duration, fetchFn func(context.Context) (T, error)) (T, error) {
	if err := ctx.Err(); err != nil {
		var zero T
		return zero, err
	}

	if val, ok := c.Get(key); ok {
		return val, nil
	}

	ch := c.sf.DoChan(key, func() (any, error) {
		bgCtx := context.WithoutCancel(ctx)
		newValue, fetchErr := fetchFn(bgCtx)
		if fetchErr != nil {
			var zero T
			return zero, fetchErr
		}
		c.Set(key, newValue, ttl)
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

// Delete removes an item from the cache.
func (c *lfuBackend[T]) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if node, exists := c.items[key]; exists {
		c.removeNode(node)
		delete(c.items, key)
	}
}

// Len returns the number of items stored in the cache.
func (c *lfuBackend[T]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

func (c *lfuBackend[T]) getBucket(freq int) *lfuList[T] {
	bucket, ok := c.freqBuckets[freq]
	if !ok {
		bucket = &lfuList[T]{}
		c.freqBuckets[freq] = bucket
	}
	return bucket
}

func (c *lfuBackend[T]) incrementFreq(node *lfuNode[T]) {
	oldBucket := c.freqBuckets[node.freq]
	if oldBucket != nil {
		oldBucket.remove(node)
		if node.freq == c.minFreq && oldBucket.len == 0 {
			c.minFreq++
		}
	}
	node.freq++
	c.getBucket(node.freq).pushFront(node)
}

func (c *lfuBackend[T]) removeNode(node *lfuNode[T]) {
	bucket := c.freqBuckets[node.freq]
	if bucket != nil {
		bucket.remove(node)
		if node.freq == c.minFreq && bucket.len == 0 {
			c.minFreq++
		}
	}
}

func (c *lfuBackend[T]) evictMinFreq() {
	bucket := c.freqBuckets[c.minFreq]
	if bucket == nil || bucket.len == 0 {
		return
	}
	victim := bucket.popTail()
	if victim != nil {
		delete(c.items, victim.key)
		c.metrics.Evictions.Inc()
	}
}

func (c *lfuBackend[T]) Close() {}
