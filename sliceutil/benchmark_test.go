// SPDX-License-Identifier: MIT

package sliceutil_test

import (
	"testing"

	"github.com/umesh0492/go-libs/sliceutil"
)

func BenchmarkFirst(b *testing.B) {
	nums := []int{1, 3, 5, 7, 9, 10, 11, 13}
	pred := func(n int) bool { return n%2 == 0 }
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = sliceutil.First(nums, pred)
	}
}

func BenchmarkReduce(b *testing.B) {
	nums := make([]int, 100)
	for i := range nums {
		nums[i] = i
	}
	fn := func(acc, v int) int { return acc + v }
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = sliceutil.Reduce(nums, 0, fn)
	}
}

func BenchmarkGroupBy(b *testing.B) {
	nums := make([]int, 100)
	for i := range nums {
		nums[i] = i
	}
	fn := func(n int) int { return n % 5 }
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = sliceutil.GroupBy(nums, fn)
	}
}

func BenchmarkFlatten(b *testing.B) {
	chunks := make([][]int, 10)
	for i := range chunks {
		chunks[i] = make([]int, 10)
		for j := range chunks[i] {
			chunks[i][j] = i*10 + j
		}
	}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = sliceutil.Flatten(chunks)
	}
}

func BenchmarkChunk(b *testing.B) {
	nums := make([]int, 100)
	for i := range nums {
		nums[i] = i
	}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = sliceutil.Chunk(nums, 10)
	}
}

func BenchmarkUnique(b *testing.B) {
	nums := []int{1, 2, 2, 3, 4, 4, 4, 5, 6, 7, 7, 8, 9, 10, 10}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = sliceutil.Unique(nums)
	}
}
