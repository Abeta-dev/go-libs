---
name: go-libs-data-security
description: Database transactions, RBAC authorization, JWKS/JWT verification, AES-GCM/HMAC cryptography, structured logging, and OpenTelemetry bootstrap using github.com/Abeta-dev/go-libs (db, rbac, rbaccontext, jwks, cryptoutil, telemetry, logger).
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

## 1. PostgreSQL Pool & Transactional Boundaries (`db`)

Use `db.WithTx` to guarantee automatic rollback on error or panic and commit on `nil` return:

```go
err := db.WithTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
	if err := orderRepo.InsertTx(ctx, tx, order); err != nil {
		return err
	}
	// Pair with go-app-kit/outbox.InsertTx or go-app-kit/audit.RecordTx here
	return nil
})
```

## 2. JWKS Verification & Context-Propagated RBAC (`jwks`, `rbac`, `rbaccontext`)

1. Validate incoming Bearer tokens against a cached remote JWKS endpoint using `jwks.Validator`.
2. Store the verified subject and roles in `context.Context` via `rbaccontext.WithPrincipal(ctx, principal)`.
3. Enforce fine-grained permissions (`invoices:approve`, `payouts:create`) at the route or service layer using `rbac.Enforcer`.

## 3. Authenticated Encryption & HMAC (`cryptoutil`)

- Use `cryptoutil.EncryptAESGCM(key32, plaintext)` and `cryptoutil.DecryptAESGCM(key32, ciphertext)` for encrypting sensitive PII/tokens at rest (automatically generates a unique 12-byte nonce per encryption).
- Use `cryptoutil.ComputeHMACSHA256` and constant-time `cryptoutil.VerifyHMACSHA256` (`hmac.Equal`) for webhook signatures and signed tokens.

## 4. OpenTelemetry & Structured Logging (`telemetry`, `logger`)

- Bootstrap OTel once in `main.go` via `telemetry.Init(ctx, telemetry.Config{...})` and register its `Shutdown` function with `shutdown.Manager`.
- Use `logger.FromContext(ctx)` so every log entry automatically carries `trace_id`, `span_id`, and `request_id` for Loki-to-Tempo correlation in `cloud-native-observability`.
