# httpmw

Unified standard-library `net/http` middleware suite providing full parity with `ginmw` while pulling **zero Gin, pgx, or OpenTelemetry dependencies** — built for lightweight stdlib BFFs and desktop `.dmg` services.

## Included Middlewares & Handlers

- **Composition**: `Chain`, `Use`
- **Security & Loopback**: `SecurityHeaders`, `LoopbackGuard`, `CORS`, `CORSWithConfig`, `LimitBody`, `LimitBodyDefault`, `LimitBodyAuth`
- **Performance & Caching**: `Compress`, `PrecompressedFileServer`, `PrecompressedFS`, `ETag`
- **Streaming**: `SSE`
- **Observability & Resilience**: `RequestID`, `Logger`, `Recovery`, `REDMetrics`, `MetricsEndpoint`, `RateLimit`, `GlobalRateLimit`, `AuthRateLimit`
- **Multi-Tenancy & Authorization**: `RequireTenant`, `WithTenantContext`, `TenantIDFromContext`, `RequirePermission`, `RequireAnyPermission`

## Usage

```go
guard, _ := loopbackguard.New()
handler := httpmw.Chain(
	mux,
	httpmw.RequestID(),
	httpmw.SecurityHeaders(securityheaders.WithDisableHSTS(), securityheaders.WithCacheControl("private, no-cache")),
	httpmw.LoopbackGuard(guard),
	httpmw.Compress(),
	httpmw.ETag(etag.WithAppVersion("1.0.0"), etag.WithIgnoreJSONKeys("cached_at_iso")),
	httpmw.REDMetrics(),
	httpmw.Logger(),
	httpmw.Recovery(),
)
```
