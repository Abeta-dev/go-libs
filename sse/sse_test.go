// SPDX-License-Identifier: MIT

package sse_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/abeta-dev/go-libs/sse"
)

type nonFlushingWriter struct {
	header http.Header
	code   int
	body   bytes.Buffer
}

func (n *nonFlushingWriter) Header() http.Header {
	if n.header == nil {
		n.header = make(http.Header)
	}
	return n.header
}
func (n *nonFlushingWriter) Write(p []byte) (int, error) { return n.body.Write(p) }
func (n *nonFlushingWriter) WriteHeader(code int)        { n.code = code }

type wrappedWriter struct {
	inner http.ResponseWriter
}

func (w *wrappedWriter) Header() http.Header         { return w.inner.Header() }
func (w *wrappedWriter) Write(p []byte) (int, error) { return w.inner.Write(p) }
func (w *wrappedWriter) WriteHeader(code int)        { w.inner.WriteHeader(code) }
func (w *wrappedWriter) Unwrap() http.ResponseWriter { return w.inner }

type failAfterNWriter struct {
	mu         sync.Mutex
	header     http.Header
	writesLeft int
	buf        bytes.Buffer
}

func (f *failAfterNWriter) Header() http.Header {
	if f.header == nil {
		f.header = make(http.Header)
	}
	return f.header
}
func (f *failAfterNWriter) WriteHeader(int) {}
func (f *failAfterNWriter) Flush()          {}
func (f *failAfterNWriter) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.writesLeft <= 0 {
		return 0, errors.New("broken pipe")
	}
	f.writesLeft--
	return f.buf.Write(p)
}

type syncRecorder struct {
	mu  sync.Mutex
	rec *httptest.ResponseRecorder
}

func newSyncRecorder() *syncRecorder {
	return &syncRecorder{rec: httptest.NewRecorder()}
}
func (s *syncRecorder) Header() http.Header {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rec.Header()
}
func (s *syncRecorder) WriteHeader(code int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rec.WriteHeader(code)
}
func (s *syncRecorder) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rec.Write(p)
}
func (s *syncRecorder) Flush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rec.Flush()
}
func (s *syncRecorder) BodyString() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rec.Body.String()
}

func TestStream_EventsJSONLastEventIDAndHeartbeat(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	req.Header.Set("Last-Event-ID", "  evt-42 ")
	rec := newSyncRecorder()
	wrapped := &wrappedWriter{inner: rec}

	extraHeaders := http.Header{"X-Stream-Mode": []string{"delta"}}
	stream, err := sse.NewStream(
		wrapped,
		req,
		nil,
		sse.WithHeaders(http.Header{}),
		sse.WithHeaders(extraHeaders),
		sse.WithRetry(2500*time.Millisecond),
		sse.WithHeartbeat(10*time.Millisecond),
		sse.WithHeartbeatComment("ping"),
	)
	if err != nil {
		t.Fatalf("NewStream failed: %v", err)
	}

	if stream.LastEventID() != "evt-42" {
		t.Fatalf("LastEventID = %q, want evt-42", stream.LastEventID())
	}
	if rec.Header().Get("Content-Type") != sse.ContentTypeEventStream {
		t.Fatalf("Content-Type = %q", rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("Cache-Control") != "no-cache, no-transform" {
		t.Fatalf("Cache-Control = %q", rec.Header().Get("Cache-Control"))
	}
	if rec.Header().Get("X-Accel-Buffering") != "no" {
		t.Fatalf("X-Accel-Buffering = %q", rec.Header().Get("X-Accel-Buffering"))
	}
	if rec.Header().Get("X-Stream-Mode") != "delta" {
		t.Fatalf("X-Stream-Mode = %q", rec.Header().Get("X-Stream-Mode"))
	}

	// Send structured multi-line event with sanitized ID/Event and retry
	if err := stream.Send(sse.Event{
		ID:    "43\r\ninjected",
		Event: "token\n",
		Data:  "line1\r\nline2\rline3",
		Retry: 1500 * time.Millisecond,
	}); err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if err := stream.SendData("progress", "50%"); err != nil {
		t.Fatalf("SendData failed: %v", err)
	}
	if err := stream.SendJSON("delta", map[string]any{"count": 3}); err != nil {
		t.Fatalf("SendJSON failed: %v", err)
	}
	if err := stream.SendJSON("bad", make(chan int)); err == nil {
		t.Fatal("expected JSON marshal error")
	}
	// Empty event emits empty data line
	if err := stream.Send(sse.Event{}); err != nil {
		t.Fatalf("empty Send failed: %v", err)
	}

	// Allow at least one heartbeat tick
	time.Sleep(25 * time.Millisecond)

	// Close is idempotent and stops heartbeat cleanly
	if err := stream.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("second Close failed: %v", err)
	}
	if err := stream.SendData("after", "x"); !errors.Is(err, sse.ErrStreamClosed) {
		t.Fatalf("expected ErrStreamClosed after Close, got %v", err)
	}

	body := rec.BodyString()
	for _, want := range []string{
		"retry: 2500\n\n",
		"id: 43injected\nevent: token\nretry: 1500\ndata: line1\ndata: line2\ndata: line3\n\n",
		"event: progress\ndata: 50%\n\n",
		"event: delta\ndata: {\"count\":3}\n\n",
		": ping\n\n",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("stream output missing %q\nfull output:\n%s", want, body)
		}
	}
}

func TestStream_ErrorsAndHandler(t *testing.T) {
	t.Run("unsupported_flusher_and_nil_writer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/events", nil)
		if _, err := sse.NewStream(&nonFlushingWriter{}, req); !errors.Is(err, sse.ErrStreamingUnsupported) {
			t.Fatalf("expected ErrStreamingUnsupported, got %v", err)
		}
		if _, err := sse.NewStream(nil, req); !errors.Is(err, sse.ErrStreamingUnsupported) {
			t.Fatalf("expected ErrStreamingUnsupported for nil writer, got %v", err)
		}
	})

	t.Run("initial_retry_write_error", func(t *testing.T) {
		fw := &failAfterNWriter{writesLeft: 0}
		req := httptest.NewRequest(http.MethodGet, "/events", nil)
		if _, err := sse.NewStream(fw, req, sse.WithRetry(time.Second)); err == nil {
			t.Fatal("expected error when initial retry write fails")
		}
	})

	t.Run("heartbeat_exits_on_write_error", func(t *testing.T) {
		fw := &failAfterNWriter{writesLeft: 0}
		req := httptest.NewRequest(http.MethodGet, "/events", nil)
		stream, err := sse.NewStream(fw, req, sse.WithHeartbeat(5*time.Millisecond))
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
		_ = stream.Close()
	})

	t.Run("context_cancellation_aborts_writes", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		req := httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(ctx)
		rec := newSyncRecorder()
		stream, err := sse.NewStream(rec, req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = stream.Close() }()
		cancel()
		if err := stream.SendData("tok", "x"); !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("http_handler_adapter", func(t *testing.T) {
		h := sse.Handler(func(ctx context.Context, s *sse.Stream) error {
			if ctx == nil {
				t.Error("nil ctx")
			}
			return s.SendData("hello", "world")
		})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sse", nil))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "event: hello\ndata: world\n\n") {
			t.Fatalf("Handler output mismatch: code=%d body=%q", rec.Code, rec.Body.String())
		}

		// Non-flushing writer produces 500
		nf := &nonFlushingWriter{}
		h.ServeHTTP(nf, httptest.NewRequest(http.MethodGet, "/sse", nil))
		if nf.code != http.StatusInternalServerError {
			t.Fatalf("expected 500 for non-flushing writer, got %d", nf.code)
		}
	})
}
