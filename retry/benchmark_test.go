// SPDX-License-Identifier: MIT

package retry_test

import (
	"context"
	"testing"
	"time"

	"github.com/umesh0492/go-libs/retry"
)

func BenchmarkRetry_SuccessImmediate(b *testing.B) {
	ctx := context.Background()
	cfg := retry.Config{
		Attempts:    3,
		InitialWait: 10 * time.Millisecond,
		Strategy:    retry.ExponentialJitter,
	}
	op := func(ctx context.Context) error {
		return nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = retry.Do(ctx, cfg, op)
	}
}

func BenchmarkRetry_SuccessImmediate_Parallel(b *testing.B) {
	ctx := context.Background()
	cfg := retry.Config{
		Attempts:    3,
		InitialWait: 10 * time.Millisecond,
		Strategy:    retry.ExponentialJitter,
	}
	op := func(ctx context.Context) error {
		return nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = retry.Do(ctx, cfg, op)
		}
	})
}

func BenchmarkRetry_DoWithResult(b *testing.B) {
	ctx := context.Background()
	cfg := retry.Config{
		Attempts:    3,
		InitialWait: 10 * time.Millisecond,
		Strategy:    retry.ExponentialJitter,
	}
	op := func(ctx context.Context) (int, error) {
		return 42, nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = retry.DoWithResult(ctx, cfg, op)
	}
}
