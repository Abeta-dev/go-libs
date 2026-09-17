# `idempotency` Package & `ginmw.Idempotency`

The `idempotency` package provides HTTP middleware and store contracts to guarantee safe retries for mutating operations (`POST`, `PUT`, `PATCH`).

## When to Use
- **Financial & Payment Processing**: Charging cards, initiating bank transfers, issuing refunds.
- **Order / Entity Creation**: Placing orders, generating invoices, creating users.
- **Network Flaky Clients**: Mobile clients or web applications with automatic retry policies.

## Why It Is Written Like That
- **The Double-Click & Network Drop Problem**: If a user clicks "Pay", the server charges the card, but a mobile network disconnect drops the HTTP response, the client automatically retries the request. Without idempotency, the customer is billed twice!
- **Header-Driven (`Idempotency-Key`)**: The client generates a unique UUID per intent and sends it via the `Idempotency-Key` HTTP header.
- **Atomic Two-Phase Store**:
  1. **Lock (`IN_PROGRESS`)**: When the request arrives, the key is locked. If a concurrent duplicate arrives while the first is still processing, it immediately returns `409 Conflict` (prevents race conditions).
  2. **Save (`COMPLETED`)**: When the handler completes, the middleware captures the status code, response headers, and response body, storing them with a TTL.
  3. **Replay**: Any subsequent request with the same key bypasses the handler completely and replays the cached HTTP response directly.

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/idempotency` |
|---|---|---|---|
| **Disabling UI Buttons** | Simple | Users refresh pages, apps crash, scripted clients ignore UI state | Server-side enforcement is required for financial correctness |
| **Unique DB Constraints alone** | Catches duplicates | Returns an ugly 500 error or SQL error; does not return the original created entity | Middleware transparently replays the original 200/201 response |
| **Bespoke handler logic per endpoint** | Customized per entity | Inconsistent across services, repetitive database boilerplate | Middleware provides zero-overhead, 1-line protection |

## Quickstart

### 1. Register Idempotency Middleware (`router.go`)
```go
import (
    "github.com/umesh0492/go-libs/idempotency"
    "github.com/umesh0492/go-libs/ginmw"
)

// NewMemoryStore is suitable for one process. Use PGStore for multi-replica deployments.
store, err := idempotency.NewPGStore(pool,
    idempotency.WithPGLockTTL(30*time.Second),
    idempotency.WithPGResponseTTL(24*time.Hour),
)
if err != nil {
    return err
}

r := gin.New()

// Protect mutating routes
orders := r.Group("/api/v1/orders")
orders.Use(ginmw.Idempotency(store))
orders.POST("", CreateOrderHandler)
```

### 2. Client HTTP Request
```http
POST /api/v1/orders HTTP/1.1
Host: api.example.com
Idempotency-Key: 9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d
Content-Type: application/json

{"item_id": "item-123", "quantity": 1}
```
- **1st Request**: Executes handler, creates order, caches response, returns `201 Created`.
- **2nd Request (Duplicate retry)**: Bypasses handler, returns cached `201 Created` immediately.
- **Concurrent Request (During processing)**: Returns `409 Conflict` (`"idempotent operation already in progress"`).

---

## 🛡️ Concurrency & Correctness Guarantees

- **`-race` Certified**: Verified under 100+ concurrent workers attempting simultaneous locks, completions, and unlocks on contended keys with 0 data races.
- **Atomic Two-Phase Locking**: Guarantees that only 1 concurrent worker can acquire the lock for a given key (`Lock`). Concurrent requests immediately receive `409 Conflict`.
- **Automatic Panic & Error Unlock**: If a handler panics or returns a 5xx error, `Unlock(ctx, key)` releases the in-progress lock so the client can safely retry without being locked out.
- **Deep-Copy Isolation**: Response headers and body payloads are deeply copied when saved and replayed, preventing memory corruption or data races across concurrent replay readers.

---

## PostgreSQL Store for Multi-Replica Deployments

`PGStore` is the built-in PostgreSQL implementation of `Store`. It accepts any `db.DBTX`, so callers may provide either a pool or a transaction. Apply `IdempotencySchemaDDL` through the application's migration system before serving requests; `NewPGStore` does not execute DDL.

```go
store, err := idempotency.NewPGStore(pool,
    idempotency.WithPGLockTTL(30*time.Second),
    idempotency.WithPGResponseTTL(24*time.Hour),
    idempotency.WithPGTableName("idempotency_keys"), // optional; identifiers are validated
)
```

`Lock` first uses `INSERT ... ON CONFLICT DO NOTHING`, then reads the conflicting row. An expired record or `IN_PROGRESS` lease is reclaimed with a conditional update, so only one replica obtains the next lease. `Save` persists the status code, headers, body, and response expiry; `Unlock` removes only an in-progress record. `PGStore` does not create a transaction around the business handler: when response persistence must be coupled to application writes, pass the transaction as `db.DBTX` and commit according to the application's transaction boundary.

## Known Limitations & Memory Characteristics

- **In-Memory Retention Eviction**: `MemoryStore` stores cached responses in process memory. By default, completed responses persist until process shutdown. To prevent unbounded memory growth in long-running services, configure bounded retention using `WithResponseTTL` (for example, 24 hours) and `WithMaxEntries` (for example, 50,000 keys).
- **Single-Process Scope**: `MemoryStore` is scoped to one Go process. In horizontally scaled deployments, use `PGStore` so replicas share locks and cached responses.
- **Client key semantics**: The store keys only on the supplied idempotency key. Scope and validate keys (for example, by authenticated principal and operation) before calling the middleware; do not reuse a key for different requests.

