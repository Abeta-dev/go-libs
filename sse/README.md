# sse

First-class Server-Sent Events (`text/event-stream`) streaming helper for AI token streaming, background sync progress, and real-time SDUI delta updates.

## Features

- **Zero External Dependencies**: Pure Go standard library Depth-0 leaf package.
- **Unbuffered Proxy-Safe Headers**: Sets `Content-Type: text/event-stream; charset=utf-8`, `Cache-Control: no-cache, no-transform`, `Connection: keep-alive`, and `X-Accel-Buffering: no`.
- **Thread-Safe Framing & Heartbeats**: Serializes event frames and periodic `WithHeartbeat` comment keepalives cleanly without goroutine leaks.
- **Resumption & Cancellation**: Exposes `LastEventID()` from the `Last-Event-ID` header and cancels `Context()` cleanly on client disconnect or `Close()`.

## Usage

```go
mux.Handle("/api/stream", sse.Handler(func(ctx context.Context, stream *sse.Stream) error {
	lastID := stream.LastEventID()
	return stream.SendJSON("delta", map[string]any{"after": lastID, "status": "ready"})
}, sse.WithHeartbeat(15*time.Second), sse.WithRetry(3*time.Second)))
```
