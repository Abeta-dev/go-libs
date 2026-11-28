# CodeGraph & Exported Symbol Index — `go-libs` (v0.6.0)

> **Step 5 of the Mandatory Grounding Order** (`AGENTS.md` -> `docs/ARCHITECTURE.md` -> `docs/FLOWS.md` -> `skills/*/SKILL.md` -> **`docs/CODEGRAPH.md`** -> targeted source reads only for verification).

This deterministic AST index maps every production package, internal/external dependency edge, and exported symbol with exact `file:line` coordinates for `github.com/abeta-dev/go-libs` (`v0.6.0`, Go `1.26.0`). Use it to jump directly to verified targets instead of scanning entire source trees.

## 1. Graph Summary

| Metric | Measured Value |
| :--- | :--- |
| **Module** | `github.com/abeta-dev/go-libs` (`v0.6.0`) |
| **Indexed Go Packages** | `41` (`29` Depth-0 internal leaf packages) |
| **Exported AST Symbols** | `776` (types, interfaces, structs, functions, methods, consts, vars) |
| **Distinct External Packages Imported** | `17` |
| **Local Semantic Graph (Optional)** | `graphify-out/GRAPH_REPORT.md` & `graphify-out/graph.json` (gitignored local cache) |

## 2. Package Dependency Graph (DAG)

| Package | DAG Depth | Fan-In | Internal Imports (`github.com/abeta-dev/go-libs/*`) | External Imports | Exported Symbols |
| :--- | :---: | :---: | :--- | :--- | :---: |
| `apperror` | 0 | 0 | _none (pure leaf)_ | _stdlib only_ | 27 |
| `async` | 0 | 0 | _none (pure leaf)_ | _stdlib only_ | 5 |
| `bodylimit` | 0 | 1 | _none (pure leaf)_ | _stdlib only_ | 4 |
| `cache` | 1 | 0 | `clock`, `metrics` | `golang.org/x/sync/singleflight` | 58 |
| `circuitbreaker` | 1 | 1 | `clock`, `metrics` | _stdlib only_ | 23 |
| `clock` | 0 | 5 | _none (pure leaf)_ | _stdlib only_ | 37 |
| `compress` | 0 | 2 | _none (pure leaf)_ | `github.com/andybalholm/brotli` | 22 |
| `cryptoutil` | 1 | 0 | `stringutil` | `golang.org/x/crypto/bcrypt` | 8 |
| `db` | 2 | 1 | `ratelimit` | `github.com/jackc/pgx/v5`, `github.com/jackc/pgx/v5/pgconn`, `github.com/jackc/pgx/v5/pgxpool` | 24 |
| `env` | 0 | 0 | _none (pure leaf)_ | _stdlib only_ | 6 |
| `etag` | 0 | 2 | _none (pure leaf)_ | _stdlib only_ | 24 |
| `fsm` | 0 | 0 | _none (pure leaf)_ | _stdlib only_ | 6 |
| `ginmw` | 4 | 0 | `compress`, `etag`, `httputil`, `idempotency`, `logger`, `loopbackguard`, `ratelimit`, `rbaccontext`, `requestid`, `securityheaders`, `sse` | `github.com/gin-gonic/gin`, `github.com/golang-jwt/jwt/v5`, `github.com/google/uuid`, `go.opentelemetry.io/otel`, `go.opentelemetry.io/otel/attribute`, `go.opentelemetry.io/otel/codes`, `go.opentelemetry.io/otel/propagation`, `go.opentelemetry.io/otel/trace` | 79 |
| `health` | 0 | 0 | _none (pure leaf)_ | _stdlib only_ | 13 |
| `httpclient` | 2 | 0 | `circuitbreaker`, `clock`, `metrics`, `retry` | `go.opentelemetry.io/otel/propagation`, `go.opentelemetry.io/otel/trace` | 20 |
| `httpmw` | 2 | 0 | `bodylimit`, `compress`, `etag`, `httputil`, `logger`, `loopbackguard`, `ratelimit`, `rbaccontext`, `requestid`, `securityheaders`, `sse` | _stdlib only_ | 44 |
| `httputil` | 0 | 2 | _none (pure leaf)_ | _stdlib only_ | 12 |
| `idempotency` | 3 | 1 | `db` | `github.com/jackc/pgx/v5` | 43 |
| `jwks` | 0 | 0 | _none (pure leaf)_ | _stdlib only_ | 7 |
| `logger` | 1 | 2 | `clock` | _stdlib only_ | 39 |
| `loopbackguard` | 0 | 2 | _none (pure leaf)_ | _stdlib only_ | 22 |
| `maputil` | 0 | 0 | _none (pure leaf)_ | _stdlib only_ | 2 |
| `metrics` | 0 | 5 | _none (pure leaf)_ | _stdlib only_ | 11 |
| `pagination` | 0 | 0 | _none (pure leaf)_ | _stdlib only_ | 21 |
| `ratelimit` | 1 | 3 | `clock`, `metrics` | _stdlib only_ | 38 |
| `rbac` | 0 | 1 | _none (pure leaf)_ | _stdlib only_ | 6 |
| `rbaccontext` | 1 | 2 | `rbac` | _stdlib only_ | 5 |
| `recovery` | 0 | 0 | _none (pure leaf)_ | `go.opentelemetry.io/otel/attribute`, `go.opentelemetry.io/otel/codes`, `go.opentelemetry.io/otel/trace` | 3 |
| `requestid` | 0 | 2 | _none (pure leaf)_ | `github.com/google/uuid` | 3 |
| `retry` | 0 | 1 | _none (pure leaf)_ | _stdlib only_ | 9 |
| `securityheaders` | 0 | 2 | _none (pure leaf)_ | _stdlib only_ | 11 |
| `shutdown` | 0 | 0 | _none (pure leaf)_ | _stdlib only_ | 6 |
| `sliceutil` | 0 | 0 | _none (pure leaf)_ | _stdlib only_ | 6 |
| `sqlite` | 0 | 0 | _none (pure leaf)_ | _stdlib only_ | 46 |
| `sse` | 0 | 2 | _none (pure leaf)_ | _stdlib only_ | 19 |
| `stringutil` | 0 | 1 | _none (pure leaf)_ | _stdlib only_ | 11 |
| `telemetry` | 0 | 0 | _none (pure leaf)_ | `go.opentelemetry.io/otel`, `go.opentelemetry.io/otel/attribute`, `go.opentelemetry.io/otel/codes`, `go.opentelemetry.io/otel/propagation`, `go.opentelemetry.io/otel/semconv/v1.24.0`, `go.opentelemetry.io/otel/trace`, `go.opentelemetry.io/otel/trace/noop` | 18 |
| `timeutil` | 0 | 0 | _none (pure leaf)_ | _stdlib only_ | 10 |
| `uuidutil` | 0 | 0 | _none (pure leaf)_ | `github.com/google/uuid` | 11 |
| `validation` | 0 | 0 | _none (pure leaf)_ | `github.com/go-playground/validator/v10` | 2 |
| `workerpool` | 1 | 0 | `metrics` | _stdlib only_ | 15 |

### High Fan-In Internal Hub Packages

- **`clock`** — imported by `5` internal packages
- **`metrics`** — imported by `5` internal packages
- **`ratelimit`** — imported by `3` internal packages
- **`compress`** — imported by `2` internal packages
- **`etag`** — imported by `2` internal packages
- **`httputil`** — imported by `2` internal packages
- **`logger`** — imported by `2` internal packages
- **`loopbackguard`** — imported by `2` internal packages
- **`rbaccontext`** — imported by `2` internal packages
- **`requestid`** — imported by `2` internal packages
- **`securityheaders`** — imported by `2` internal packages
- **`sse`** — imported by `2` internal packages
- **`bodylimit`** — imported by `1` internal packages
- **`circuitbreaker`** — imported by `1` internal packages
- **`db`** — imported by `1` internal packages
- **`idempotency`** — imported by `1` internal packages
- **`rbac`** — imported by `1` internal packages
- **`retry`** — imported by `1` internal packages
- **`stringutil`** — imported by `1` internal packages

## 3. Exported Symbol Coordinates (`file:line`)

Use these exact `file:line` coordinates in **Step 6** to verify signatures and edge cases without broad directory scans.

### `apperror` (27 symbols)

- **Types / Interfaces / Structs**: `Code` (`apperror/apperror.go:24`), `Error` (`apperror/apperror.go:49`)
- **Functions / Methods**: `*Error.Error` (`apperror/apperror.go:61`), `*Error.Unwrap` (`apperror/apperror.go:69`), `*Error.GetCode` (`apperror/apperror.go:72`), `*Error.GetStatus` (`apperror/apperror.go:76`), `New` (`apperror/apperror.go:100`), `NewWithDetails` (`apperror/apperror.go:105`), `Wrap` (`apperror/apperror.go:110`), `BadRequest` (`apperror/apperror.go:117`), `Unauthorized` (`apperror/apperror.go:120`), `Forbidden` (`apperror/apperror.go:123`), `NotFound` (`apperror/apperror.go:126`), `Conflict` (`apperror/apperror.go:129`), `Unprocessable` (`apperror/apperror.go:132`), `Internal` (`apperror/apperror.go:135`), `Unavailable` (`apperror/apperror.go:138`), `Is` (`apperror/apperror.go:144`), `CodeOf` (`apperror/apperror.go:154`)
- **Constants / Vars**: `CodeBadRequest` (`apperror/apperror.go:30`), `CodeUnauthorized` (`apperror/apperror.go:32`), `CodeForbidden` (`apperror/apperror.go:34`), `CodeNotFound` (`apperror/apperror.go:36`), `CodeConflict` (`apperror/apperror.go:38`), `CodeUnprocessable` (`apperror/apperror.go:40`), `CodeInternal` (`apperror/apperror.go:42`), `CodeUnavailable` (`apperror/apperror.go:44`)

### `async` (5 symbols)

- **Types / Interfaces / Structs**: `Task` (`async/parallel.go:14`)
- **Functions / Methods**: `Go` (`async/async.go:13`), `GoCtx` (`async/async.go:29`), `Parallel` (`async/parallel.go:20`), `ParallelLimit` (`async/parallel.go:70`)

### `bodylimit` (4 symbols)

- **Functions / Methods**: `New` (`bodylimit/bodylimit.go:20`), `MB` (`bodylimit/bodylimit.go:32`), `KB` (`bodylimit/bodylimit.go:37`), `String` (`bodylimit/bodylimit.go:43`)

### `cache` (58 symbols)

- **Types / Interfaces / Structs**: `Item` (`cache/cache.go:36`), `Metrics` (`cache/cache.go:43`), `EvictionPolicy` (`cache/cache.go:50`), `Option` (`cache/cache.go:72`), `Cache` (`cache/cache.go:109`), `TypedCache` (`cache/cache.go:123`), `MemoryCache` (`cache/cache.go:395`), `FetchStatus` (`cache/tags.go:12`), `InvalidatingCache` (`cache/tags.go:30`)
- **Functions / Methods**: `WithEvictionPolicy` (`cache/cache.go:65`), `WithClock` (`cache/cache.go:76`), `WithMetrics` (`cache/cache.go:85`), `WithEvictionInterval` (`cache/cache.go:93`), `WithCapacity` (`cache/cache.go:102`), `NewTypedCache` (`cache/cache.go:190`), `*TypedCache[T].Close` (`cache/cache.go:386`), `NewMemoryCache` (`cache/cache.go:398`), `*TypedCache[T].GetOrFetch` (`cache/cache.go:485`), `*TypedCache[T].Get` (`cache/cache.go:491`), `*TypedCache[T].Set` (`cache/cache.go:529`), `*TypedCache[T].Len` (`cache/cache.go:582`), `*TypedCache[T].Delete` (`cache/cache.go:601`), `*fifoBackend[T].Get` (`cache/fifo.go:65`), `*fifoBackend[T].Set` (`cache/fifo.go:93`), `*fifoBackend[T].GetOrFetch` (`cache/fifo.go:122`), `*fifoBackend[T].GetOrFetchWithStatus` (`cache/fifo.go:129`), `*fifoBackend[T].Delete` (`cache/fifo.go:172`), `*fifoBackend[T].DeletePrefix` (`cache/fifo.go:185`), `*fifoBackend[T].Len` (`cache/fifo.go:201`), `*lfuBackend[T].Get` (`cache/lfu.go:114`), `*lfuBackend[T].Set` (`cache/lfu.go:143`), `*lfuBackend[T].GetOrFetch` (`cache/lfu.go:175`), `*lfuBackend[T].GetOrFetchWithStatus` (`cache/lfu.go:182`), `*lfuBackend[T].Delete` (`cache/lfu.go:225`), `*lfuBackend[T].DeletePrefix` (`cache/lfu.go:238`), `*lfuBackend[T].Len` (`cache/lfu.go:254`), `*lruBackend[T].Get` (`cache/lru.go:64`), `*lruBackend[T].Set` (`cache/lru.go:93`), `*lruBackend[T].GetOrFetch` (`cache/lru.go:123`), `*lruBackend[T].GetOrFetchWithStatus` (`cache/lru.go:130`), `*lruBackend[T].Delete` (`cache/lru.go:173`), `*lruBackend[T].DeletePrefix` (`cache/lru.go:186`), `*lruBackend[T].Len` (`cache/lru.go:202`), `FetchStatus.String` (`cache/tags.go:25`), `*TypedCache[T].GetOrFetchWithStatus` (`cache/tags.go:54`), `*TypedCache[T].GetOrFetchWithTags` (`cache/tags.go:60`), `*TypedCache[T].SetWithTags` (`cache/tags.go:79`), `*TypedCache[T].DeletePrefix` (`cache/tags.go:90`), `*TypedCache[T].InvalidateTag` (`cache/tags.go:135`), `*TypedCache[T].InvalidateTags` (`cache/tags.go:141`)
- **Constants / Vars**: `DefaultCapacity` (`cache/cache.go:19`), `EvictionSampledLRU` (`cache/cache.go:54`), `EvictionLRU` (`cache/cache.go:56`), `EvictionLFU` (`cache/cache.go:58`), `EvictionFIFO` (`cache/cache.go:60`), `FetchStatusHit` (`cache/tags.go:16`), `FetchStatusMiss` (`cache/tags.go:18`), `FetchStatusCoalesced` (`cache/tags.go:21`)

### `circuitbreaker` (23 symbols)

- **Types / Interfaces / Structs**: `Breaker` (`circuitbreaker/circuitbreaker.go:18`), `State` (`circuitbreaker/circuitbreaker.go:24`), `Metrics` (`circuitbreaker/circuitbreaker.go:48`), `Option` (`circuitbreaker/circuitbreaker.go:62`), `ConsecutiveBreaker` (`circuitbreaker/consecutive.go:17`), `RatioBreaker` (`circuitbreaker/ratio.go:23`)
- **Functions / Methods**: `State.String` (`circuitbreaker/circuitbreaker.go:34`), `WithClock` (`circuitbreaker/circuitbreaker.go:66`), `WithOnStateChange` (`circuitbreaker/circuitbreaker.go:75`), `WithMetrics` (`circuitbreaker/circuitbreaker.go:82`), `WithHalfOpenProbes` (`circuitbreaker/circuitbreaker.go:89`), `NewConsecutiveBreaker` (`circuitbreaker/consecutive.go:33`), `*ConsecutiveBreaker.Execute` (`circuitbreaker/consecutive.go:116`), `*ConsecutiveBreaker.State` (`circuitbreaker/consecutive.go:264`), `*ConsecutiveBreaker.Failures` (`circuitbreaker/consecutive.go:281`), `NewRatioBreaker` (`circuitbreaker/ratio.go:45`), `*RatioBreaker.Execute` (`circuitbreaker/ratio.go:152`), `*RatioBreaker.State` (`circuitbreaker/ratio.go:324`), `*RatioBreaker.Failures` (`circuitbreaker/ratio.go:341`)
- **Constants / Vars**: `ErrCircuitOpen` (`circuitbreaker/circuitbreaker.go:15`), `StateClosed` (`circuitbreaker/circuitbreaker.go:28`), `StateOpen` (`circuitbreaker/circuitbreaker.go:29`), `StateHalfOpen` (`circuitbreaker/circuitbreaker.go:30`)

### `clock` (37 symbols)

- **Types / Interfaces / Structs**: `Clock` (`clock/clock.go:9`), `Timer` (`clock/clock.go:33`), `Ticker` (`clock/clock.go:49`), `FakeClock` (`clock/fake.go:12`), `RealClock` (`clock/real.go:8`)
- **Functions / Methods**: `NewFake` (`clock/fake.go:22`), `NewFakeAt` (`clock/fake.go:27`), `*FakeClock.Now` (`clock/fake.go:36`), `*FakeClock.Since` (`clock/fake.go:43`), `*FakeClock.Until` (`clock/fake.go:50`), `*FakeClock.Set` (`clock/fake.go:57`), `*FakeClock.Add` (`clock/fake.go:64`), `*FakeClock.Sleep` (`clock/fake.go:134`), `*FakeClock.After` (`clock/fake.go:151`), `*FakeClock.NewTimer` (`clock/fake.go:156`), `*FakeClock.NewTicker` (`clock/fake.go:178`), `*FakeClock.BlockUntil` (`clock/fake.go:197`), `*fakeTimer.C` (`clock/fake.go:232`), `*fakeTimer.Reset` (`clock/fake.go:236`), `*fakeTimer.Stop` (`clock/fake.go:267`), `*fakeTicker.C` (`clock/fake.go:287`), `*fakeTicker.Reset` (`clock/fake.go:291`), `*fakeTicker.Stop` (`clock/fake.go:312`), `NewReal` (`clock/real.go:11`), `*RealClock.Now` (`clock/real.go:16`), `*RealClock.Since` (`clock/real.go:21`), `*RealClock.Until` (`clock/real.go:26`), `*RealClock.Sleep` (`clock/real.go:31`), `*RealClock.After` (`clock/real.go:36`), `*RealClock.NewTimer` (`clock/real.go:41`), `*RealClock.NewTicker` (`clock/real.go:46`), `*realTimer.C` (`clock/real.go:54`), `*realTimer.Reset` (`clock/real.go:58`), `*realTimer.Stop` (`clock/real.go:62`), `*realTicker.C` (`clock/real.go:70`), `*realTicker.Reset` (`clock/real.go:74`), `*realTicker.Stop` (`clock/real.go:78`)

### `compress` (22 symbols)

- **Types / Interfaces / Structs**: `Config` (`compress/compress.go:36`), `FileServerOption` (`compress/fileserver.go:37`)
- **Functions / Methods**: `DefaultConfig` (`compress/compress.go:68`), `Negotiate` (`compress/compress.go:114`), `Middleware` (`compress/compress.go:187`), `*compressResponseWriter.WriteHeader` (`compress/compress.go:332`), `*compressResponseWriter.Write` (`compress/compress.go:342`), `*compressResponseWriter.Flush` (`compress/compress.go:360`), `*compressResponseWriter.Unwrap` (`compress/compress.go:372`), `WithSPAFallback` (`compress/fileserver.go:42`), `WithFallbackExcludePrefixes` (`compress/fileserver.go:50`), `WithCSRFMetaToken` (`compress/fileserver.go:63`), `WithCSRFMetaTokenProvider` (`compress/fileserver.go:71`), `WithCSRFMetaName` (`compress/fileserver.go:79`), `WithImmutableCacheControl` (`compress/fileserver.go:89`), `WithHTMLCacheControl` (`compress/fileserver.go:97`), `WithPathPrefix` (`compress/fileserver.go:106`), `PrecompressedFileServer` (`compress/fileserver.go:116`), `PrecompressedFS` (`compress/fileserver.go:122`)
- **Constants / Vars**: `EncodingBrotli` (`compress/compress.go:25`), `EncodingGzip` (`compress/compress.go:26`), `EncodingIdentity` (`compress/compress.go:27`)

### `cryptoutil` (8 symbols)

- **Functions / Methods**: `GenerateTempPassword` (`cryptoutil/cryptoutil.go:49`), `HashPassword` (`cryptoutil/cryptoutil.go:60`), `ComparePassword` (`cryptoutil/cryptoutil.go:71`), `IsCorrectPassword` (`cryptoutil/cryptoutil.go:83`)
- **Constants / Vars**: `DefaultPasswordLength` (`cryptoutil/cryptoutil.go:22`), `BcryptCost` (`cryptoutil/cryptoutil.go:26`), `ErrPasswordTooShort` (`cryptoutil/cryptoutil.go:39`), `ErrInvalidHash` (`cryptoutil/cryptoutil.go:41`)

### `db` (24 symbols)

- **Types / Interfaces / Structs**: `DBTX` (`db/context.go:18`), `CopyDBTX` (`db/context.go:25`), `TxBeginner` (`db/context.go:31`), `Option` (`db/db.go:14`), `PGRateLimiter` (`db/ratelimit.go:34`), `PGRateLimiterOption` (`db/ratelimit.go:47`)
- **Functions / Methods**: `WithQuerier` (`db/context.go:41`), `GetQuerier` (`db/context.go:47`), `WithTx` (`db/context.go:61`), `WithMaxConns` (`db/db.go:17`), `WithMinConns` (`db/db.go:24`), `Connect` (`db/db.go:38`), `WithPGRateLimiterTableName` (`db/ratelimit.go:50`), `WithPGRateLimiterTimeout` (`db/ratelimit.go:57`), `NewPGRateLimiter` (`db/ratelimit.go:66`), `*PGRateLimiter.Allow` (`db/ratelimit.go:98`), `*PGRateLimiter.AllowWithContext` (`db/ratelimit.go:103`), `*PGRateLimiter.AllowN` (`db/ratelimit.go:108`), `*PGRateLimiter.AllowNWithContext` (`db/ratelimit.go:113`), `*PGRateLimiter.Remaining` (`db/ratelimit.go:163`), `*PGRateLimiter.Reset` (`db/ratelimit.go:199`)
- **Constants / Vars**: `ConnKey` (`db/context.go:38`), `DefaultRateLimitTableName` (`db/ratelimit.go:18`), `RateLimitSchemaDDL` (`db/ratelimit.go:21`)

### `env` (6 symbols)

- **Functions / Methods**: `String` (`env/env.go:17`), `Int` (`env/env.go:26`), `Bool` (`env/env.go:40`), `Duration` (`env/env.go:54`), `MustString` (`env/env.go:68`), `MustInt` (`env/env.go:77`)

### `etag` (24 symbols)

- **Types / Interfaces / Structs**: `Option` (`etag/etag.go:42`)
- **Functions / Methods**: `WithAppVersion` (`etag/etag.go:46`), `WithPageVersion` (`etag/etag.go:54`), `WithIgnoreJSONKeys` (`etag/etag.go:63`), `WithCacheControl` (`etag/etag.go:78`), `WithWeak` (`etag/etag.go:86`), `WithClientVersionHeader` (`etag/etag.go:94`), `WithMaxBufferBytes` (`etag/etag.go:105`), `WithMethods` (`etag/etag.go:115`), `Compute` (`etag/etag.go:148`), `Match` (`etag/etag.go:222`), `WriteVersionedJSON` (`etag/etag.go:283`), `Middleware` (`etag/etag.go:337`), `*etagResponseWriter.WriteHeader` (`etag/etag.go:371`), `*etagResponseWriter.Write` (`etag/etag.go:383`), `*etagResponseWriter.Flush` (`etag/etag.go:397`), `*etagResponseWriter.Unwrap` (`etag/etag.go:406`)
- **Constants / Vars**: `HeaderETag` (`etag/etag.go:17`), `HeaderIfNoneMatch` (`etag/etag.go:18`), `HeaderAppVersion` (`etag/etag.go:19`), `HeaderPageVersion` (`etag/etag.go:20`), `HeaderClientAppVersion` (`etag/etag.go:21`), `DefaultCacheControl` (`etag/etag.go:24`), `DefaultMaxBufferBytes` (`etag/etag.go:26`)

### `fsm` (6 symbols)

- **Types / Interfaces / Structs**: `Transition` (`fsm/fsm.go:10`), `Config` (`fsm/fsm.go:17`), `Machine` (`fsm/fsm.go:23`)
- **Functions / Methods**: `NewMachine` (`fsm/fsm.go:33`), `*Machine.CanTransition` (`fsm/fsm.go:51`), `*Machine.ValidateTransition` (`fsm/fsm.go:57`)

### `ginmw` (79 symbols)

- **Types / Interfaces / Structs**: `ErrorHandlerOption` (`ginmw/error_handler.go:16`), `Claims` (`ginmw/jwt.go:18`), `AuthOption` (`ginmw/jwt.go:96`), `LoggerOption` (`ginmw/logger.go:15`), `PermissionProvider` (`ginmw/rbac.go:19`), `RecoveryOption` (`ginmw/recovery.go:19`), `MetricsTracker` (`ginmw/red_metrics.go:18`), `Envelope` (`ginmw/response.go:12`), `Response` (`ginmw/response.go:19`), `APIError` (`ginmw/response.go:22`), `TenantOption` (`ginmw/tenant.go:28`)
- **Functions / Methods**: `LimitBodyDefault` (`ginmw/bodylimit.go:18`), `LimitBodyAuth` (`ginmw/bodylimit.go:28`), `*ginCompressWriter.Write` (`ginmw/compress.go:18`), `*ginCompressWriter.WriteString` (`ginmw/compress.go:22`), `*ginCompressWriter.WriteHeader` (`ginmw/compress.go:26`), `*ginCompressWriter.Flush` (`ginmw/compress.go:31`), `*ginCompressWriter.Unwrap` (`ginmw/compress.go:37`), `Compress` (`ginmw/compress.go:43`), `PrecompressedFileServer` (`ginmw/compress.go:69`), `PrecompressedFS` (`ginmw/compress.go:75`), `CORS` (`ginmw/cors.go:31`), `WithErrorHandlerLogger` (`ginmw/error_handler.go:23`), `ErrorHandler` (`ginmw/error_handler.go:33`), `*ginETagWriter.Write` (`ginmw/etag.go:17`), `*ginETagWriter.WriteString` (`ginmw/etag.go:21`), `*ginETagWriter.WriteHeader` (`ginmw/etag.go:25`), `*ginETagWriter.Flush` (`ginmw/etag.go:30`), `*ginETagWriter.Unwrap` (`ginmw/etag.go:36`), `ETag` (`ginmw/etag.go:43`), `WriteVersionedJSON` (`ginmw/etag.go:64`), `*responseBuffer.Write` (`ginmw/idempotency.go:23`), `*responseBuffer.WriteString` (`ginmw/idempotency.go:29`), `Idempotency` (`ginmw/idempotency.go:36`), `*Claims.PrimaryRole` (`ginmw/jwt.go:30`), `*Claims.HasRole` (`ginmw/jwt.go:38`), `SetJWTSecret` (`ginmw/jwt.go:65`), `GenerateToken` (`ginmw/jwt.go:78`), `WithAllowQueryToken` (`ginmw/jwt.go:100`), `AuthMiddleware` (`ginmw/jwt.go:108`), `GetClaims` (`ginmw/jwt.go:143`), `WithLogger` (`ginmw/logger.go:22`), `Logger` (`ginmw/logger.go:30`), `LoopbackGuard` (`ginmw/loopbackguard.go:14`), `NewLoopbackGuard` (`ginmw/loopbackguard.go:37`), `GlobalRateLimit` (`ginmw/ratelimit.go:15`), `AuthRateLimit` (`ginmw/ratelimit.go:20`), `RateLimitWithLimiter` (`ginmw/ratelimit.go:42`), `RBAC` (`ginmw/rbac.go:26`), `RequirePermission` (`ginmw/rbac.go:56`), `RequireAnyPermission` (`ginmw/rbac.go:67`), `RequireRole` (`ginmw/rbac.go:79`), `AdminOnly` (`ginmw/rbac.go:99`), `WithRecoveryLogger` (`ginmw/recovery.go:26`), `Recovery` (`ginmw/recovery.go:35`), `REDMetrics` (`ginmw/red_metrics.go:32`), `MetricsEndpoint` (`ginmw/red_metrics.go:66`), `RequestID` (`ginmw/requestid.go:15`), `GetRequestID` (`ginmw/requestid.go:33`), `RespondSuccess` (`ginmw/response.go:29`), `RespondCreated` (`ginmw/response.go:37`), `RespondError` (`ginmw/response.go:45`), `RespondErrorWithDetail` (`ginmw/response.go:56`), `SecurityHeaders` (`ginmw/security.go:14`), `SecurityHeadersWithOptions` (`ginmw/security.go:25`), `SSE` (`ginmw/sse.go:14`), `RouteLabel` (`ginmw/telemetry.go:20`), `Telemetry` (`ginmw/telemetry.go:34`), `WithTenantContextKey` (`ginmw/tenant.go:31`), `WithTenantHeader` (`ginmw/tenant.go:38`), `WithTenantQuery` (`ginmw/tenant.go:45`), `WithBypassRoles` (`ginmw/tenant.go:53`), `RequireTenant` (`ginmw/tenant.go:70`), `ExtractTenantID` (`ginmw/tenant.go:114`)
- **Constants / Vars**: `DefaultMetrics` (`ginmw/red_metrics.go:29`), `RequestIDHeader` (`ginmw/requestid.go:38`), `DefaultTenantContextKey` (`ginmw/tenant.go:14`), `DefaultTenantHeader` (`ginmw/tenant.go:15`), `DefaultTenantQuery` (`ginmw/tenant.go:16`)

### `health` (13 symbols)

- **Types / Interfaces / Structs**: `Response` (`health/health.go:34`), `CheckResult` (`health/health.go:43`), `RuntimeInfo` (`health/health.go:49`), `Checker` (`health/health.go:62`), `Service` (`health/health.go:67`), `Option` (`health/health.go:74`)
- **Functions / Methods**: `WithStartTime` (`health/health.go:77`), `WithChecker` (`health/health.go:83`), `WithRuntimeInfo` (`health/health.go:88`), `New` (`health/health.go:93`), `*Service.Handler` (`health/health.go:106`), `LivenessHandler` (`health/health.go:176`), `ReadinessHandler` (`health/health.go:185`)

### `httpclient` (20 symbols)

- **Types / Interfaces / Structs**: `RateLimiter` (`httpclient/options.go:17`), `Metrics` (`httpclient/options.go:22`), `Option` (`httpclient/options.go:43`)
- **Functions / Methods**: `New` (`httpclient/client.go:13`), `NewRoundTripper` (`httpclient/client.go:20`), `WithTransport` (`httpclient/options.go:46`), `WithCircuitBreaker` (`httpclient/options.go:55`), `WithRetry` (`httpclient/options.go:62`), `WithRateLimiter` (`httpclient/options.go:69`), `WithRateLimiterKey` (`httpclient/options.go:79`), `WithRateLimiterFunc` (`httpclient/options.go:87`), `WithPerAttemptTimeout` (`httpclient/options.go:94`), `WithTotalTimeout` (`httpclient/options.go:101`), `WithMetrics` (`httpclient/options.go:108`), `WithTelemetry` (`httpclient/options.go:115`), `WithClock` (`httpclient/options.go:122`), `*resilientTransport.RoundTrip` (`httpclient/roundtripper.go:34`)
- **Constants / Vars**: `ErrRateLimited` (`httpclient/errors.go:9`), `ErrAttemptTimeout` (`httpclient/errors.go:12`), `DefaultMaxRetryBodySize` (`httpclient/roundtripper.go:23`)

### `httpmw` (44 symbols)

- **Types / Interfaces / Structs**: `Middleware` (`httpmw/httpmw.go:24`), `RecoveryOption` (`httpmw/observability.go:74`), `LoggerOption` (`httpmw/observability.go:130`), `MetricsTracker` (`httpmw/observability.go:191`), `TenantOption` (`httpmw/observability.go:287`)
- **Functions / Methods**: `Chain` (`httpmw/httpmw.go:29`), `Use` (`httpmw/httpmw.go:41`), `RequestID` (`httpmw/httpmw.go:48`), `LimitBody` (`httpmw/httpmw.go:53`), `LimitBodyDefault` (`httpmw/httpmw.go:58`), `LimitBodyAuth` (`httpmw/httpmw.go:63`), `Compress` (`httpmw/httpmw.go:69`), `PrecompressedFileServer` (`httpmw/httpmw.go:75`), `PrecompressedFS` (`httpmw/httpmw.go:81`), `ETag` (`httpmw/httpmw.go:87`), `SecurityHeaders` (`httpmw/httpmw.go:92`), `LoopbackGuard` (`httpmw/httpmw.go:98`), `CORS` (`httpmw/httpmw.go:106`), `CORSWithConfig` (`httpmw/httpmw.go:111`), `RateLimit` (`httpmw/httpmw.go:116`), `GlobalRateLimit` (`httpmw/httpmw.go:121`), `AuthRateLimit` (`httpmw/httpmw.go:126`), `SSE` (`httpmw/httpmw.go:131`), `RequirePermission` (`httpmw/httpmw.go:136`), `RequireAnyPermission` (`httpmw/httpmw.go:150`), `*statusResponseWriter.WriteHeader` (`httpmw/observability.go:40`), `*statusResponseWriter.Write` (`httpmw/observability.go:48`), `*statusResponseWriter.Flush` (`httpmw/observability.go:57`), `*statusResponseWriter.Unwrap` (`httpmw/observability.go:63`), `WithRecoveryLogger` (`httpmw/observability.go:77`), `Recovery` (`httpmw/observability.go:88`), `WithLogger` (`httpmw/observability.go:133`), `WithTrustedProxies` (`httpmw/observability.go:140`), `Logger` (`httpmw/observability.go:149`), `REDMetrics` (`httpmw/observability.go:205`), `MetricsEndpoint` (`httpmw/observability.go:240`), `WithTenantHeader` (`httpmw/observability.go:290`), `WithTenantQuery` (`httpmw/observability.go:299`), `WithTenantContext` (`httpmw/observability.go:308`), `TenantIDFromContext` (`httpmw/observability.go:313`), `RequireTenant` (`httpmw/observability.go:321`)
- **Constants / Vars**: `DefaultTenantHeader` (`httpmw/observability.go:25`), `DefaultTenantQuery` (`httpmw/observability.go:26`), `DefaultMetrics` (`httpmw/observability.go:202`)

### `httputil` (12 symbols)

- **Types / Interfaces / Structs**: `CORSConfig` (`httputil/cors.go:11`), `APIError` (`httputil/response.go:15`), `ErrResponse` (`httputil/response.go:23`)
- **Functions / Methods**: `DefaultCORSConfig` (`httputil/cors.go:20`), `CORS` (`httputil/cors.go:54`), `WriteJSON` (`httputil/response.go:30`), `OK` (`httputil/response.go:40`), `Created` (`httputil/response.go:45`), `NoContent` (`httputil/response.go:50`), `Error` (`httputil/response.go:55`), `ErrorFromDomain` (`httputil/response.go:62`), `ValidationError` (`httputil/response.go:75`)

### `idempotency` (43 symbols)

- **Types / Interfaces / Structs**: `MemoryStore` (`idempotency/memory.go:20`), `Option` (`idempotency/memory.go:29`), `MiddlewareOptions` (`idempotency/middleware.go:15`), `MiddlewareOption` (`idempotency/middleware.go:23`), `PGStore` (`idempotency/postgres.go:46`), `PGOption` (`idempotency/postgres.go:55`), `Response` (`idempotency/store.go:15`), `Status` (`idempotency/store.go:22`), `Record` (`idempotency/store.go:31`), `Store` (`idempotency/store.go:37`)
- **Functions / Methods**: `WithLockTTL` (`idempotency/memory.go:33`), `WithResponseTTL` (`idempotency/memory.go:41`), `WithMaxEntries` (`idempotency/memory.go:49`), `NewMemoryStore` (`idempotency/memory.go:56`), `*MemoryStore.Lock` (`idempotency/memory.go:67`), `*MemoryStore.Save` (`idempotency/memory.go:121`), `*MemoryStore.Unlock` (`idempotency/memory.go:149`), `*MemoryStore.Delete` (`idempotency/memory.go:166`), `*MemoryStore.Len` (`idempotency/memory.go:179`), `*MemoryStore.EvictExpired` (`idempotency/memory.go:187`), `WithHeaderName` (`idempotency/middleware.go:26`), `WithEnforceHeader` (`idempotency/middleware.go:36`), `WithIgnoredMethods` (`idempotency/middleware.go:43`), `WithStatusCodeMatcher` (`idempotency/middleware.go:53`), `*responseRecorder.WriteHeader` (`idempotency/middleware.go:67`), `*responseRecorder.Write` (`idempotency/middleware.go:72`), `Middleware` (`idempotency/middleware.go:78`), `WithPGLockTTL` (`idempotency/postgres.go:58`), `WithPGResponseTTL` (`idempotency/postgres.go:67`), `WithPGTableName` (`idempotency/postgres.go:76`), `NewPGStore` (`idempotency/postgres.go:83`), `*PGStore.Lock` (`idempotency/postgres.go:110`), `*PGStore.Save` (`idempotency/postgres.go:226`), `*PGStore.Unlock` (`idempotency/postgres.go:252`), `*PGStore.Delete` (`idempotency/postgres.go:267`)
- **Constants / Vars**: `DefaultHeaderName` (`idempotency/middleware.go:12`), `DefaultTableName` (`idempotency/postgres.go:19`), `DefaultLockTTL` (`idempotency/postgres.go:21`), `DefaultResponseTTL` (`idempotency/postgres.go:23`), `IdempotencySchemaDDL` (`idempotency/postgres.go:27`), `ErrAlreadyInProgress` (`idempotency/store.go:12`), `StatusInProgress` (`idempotency/store.go:26`), `StatusCompleted` (`idempotency/store.go:27`)

### `jwks` (7 symbols)

- **Types / Interfaces / Structs**: `Key` (`jwks/jwks.go:21`), `Set` (`jwks/jwks.go:31`), `Resolver` (`jwks/jwks.go:36`)
- **Functions / Methods**: `NewResolver` (`jwks/jwks.go:51`), `*Resolver.GetECPublicKey` (`jwks/jwks.go:60`), `GetECPublicKey` (`jwks/jwks.go:171`)
- **Constants / Vars**: `DefaultResolver` (`jwks/jwks.go:46`)

### `logger` (39 symbols)

- **Types / Interfaces / Structs**: `MiddlewareOptions` (`logger/middleware.go:16`), `MiddlewareOption` (`logger/middleware.go:23`), `RedactOption` (`logger/redactor.go:16`), `RedactingHandler` (`logger/redactor.go:69`), `Sampler` (`logger/sampler.go:15`), `SamplingOption` (`logger/sampler.go:20`), `SamplingHandler` (`logger/sampler.go:111`)
- **Functions / Methods**: `Default` (`logger/logger.go:24`), `WithContext` (`logger/logger.go:47`), `FromContext` (`logger/logger.go:56`), `WithField` (`logger/logger.go:66`), `WithFields` (`logger/logger.go:72`), `WithLogger` (`logger/middleware.go:26`), `WithRequestIDHeader` (`logger/middleware.go:35`), `WithExtraAttributes` (`logger/middleware.go:44`), `*statusRecorder.WriteHeader` (`logger/middleware.go:56`), `*statusRecorder.Write` (`logger/middleware.go:61`), `Middleware` (`logger/middleware.go:106`), `WithRedactedKeys` (`logger/redactor.go:25`), `WithRedactedPattern` (`logger/redactor.go:36`), `WithMask` (`logger/redactor.go:45`), `NewRedactingHandler` (`logger/redactor.go:77`), `*RedactingHandler.Enabled` (`logger/redactor.go:96`), `*RedactingHandler.Handle` (`logger/redactor.go:101`), `*RedactingHandler.WithAttrs` (`logger/redactor.go:118`), `*RedactingHandler.WithGroup` (`logger/redactor.go:132`), `WithSamplingClock` (`logger/sampler.go:30`), `WithBypassLevel` (`logger/sampler.go:38`), `WithSamplingKeyFunc` (`logger/sampler.go:45`), `WithSampler` (`logger/sampler.go:52`), `*tokenBucketSampler.Allow` (`logger/sampler.go:87`), `NewSamplingHandler` (`logger/sampler.go:121`), `*SamplingHandler.Enabled` (`logger/sampler.go:145`), `*SamplingHandler.Handle` (`logger/sampler.go:151`), `*SamplingHandler.WithAttrs` (`logger/sampler.go:162`), `*SamplingHandler.WithGroup` (`logger/sampler.go:172`)
- **Constants / Vars**: `DefaultRequestIDHeader` (`logger/middleware.go:13`), `DefaultMask` (`logger/redactor.go:13`), `DefaultRedactedKeys` (`logger/redactor.go:54`)

### `loopbackguard` (22 symbols)

- **Types / Interfaces / Structs**: `Option` (`loopbackguard/loopbackguard.go:106`), `Guard` (`loopbackguard/loopbackguard.go:213`)
- **Functions / Methods**: `ValidateLoopbackAddr` (`loopbackguard/loopbackguard.go:39`), `GenerateCSRFToken` (`loopbackguard/loopbackguard.go:80`), `WithAllowedHosts` (`loopbackguard/loopbackguard.go:110`), `WithAllowedOrigins` (`loopbackguard/loopbackguard.go:122`), `WithCSRFToken` (`loopbackguard/loopbackguard.go:134`), `WithCSRFHeader` (`loopbackguard/loopbackguard.go:142`), `WithDisableCSRF` (`loopbackguard/loopbackguard.go:152`), `WithSafeMethods` (`loopbackguard/loopbackguard.go:160`), `WithExemptPaths` (`loopbackguard/loopbackguard.go:176`), `WithStrictSecFetchSite` (`loopbackguard/loopbackguard.go:188`), `WithRequireLoopbackPeer` (`loopbackguard/loopbackguard.go:195`), `WithRandReader` (`loopbackguard/loopbackguard.go:203`), `New` (`loopbackguard/loopbackguard.go:220`), `*Guard.Token` (`loopbackguard/loopbackguard.go:258`), `*Guard.VerifyToken` (`loopbackguard/loopbackguard.go:264`), `*Guard.Middleware` (`loopbackguard/loopbackguard.go:272`)
- **Constants / Vars**: `DefaultCSRFHeader` (`loopbackguard/loopbackguard.go:22`), `DefaultCSRFTokenBytes` (`loopbackguard/loopbackguard.go:24`), `ErrInvalidAddress` (`loopbackguard/loopbackguard.go:29`), `ErrNonLoopbackAddress` (`loopbackguard/loopbackguard.go:31`)

### `maputil` (2 symbols)

- **Functions / Methods**: `Merge` (`maputil/maputil.go:9`), `Filter` (`maputil/maputil.go:21`)

### `metrics` (11 symbols)

- **Types / Interfaces / Structs**: `Counter` (`metrics/metrics.go:20`), `Gauge` (`metrics/metrics.go:28`), `Histogram` (`metrics/metrics.go:36`), `NoopCounter` (`metrics/metrics.go:45`), `NoopGauge` (`metrics/metrics.go:54`), `NoopHistogram` (`metrics/metrics.go:63`)
- **Functions / Methods**: `NoopCounter.Inc` (`metrics/metrics.go:48`), `NoopCounter.Add` (`metrics/metrics.go:51`), `NoopGauge.Set` (`metrics/metrics.go:57`), `NoopGauge.Add` (`metrics/metrics.go:60`), `NoopHistogram.Observe` (`metrics/metrics.go:66`)

### `pagination` (21 symbols)

- **Types / Interfaces / Structs**: `CursorParams` (`pagination/cursor.go:11`), `CursorResponse` (`pagination/cursor.go:26`), `TypedCursorResponse` (`pagination/cursor.go:44`), `Params` (`pagination/pagination.go:20`), `Response` (`pagination/pagination.go:52`), `TypedResponse` (`pagination/pagination.go:72`), `PageMeta` (`pagination/slice.go:6`)
- **Functions / Methods**: `ParseCursor` (`pagination/cursor.go:18`), `NewCursorResponse` (`pagination/cursor.go:34`), `NewTypedCursorResponse` (`pagination/cursor.go:52`), `EncodeCursor` (`pagination/cursor.go:62`), `DecodeCursor` (`pagination/cursor.go:67`), `Parse` (`pagination/pagination.go:27`), `Params.Offset` (`pagination/pagination.go:35`), `TotalPages` (`pagination/pagination.go:40`), `NewResponse` (`pagination/pagination.go:61`), `NewTypedResponse` (`pagination/pagination.go:81`), `PaginateSlice` (`pagination/slice.go:17`)
- **Constants / Vars**: `DefaultPage` (`pagination/pagination.go:14`), `DefaultLimit` (`pagination/pagination.go:15`), `MaxLimit` (`pagination/pagination.go:16`)

### `ratelimit` (38 symbols)

- **Types / Interfaces / Structs**: `Limiter` (`ratelimit/limiter.go:11`), `KeyFunc` (`ratelimit/limiter.go:23`), `Metrics` (`ratelimit/ratelimit.go:33`), `Option` (`ratelimit/ratelimit.go:45`), `Middleware` (`ratelimit/ratelimit.go:87`), `TrustedProxies` (`ratelimit/realip.go:22`), `SlidingWindowLimiter` (`ratelimit/sliding_window.go:15`), `TokenBucketLimiter` (`ratelimit/token_bucket.go:14`)
- **Functions / Methods**: `DefaultKeyFunc` (`ratelimit/limiter.go:26`), `NewWithLimiter` (`ratelimit/limiter.go:31`), `WithClock` (`ratelimit/ratelimit.go:49`), `WithMetrics` (`ratelimit/ratelimit.go:58`), `WithTrustedProxies` (`ratelimit/ratelimit.go:65`), `WithTrustedIPNets` (`ratelimit/ratelimit.go:73`), `WithDefaultTrustedProxies` (`ratelimit/ratelimit.go:80`), `New` (`ratelimit/ratelimit.go:94`), `NewGlobal` (`ratelimit/ratelimit.go:146`), `NewAuth` (`ratelimit/ratelimit.go:152`), `TrustedProxies.IPNets` (`ratelimit/realip.go:25`), `TrustedProxies.RealIP` (`ratelimit/realip.go:30`), `RealIP` (`ratelimit/realip.go:46`), `RealIPWithProxies` (`ratelimit/realip.go:102`), `IsTrusted` (`ratelimit/realip.go:107`), `ExtractIP` (`ratelimit/realip.go:117`), `ParseCIDRorIP` (`ratelimit/realip.go:131`), `ParseTrustedProxies` (`ratelimit/realip.go:153`), `KeyFuncWithTrustedProxies` (`ratelimit/realip.go:171`), `NewSlidingWindow` (`ratelimit/sliding_window.go:37`), `*SlidingWindowLimiter.Allow` (`ratelimit/sliding_window.go:165`), `*SlidingWindowLimiter.AllowN` (`ratelimit/sliding_window.go:170`), `*SlidingWindowLimiter.Remaining` (`ratelimit/sliding_window.go:190`), `*SlidingWindowLimiter.Reset` (`ratelimit/sliding_window.go:200`), `NewTokenBucket` (`ratelimit/token_bucket.go:35`), `*TokenBucketLimiter.Allow` (`ratelimit/token_bucket.go:139`), `*TokenBucketLimiter.AllowN` (`ratelimit/token_bucket.go:144`), `*TokenBucketLimiter.Remaining` (`ratelimit/token_bucket.go:173`), `*TokenBucketLimiter.Reset` (`ratelimit/token_bucket.go:189`)
- **Constants / Vars**: `DefaultTrustedCIDRs` (`ratelimit/realip.go:12`)

### `rbac` (6 symbols)

- **Types / Interfaces / Structs**: `Engine` (`rbac/engine.go:12`)
- **Functions / Methods**: `NewEngine` (`rbac/engine.go:15`), `*Engine.Match` (`rbac/engine.go:25`), `*Engine.HasPermission` (`rbac/engine.go:74`), `*Engine.HasAnyPermission` (`rbac/engine.go:84`), `*Engine.HasAllPermissions` (`rbac/engine.go:94`)

### `rbaccontext` (5 symbols)

- **Functions / Methods**: `WithPermissions` (`rbaccontext/rbaccontext.go:24`), `Permissions` (`rbaccontext/rbaccontext.go:35`), `Can` (`rbaccontext/rbaccontext.go:49`), `CanAny` (`rbaccontext/rbaccontext.go:63`), `CanAll` (`rbaccontext/rbaccontext.go:72`)

### `recovery` (3 symbols)

- **Types / Interfaces / Structs**: `Option` (`recovery/recovery.go:20`)
- **Functions / Methods**: `WithLogger` (`recovery/recovery.go:27`), `Middleware` (`recovery/recovery.go:35`)

### `requestid` (3 symbols)

- **Functions / Methods**: `Middleware` (`requestid/requestid.go:50`), `FromContext` (`requestid/requestid.go:68`)
- **Constants / Vars**: `Header` (`requestid/requestid.go:27`)

### `retry` (9 symbols)

- **Types / Interfaces / Structs**: `Strategy` (`retry/retry.go:33`), `Config` (`retry/retry.go:48`)
- **Functions / Methods**: `Do` (`retry/retry.go:70`), `DoWithResult` (`retry/retry.go:79`)
- **Constants / Vars**: `Constant` (`retry/retry.go:37`), `Linear` (`retry/retry.go:39`), `Exponential` (`retry/retry.go:41`), `ExponentialJitter` (`retry/retry.go:44`), `ErrMaxAttemptsReached` (`retry/retry.go:64`)

### `securityheaders` (11 symbols)

- **Types / Interfaces / Structs**: `Config` (`securityheaders/securityheaders.go:36`), `Option` (`securityheaders/securityheaders.go:57`)
- **Functions / Methods**: `WithServerName` (`securityheaders/securityheaders.go:60`), `WithHSTSMaxAge` (`securityheaders/securityheaders.go:67`), `WithDisableHSTS` (`securityheaders/securityheaders.go:75`), `WithCSP` (`securityheaders/securityheaders.go:82`), `WithPermissionsPolicy` (`securityheaders/securityheaders.go:89`), `WithCacheControl` (`securityheaders/securityheaders.go:98`), `New` (`securityheaders/securityheaders.go:120`)
- **Constants / Vars**: `DefaultConfig` (`securityheaders/securityheaders.go:106`), `Default` (`securityheaders/securityheaders.go:116`)

### `shutdown` (6 symbols)

- **Types / Interfaces / Structs**: `Hook` (`shutdown/shutdown.go:18`), `Manager` (`shutdown/shutdown.go:24`)
- **Functions / Methods**: `New` (`shutdown/shutdown.go:31`), `*Manager.Register` (`shutdown/shutdown.go:36`), `*Manager.Execute` (`shutdown/shutdown.go:43`), `*Manager.Wait` (`shutdown/shutdown.go:105`)

### `sliceutil` (6 symbols)

- **Functions / Methods**: `Reduce` (`sliceutil/sliceutil.go:9`), `GroupBy` (`sliceutil/sliceutil.go:18`), `Chunk` (`sliceutil/sliceutil.go:29`), `Unique` (`sliceutil/sliceutil.go:50`), `Flatten` (`sliceutil/sliceutil.go:63`), `First` (`sliceutil/sliceutil.go:77`)

### `sqlite` (46 symbols)

- **Types / Interfaces / Structs**: `SynchronousMode` (`sqlite/sqlite.go:21`), `CheckpointMode` (`sqlite/sqlite.go:33`), `Querier` (`sqlite/sqlite.go:67`), `Config` (`sqlite/sqlite.go:90`), `Option` (`sqlite/sqlite.go:104`), `DB` (`sqlite/sqlite.go:187`), `Migration` (`sqlite/sqlite.go:449`), `MigrationResult` (`sqlite/sqlite.go:457`), `CheckpointStats` (`sqlite/sqlite.go:572`)
- **Functions / Methods**: `WithQuerier` (`sqlite/sqlite.go:77`), `GetQuerier` (`sqlite/sqlite.go:82`), `WithDriver` (`sqlite/sqlite.go:108`), `WithBusyTimeout` (`sqlite/sqlite.go:115`), `WithJournalMode` (`sqlite/sqlite.go:124`), `WithSynchronous` (`sqlite/sqlite.go:131`), `WithForeignKeys` (`sqlite/sqlite.go:138`), `WithTempStoreMemory` (`sqlite/sqlite.go:145`), `WithCacheSizeKB` (`sqlite/sqlite.go:152`), `WithMaxOpenConns` (`sqlite/sqlite.go:162`), `WithReadOnly` (`sqlite/sqlite.go:171`), `WithEnsureDir` (`sqlite/sqlite.go:179`), `Open` (`sqlite/sqlite.go:239`), `Wrap` (`sqlite/sqlite.go:283`), `*DB.SQL` (`sqlite/sqlite.go:345`), `*DB.Close` (`sqlite/sqlite.go:348`), `*DB.Ping` (`sqlite/sqlite.go:351`), `*DB.ExecContext` (`sqlite/sqlite.go:354`), `*DB.QueryContext` (`sqlite/sqlite.go:359`), `*DB.QueryRowContext` (`sqlite/sqlite.go:364`), `*DB.WithImmediateTx` (`sqlite/sqlite.go:372`), `*DB.WithTx` (`sqlite/sqlite.go:410`), `*DB.UserVersion` (`sqlite/sqlite.go:439`), `*DB.Migrate` (`sqlite/sqlite.go:466`), `*DB.Backup` (`sqlite/sqlite.go:540`), `*DB.Checkpoint` (`sqlite/sqlite.go:579`)
- **Constants / Vars**: `SyncOff` (`sqlite/sqlite.go:25`), `SyncNormal` (`sqlite/sqlite.go:27`), `SyncFull` (`sqlite/sqlite.go:29`), `CheckpointPassive` (`sqlite/sqlite.go:37`), `CheckpointFull` (`sqlite/sqlite.go:39`), `CheckpointRestart` (`sqlite/sqlite.go:41`), `CheckpointTruncate` (`sqlite/sqlite.go:43`), `DefaultBusyTimeout` (`sqlite/sqlite.go:48`), `DefaultJournalMode` (`sqlite/sqlite.go:50`), `ErrNoSQLiteDriver` (`sqlite/sqlite.go:55`), `ErrInvalidConfig` (`sqlite/sqlite.go:57`)

### `sse` (19 symbols)

- **Types / Interfaces / Structs**: `Event` (`sse/sse.go:28`), `Option` (`sse/sse.go:47`), `Stream` (`sse/sse.go:96`)
- **Functions / Methods**: `WithHeartbeat` (`sse/sse.go:51`), `WithHeartbeatComment` (`sse/sse.go:61`), `WithRetry` (`sse/sse.go:70`), `WithHeaders` (`sse/sse.go:79`), `NewStream` (`sse/sse.go:112`), `*Stream.LastEventID` (`sse/sse.go:197`), `*Stream.Context` (`sse/sse.go:203`), `*Stream.Send` (`sse/sse.go:208`), `*Stream.SendData` (`sse/sse.go:240`), `*Stream.SendJSON` (`sse/sse.go:245`), `*Stream.SendComment` (`sse/sse.go:254`), `*Stream.Close` (`sse/sse.go:286`), `Handler` (`sse/sse.go:310`)
- **Constants / Vars**: `ContentTypeEventStream` (`sse/sse.go:18`), `ErrStreamingUnsupported` (`sse/sse.go:22`), `ErrStreamClosed` (`sse/sse.go:24`)

### `stringutil` (11 symbols)

- **Functions / Methods**: `OrDefault` (`stringutil/stringutil.go:19`), `Truncate` (`stringutil/stringutil.go:28`), `MaskEmail` (`stringutil/stringutil.go:41`), `MaskPhone` (`stringutil/stringutil.go:58`), `TrimAndLower` (`stringutil/stringutil.go:80`), `IsBlank` (`stringutil/stringutil.go:85`), `ContainsAny` (`stringutil/stringutil.go:90`), `RandomStringFromReader` (`stringutil/stringutil.go:102`), `RandomStringFromCharset` (`stringutil/stringutil.go:118`), `RandomSecureString` (`stringutil/stringutil.go:124`), `RandomAlphanumeric` (`stringutil/stringutil.go:131`)

### `telemetry` (18 symbols)

- **Types / Interfaces / Structs**: `MiddlewareOptions` (`telemetry/middleware.go:17`), `MiddlewareOption` (`telemetry/middleware.go:22`), `Config` (`telemetry/telemetry.go:35`), `Option` (`telemetry/telemetry.go:43`), `TracerProvider` (`telemetry/telemetry.go:75`)
- **Functions / Methods**: `WithPropagator` (`telemetry/middleware.go:25`), `*statusRecorder.WriteHeader` (`telemetry/middleware.go:38`), `Middleware` (`telemetry/middleware.go:46`), `InitProvider` (`telemetry/telemetry.go:26`), `WithServiceName` (`telemetry/telemetry.go:46`), `WithServiceVersion` (`telemetry/telemetry.go:53`), `WithEnvironment` (`telemetry/telemetry.go:60`), `WithSampleRate` (`telemetry/telemetry.go:67`), `*TracerProvider.Config` (`telemetry/telemetry.go:81`), `*TracerProvider.Tracer` (`telemetry/telemetry.go:89`), `*TracerProvider.Shutdown` (`telemetry/telemetry.go:97`), `NewTracerProvider` (`telemetry/telemetry.go:104`), `StartSpan` (`telemetry/telemetry.go:125`)

### `timeutil` (10 symbols)

- **Functions / Methods**: `ParseTime` (`timeutil/timeutil.go:38`), `NowIn` (`timeutil/timeutil.go:54`), `FormatIn` (`timeutil/timeutil.go:63`), `StartOfDay` (`timeutil/timeutil.go:74`), `EndOfDay` (`timeutil/timeutil.go:83`), `AddBusinessDays` (`timeutil/timeutil.go:165`), `BusinessDays` (`timeutil/timeutil.go:185`), `BusinessDaysBetween` (`timeutil/timeutil.go:192`)
- **Constants / Vars**: `ErrInvalidTimeFormat` (`timeutil/timeutil.go:12`), `CommonLayouts` (`timeutil/timeutil.go:15`)

### `uuidutil` (11 symbols)

- **Types / Interfaces / Structs**: `UUID` (`uuidutil/uuidutil.go:12`)
- **Functions / Methods**: `New` (`uuidutil/uuidutil.go:18`), `NewString` (`uuidutil/uuidutil.go:23`), `NewV7` (`uuidutil/uuidutil.go:28`), `NewV7String` (`uuidutil/uuidutil.go:37`), `Parse` (`uuidutil/uuidutil.go:42`), `ParsePtr` (`uuidutil/uuidutil.go:47`), `MustParse` (`uuidutil/uuidutil.go:59`), `IsValid` (`uuidutil/uuidutil.go:64`), `IsNil` (`uuidutil/uuidutil.go:70`)
- **Constants / Vars**: `Nil` (`uuidutil/uuidutil.go:15`)

### `validation` (2 symbols)

- **Types / Interfaces / Structs**: `FieldError` (`validation/validation.go:15`)
- **Functions / Methods**: `FormatErrors` (`validation/validation.go:22`)

### `workerpool` (15 symbols)

- **Types / Interfaces / Structs**: `Metrics` (`workerpool/workerpool.go:23`), `Option` (`workerpool/workerpool.go:32`), `Pool` (`workerpool/workerpool.go:56`)
- **Functions / Methods**: `WithLogger` (`workerpool/workerpool.go:35`), `WithPanicHandler` (`workerpool/workerpool.go:42`), `WithMetrics` (`workerpool/workerpool.go:49`), `New` (`workerpool/workerpool.go:72`), `*Pool.Submit` (`workerpool/workerpool.go:163`), `*Pool.SubmitContext` (`workerpool/workerpool.go:197`), `*Pool.Stop` (`workerpool/workerpool.go:240`), `*Pool.StopWait` (`workerpool/workerpool.go:251`), `*Pool.Drain` (`workerpool/workerpool.go:258`), `*Pool.IsClosed` (`workerpool/workerpool.go:263`)
- **Constants / Vars**: `ErrPoolClosed` (`workerpool/workerpool.go:17`), `ErrQueueFull` (`workerpool/workerpool.go:20`)

## 4. Verification Lookup Commands

Before reading full source files, verify a symbol signature directly with `go doc`:

```bash
go doc github.com/abeta-dev/go-libs/<pkg>.<SymbolName>
```
