# `bodylimit` Package

The `bodylimit` package provides HTTP request body bounding middleware, protecting microservices against Denial of Service (DoS) attacks and Out-of-Memory (OOM) crashes caused by oversized payload ingestion.

## When to Use
- **Inbound Edge Protection**: Enforcing maximum request payload sizes globally across all REST API routes (e.g. 1MB default).
- **File & Document Upload Routes**: Setting explicit higher thresholds (e.g. 15MB) specifically on multipart upload endpoints.
- **OOM Kill Prevention**: Preventing malicious or buggy clients from streaming multi-gigabyte JSON payloads into unbuffered unmarshalers.

## Why It Is Written Like That
- **Streaming Socket Truncation**: Utilizes `http.MaxBytesReader` to wrap the raw socket reader. If incoming bytes exceed the configured limit, the read is terminated immediately without waiting for the full transmission.
- **Immediate TCP Termination**: Halts buffering immediately and returns HTTP `413 Payload Too Large` via standardized `httputil.APIError`, preserving RAM and CPU.
- **Zero Allocations**: Evaluates limits on the fly during stream reading without buffering payload copies in heap memory.

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/bodylimit` |
|---|---|---|---|
| **No Inbound Bounding** | Zero setup | A single 2GB POST payload can crash the container with an OOM error | Defensive bounds prevent memory exhaustion and pod restarts |
| **Nginx / Ingress `client_max_body_size`** | Handled at proxy layer | Does not protect internal mesh traffic or services without an ingress reverse proxy; difficult to vary per route | Service-level enforcement ensures protection across all environments and enables per-route size policies |
| **Reading Entire Body First (`io.ReadAll`)** | Easy size check via `len(bytes)` | Buffers the entire oversized payload into heap memory before checking size, defeating the purpose | Streams through `http.MaxBytesReader` to fail fast before memory is consumed |

---

## Quickstart

```go
import (
    "github.com/gin-gonic/gin"
    "github.com/umesh0492/go-libs/bodylimit"
)

r := gin.New()

// 1. Global protection: Default limits strictly to 1MB max bounds
r.Use(bodylimit.LimitBodyDefault())

// 2. Specific route protection for heavy document uploads (e.g. 15MB)
r.POST("/api/v1/invoices/upload", bodylimit.LimitBodyAuth(15), invoiceUploadHandler)
```

---

## 🛡️ Edge Cases Handled
- **Early Stream Abortion**: Uses standard library `http.MaxBytesReader` to close the socket as soon as the threshold is crossed, saving upstream client bandwidth and server memory.
- **Standardized Error Envelope**: Integrates with `httputil` to return RFC-compliant 413 error payloads with clear diagnostic error codes.
