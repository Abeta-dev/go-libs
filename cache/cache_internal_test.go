// SPDX-License-Identifier: MIT

package cache

import (
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTypedCache_Internal_LockStripingDistribution(t *testing.T) {
	c := NewTypedCache[int]()

	// Verify that fnv32 hashes distribute keys across all numStripes (16)
	stripeHits := make(map[int]int)
	for i := 0; i < 200; i++ {
		key := fmt.Sprintf("key-%d", i)
		idx := int(fnv32(key) % numStripes)
		stripeHits[idx]++
	}

	assert.Equal(t, numStripes, len(stripeHits), "All 16 lock stripes must receive key assignments")
	for idx, count := range stripeHits {
		assert.Greater(t, count, 0, "Stripe %d should have at least one key assigned", idx)
	}

	// Verify getStripe returns non-nil mutex pointer within stripes array
	for i := 0; i < 50; i++ {
		mu := c.getStripe(strconv.Itoa(i))
		assert.NotNil(t, mu)
	}
}

func TestTypedCache_Internal_CounterMatchesMapCountUnderConcurrency(t *testing.T) {
	c := NewTypedCache[int](WithCapacity[int](25))
	var wg sync.WaitGroup
	workers := 25
	opsPerWorker := 200

	for w := 0; w < workers; w++ {
		wg.Add(1)
		wid := w
		go func() {
			defer wg.Done()
			for i := 0; i < opsPerWorker; i++ {
				key := strconv.Itoa((wid*11 + i) % 50)
				if (wid+i)%4 == 0 {
					c.Delete(key)
				} else {
					c.Set(key, i)
				}
			}
		}()
	}

	wg.Wait()

	// Count actual keys physically present in sync.Map
	var actualCount int
	c.data.Range(func(k, v any) bool {
		actualCount++
		return true
	})

	assert.Equal(t, actualCount, c.Len(), "c.Len() must exactly match actual keys present in sync.Map")
	assert.LessOrEqual(t, c.Len(), 25, "Cache size must be within configured capacity")
}

func TestTypedCache_Internal_EvictionOffsetAdvances(t *testing.T) {
	c := NewTypedCache[int](WithCapacity[int](10))

	// Fill to capacity
	for i := 0; i < 10; i++ {
		c.Set(strconv.Itoa(i), i)
	}

	prevOffset := c.evictOffset.Load()
	// Adding 5 more keys triggers 5 evictions
	for i := 10; i < 15; i++ {
		c.Set(strconv.Itoa(i), i)
	}

	currOffset := c.evictOffset.Load()
	assert.Greater(t, currOffset, prevOffset, "evictOffset should advance with each eviction round to sample keyspace uniformly")
}
