// SPDX-License-Identifier: MIT

package maputil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/umesh0492/go-libs/maputil"
)

func TestMerge(t *testing.T) {
	m1 := map[string]int{"a": 1, "b": 2}
	m2 := map[string]int{"b": 20, "c": 30}
	merged := maputil.Merge(m1, m2)
	assert.Equal(t, 1, merged["a"])
	assert.Equal(t, 20, merged["b"]) // overridden by later map
	assert.Equal(t, 30, merged["c"])
	assert.Equal(t, 0, len(maputil.Merge[string, int]()))
}

func TestFilter(t *testing.T) {
	m := map[string]int{"apple": 5, "banana": 2, "cherry": 8}
	filtered := maputil.Filter(m, func(k string, v int) bool {
		return v > 3
	})
	assert.Equal(t, map[string]int{"apple": 5, "cherry": 8}, filtered)
}
