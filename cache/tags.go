// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"strings"
	"time"
)

// FetchStatus describes how a GetOrFetchWithStatus or GetOrFetchWithTags call was satisfied.
type FetchStatus string

const (
	// FetchStatusHit indicates the entry was returned directly from the cache.
	FetchStatusHit FetchStatus = "HIT"
	// FetchStatusMiss indicates this caller executed fetchFn to populate the cache.
	FetchStatusMiss FetchStatus = "MISS"
	// FetchStatusCoalesced indicates this caller waited on and shared an in-flight
	// singleflight fetch initiated by another goroutine.
	FetchStatusCoalesced FetchStatus = "COALESCED"
)

// String returns the canonical string representation of FetchStatus ("HIT", "MISS", or "COALESCED").
func (s FetchStatus) String() string {
	return string(s)
}

// InvalidatingCache extends Cache with prefix/tag invalidation and fetch status reporting.
type InvalidatingCache[T any] interface {
	Cache[T]
	// GetOrFetchWithStatus behaves like GetOrFetch and also returns FetchStatus
	// (HIT, MISS, or COALESCED) when err is nil.
	GetOrFetchWithStatus(ctx context.Context, key string, ttl time.Duration, fetchFn func(context.Context) (T, error)) (T, FetchStatus, error)
	// GetOrFetchWithTags behaves like GetOrFetchWithStatus and associates tags
	// with key when fetchFn populates the cache.
	GetOrFetchWithTags(ctx context.Context, key string, ttl time.Duration, tags []string, fetchFn func(context.Context) (T, error)) (T, FetchStatus, error)
	// DeletePrefix removes all entries whose keys start with prefix and returns the count removed.
	DeletePrefix(prefix string) int
	// SetWithTags stores an item with the given TTL (<= 0 means no expiration) and associates tags with key.
	SetWithTags(key string, value T, ttl time.Duration, tags ...string)
	// InvalidateTag removes all entries associated with tag and returns the count removed.
	InvalidateTag(tag string) int
	// InvalidateTags removes all entries associated with any of the given tags and returns the unique count removed.
	InvalidateTags(tags ...string) int
}

// Compile-time check that TypedCache satisfies InvalidatingCache.
var _ InvalidatingCache[any] = (*TypedCache[any])(nil)

// GetOrFetchWithStatus attempts to get a value from cache or fetches it under
// singleflight stampede protection, returning FetchStatusHit, FetchStatusMiss,
// or FetchStatusCoalesced when err is nil.
func (c *TypedCache[T]) GetOrFetchWithStatus(ctx context.Context, key string, ttl time.Duration, fetchFn func(context.Context) (T, error)) (T, FetchStatus, error) {
	return c.GetOrFetchWithTags(ctx, key, ttl, nil, fetchFn)
}

// GetOrFetchWithTags behaves like GetOrFetchWithStatus and associates the given
// invalidation tags with key when fetchFn populates the cache.
func (c *TypedCache[T]) GetOrFetchWithTags(ctx context.Context, key string, ttl time.Duration, tags []string, fetchFn func(context.Context) (T, error)) (T, FetchStatus, error) {
	c.ensureInit()
	onStore := func() {
		c.replaceKeyTags(key, tags)
	}
	switch c.policy {
	case EvictionLRU:
		return c.lru.GetOrFetchWithStatus(ctx, key, ttl, onStore, fetchFn)
	case EvictionLFU:
		return c.lfu.GetOrFetchWithStatus(ctx, key, ttl, onStore, fetchFn)
	case EvictionFIFO:
		return c.fifo.GetOrFetchWithStatus(ctx, key, ttl, onStore, fetchFn)
	default:
		return c.getOrFetchSampledLRU(ctx, key, ttl, tags, fetchFn)
	}
}

// SetWithTags stores an item with the given TTL (<= 0 means no expiration) and
// associates the provided invalidation tags with key.
func (c *TypedCache[T]) SetWithTags(key string, value T, ttl time.Duration, tags ...string) {
	if ttl > 0 {
		c.setValue(key, value, ttl)
	} else {
		c.setValue(key, value)
	}
	c.replaceKeyTags(key, tags)
}

// DeletePrefix removes every entry whose key starts with prefix across any
// configured eviction policy and returns the number of removed entries.
func (c *TypedCache[T]) DeletePrefix(prefix string) int {
	c.ensureInit()
	var deleted []string
	switch c.policy {
	case EvictionLRU:
		deleted = c.lru.DeletePrefix(prefix)
	case EvictionLFU:
		deleted = c.lfu.DeletePrefix(prefix)
	case EvictionFIFO:
		deleted = c.fifo.DeletePrefix(prefix)
	default:
		deleted = c.deleteSampledLRUPrefix(prefix)
	}
	for _, k := range deleted {
		c.removeKeyTags(k)
	}
	return len(deleted)
}

func (c *TypedCache[T]) deleteSampledLRUPrefix(prefix string) []string {
	var deleted []string
	c.data.Range(func(k, _ any) bool {
		keyStr, ok := k.(string)
		if !ok || !strings.HasPrefix(keyStr, prefix) {
			return true
		}
		if c.capacity > 0 {
			stripe := c.getStripe(keyStr)
			stripe.Lock()
			if _, loaded := c.data.LoadAndDelete(keyStr); loaded {
				c.size.Add(-1)
				deleted = append(deleted, keyStr)
			}
			stripe.Unlock()
		} else if _, loaded := c.data.LoadAndDelete(keyStr); loaded {
			c.size.Add(-1)
			deleted = append(deleted, keyStr)
		}
		return true
	})
	return deleted
}

// InvalidateTag removes all entries associated with tag and returns the number
// of active cache entries deleted.
func (c *TypedCache[T]) InvalidateTag(tag string) int {
	return c.InvalidateTags(tag)
}

// InvalidateTags removes all entries associated with any of the provided tags
// and returns the number of unique active cache entries deleted.
func (c *TypedCache[T]) InvalidateTags(tags ...string) int {
	c.ensureInit()
	keys := c.popKeysForTags(tags)
	removed := 0
	for _, k := range keys {
		if c.deleteKey(k) {
			removed++
		}
	}
	return removed
}

func (c *TypedCache[T]) replaceKeyTags(key string, tags []string) {
	c.tagMu.Lock()
	defer c.tagMu.Unlock()

	c.removeKeyTagsLocked(key)
	if len(tags) == 0 {
		return
	}
	if c.tagToKeys == nil {
		c.tagToKeys = make(map[string]map[string]struct{})
		c.keyToTags = make(map[string]map[string]struct{})
	}
	var keySet map[string]struct{}
	for _, tag := range tags {
		if tag == "" {
			continue
		}
		if keySet == nil {
			keySet = make(map[string]struct{}, len(tags))
			c.keyToTags[key] = keySet
		}
		keySet[tag] = struct{}{}
		ks, ok := c.tagToKeys[tag]
		if !ok {
			ks = make(map[string]struct{})
			c.tagToKeys[tag] = ks
		}
		ks[key] = struct{}{}
	}
}

func (c *TypedCache[T]) removeKeyTags(key string) {
	c.tagMu.Lock()
	c.removeKeyTagsLocked(key)
	c.tagMu.Unlock()
}

func (c *TypedCache[T]) removeKeyTagsLocked(key string) {
	tags, ok := c.keyToTags[key]
	if !ok {
		return
	}
	for tag := range tags {
		if ks, exists := c.tagToKeys[tag]; exists {
			delete(ks, key)
			if len(ks) == 0 {
				delete(c.tagToKeys, tag)
			}
		}
	}
	delete(c.keyToTags, key)
}

func (c *TypedCache[T]) popKeysForTags(tags []string) []string {
	c.tagMu.Lock()
	defer c.tagMu.Unlock()

	if len(tags) == 0 || len(c.tagToKeys) == 0 {
		return nil
	}
	unique := make(map[string]struct{})
	for _, tag := range tags {
		if ks, ok := c.tagToKeys[tag]; ok {
			for k := range ks {
				unique[k] = struct{}{}
			}
		}
	}
	keys := make([]string, 0, len(unique))
	for k := range unique {
		c.removeKeyTagsLocked(k)
		keys = append(keys, k)
	}
	return keys
}
