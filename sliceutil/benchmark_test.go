// SPDX-License-Identifier: MIT

package sliceutil_test

import (
	"strconv"
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

func BenchmarkMap(b *testing.B) {
	nums := make([]int, 100)
	for i := range nums {
		nums[i] = i
	}
	fn := func(n int) string { return strconv.Itoa(n) }
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = sliceutil.Map(nums, fn)
	}
}

func BenchmarkFilter(b *testing.B) {
	nums := make([]int, 100)
	for i := range nums {
		nums[i] = i
	}
	fn := func(n int) bool { return n%2 == 0 }
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = sliceutil.Filter(nums, fn)
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
