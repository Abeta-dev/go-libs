# loopbackguard

Local-first loopback security and CSRF protection for desktop and localhost BFFs:

- **`ValidateLoopbackAddr`**: Guarantees listeners only bind to `127.0.0.0/8`, `::1`, or `localhost` (rejecting `:port`, `0.0.0.0`, `[::]`, and LAN/external addresses).
- **DNS-Rebinding `Host` Allowlist**: Rejects non-loopback `Host` headers (`403 DNS_REBINDING_BLOCKED`).
- **`Origin` & `Sec-Fetch-Site` Same-Origin Guards**: Blocks cross-origin and cross-site subresource/mutating requests (`403 ORIGIN_MISMATCH` / `403 CROSS_SITE_BLOCKED`).
- **Constant-Time Per-Boot Ephemeral CSRF Token**: Auto-generates a 256-bit `crypto/rand` token per process boot and verifies unsafe HTTP methods using `crypto/subtle.ConstantTimeCompare`.
- **Zero External Dependencies**: Pure Go standard library Depth-0 leaf package.

## Usage

```go
if err := loopbackguard.ValidateLoopbackAddr("127.0.0.1:8080"); err != nil {
	log.Fatal(err)
}
guard, err := loopbackguard.New()
if err != nil {
	log.Fatal(err)
}
// Inject guard.Token() into index.html via compress.WithCSRFMetaToken(guard.Token())
handler := guard.Middleware(mux)
```
