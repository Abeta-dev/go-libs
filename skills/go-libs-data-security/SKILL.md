---
name: go-libs-data-security
description: Database pools, RBAC authorization, JWKS public key resolution, bcrypt cryptography, structured logging, and OpenTelemetry tracing using github.com/abeta-dev/go-libs (db, rbac, rbaccontext, jwks, cryptoutil, telemetry, logger).
---

# `go-libs-data-security` Subskill (`v0.4.0`)

Packages covered:
- `github.com/abeta-dev/go-libs/db`
- `github.com/abeta-dev/go-libs/rbac`
- `github.com/abeta-dev/go-libs/rbaccontext`
- `github.com/abeta-dev/go-libs/jwks`
- `github.com/abeta-dev/go-libs/cryptoutil`
- `github.com/abeta-dev/go-libs/telemetry`
- `github.com/abeta-dev/go-libs/logger`

## 1. PostgreSQL Pool, `DBTX` Abstraction & Transactions (`db`)

Connect using `db.Connect(ctx, dsn, db.WithMaxConns(25))`, run atomic transactions with `db.WithTx`, and accept `db.DBTX` in repositories so methods run transparently against either `*pgxpool.Pool` or an active `pgx.Tx` transaction:

```go
pool, err := db.Connect(ctx, dsn, db.WithMaxConns(25))

err = db.WithTx(ctx, pool, func(txCtx context.Context) error {
	querier := db.GetQuerier(txCtx, pool) // Returns context-bound pgx.Tx inside WithTx, else pool
	_, err := querier.Exec(txCtx, "INSERT INTO orders (id) VALUES ($1)", orderID)
	return err
})
```

## 2. JWKS Verification & Dynamic RBAC (`jwks`, `rbac`, `rbaccontext`)

1. Resolve and cache remote ECDSA P-256 JWKS public keys using `jwks.NewResolver(jwksURL, 15*time.Minute)` and `resolver.GetECPublicKey(kid)`.
2. Evaluate wildcard permissions (`orders:*`, `invoices:approve`) using `rbac.NewEngine()` or `rbac.HasPermission(granted, required)`.
3. Attach and verify request permissions in handlers via `rbaccontext.WithPermissions(ctx, perms)` and `rbaccontext.Can(ctx, "invoices:approve")`.

## 3. Password Hashing & CSPRNG Generation (`cryptoutil`)

- Use `cryptoutil.HashPassword(plain)` (bcrypt cost 12) and `cryptoutil.ComparePassword(hash, plain)` / `cryptoutil.IsCorrectPassword(hash, plain)`.
- Generate cryptographically secure temporary passwords using `cryptoutil.GenerateTempPassword(length)`.

## 4. OpenTelemetry & Context-Aware `slog` Logging (`telemetry`, `logger`)

- Initialize OpenTelemetry tracing in `main.go` via `telemetry.NewTracerProvider("order-service")` (or `telemetry.InitProvider(ctx, cfg)`) and create child spans with `telemetry.StartSpan(ctx, "service", "operation")`.
- Use `logger.FromContext(ctx)` paired with `logger.NewRedactingHandler` and `logger.NewSamplingHandler` so logs automatically redact sensitive keys and include request correlation attributes.
