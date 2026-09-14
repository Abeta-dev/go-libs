// SPDX-License-Identifier: MIT
package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/umesh0492/go-libs/cache"
	"github.com/umesh0492/go-libs/clock"
)

func TestTypedCache_WithFakeClock_TTLExpiration_AllPolicies(t *testing.T) {
	policies := []struct {
		name   string
		policy cache.EvictionPolicy
	}{
		{"SampledLRU", cache.EvictionSampledLRU},
		{"LRU", cache.EvictionLRU},
		{"LFU", cache.EvictionLFU},
		{"FIFO", cache.EvictionFIFO},
	}

	for _, tc := range policies {
		t.Run(tc.name, func(t *testing.T) {
			fc := clock.NewFake()
			c := cache.NewTypedCache[string](
				cache.WithCapacity[string](10),
				cache.WithEvictionPolicy[string](tc.policy),
				cache.WithClock[string](fc),
			)
			defer c.Close()

			c.Set("k1", "v1", 10*time.Second)

			// Immediate get should succeed
			val, ok := c.Get("k1")
			if !ok || val != "v1" {
				t.Fatalf("expected k1 to be present, got %v (ok=%v)", val, ok)
			}

			// Advance by 9 seconds -> still present
			fc.Add(9 * time.Second)
			val, ok = c.Get("k1")
			if !ok || val != "v1" {
				t.Fatalf("expected k1 to still be present at 9s, got %v (ok=%v)", val, ok)
			}

			// Advance by 2 seconds (11s total > 10s TTL) -> expired
			fc.Add(2 * time.Second)
			_, ok = c.Get("k1")
			if ok {
				t.Fatal("expected k1 to be expired after 11s")
			}
		})
	}
}

func TestTypedCache_WithFakeClock_GetOrFetch_TTLExpiration(t *testing.T) {
	fc := clock.NewFake()
	c := cache.NewTypedCache[int](
		cache.WithCapacity[int](10),
		cache.WithClock[int](fc),
	)
	defer c.Close()

	fetches := 0
	fetchFn := func(ctx context.Context) (int, error) {
		fetches++
		return 42, nil
	}

	// First fetch
	val, err := c.GetOrFetch(context.Background(), "num", 5*time.Second, fetchFn)
	if err != nil || val != 42 || fetches != 1 {
		t.Fatalf("first fetch failed: val=%v, err=%v, fetches=%d", val, err, fetches)
	}

	// Advance 4s -> cached
	fc.Add(4 * time.Second)
	val, err = c.GetOrFetch(context.Background(), "num", 5*time.Second, fetchFn)
	if err != nil || val != 42 || fetches != 1 {
		t.Fatalf("second fetch should have hit cache: val=%v, err=%v, fetches=%d", val, err, fetches)
	}

	// Advance 2s (6s total > 5s TTL) -> expired, re-fetches
	fc.Add(2 * time.Second)
	val, err = c.GetOrFetch(context.Background(), "num", 5*time.Second, fetchFn)
	if err != nil || val != 42 || fetches != 2 {
		t.Fatalf("third fetch should have re-fetched: val=%v, err=%v, fetches=%d", val, err, fetches)
	}
}
