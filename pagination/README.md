# `pagination` Package

The `pagination` package provides high-performance, standard-library pagination utilities supporting **two industry-standard pagination strategies**: **Offset Pagination** and **Keyset / Cursor-Based Pagination**, with strictly zero external dependencies.

## When to Use
- **Large Dataset Queries**: Segmenting massive relational database results to prevent high latency and out-of-memory errors on both server and client.
- **Admin UI & Data Tables**: Supplying offset, limit, total count, and page numbers when users need random access navigation (e.g. Page 1, Page 5).
- **High-Throughput Feeds & Mobile Infinite Scroll**: Supplying opaque cursor tokens for stable $O(1)$ keyset queries that never drift when rows are inserted or deleted.
- **Defensive API Gatekeeping**: Clamping unbounded client query limits (e.g. `?limit=1000000`) to safe, configurable boundaries (`max=100`).

## Why It Is Written Like That
- **Dual Strategy Support**: Gives microservices the choice between Offset pagination (for random access tables) and Keyset/Cursor pagination (for high-volume feeds and audit streams) in a single library.
- **Zero External Dependencies**: Implemented strictly with standard library packages (`net/http`, `net/url`, `encoding/base64`, `strconv`), avoiding ORM lock-in or heavy frameworks.
- **Safe Param Clamping**: Automatically validates and sanitizes input queries, preventing negative limits, zero offsets, or denial-of-service query attacks without returning ugly validation errors.
- **Opaque URL-Safe Encoding**: Uses unpadded URL-safe Base64 (`base64.RawURLEncoding`) for cursor state, preventing URI encoding corruption across HTTP proxies.

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/pagination` |
|---|---|---|---|
| **Raw SQL `LIMIT` / `OFFSET` Strings** | Native to SQL | Brittle parameter parsing; susceptible to SQL injection if unparameterized; high DB scan penalty on deep offsets | Safe HTTP parameter extraction, input boundary clamping, and standardized JSON envelopes |
| **GORM / ORM Pagination Plugins** | Coupled directly to ORM models | Locks services into heavy ORM runtimes; cannot paginate raw SQL, Elasticsearch, or in-memory collections | Storage-agnostic; works with raw SQL, `pgx`, GORM, or memory slices |
| **Ad-hoc Per-Service Query Parsing** | No shared library | Inconsistent query parameter naming (`?size=` vs `?limit=`, `?p=` vs `?page=`); duplicated envelope boilerplate | Enforces uniform API contracts across the entire microservice ecosystem |

---

## 🎯 2 Industry-Standard Pagination Strategies

| Strategy | Helpers | Best For | DB Complexity | Consistency Under Mutations |
|:---|:---|:---|:---|:---|
| **Offset Pagination** | `Parse`, `NewResponse`, `NewTypedResponse` | Administrative search tables, catalogs with random page jumping (e.g. Page 1, 2, 5) | $O(N)$ (scans and discards skipped rows) | Vulnerable to page-drift / duplicate items when rows are inserted/deleted |
| **Cursor / Keyset Pagination** | `ParseCursor`, `NewCursorResponse`, `EncodeCursor`, `DecodeCursor` | High-throughput feeds, infinite scroll, real-time transaction ledgers, event logs | $O(1)$ (indexed `WHERE id > :cursor LIMIT :n`) | Immune to page-drift and duplicates |

---

## 💻 How to Use It?

### 1. Cursor-Based Pagination (Recommended for Scale & Feeds)
```go
import (
    "encoding/json"
    "net/http"
    "github.com/umesh0492/go-libs/pagination"
)

func ListAuditEventsHandler(w http.ResponseWriter, r *http.Request) {
    p := pagination.ParseCursor(r) // extracts ?cursor=...&limit=20
    
    rawID, _ := pagination.DecodeCursor(p.Cursor)
    events, nextID, hasMore, err := db.GetEventsAfter(r.Context(), rawID, p.Limit)
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }

    nextCursor := ""
    if hasMore {
        nextCursor = pagination.EncodeCursor(nextID)
    }

    resp := pagination.NewTypedCursorResponse(events, nextCursor, hasMore, p.Limit)
    json.NewEncoder(w).Encode(resp)
}
```

### 2. Offset-Based Pagination (Admin Tables & Page Jumpers)
```go
func ListUsersHandler(w http.ResponseWriter, r *http.Request) {
    p := pagination.Parse(r) // extracts ?page=1&limit=20 (clamped 1-100)
    
    users, total, err := db.GetUsers(r.Context(), p.Limit, p.Offset())
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }

    resp := pagination.NewTypedResponse(users, total, p)
    json.NewEncoder(w).Encode(resp)
}
```

---

## 🛡️ Edge Cases Handled
- **Negative & Malformed Parameter Bypassing**: Values like `?limit=-999` or `?page=invalid` are defensively clamped to stable defaults (`page=1`, `limit=20`, `max=100`) with zero panic risks.
- **Opaque URL-Safe Cursors**: `EncodeCursor` and `DecodeCursor` use URL-safe, unpadded Base64 encoding (`base64.RawURLEncoding`) so cursors can safely be passed in query strings without percent-encoding corruption.
- **Zero External Dependencies**: Pure Go standard library.
