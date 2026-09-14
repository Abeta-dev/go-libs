// SPDX-License-Identifier: MIT

package cache_test

import (
	"strconv"
	"testing"

	"github.com/umesh0492/go-libs/cache"
)

func BenchmarkTTL_GetSet(b *testing.B) {
	c := cache.NewTypedCache[int]()
	for i := 0; i < 1000; i++ {
		c.Set(strconv.Itoa(i), i)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := strconv.Itoa(i % 1000)
		_, _ = c.Get(key)
	}
}

func BenchmarkLRU_GetSet(b *testing.B) {
	lru := cache.NewTypedCache[int](cache.WithCapacity[int](1000), cache.WithEvictionPolicy[int](cache.EvictionLRU))
	for i := 0; i < 1000; i++ {
		lru.Set(strconv.Itoa(i), i)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := strconv.Itoa(i % 1000)
		_, _ = lru.Get(key)
	}
}

func BenchmarkLFU_GetSet(b *testing.B) {
	lfu := cache.NewTypedCache[int](cache.WithCapacity[int](1000), cache.WithEvictionPolicy[int](cache.EvictionLFU))
	for i := 0; i < 1000; i++ {
		lfu.Set(strconv.Itoa(i), i)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := strconv.Itoa(i % 1000)
		_, _ = lfu.Get(key)
	}
}

func BenchmarkFIFO_GetSet(b *testing.B) {
	fifo := cache.NewTypedCache[int](cache.WithCapacity[int](1000), cache.WithEvictionPolicy[int](cache.EvictionFIFO))
	for i := 0; i < 1000; i++ {
		fifo.Set(strconv.Itoa(i), i)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := strconv.Itoa(i % 1000)
		_, _ = fifo.Get(key)
	}
}

func BenchmarkCache_CapacityEviction_Concurrent(b *testing.B) {
	const capLimit = 100
	c := cache.NewTypedCache[int](cache.WithCapacity[int](capLimit))

	// Pre-fill cache to capacity limit
	for i := 0; i < capLimit; i++ {
		c.Set(strconv.Itoa(i), i)
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		var i int
		for pb.Next() {
			key := strconv.Itoa(i)
			c.Set(key, i)
			i++
		}
	})
}
