# `maputil` Package

The `maputil` package provides generic, zero-dependency helpers for Go map operations (`Go 1.18+`).

## When to Use
- **Configuration & Metadata Merging**: Merging multiple configuration dictionaries or HTTP headers with override semantics (`Merge`).
- **Filtering Maps**: Removing sensitive or stale entries from key-value stores (`Filter`).

## Why It Is Written Like That
- **Type Safety via Go Generics**: Works over any `comparable` key type and `any` value type without type assertions or reflection.
- **Safe Copy Semantics**: `Merge` produces a new copy rather than modifying inputs in place, avoiding concurrent read/write panics.

## Available Functions

- `Merge[K comparable, V any](maps ...map[K]V) map[K]V`: Combines maps from left to right; later maps override earlier duplicate keys.
- `Filter[K comparable, V any](m map[K]V, fn func(K, V) bool) map[K]V`: Returns entries satisfying the predicate.

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/maputil` |
|---|---|---|---|
| **`samber/lo`** | Large ecosystem | Heavyweight dependency | Focused generic helpers with zero external dependencies |
| **`golang.org/x/exp/maps` / stdlib `maps`** | Built-in maps package | Lacks high-level merging with precedence and predicate filtering | Cleanly complements standard library `maps` |
| **Inline loops** | Zero imports | Repetitive boilerplate across dozens of microservice handlers | Standardized, readable, consistent implementation |

## Quickstart

```go
package main

import (
    "fmt"
    "github.com/umesh0492/go-libs/maputil"
)

func main() {
    defaultSettings := map[string]string{
        "theme": "dark",
        "lang":  "en",
        "env":   "production",
    }

    userOverrides := map[string]string{
        "lang": "es",
    }

    // Merge settings
    finalSettings := maputil.Merge(defaultSettings, userOverrides)
    fmt.Println("Language:", finalSettings["lang"]) // "es"

    // Filter settings: keep non-theme keys
    prodSettings := maputil.Filter(finalSettings, func(k, v string) bool {
        return k != "theme"
    })
    fmt.Println("Filtered settings count:", len(prodSettings))
}
```
