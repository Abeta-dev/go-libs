// SPDX-License-Identifier: MIT

// Package maputil provides generic, zero-dependency helpers for map
// operations that are omitted from the standard library `maps` package.
package maputil

// Merge combines multiple maps into a new map. In case of key collisions,
// later maps override earlier ones.
func Merge[K comparable, V any](maps ...map[K]V) map[K]V {
	out := make(map[K]V)
	for _, m := range maps {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

// Filter returns a new map containing only key-value pairs for which
// fn returns true.
func Filter[K comparable, V any](m map[K]V, fn func(K, V) bool) map[K]V {
	out := make(map[K]V)
	for k, v := range m {
		if fn(k, v) {
			out[k] = v
		}
	}
	return out
}
