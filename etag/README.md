# etag

RFC 9110 `ETag` / `If-None-Match` (`304 Not Modified`) and version-gated HTTP middleware with volatile JSON key stripping, `X-App-Version` / `X-Page-Version` headers, and `X-Client-App-Version` mismatch auto-bust.

## Features

- **Zero External Dependencies**: Pure Go standard library (`net/http`, `crypto/sha256`, `encoding/json`) Depth-0 leaf package.
- **Volatile JSON Key Stripping**: `WithIgnoreJSONKeys("cached_at_iso", "generated_at_iso")` computes canonical hashes over stable payload fields while returning the full response body verbatim.
- **Version-Gated Auto-Bust**: `WithAppVersion(v)` stamps `X-App-Version`, mixes `v` into the ETag digest, sets `Vary: X-Client-App-Version`, and forces `200 OK` whenever a client sends a mismatched `X-Client-App-Version`.
- **SSE & Stream Safe**: Immediately passes through `text/event-stream`, `Flush()`, non-2xx responses, handler-explicit `Cache-Control: no-store`, and payloads exceeding `WithMaxBufferBytes`.
- **Direct Handler Helper**: `WriteVersionedJSON` replaces bespoke versioned JSON response helpers in BFFs and SDUI handlers.

## Usage

```go
mw := etag.Middleware(
	etag.WithAppVersion("1.4.0"),
	etag.WithIgnoreJSONKeys("cached_at_iso", "generated_at_iso"),
	etag.WithCacheControl("private, no-cache"),
)

mux := http.NewServeMux()
mux.HandleFunc("/api/screen/people", func(w http.ResponseWriter, r *http.Request) {
	_ = etag.WriteVersionedJSON(w, r, http.StatusOK, payload,
		etag.WithAppVersion("1.4.0"),
		etag.WithIgnoreJSONKeys("cached_at_iso"),
	)
})
handler := mw(mux)
```
