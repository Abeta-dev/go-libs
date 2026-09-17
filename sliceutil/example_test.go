// SPDX-License-Identifier: MIT

package sliceutil_test

import (
	"fmt"

	"github.com/umesh0492/go-libs/sliceutil"
)

func ExampleChunk() {
	nums := []int{1, 2, 3, 4, 5}
	chunks := sliceutil.Chunk(nums, 2)
	fmt.Println(chunks)
	// Output:
	// [[1 2] [3 4] [5]]
}

func ExampleGroupBy() {
	words := []string{"apple", "banana", "avocado", "blueberry"}
	grouped := sliceutil.GroupBy(words, func(s string) byte { return s[0] })
	fmt.Println(grouped['a'])
	fmt.Println(grouped['b'])
	// Output:
	// [apple avocado]
	// [banana blueberry]
}

func ExampleFlatten() {
	nested := [][]int{{1, 2}, {3, 4}, {5}}
	flat := sliceutil.Flatten(nested)
	fmt.Println(flat)
	// Output:
	// [1 2 3 4 5]
}

func ExampleFirst() {
	nums := []int{1, 3, 5, 8, 9}
	val, ok := sliceutil.First(nums, func(n int) bool { return n%2 == 0 })
	fmt.Println(val, ok)
	// Output:
	// 8 true
}

func ExampleReduce() {
	nums := []int{1, 2, 3, 4, 5}
	sum := sliceutil.Reduce(nums, 0, func(acc, v int) int { return acc + v })
	fmt.Println(sum)
	// Output:
	// 15
}

func ExampleUnique() {
	fmt.Println(sliceutil.Unique([]int{3, 1, 2, 1, 3}))
	// Output:
	// [3 1 2]
}
