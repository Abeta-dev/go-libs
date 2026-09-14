// SPDX-License-Identifier: MIT

package ratelimit

import (
	"testing"
	"time"
)

func BenchmarkTokenBucket_Allow(b *testing.B) {
	limiter := NewTokenBucket(1_000_000, time.Nanosecond)
	key := "bench-tb"
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = limiter.Allow(key)
	}
}

func BenchmarkSlidingWindow_Allow(b *testing.B) {
	limiter := NewSlidingWindow(1_000_000, time.Minute)
	key := "bench-sw"
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = limiter.Allow(key)
	}
}
