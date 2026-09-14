# `sliceutil` Package

The `sliceutil` package provides functional, zero-dependency generic utilities for Go slice manipulation (`Go 1.18+`).

## When to Use
- **Data Transformation**: Transforming domain model slices into DTOs or response envelopes (`Map`).
- **Data Filtering**: Filtering item collections based on criteria, tags, or state (`Filter`).
- **Batch Processing**: Splitting large collections into fixed-size chunks for batch database queries or API calls (`Chunk`).
- **Deduplication & Partitioning**: Extracting distinct elements (`Unique`) or grouping elements by key (`GroupBy`).

## Why It Is Written Like That
- **Zero External Dependencies**: Implements essential functional algorithms natively without pulling in external utility dependencies.
- **Go Generics**: Fully type-safe at compile-time with zero interface boxing/unboxing overhead.
- **Preallocated Slices**: Where the output size is known (such as `Map`), capacities are preallocated upfront to minimize GC pressure and memory reallocations.

## Available Functions

- `Map[T, U any](s []T, fn func(T) U) []U`
- `Filter[T any](s []T, fn func(T) bool) []T`
- `Reduce[T, U any](s []T, init U, fn func(U, T) U) U`
- `GroupBy[T any, K comparable](s []T, fn func(T) K) map[K][]T`
- `Chunk[T any](s []T, size int) [][]T`
- `Unique[T comparable](s []T) []T`
- `Contains[T comparable](s []T, target T) bool`
- `Flatten[T any](slices [][]T) []T`
- `First[T any](s []T, fn func(T) bool) (T, bool)`

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/sliceutil` |
|---|---|---|---|
| **`samber/lo`** | Massive functional library | Heavy dependency tree (100+ functions), frequent releases | Lightweight, focused set of fundamental primitives with zero dependencies |
| **Stdlib `slices` package** | Built into Go 1.21+ | Primarily mutative in-place algorithms; lacks functional transforms like `GroupBy`, `Chunk`, or `Map` | Seamlessly complements stdlib `slices` with non-mutative functional pipelines |
| **Manual `for` loops** | No imports required | High boilerplate, duplicate code across handlers, prone to allocation inefficiencies | Clean, readable, declarative pipelines |

## Quickstart

```go
package main

import (
    "fmt"
    "strings"
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

    // 1. Filter: Find all Admins
    admins := sliceutil.Filter(users, func(u User) bool {
        return u.Role == "Admin"
    })

    // 2. Map: Extract names
    names := sliceutil.Map(admins, func(u User) string {
        return strings.ToUpper(u.Name)
    })

    // 3. Chunk: Batch into size 2
    batches := sliceutil.Chunk(users, 2)

    fmt.Println("Admin Names:", names)
    fmt.Println("Batch count:", len(batches))
}
```
