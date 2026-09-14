// SPDX-License-Identifier: MIT

package circuitbreaker_test

import (
	"context"
	"testing"
	"time"

	"github.com/umesh0492/go-libs/circuitbreaker"
)

func BenchmarkCircuitBreaker_Execute_Closed(b *testing.B) {
	cb := circuitbreaker.NewConsecutiveBreaker(5, time.Minute)
	ctx := context.Background()
	noop := func() error { return nil }

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = cb.Execute(ctx, noop)
	}
}

func BenchmarkCircuitBreaker_Execute_Parallel(b *testing.B) {
	cb := circuitbreaker.NewConsecutiveBreaker(5, time.Minute)
	ctx := context.Background()
	noop := func() error { return nil }

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = cb.Execute(ctx, noop)
		}
	})
}

func BenchmarkRatioBreaker_Execute_Closed(b *testing.B) {
	rb := circuitbreaker.NewRatioBreaker(0.5, 10, time.Minute, time.Minute)
	ctx := context.Background()
	noop := func() error { return nil }

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = rb.Execute(ctx, noop)
	}
}

func BenchmarkRatioBreaker_Execute_Parallel(b *testing.B) {
	rb := circuitbreaker.NewRatioBreaker(0.5, 10, time.Minute, time.Minute)
	ctx := context.Background()
	noop := func() error { return nil }

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = rb.Execute(ctx, noop)
		}
	})
}
