// SPDX-License-Identifier: MIT

// Package maputil provides generic, zero-dependency helpers for map
// operations using Go generics (Go 1.18+).
package maputil

// Keys returns all keys of m in undefined order.
func Keys[K comparable, V any](m map[K]V) []K {
	if m == nil {
		return nil
	}
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Values returns all values of m in undefined order.
func Values[K comparable, V any](m map[K]V) []V {
	if m == nil {
		return nil
	}
	vals := make([]V, 0, len(m))
	for _, v := range m {
		vals = append(vals, v)
	}
	return vals
}

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
