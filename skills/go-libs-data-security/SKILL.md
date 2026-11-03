---
name: go-libs-data-security
description: Database pools, RBAC authorization, JWKS public key resolution, bcrypt cryptography, structured logging, and OpenTelemetry tracing using github.com/Abeta-dev/go-libs (db, rbac, rbaccontext, jwks, cryptoutil, telemetry, logger).
---

# `go-libs-data-security` Subskill (`v0.3.2`)

Packages covered:
- `github.com/Abeta-dev/go-libs/db`
- `github.com/Abeta-dev/go-libs/rbac`
- `github.com/Abeta-dev/go-libs/rbaccontext`
- `github.com/Abeta-dev/go-libs/jwks`
- `github.com/Abeta-dev/go-libs/cryptoutil`
- `github.com/Abeta-dev/go-libs/telemetry`
- `github.com/Abeta-dev/go-libs/logger`

## 1. PostgreSQL Pool & `DBTX` Abstraction (`db`)

Connect using `db.Connect(ctx, dsn, db.WithMaxConns(25))` and accept `db.DBTX` in repositories so methods run transparently against either `*pgxpool.Pool` or an active `pgx.Tx` transaction:

```go
pool, err := db.Connect(ctx, dsn, db.WithMaxConns(25))
querier := db.GetQuerier(ctx, pool) // Returns context-bound pgx.Tx if active, else pool
```

## 2. JWKS Verification & Dynamic RBAC (`jwks`, `rbac`, `rbaccontext`)

1. Resolve and cache remote ECDSA P-256 JWKS public keys using `jwks.New(jwksURL)`.
2. Evaluate wildcard permissions (`orders:*`, `invoices:approve`) using `rbac.NewEngine()` or `rbac.HasPermission(granted, required)`.
3. Attach and verify request permissions in handlers via `rbaccontext.WithPermissions(ctx, perms)` and `rbaccontext.Can(ctx, "invoices:approve")`.

## 3. Password Hashing & CSPRNG Generation (`cryptoutil`)

- Use `cryptoutil.HashPassword(plain)` (bcrypt cost 12) and `cryptoutil.ComparePassword(hash, plain)` / `cryptoutil.IsCorrectPassword(hash, plain)`.
- Generate cryptographically secure temporary passwords using `cryptoutil.GenerateTempPassword(length)`.

## 4. OpenTelemetry & Context-Aware `slog` Logging (`telemetry`, `logger`)

- Initialize OpenTelemetry tracing in `main.go` via `telemetry.NewTracerProvider(telemetry.WithServiceName("..."))` and create child spans with `telemetry.StartSpan(ctx, "operation")`.
- Use `logger.FromContext(ctx)` paired with `logger.NewRedactingHandler` and `logger.NewSamplingHandler` so logs automatically redact sensitive keys and include request correlation attributes.
