// SPDX-License-Identifier: MIT

package workerpool_test

import (
	"testing"

	"github.com/umesh0492/go-libs/workerpool"
)

func BenchmarkWorkerPool_Submit(b *testing.B) {
	pool := workerpool.New(16, b.N+100)
	defer pool.StopWait()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = pool.Submit(func() {})
	}
}

func BenchmarkWorkerPool_SubmitParallel(b *testing.B) {
	pool := workerpool.New(32, 1000000)
	defer pool.StopWait()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = pool.Submit(func() {})
		}
	})
}
