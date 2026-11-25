// SPDX-License-Identifier: MIT

package cache_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/abeta-dev/go-libs/cache"
	"github.com/abeta-dev/go-libs/clock"
)

func TestTypedCache_GetOrFetchWithStatus_AllPolicies(t *testing.T) {
	policies := []struct {
		name   string
		policy cache.EvictionPolicy
	}{
		{"SampledLRU", cache.EvictionSampledLRU},
		{"LRU", cache.EvictionLRU},
		{"LFU", cache.EvictionLFU},
		{"FIFO", cache.EvictionFIFO},
	}

	if cache.FetchStatusHit.String() != "HIT" ||
		cache.FetchStatusMiss.String() != "MISS" ||
		cache.FetchStatusCoalesced.String() != "COALESCED" {
		t.Fatalf("unexpected FetchStatus string values")
	}

	for _, tc := range policies {
		t.Run(tc.name, func(t *testing.T) {
			c := cache.NewTypedCache[string](
				cache.WithCapacity[string](16),
				cache.WithEvictionPolicy[string](tc.policy),
			)
			defer c.Close()

			// 1. Concurrent stampede -> exactly 1 MISS + (N-1) COALESCED
			const callers = 8
			entered := make(chan struct{})
			release := make(chan struct{})
			var fetches atomic.Int32
			var misses atomic.Int32
			var coalesced atomic.Int32
			var wg sync.WaitGroup

			// Launch leader first so it deterministically owns the singleflight flight
			wg.Add(1)
			go func() {
				defer wg.Done()
				v, st, err := c.GetOrFetchWithTags(
					context.Background(),
					"screen:people:u1",
					time.Minute,
					[]string{"screen:people", "tenant:t1"},
					func(_ context.Context) (string, error) {
						fetches.Add(1)
						close(entered)
						<-release
						return "payload-v1", nil
					},
				)
				if err != nil || v != "payload-v1" {
					t.Errorf("leader failed: v=%q err=%v", v, err)
					return
				}
				switch st {
				case cache.FetchStatusMiss:
					misses.Add(1)
				case cache.FetchStatusCoalesced:
					coalesced.Add(1)
				default:
					t.Errorf("unexpected leader status: %q", st)
				}
			}()

			<-entered

			// Launch followers while singleflight is in flight
			for i := 1; i < callers; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					v, st, err := c.GetOrFetchWithStatus(
						context.Background(),
						"screen:people:u1",
						time.Minute,
						func(_ context.Context) (string, error) {
							fetches.Add(1)
							return "should-not-run", nil
						},
					)
					if err != nil || v != "payload-v1" {
						t.Errorf("follower failed: v=%q err=%v", v, err)
						return
					}
					if st == cache.FetchStatusCoalesced {
						coalesced.Add(1)
					} else {
						t.Errorf("expected COALESCED for follower, got %q", st)
					}
				}()
			}

			// Give followers a brief window to subscribe to singleflight DoChan before releasing
			time.Sleep(15 * time.Millisecond)
			close(release)
			wg.Wait()

			if fetches.Load() != 1 {
				t.Fatalf("expected 1 fetch, got %d", fetches.Load())
			}
			if misses.Load() != 1 || coalesced.Load() != callers-1 {
				t.Fatalf("expected 1 MISS and %d COALESCED, got misses=%d coalesced=%d", callers-1, misses.Load(), coalesced.Load())
			}

			// 2. Subsequent call -> HIT
			val, status, err := c.GetOrFetchWithStatus(context.Background(), "screen:people:u1", time.Minute, func(_ context.Context) (string, error) {
				return "unexpected", nil
			})
			if err != nil || val != "payload-v1" || status != cache.FetchStatusHit {
				t.Fatalf("expected HIT payload-v1, got val=%q status=%q err=%v", val, status, err)
			}

			// 3. Error fetch returns empty status and error
			wantErr := errors.New("upstream down")
			_, errStatus, err := c.GetOrFetchWithStatus(context.Background(), "err-key", time.Minute, func(_ context.Context) (string, error) {
				return "", wantErr
			})
			if !errors.Is(err, wantErr) || errStatus != "" {
				t.Fatalf("expected wantErr and empty status, got err=%v status=%q", err, errStatus)
			}

			// 4. Tag invalidation from GetOrFetchWithTags removes the cached item
			if removed := c.InvalidateTag("screen:people"); removed != 1 {
				t.Fatalf("expected 1 removed by tag screen:people, got %d", removed)
			}
			if _, ok := c.Get("screen:people:u1"); ok {
				t.Fatalf("expected screen:people:u1 to be invalidated")
			}
		})
	}
}

func TestTypedCache_DeletePrefixAndTags_AllPolicies(t *testing.T) {
	policies := []struct {
		name     string
		policy   cache.EvictionPolicy
		capacity int
	}{
		{"SampledLRU_Unbounded", cache.EvictionSampledLRU, 0},
		{"SampledLRU_Bounded", cache.EvictionSampledLRU, 8},
		{"LRU", cache.EvictionLRU, 8},
		{"LFU", cache.EvictionLFU, 8},
		{"FIFO", cache.EvictionFIFO, 8},
	}

	for _, tc := range policies {
		t.Run(tc.name, func(t *testing.T) {
			fc := clock.NewFake()
			c := cache.NewTypedCache[int](
				cache.WithCapacity[int](tc.capacity),
				cache.WithEvictionPolicy[int](tc.policy),
				cache.WithClock[int](fc),
			)
			defer c.Close()

			// Empty tag invalidation on fresh cache -> 0; deleting non-existent key is a safe no-op
			c.Delete("non-existent-key")
			if n := c.InvalidateTag("missing"); n != 0 {
				t.Fatalf("expected 0 on empty cache, got %d", n)
			}

			// SetWithTags (with TTL and without TTL, including empty tag ignored)
			c.SetWithTags("screen:people:1", 1, time.Minute, "screen:people", "tenant:a", "")
			c.SetWithTags("screen:people:2", 2, 0, "screen:people", "tenant:a")
			c.SetWithTags("screen:deals:1", 3, time.Minute, "screen:deals", "tenant:a")
			c.SetWithTags("screen:deals:2", 4, time.Minute, "screen:deals", "tenant:b")
			c.Set("other:1", 5)

			// DeletePrefix removes matching keys only
			if removed := c.DeletePrefix("screen:people:"); removed != 2 {
				t.Fatalf("expected 2 removed for prefix screen:people:, got %d", removed)
			}
			if _, ok := c.Get("screen:people:1"); ok {
				t.Fatalf("expected screen:people:1 deleted")
			}
			if _, ok := c.Get("screen:deals:1"); !ok {
				t.Fatalf("expected screen:deals:1 intact")
			}

			// Overwrite tagged key with untagged Set clears old tags
			c.Set("screen:deals:2", 44)
			if removed := c.InvalidateTag("tenant:b"); removed != 0 {
				t.Fatalf("expected 0 removed after untagged overwrite, got %d", removed)
			}

			// Re-tag and invalidate multiple tags at once
			c.SetWithTags("screen:deals:2", 44, time.Minute, "screen:deals", "tenant:b")
			if removed := c.InvalidateTags("screen:deals", "tenant:a"); removed != 2 {
				t.Fatalf("expected 2 unique keys removed, got %d", removed)
			}
			if c.Len() != 1 {
				t.Fatalf("expected only other:1 left, got Len=%d", c.Len())
			}

			// Test expiration and capacity eviction tag cleanup paths
			c.SetWithTags("exp:1", 10, 5*time.Second, "ephemeral")
			fc.Add(10 * time.Second)
			if _, ok := c.Get("exp:1"); ok {
				t.Fatalf("expected exp:1 expired")
			}
			if removed := c.InvalidateTag("ephemeral"); removed != 0 {
				t.Fatalf("expected expired key tag cleaned up, got removed=%d", removed)
			}

			if tc.capacity > 0 {
				// Fill beyond capacity to exercise onEvict tag cleanup
				for i := 0; i < tc.capacity+3; i++ {
					key := "cap:" + string(rune('a'+i))
					c.SetWithTags(key, i, time.Minute, "cap-tag")
				}
				if c.Len() > tc.capacity {
					t.Fatalf("Len=%d exceeds capacity %d", c.Len(), tc.capacity)
				}
			}
		})
	}
}
