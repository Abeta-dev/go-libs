// SPDX-License-Identifier: MIT

package sliceutil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/umesh0492/go-libs/sliceutil"
)

func TestReduce(t *testing.T) {
	sum := sliceutil.Reduce([]int{1, 2, 3, 4}, 0, func(acc, v int) int { return acc + v })
	assert.Equal(t, 10, sum)
	assert.Equal(t, 0, sliceutil.Reduce([]int{}, 0, func(acc, v int) int { return acc + v }))
}

func TestGroupBy(t *testing.T) {
	groups := sliceutil.GroupBy([]string{"a", "bb", "cc", "ddd"}, func(s string) int { return len(s) })
	assert.Equal(t, []string{"a"}, groups[1])
	assert.ElementsMatch(t, []string{"bb", "cc"}, groups[2])
	assert.Equal(t, []string{"ddd"}, groups[3])
}

func TestChunk(t *testing.T) {
	chunks := sliceutil.Chunk([]int{1, 2, 3, 4, 5}, 2)
	assert.Equal(t, [][]int{{1, 2}, {3, 4}, {5}}, chunks)
	assert.Equal(t, [][]int{{1, 2, 3}}, sliceutil.Chunk([]int{1, 2, 3}, 5))
	assert.Nil(t, sliceutil.Chunk([]int{}, 2))
	assert.Nil(t, sliceutil.Chunk([]int{1, 2}, 0))
	assert.Nil(t, sliceutil.Chunk([]int{1, 2}, -1))
}

func TestUnique(t *testing.T) {
	assert.Equal(t, []int{1, 2, 3}, sliceutil.Unique([]int{1, 2, 2, 3, 1}))
	assert.Equal(t, []int{}, sliceutil.Unique([]int{}))
}

func TestFlatten(t *testing.T) {
	result := sliceutil.Flatten([][]int{{1, 2}, {3}, {4, 5}})
	assert.Equal(t, []int{1, 2, 3, 4, 5}, result)
	assert.Equal(t, []int{}, sliceutil.Flatten([][]int{}))
}

func TestFirst(t *testing.T) {
	val, ok := sliceutil.First([]int{1, 2, 3, 4}, func(v int) bool { return v > 2 })
	assert.True(t, ok)
	assert.Equal(t, 3, val)

	_, ok = sliceutil.First([]int{1, 2}, func(v int) bool { return v > 10 })
	assert.False(t, ok)

	_, ok = sliceutil.First([]int{}, func(v int) bool { return true })
	assert.False(t, ok)
}
