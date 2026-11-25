// SPDX-License-Identifier: MIT

package etag_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/abeta-dev/go-libs/etag"
)

func TestComputeAndMatch(t *testing.T) {
	body1 := []byte(`{"id":1,"name":"Ada","cached_at_iso":"2026-11-25T00:00:00Z","meta":{"generated_at_iso":"t1","nested":[{"cached_at_iso":"x","v":10}]}}`)
	body2 := []byte(`{"name":"Ada","id":1,"cached_at_iso":"2026-11-25T01:00:00Z","meta":{"nested":[{"v":10,"cached_at_iso":"y"}],"generated_at_iso":"t2"}}`)

	opts := make([]etag.Option, 0, 5)
	opts = append(opts,
		nil,
		etag.WithAppVersion("2.1.0"),
		etag.WithPageVersion("screen-v3"),
		etag.WithIgnoreJSONKeys("cached_at_iso", "generated_at_iso", " "),
	)
	tag1 := etag.Compute(body1, opts...)
	tag2 := etag.Compute(body2, opts...)
	if tag1 != tag2 {
		t.Fatalf("expected identical ETag after ignoring volatile keys: %q vs %q", tag1, tag2)
	}

	weakTag := etag.Compute(body1, append(opts, etag.WithWeak(true))...)
	if !strings.HasPrefix(weakTag, `W/"`) {
		t.Fatalf("expected weak ETag prefix, got %q", weakTag)
	}
	if !etag.Match(weakTag, tag1) {
		t.Fatalf("RFC 9110 weak comparison should match %q with %q", weakTag, tag1)
	}
	if !etag.Match(`"other", `+strings.ToLower(weakTag), tag1) {
		t.Fatalf("expected comma-separated lowercase w/ match")
	}
	if !etag.Match("*", tag1) {
		t.Fatalf("expected wildcard match")
	}
	if etag.Match("", tag1) || etag.Match(tag1, "") || etag.Match(`"nope"`, tag1) {
		t.Fatalf("expected non-matches to return false")
	}

	// Non-JSON payload with ignore keys still hashes raw bytes cleanly
	rawTag := etag.Compute([]byte("not-json-plain"), etag.WithIgnoreJSONKeys("cached_at_iso"))
	if rawTag == "" {
		t.Fatalf("expected non-empty ETag for non-JSON payload")
	}
}

func TestMiddleware_ETag304AndVersionAutoBust(t *testing.T) {
	calls := 0
	mw := etag.Middleware(
		etag.WithAppVersion("1.5.0"),
		etag.WithIgnoreJSONKeys("cached_at_iso", "generated_at_iso"),
		etag.WithCacheControl("private, no-cache"),
		etag.WithClientVersionHeader("X-Client-App-Version"),
		etag.WithMethods("GET", "HEAD"),
	)

	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Vary", "Accept-Encoding")
		w.WriteHeader(http.StatusOK)
		w.WriteHeader(http.StatusAccepted) // duplicate WriteHeader ignored
		if calls == 1 {
			_, _ = w.Write([]byte(`{"screen":"people","cached_at_iso":"2026-11-25T00:00:00Z"}`))
		} else {
			_, _ = w.Write([]byte(`{"screen":"people","cached_at_iso":"2026-11-25T00:05:00Z"}`))
		}
	}))

	// 1. Initial GET -> 200 OK with ETag, X-App-Version, X-Page-Version, Cache-Control
	req1 := httptest.NewRequest(http.MethodGet, "/api/screen/people", nil)
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec1.Code)
	}
	tag := rec1.Header().Get(etag.HeaderETag)
	if tag == "" {
		t.Fatal("missing ETag header")
	}
	if got := rec1.Header().Get(etag.HeaderAppVersion); got != "1.5.0" {
		t.Fatalf("X-App-Version = %q, want 1.5.0", got)
	}
	if got := rec1.Header().Get(etag.HeaderPageVersion); got == "" {
		t.Fatal("missing X-Page-Version header")
	}
	if got := rec1.Header().Get("Cache-Control"); got != "private, no-cache" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if !strings.Contains(rec1.Body.String(), "cached_at_iso") {
		t.Fatalf("response body should keep volatile keys verbatim, got %q", rec1.Body.String())
	}

	// 2. Revalidate with matching If-None-Match + matching client version -> 304 Not Modified (0 bytes)
	req2 := httptest.NewRequest(http.MethodGet, "/api/screen/people", nil)
	req2.Header.Set(etag.HeaderIfNoneMatch, tag)
	req2.Header.Set(etag.HeaderClientAppVersion, "1.5.0")
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", rec2.Code)
	}
	if rec2.Body.Len() != 0 {
		t.Fatalf("expected 0-byte body on 304, got %d bytes", rec2.Body.Len())
	}
	if rec2.Header().Get(etag.HeaderETag) != tag {
		t.Fatalf("expected ETag preserved on 304")
	}

	// 3. Revalidate with matching If-None-Match BUT stale X-Client-App-Version -> auto-bust 200 OK!
	req3 := httptest.NewRequest(http.MethodGet, "/api/screen/people", nil)
	req3.Header.Set(etag.HeaderIfNoneMatch, tag)
	req3.Header.Set(etag.HeaderClientAppVersion, "1.4.0")
	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusOK {
		t.Fatalf("expected auto-bust 200 on client version mismatch, got %d", rec3.Code)
	}
	if rec3.Body.Len() == 0 {
		t.Fatal("expected full JSON body on auto-bust")
	}

	// 4. HEAD returns headers with 0-byte body
	reqHead := httptest.NewRequest(http.MethodHead, "/api/screen/people", nil)
	recHead := httptest.NewRecorder()
	h.ServeHTTP(recHead, reqHead)
	if recHead.Code != http.StatusOK || recHead.Body.Len() != 0 || recHead.Header().Get(etag.HeaderETag) != tag {
		t.Fatalf("HEAD failed: status=%d len=%d etag=%q", recHead.Code, recHead.Body.Len(), recHead.Header().Get(etag.HeaderETag))
	}

	// 5. POST bypasses ETag middleware
	reqPost := httptest.NewRequest(http.MethodPost, "/api/screen/people", nil)
	recPost := httptest.NewRecorder()
	h.ServeHTTP(recPost, reqPost)
	if recPost.Header().Get(etag.HeaderETag) != "" {
		t.Fatalf("POST should bypass ETag middleware")
	}
}

func TestMiddleware_PassthroughAndEdgeCases(t *testing.T) {
	t.Run("sse_flushes_and_bypasses_buffering", func(t *testing.T) {
		var unwrapped http.ResponseWriter
		h := etag.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if u, ok := w.(interface{ Unwrap() http.ResponseWriter }); ok {
				unwrapped = u.Unwrap()
			}
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("data: live\n\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/events", nil))
		if unwrapped != rec || rec.Header().Get(etag.HeaderETag) != "" || rec.Body.String() != "data: live\n\n" {
			t.Fatalf("SSE passthrough failed: etag=%q body=%q", rec.Header().Get(etag.HeaderETag), rec.Body.String())
		}
	})

	t.Run("flush_before_write_triggers_passthrough", func(t *testing.T) {
		h := etag.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			_, _ = w.Write([]byte("streamed"))
		}))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/stream", nil))
		if rec.Code != http.StatusOK || rec.Body.String() != "streamed" || rec.Header().Get(etag.HeaderETag) != "" {
			t.Fatalf("flush passthrough failed: status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("max_buffer_bytes_overflow_streams_directly", func(t *testing.T) {
		h := etag.Middleware(etag.WithMaxBufferBytes(8))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("1234"))
			_, _ = w.Write([]byte("567890"))
		}))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/big", nil))
		if rec.Body.String() != "1234567890" || rec.Header().Get(etag.HeaderETag) != "" {
			t.Fatalf("overflow passthrough failed: body=%q etag=%q", rec.Body.String(), rec.Header().Get(etag.HeaderETag))
		}
	})

	t.Run("handler_no_store_skips_etag_while_outer_no_store_is_overridden", func(t *testing.T) {
		// Handler explicitly sets Cache-Control: no-store
		hNoStore := etag.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			_, _ = w.Write([]byte(`{"secret":true}`))
		}))
		rec1 := httptest.NewRecorder()
		hNoStore.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/secret", nil))
		if rec1.Header().Get(etag.HeaderETag) != "" || rec1.Body.String() != `{"secret":true}` {
			t.Fatalf("expected handler no-store to skip ETag, got etag=%q", rec1.Header().Get(etag.HeaderETag))
		}

		// Outer middleware sets Cache-Control: no-store (e.g. securityheaders.Default), handler does not override
		outer := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Cache-Control", "no-store")
				w.Header().Set("Vary", "X-Client-App-Version")
				next.ServeHTTP(w, r)
			})
		}
		hOuter := outer(etag.Middleware(etag.WithAppVersion("1.0.0"), etag.WithPageVersion("p1"))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("hello"))
		})))
		rec2 := httptest.NewRecorder()
		hOuter.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/public", nil))
		if rec2.Header().Get(etag.HeaderETag) == "" || rec2.Header().Get("Cache-Control") != "private, no-cache" {
			t.Fatalf("expected etag to override outer no-store: etag=%q cc=%q", rec2.Header().Get(etag.HeaderETag), rec2.Header().Get("Cache-Control"))
		}
		if len(rec2.Header().Values("Vary")) != 1 {
			t.Fatalf("expected deduplicated Vary header, got %v", rec2.Header().Values("Vary"))
		}
	})

	t.Run("outer_vs_inner_cache_control_diff_values", func(t *testing.T) {
		outer := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Cache-Control", "private")
				next.ServeHTTP(w, r)
			})
		}
		h := outer(etag.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			_, _ = w.Write([]byte("sensitive"))
		})))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/s", nil))
		if rec.Header().Get(etag.HeaderETag) != "" {
			t.Fatalf("expected ETag skipped when handler replaced Cache-Control with no-store")
		}
	})

	t.Run("non_2xx_and_empty_and_no_write", func(t *testing.T) {
		// 404 bypasses
		h404 := etag.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("not found"))
		}))
		rec404 := httptest.NewRecorder()
		h404.ServeHTTP(rec404, httptest.NewRequest(http.MethodGet, "/missing", nil))
		if rec404.Code != http.StatusNotFound || rec404.Header().Get(etag.HeaderETag) != "" {
			t.Fatalf("expected 404 without ETag")
		}

		// Empty handler (neither WriteHeader nor Write)
		hNoOp := etag.Middleware()(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
		recNoOp := httptest.NewRecorder()
		hNoOp.ServeHTTP(recNoOp, httptest.NewRequest(http.MethodGet, "/noop", nil))
		if recNoOp.Header().Get(etag.HeaderETag) != "" {
			t.Fatalf("expected empty handler to skip ETag")
		}

		// 200 with empty body
		hEmpty := etag.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		recEmpty := httptest.NewRecorder()
		hEmpty.ServeHTTP(recEmpty, httptest.NewRequest(http.MethodGet, "/empty", nil))
		if recEmpty.Code != http.StatusOK || recEmpty.Header().Get(etag.HeaderETag) != "" {
			t.Fatalf("expected 200 empty body without ETag")
		}

		// No Content-Type + no AppVersion with WithIgnoreJSONKeys exercises default JSON sniffing and empty appVersion
		hNoCT := etag.Middleware(etag.WithIgnoreJSONKeys("ts"))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"ok":true,"ts":1}`))
		}))
		recNoCT := httptest.NewRecorder()
		hNoCT.ServeHTTP(recNoCT, httptest.NewRequest(http.MethodGet, "/noct", nil))
		if recNoCT.Header().Get(etag.HeaderETag) == "" {
			t.Fatalf("expected ETag even when Content-Type header omitted")
		}
	})
}

func TestWriteVersionedJSON(t *testing.T) {
	opts := []etag.Option{
		etag.WithAppVersion("3.0.0"),
		etag.WithIgnoreJSONKeys("generated_at_iso"),
		etag.WithCacheControl("private, no-cache"),
	}

	// 1. First write -> 200 OK
	req1 := httptest.NewRequest(http.MethodGet, "/v", nil)
	rec1 := httptest.NewRecorder()
	err := etag.WriteVersionedJSON(rec1, req1, http.StatusOK, map[string]any{
		"items":            []string{"a", "b"},
		"generated_at_iso": "2026-11-25T00:00:00Z",
	}, opts...)
	if err != nil {
		t.Fatalf("WriteVersionedJSON error: %v", err)
	}
	tag := rec1.Header().Get(etag.HeaderETag)
	if rec1.Code != http.StatusOK || tag == "" || rec1.Header().Get(etag.HeaderPageVersion) == "" {
		t.Fatalf("unexpected first response: status=%d etag=%q", rec1.Code, tag)
	}

	// 2. Revalidate with updated volatile field -> 304 Not Modified (0 bytes)
	req2 := httptest.NewRequest(http.MethodGet, "/v", nil)
	req2.Header.Set(etag.HeaderIfNoneMatch, tag)
	rec2 := httptest.NewRecorder()
	err = etag.WriteVersionedJSON(rec2, req2, http.StatusOK, map[string]any{
		"items":            []string{"a", "b"},
		"generated_at_iso": "2026-11-25T99:99:99Z",
	}, opts...)
	if err != nil {
		t.Fatalf("WriteVersionedJSON 304 error: %v", err)
	}
	if rec2.Code != http.StatusNotModified || rec2.Body.Len() != 0 {
		t.Fatalf("expected 304 with 0 bytes, got status=%d len=%d", rec2.Code, rec2.Body.Len())
	}

	// 3. Unmarshalable payload returns error
	recErr := httptest.NewRecorder()
	if err := etag.WriteVersionedJSON(recErr, req1, http.StatusOK, make(chan int)); err == nil {
		t.Fatalf("expected json.Marshal error for channel payload")
	}
}
