# `compress` — Brotli (`br`) & Gzip (`gzip`) HTTP Compression Middleware

`compress` provides a zero-leak, leaf-safe `net/http` middleware supporting RFC 9110 `Accept-Encoding` quality negotiation (`br` preferred over `gzip`), `sync.Pool`-backed writers, minimum size threshold buffering, and bounded request body decompression with zip-bomb protection.

## Features

- **Brotli (`br`) & Gzip (`gzip`) Content Negotiation**: Parses `Accept-Encoding` quality weights (`q=0.0`..`1.0`) and wildcards (`*`), preferring Brotli when both encodings share equal positive quality.
- **Pooled Zero-Leak Writers**: Reuses `*brotli.Writer` and `*gzip.Writer` instances via `sync.Pool` to minimize GC pressure on hot endpoints.
- **Threshold Buffering (`MinSize`)**: Buffers up to `MinSize` bytes (default `1024`) before committing to compression, avoiding compression overhead on small responses.
- **Zip-Bomb Safe Request Decompression**: When `DecompressRequest: true`, transparently decompresses incoming `Content-Encoding: br` or `gzip` payloads up to `MaxDecompressedBytes` (default `10 MiB`), returning `413 Request Entity Too Large` when exceeded or `400 Bad Request` on malformed streams.
- **SSE-Safe Streaming (`text/event-stream`)**: Explicitly excludes `text/event-stream` from `"text/"` matching and bypasses threshold buffering immediately on `WriteHeader` so Server-Sent Events flush straight through uncompressed.
- **Precompressed Static / SPA File Server (`PrecompressedFileServer` / `PrecompressedFS`)**: Serves build-time `<file>.br` and `<file>.gz` sidecars (such as KMP WasmJS bundles) with RFC 9110 `Accept-Encoding` negotiation, SPA `index.html` fallback, and optional HTML `<meta>` CSRF token injection.

## Usage

```go
package main

import (
	"net/http"

	"github.com/abeta-dev/go-libs/compress"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/catalog", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"items":[...]}`))
	})

	handler := compress.Middleware(compress.Config{
		MinSize:              1024,
		DecompressRequest:    true,
		MaxDecompressedBytes: 10 << 20, // 10 MiB zip-bomb limit
	})(mux)

	_ = http.ListenAndServe(":8080", handler)
}
```
