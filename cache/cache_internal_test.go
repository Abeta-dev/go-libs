// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"testing"
	"time"
)

func TestCache_InternalBranches(t *testing.T) {
	// 1. Zero-value ensureInit for EvictionLRU, EvictionLFU, EvictionFIFO + Close()
	for _, policy := range []EvictionPolicy{EvictionLRU, EvictionLFU, EvictionFIFO} {
		tc := &TypedCache[int]{policy: policy}
		tc.Set("a", 1)
		tc.Set("b", 2)
		tc.Set("c", 3)
		if tc.Len() != 3 {
			t.Fatalf("expected Len 3 for policy %v, got %d", policy, tc.Len())
		}
		tc.Delete("b") // deletes middle node (node.prev != nil && node.next != nil)
		tc.Delete("a")
		tc.Delete("c")
		tc.Close()
	}

	// 2. NewTypedCache with empty Metrics{} and nil clock option
	tcNil := NewTypedCache[int](
		WithMetrics[int](Metrics{}),
		func(c *TypedCache[int]) { c.clock = nil },
	)
	tcNil.Set("k", 1)
	tcNil.size.Store(-5)
	if tcNil.Len() != 0 {
		t.Fatalf("expected negative size to clamp to 0 in Len(), got %d", tcNil.Len())
	}

	// 3. findFallbackEvictionCandidate & findSampledEvictionCandidate branches
	now := time.Now().UnixNano()
	tcEvict := NewTypedCache[int](WithCapacity[int](2))
	tcEvict.evictIfNeeded("noop") // capacity > 0, size == 0

	tcUnbounded := NewTypedCache[int]()
	tcUnbounded.evictIfNeeded("noop") // capacity <= 0 early return

	// Empty data with artificially high size -> targetKey == nil break
	tcEvict.size.Store(5)
	tcEvict.evictIfNeeded("k")
	tcEvict.size.Store(0)

	// findFallbackEvictionCandidate:
	// - skips newKey
	// - handles non-Item[T] entry
	// - handles oldest CreatedAt Item[T]
	// - handles expired Item[T]
	tcEvict.data.Store("newKey", Item[int]{Value: 1, CreatedAt: now})
	tcEvict.data.Store("rawVal", "not-an-item")
	tcEvict.data.Store("older", Item[int]{Value: 2, CreatedAt: now - 1000})
	cand := tcEvict.findFallbackEvictionCandidate("newKey", now)
	if cand == nil {
		t.Fatal("expected non-nil fallback candidate")
	}

	tcEvict.data.Store("expired", Item[int]{Value: 3, Expiration: now - 10, CreatedAt: now})
	expCand := tcEvict.findFallbackEvictionCandidate("newKey", now)
	if expCand != "expired" {
		t.Fatalf("expected expired key candidate, got %v", expCand)
	}

	// findSampledEvictionCandidate:
	// - stride > 1 (size >= 32)
	// - fallback when offset skips only element
	tcStride := NewTypedCache[int](WithCapacity[int](32))
	tcStride.size.Store(32)       // stride = 32 / 16 = 2
	tcStride.evictOffset.Store(0) // Add(1) % 2 == 1 -> offset = 1
	tcStride.data.Store("only-one", Item[int]{Value: 10, CreatedAt: now})
	if got := tcStride.findSampledEvictionCandidate("newKey", now); got != "only-one" {
		t.Fatalf("expected fallback to find only-one, got %v", got)
	}

	// findSampledEvictionCandidate with expired item and non-Item value
	tcSample := NewTypedCache[int](WithCapacity[int](4))
	tcSample.size.Store(2)
	tcSample.data.Store("raw", "not-an-item")
	_ = tcSample.findSampledEvictionCandidate("newKey", now)

	tcSampleExp := NewTypedCache[int](WithCapacity[int](4))
	tcSampleExp.size.Store(1)
	tcSampleExp.data.Store("expKey", Item[int]{Value: 1, Expiration: now - 5, CreatedAt: now})
	if got := tcSampleExp.findSampledEvictionCandidate("newKey", now); got != "expKey" {
		t.Fatalf("expected expKey from sampled candidate, got %v", got)
	}

	// 4. getOrFetchSampledLRU pre-cancelled ctx and post-swap over-capacity eviction
	ctxCancel, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := tcEvict.GetOrFetch(ctxCancel, "k", time.Minute, func(ctx context.Context) (int, error) {
		return 1, nil
	}); err == nil {
		t.Fatal("expected error for cancelled context in getOrFetchSampledLRU")
	}

	tcCap1 := NewTypedCache[int](WithCapacity[int](1))
	tcCap1.data.Store("existing2", Item[int]{Value: 1, Expiration: now - 100, CreatedAt: now - 200})
	tcCap1.data.Store("extra", Item[int]{Value: 99, CreatedAt: now - 100})
	tcCap1.size.Store(2) // existing2 exists (expired), so pre-swap evictIfNeeded is skipped and post-swap evictIfNeeded runs
	_, err := tcCap1.GetOrFetch(context.Background(), "existing2", time.Minute, func(ctx context.Context) (int, error) {
		return 2, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 5. Direct backend constructors with <=0 capacity, nil clock, nil metrics, empty evictions, and GetOrFetch branches
	bFIFO := newFIFOBackend[int](0, Metrics{}, nil)
	bFIFO.evictOldest() // tail == nil
	bFIFO.Set("k1", 10, time.Minute)
	if v, err := bFIFO.GetOrFetch(context.Background(), "k1", time.Minute, func(ctx context.Context) (int, error) {
		return 99, nil
	}); err != nil || v != 10 {
		t.Fatalf("expected FIFO GetOrFetch cache hit 10, got %v, %v", v, err)
	}
	if _, err := bFIFO.GetOrFetch(ctxCancel, "k2", time.Minute, func(ctx context.Context) (int, error) {
		return 99, nil
	}); err == nil {
		t.Fatal("expected error on pre-cancelled ctx in FIFO GetOrFetch")
	}
	ctxFIFOCancel, cancelFIFO := context.WithCancel(context.Background())
	_, err = bFIFO.GetOrFetch(ctxFIFOCancel, "k-slow", time.Minute, func(ctx context.Context) (int, error) {
		cancelFIFO()
		time.Sleep(15 * time.Millisecond)
		return 1, nil
	})
	if err == nil {
		t.Fatal("expected context cancel error during FIFO GetOrFetch")
	}

	bLFU := newLFUBackend[int](0, Metrics{}, nil)
	bLFU.evictMinFreq() // empty bucket
	var emptyList lfuList[int]
	if emptyList.popTail() != nil {
		t.Fatal("expected nil from empty lfuList.popTail()")
	}
	bLFU.Set("k1", 10, time.Minute)
	if v, err := bLFU.GetOrFetch(context.Background(), "k1", time.Minute, func(ctx context.Context) (int, error) {
		return 99, nil
	}); err != nil || v != 10 {
		t.Fatalf("expected LFU GetOrFetch cache hit 10, got %v, %v", v, err)
	}
	if _, err := bLFU.GetOrFetch(ctxCancel, "k2", time.Minute, func(ctx context.Context) (int, error) {
		return 99, nil
	}); err == nil {
		t.Fatal("expected error on pre-cancelled ctx in LFU GetOrFetch")
	}
	ctxLFUCancel, cancelLFU := context.WithCancel(context.Background())
	_, err = bLFU.GetOrFetch(ctxLFUCancel, "k-slow", time.Minute, func(ctx context.Context) (int, error) {
		cancelLFU()
		time.Sleep(15 * time.Millisecond)
		return 1, nil
	})
	if err == nil {
		t.Fatal("expected context cancel error during LFU GetOrFetch")
	}

	bLRU := newLRUBackend[int](0, Metrics{}, nil)
	bLRU.evictOldest() // tail == nil
	bLRU.Set("k1", 10, time.Minute)
	if v, err := bLRU.GetOrFetch(context.Background(), "k1", time.Minute, func(ctx context.Context) (int, error) {
		return 99, nil
	}); err != nil || v != 10 {
		t.Fatalf("expected LRU GetOrFetch cache hit 10, got %v, %v", v, err)
	}
	if _, err := bLRU.GetOrFetch(ctxCancel, "k2", time.Minute, func(ctx context.Context) (int, error) {
		return 99, nil
	}); err == nil {
		t.Fatal("expected error on pre-cancelled ctx in LRU GetOrFetch")
	}
	ctxLRUCancel, cancelLRU := context.WithCancel(context.Background())
	_, err = bLRU.GetOrFetch(ctxLRUCancel, "k-slow", time.Minute, func(ctx context.Context) (int, error) {
		cancelLRU()
		time.Sleep(15 * time.Millisecond)
		return 1, nil
	})
	if err == nil {
		t.Fatal("expected context cancel error during LRU GetOrFetch")
	}
}
