// SPDX-License-Identifier: MIT

// Package sliceutil provides generic, zero-dependency algorithmic helpers for slice
// operations that are intentionally omitted from the standard library `slices` package.
package sliceutil

// Reduce accumulates a result by applying fn to each element in s,
// starting from init.
func Reduce[T, U any](s []T, init U, fn func(U, T) U) U {
	acc := init
	for _, v := range s {
		acc = fn(acc, v)
	}
	return acc
}

// GroupBy partitions s into a map of slices keyed by the result of fn.
func GroupBy[T any, K comparable](s []T, fn func(T) K) map[K][]T {
	out := make(map[K][]T)
	for _, v := range s {
		k := fn(v)
		out[k] = append(out[k], v)
	}
	return out
}

// Chunk splits s into consecutive sub-slices of at most size elements.
// If size <= 0, Chunk returns nil.
func Chunk[T any](s []T, size int) [][]T {
	if size <= 0 {
		return nil
	}
	if len(s) == 0 {
		return nil
	}
	chunks := make([][]T, 0, (len(s)+size-1)/size)
	for len(s) > 0 {
		end := size
		if end > len(s) {
			end = len(s)
		}
		chunks = append(chunks, s[:end])
		s = s[end:]
	}
	return chunks
}

// Unique returns a new slice with duplicate comparable elements removed,
// preserving first-occurrence order.
func Unique[T comparable](s []T) []T {
	seen := make(map[T]struct{}, len(s))
	out := make([]T, 0, len(s))
	for _, v := range s {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	return out
}

// Flatten concatenates a slice of slices into a single slice.
func Flatten[T any](slices [][]T) []T {
	total := 0
	for _, s := range slices {
		total += len(s)
	}
	out := make([]T, 0, total)
	for _, s := range slices {
		out = append(out, s...)
	}
	return out
}

// First returns the first element that satisfies fn, and true if found.
// Returns the zero value and false if no element matches.
func First[T any](s []T, fn func(T) bool) (T, bool) {
	for _, v := range s {
		if fn(v) {
			return v, true
		}
	}
	var zero T
	return zero, false
}
