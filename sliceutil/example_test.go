// SPDX-License-Identifier: MIT

package sliceutil_test

import (
	"fmt"
	"strings"

	"github.com/umesh0492/go-libs/sliceutil"
)

func ExampleMap() {
	names := []string{"alice", "bob", "carol"}
	upper := sliceutil.Map(names, strings.ToUpper)
	fmt.Println(upper)
	// Output:
	// [ALICE BOB CAROL]
}

func ExampleFilter() {
	nums := []int{1, 2, 3, 4, 5, 6}
	evens := sliceutil.Filter(nums, func(n int) bool { return n%2 == 0 })
	fmt.Println(evens)
	// Output:
	// [2 4 6]
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
