// SPDX-License-Identifier: MIT

package sse

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ContentTypeEventStream is the canonical MIME type for Server-Sent Events.
const ContentTypeEventStream = "text/event-stream; charset=utf-8"

var (
	// ErrStreamingUnsupported is returned when the http.ResponseWriter does not support flushing.
	ErrStreamingUnsupported = errors.New("sse: response writer does not support flushing")
	// ErrStreamClosed is returned when attempting to write to a closed Stream.
	ErrStreamClosed = errors.New("sse: stream is closed")
)

// Event represents a single Server-Sent Event frame.
type Event struct {
	// ID sets the optional "id:" field for client Last-Event-ID resumption.
	ID string
	// Event sets the optional "event:" field (for example "token", "progress", or "delta").
	Event string
	// Data sets the payload emitted across one or more "data:" lines.
	Data string
	// Retry sets the optional "retry:" reconnection delay in milliseconds when > 0.
	Retry time.Duration
}

type config struct {
	heartbeat        time.Duration
	heartbeatComment string
	retry            time.Duration
	headers          http.Header
}

// Option configures a Stream or Handler.
type Option func(*config)

// WithHeartbeat enables periodic SSE comment keepalives to prevent idle proxy
// and load-balancer timeouts while streaming.
func WithHeartbeat(interval time.Duration) Option {
	return func(c *config) {
		if interval > 0 {
			c.heartbeat = interval
		}
	}
}

// WithHeartbeatComment overrides the comment text sent on each heartbeat tick.
// Defaults to "keepalive".
func WithHeartbeatComment(comment string) Option {
	return func(c *config) {
		if trimmed := strings.TrimSpace(comment); trimmed != "" {
			c.heartbeatComment = trimmed
		}
	}
}

// WithRetry sends an initial "retry: <ms>" directive immediately when opening the stream.
func WithRetry(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.retry = d
		}
	}
}

// WithHeaders appends extra response headers before writing HTTP 200 OK.
func WithHeaders(h http.Header) Option {
	return func(c *config) {
		if len(h) == 0 {
			return
		}
		if c.headers == nil {
			c.headers = make(http.Header, len(h))
		}
		for k, vals := range h {
			for _, v := range vals {
				c.headers.Add(k, v)
			}
		}
	}
}

// Stream manages a thread-safe Server-Sent Events HTTP response stream.
type Stream struct {
	w           http.ResponseWriter
	flusher     http.Flusher
	lastEventID string
	ctx         context.Context
	cancel      context.CancelFunc

	mu            sync.Mutex
	closed        bool
	heartbeatDone chan struct{}
}

// NewStream negotiates an SSE response on w/r, writes canonical unbuffered headers
// (Content-Type: text/event-stream, Cache-Control: no-cache, no-transform,
// Connection: keep-alive, X-Accel-Buffering: no), flushes the handshake, and
// optionally starts a heartbeat goroutine. Always defer stream.Close().
func NewStream(w http.ResponseWriter, r *http.Request, opts ...Option) (*Stream, error) {
	flusher, ok := resolveFlusher(w)
	if !ok {
		return nil, ErrStreamingUnsupported
	}

	cfg := config{
		heartbeatComment: "keepalive",
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	h := w.Header()
	for k, vals := range cfg.headers {
		for _, v := range vals {
			h.Add(k, v)
		}
	}
	h.Set("Content-Type", ContentTypeEventStream)
	h.Set("Cache-Control", "no-cache, no-transform")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")

	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx, cancel := context.WithCancel(r.Context())
	s := &Stream{
		w:           w,
		flusher:     flusher,
		lastEventID: strings.TrimSpace(r.Header.Get("Last-Event-ID")),
		ctx:         ctx,
		cancel:      cancel,
	}

	if cfg.retry > 0 {
		if err := s.writeFrame([]byte("retry: " + strconv.FormatInt(cfg.retry.Milliseconds(), 10) + "\n\n")); err != nil {
			cancel()
			return nil, err
		}
	}

	if cfg.heartbeat > 0 {
		s.heartbeatDone = make(chan struct{})
		go s.runHeartbeat(cfg.heartbeat, cfg.heartbeatComment)
	}

	return s, nil
}

func resolveFlusher(w http.ResponseWriter) (http.Flusher, bool) {
	for w != nil {
		if f, ok := w.(http.Flusher); ok {
			return f, true
		}
		u, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			return nil, false
		}
		w = u.Unwrap()
	}
	return nil, false
}

func (s *Stream) runHeartbeat(interval time.Duration, comment string) {
	defer close(s.heartbeatDone)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.SendComment(comment); err != nil {
				return
			}
		}
	}
}

// LastEventID returns the client's Last-Event-ID header for stream resumption.
func (s *Stream) LastEventID() string {
	return s.lastEventID
}

// Context returns the stream's lifecycle context, cancelled when the client
// disconnects or Close is called.
func (s *Stream) Context() context.Context {
	return s.ctx
}

// Send formats and flushes a single Event frame. Safe for concurrent calls.
func (s *Stream) Send(ev Event) error {
	var buf bytes.Buffer
	if ev.ID != "" {
		buf.WriteString("id: ")
		buf.WriteString(sanitizeLine(ev.ID))
		buf.WriteByte('\n')
	}
	if ev.Event != "" {
		buf.WriteString("event: ")
		buf.WriteString(sanitizeLine(ev.Event))
		buf.WriteByte('\n')
	}
	if ev.Retry > 0 {
		buf.WriteString("retry: ")
		buf.WriteString(strconv.FormatInt(ev.Retry.Milliseconds(), 10))
		buf.WriteByte('\n')
	}

	if ev.Data != "" || buf.Len() == 0 {
		normalized := strings.ReplaceAll(ev.Data, "\r\n", "\n")
		normalized = strings.ReplaceAll(normalized, "\r", "\n")
		for _, line := range strings.Split(normalized, "\n") {
			buf.WriteString("data: ")
			buf.WriteString(line)
			buf.WriteByte('\n')
		}
	}
	buf.WriteByte('\n')
	return s.writeFrame(buf.Bytes())
}

// SendData sends an event with the given event type and text payload.
func (s *Stream) SendData(eventType, data string) error {
	return s.Send(Event{Event: eventType, Data: data})
}

// SendJSON marshals payload as compact JSON and sends it under eventType.
func (s *Stream) SendJSON(eventType string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return s.Send(Event{Event: eventType, Data: string(raw)})
}

// SendComment writes an SSE comment frame (": <comment>\n\n") and flushes.
func (s *Stream) SendComment(comment string) error {
	var buf bytes.Buffer
	normalized := strings.ReplaceAll(comment, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	for _, line := range strings.Split(normalized, "\n") {
		buf.WriteString(": ")
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	buf.WriteByte('\n')
	return s.writeFrame(buf.Bytes())
}

func (s *Stream) writeFrame(frame []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStreamClosed
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if _, err := s.w.Write(frame); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// Close cancels the stream context, stops any heartbeat goroutine, and prevents
// further writes. Safe to call multiple times.
func (s *Stream) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	done := s.heartbeatDone
	s.mu.Unlock()

	s.cancel()
	if done != nil {
		<-done
	}
	return nil
}

func sanitizeLine(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	return strings.ReplaceAll(s, "\n", "")
}

// Handler wraps fn as a standard net/http HandlerFunc that opens a Stream,
// closes it cleanly on exit, and runs fn until completion or cancellation.
func Handler(fn func(ctx context.Context, stream *Stream) error, opts ...Option) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stream, err := NewStream(w, r, opts...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer func() { _ = stream.Close() }()
		if fn != nil {
			_ = fn(stream.Context(), stream)
		}
	}
}
