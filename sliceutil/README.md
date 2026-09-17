# `sliceutil` Package

The `sliceutil` package provides functional, zero-dependency generic utilities for Go slice manipulation (`Go 1.18+`).

## When to Use
- **Batch Processing**: Splitting large collections into fixed-size chunks for batch database queries or API calls (`Chunk`).
- **Grouping & Partitioning**: Partitioning elements by key into buckets (`GroupBy`).
- **Deduplication**: Extracting distinct elements preserving order (`Unique`).
- **Reduction & Folding**: Accumulating values over a collection (`Reduce`).
- **Flattening**: Flattening nested slices into a single linear slice (`Flatten`).
- **Predicated Search**: Finding the first element satisfying a predicate (`First`).

## Why It Is Written Like That
- **Zero External Dependencies**: Implements essential functional algorithms natively without pulling in external utility dependencies.
- **Go Generics**: Fully type-safe at compile-time with zero interface boxing/unboxing overhead.
- **Allocation-Conscious**: Where output slice capacities are knowable upfront (such as `Chunk` and `Flatten`), capacities are preallocated to minimize GC pressure and memory reallocations.

## Available Functions

- `Reduce[T, U any](s []T, init U, fn func(U, T) U) U`
- `GroupBy[T any, K comparable](s []T, fn func(T) K) map[K][]T`
- `Chunk[T any](s []T, size int) [][]T`
- `Unique[T comparable](s []T) []T`
- `Flatten[T any](slices [][]T) []T`
- `First[T any](s []T, fn func(T) bool) (T, bool)`

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/sliceutil` |
|---|---|---|---|
| **`samber/lo`** | Massive functional library | Heavy dependency tree (100+ functions), frequent releases | Lightweight, focused set of fundamental primitives with zero dependencies |
| **Stdlib `slices` package** | Built into Go 1.21+ | Primarily mutative in-place algorithms; lacks functional transforms like `GroupBy` or `Chunk` | Seamlessly complements stdlib `slices` with non-mutative algorithmic primitives |
| **Manual `for` loops** | No imports required | High boilerplate, duplicate code across handlers, prone to allocation inefficiencies | Clean, readable, declarative pipelines |

## Quickstart

```go
package main

import (
    "fmt"
    "github.com/umesh0492/go-libs/sliceutil"
)

type User struct {
    ID   string
    Name string
    Role string
}

func main() {
    users := []User{
        {ID: "1", Name: "Alice", Role: "Admin"},
        {ID: "2", Name: "Bob", Role: "User"},
        {ID: "3", Name: "Charlie", Role: "Admin"},
    }

    // 1. GroupBy: Bucket users by role
    byRole := sliceutil.GroupBy(users, func(u User) string {
        return u.Role
    })

    // 2. Chunk: Batch into partitions of size 2
    batches := sliceutil.Chunk(users, 2)

    // 3. First: Find first user with specific role
    firstAdmin, ok := sliceutil.First(users, func(u User) bool {
        return u.Role == "Admin"
    })

    fmt.Println("Admin count:", len(byRole["Admin"]))
    fmt.Println("Batch count:", len(batches))
    if ok {
        fmt.Println("First Admin:", firstAdmin.Name)
    }
}
```
