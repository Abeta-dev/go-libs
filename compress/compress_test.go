// SPDX-License-Identifier: MIT

package compress_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/abeta-dev/go-libs/compress"
	"github.com/andybalholm/brotli"
)

func TestNegotiate(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"empty", "", ""},
		{"whitespace", "   ", ""},
		{"empty_tokens", " , , ", ""},
		{"gzip_only", "gzip", compress.EncodingGzip},
		{"xgzip", "x-gzip", compress.EncodingGzip},
		{"brotli_only", "br", compress.EncodingBrotli},
		{"both_equal_prefers_brotli", "gzip, deflate, br", compress.EncodingBrotli},
		{"gzip_higher_q", "br;q=0.7, gzip;q=0.9", compress.EncodingGzip},
		{"brotli_higher_q", "br;q=1.0, gzip;q=0.8, *;q=0.1", compress.EncodingBrotli},
		{"wildcard_prefers_brotli", "*;q=0.5", compress.EncodingBrotli},
		{"wildcard_with_br_zero_falls_back_to_gzip", "br;q=0, *;q=0.5", compress.EncodingGzip},
		{"wildcard_with_gzip_zero_uses_br", "gzip;q=0, *;q=0.5", compress.EncodingBrotli},
		{"all_zero", "br;q=0, gzip;q=0", ""},
		{"invalid_q_nan", "br;q=NaN", ""},
		{"invalid_q_negative", "br;q=-0.1", ""},
		{"invalid_q_above_one", "br;q=1.5", ""},
		{"invalid_q_text", "br;q=abc, gzip;q=0.8", compress.EncodingGzip},
		{"unsupported_only", "deflate, identity", ""},
		{"extra_params", "br;level=4;q=0.9", compress.EncodingBrotli},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := compress.Negotiate(tc.header); got != tc.want {
				t.Errorf("Negotiate(%q) = %q, want %q", tc.header, got, tc.want)
			}
		})
	}
}

func TestMiddleware_BrotliAndGzipCompression(t *testing.T) {
	payload := strings.Repeat(`{"status":"ok","message":"compressible json payload"}`, 40)

	handler := compress.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Length", "9999")
		w.WriteHeader(http.StatusOK)
		w.WriteHeader(http.StatusAccepted) // duplicate WriteHeader call is ignored
		_, _ = w.Write([]byte(payload[:100]))
		_, _ = w.Write([]byte(payload[100:1200]))
		_, _ = w.Write([]byte(payload[1200:]))
	}))

	// 1. Brotli compression
	reqBr := httptest.NewRequest(http.MethodGet, "/data", nil)
	reqBr.Header.Set("Accept-Encoding", "gzip, br")
	recBr := httptest.NewRecorder()
	handler.ServeHTTP(recBr, reqBr)

	if recBr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recBr.Code)
	}
	if got := recBr.Header().Get("Content-Encoding"); got != compress.EncodingBrotli {
		t.Fatalf("Content-Encoding = %q, want %q", got, compress.EncodingBrotli)
	}
	if got := recBr.Header().Get("Content-Length"); got != "" {
		t.Fatalf("Content-Length should be stripped, got %q", got)
	}
	if got := recBr.Header().Get("Vary"); got != "Accept-Encoding" {
		t.Fatalf("Vary = %q, want Accept-Encoding", got)
	}
	decompressedBr, err := io.ReadAll(brotli.NewReader(recBr.Body))
	if err != nil {
		t.Fatalf("brotli read error: %v", err)
	}
	if string(decompressedBr) != payload {
		t.Fatalf("brotli payload mismatch: len=%d want=%d", len(decompressedBr), len(payload))
	}

	// 2. Gzip compression
	reqGz := httptest.NewRequest(http.MethodGet, "/data", nil)
	reqGz.Header.Set("Accept-Encoding", "gzip")
	recGz := httptest.NewRecorder()
	handler.ServeHTTP(recGz, reqGz)

	if got := recGz.Header().Get("Content-Encoding"); got != compress.EncodingGzip {
		t.Fatalf("Content-Encoding = %q, want %q", got, compress.EncodingGzip)
	}
	gzReader, err := gzip.NewReader(recGz.Body)
	if err != nil {
		t.Fatalf("gzip.NewReader error: %v", err)
	}
	decompressedGz, err := io.ReadAll(gzReader)
	_ = gzReader.Close()
	if err != nil {
		t.Fatalf("gzip read error: %v", err)
	}
	if string(decompressedGz) != payload {
		t.Fatalf("gzip payload mismatch: len=%d want=%d", len(decompressedGz), len(payload))
	}
}

func TestMiddleware_BypassAndEdgeCases(t *testing.T) {
	largeText := strings.Repeat("hello compressible world ", 100)

	// 1. Small payload (< MinSize) stays uncompressed
	t.Run("below_min_size", func(t *testing.T) {
		h := compress.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"small":true}`))
		}))
		req := httptest.NewRequest(http.MethodGet, "/small", nil)
		req.Header.Set("Accept-Encoding", "br, gzip")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if got := rec.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("expected no Content-Encoding, got %q", got)
		}
		if rec.Body.String() != `{"small":true}` {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	// 2. Non-compressible Content-Type across multiple writes >= MinSize
	t.Run("non_compressible_content_type", func(t *testing.T) {
		h := compress.Middleware(compress.Config{
			MinSize:              64,
			GzipLevel:            99, // invalid -> normalized to DefaultCompression
			BrotliLevel:          99, // invalid -> normalized to DefaultCompression
			MaxDecompressedBytes: -1, // <= 0 -> normalized
		})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte(strings.Repeat("a", 100)))
			_, _ = w.Write([]byte(strings.Repeat("b", 100)))
		}))
		req := httptest.NewRequest(http.MethodGet, "/img", nil)
		req.Header.Set("Accept-Encoding", "br")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if got := rec.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("expected no Content-Encoding for image/png, got %q", got)
		}
		if rec.Body.Len() != 200 {
			t.Fatalf("expected 200 bytes, got %d", rec.Body.Len())
		}
	})

	// 3. Content-Type sniffing when header omitted + prefix match ("text/")
	t.Run("sniff_content_type_and_flush", func(t *testing.T) {
		var unwrapped http.ResponseWriter
		h := compress.Middleware(compress.Config{MinSize: -1})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if u, ok := w.(interface{ Unwrap() http.ResponseWriter }); ok {
				unwrapped = u.Unwrap()
			}
			if f, ok := w.(http.Flusher); ok {
				// Flush before writing anything, then write
				_, _ = w.Write([]byte(largeText))
				f.Flush()
			}
		}))
		req := httptest.NewRequest(http.MethodGet, "/text", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if unwrapped != rec {
			t.Fatalf("expected Unwrap() to return underlying ResponseRecorder")
		}
		if got := rec.Header().Get("Content-Encoding"); got != compress.EncodingGzip {
			t.Fatalf("expected gzip, got %q", got)
		}
		if !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain") {
			t.Fatalf("expected sniffed text/plain Content-Type, got %q", rec.Header().Get("Content-Type"))
		}
	})

	// 4. Flush before reaching MinSize flushes uncompressed
	t.Run("flush_before_min_size", func(t *testing.T) {
		h := compress.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("chunk1"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			_, _ = w.Write([]byte("chunk2"))
		}))
		req := httptest.NewRequest(http.MethodGet, "/stream", nil)
		req.Header.Set("Accept-Encoding", "br")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if got := rec.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("expected uncompressed on early flush < MinSize, got %q", got)
		}
		if rec.Body.String() != "chunk1chunk2" {
			t.Fatalf("got %q, want chunk1chunk2", rec.Body.String())
		}
	})

	// 5. Status 204 No Content, 304 Not Modified, HEAD, Upgrade, existing Content-Encoding, Cache-Control: no-transform
	t.Run("skip_conditions", func(t *testing.T) {
		// 204 No Content
		h204 := compress.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		req204 := httptest.NewRequest(http.MethodGet, "/no-content", nil)
		req204.Header.Set("Accept-Encoding", "br")
		rec204 := httptest.NewRecorder()
		rec204.Header().Set("Vary", "Origin, Accept-Encoding") // pre-existing Vary should not duplicate
		h204.ServeHTTP(rec204, req204)
		if rec204.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rec204.Code)
		}
		if len(rec204.Header().Values("Vary")) != 1 {
			t.Fatalf("Vary duplicated: %v", rec204.Header().Values("Vary"))
		}

		// 304 Not Modified even if handler attempts write
		h304 := compress.Middleware(compress.Config{MinSize: 10})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusNotModified)
			_, _ = w.Write([]byte(strings.Repeat("x", 32)))
		}))
		req304 := httptest.NewRequest(http.MethodGet, "/304", nil)
		req304.Header.Set("Accept-Encoding", "br")
		rec304 := httptest.NewRecorder()
		h304.ServeHTTP(rec304, req304)
		if rec304.Header().Get("Content-Encoding") != "" {
			t.Fatalf("expected no compression on 304")
		}

		// Pre-existing Content-Encoding
		hPre := compress.Middleware(compress.Config{MinSize: 10})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.Header().Set("Content-Encoding", compress.EncodingIdentity)
			_, _ = w.Write([]byte(strings.Repeat("x", 32)))
		}))
		reqPre := httptest.NewRequest(http.MethodGet, "/pre", nil)
		reqPre.Header.Set("Accept-Encoding", "br")
		recPre := httptest.NewRecorder()
		hPre.ServeHTTP(recPre, reqPre)
		if got := recPre.Header().Get("Content-Encoding"); got != compress.EncodingIdentity {
			t.Fatalf("Content-Encoding = %q, want identity", got)
		}

		// Cache-Control: no-transform
		hNoTransform := compress.Middleware(compress.Config{MinSize: 10})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.Header().Set("Cache-Control", "private, no-transform")
			_, _ = w.Write([]byte(strings.Repeat("x", 32)))
		}))
		reqNT := httptest.NewRequest(http.MethodGet, "/nt", nil)
		reqNT.Header.Set("Accept-Encoding", "br")
		recNT := httptest.NewRecorder()
		hNoTransform.ServeHTTP(recNT, reqNT)
		if got := recNT.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("expected no compression with Cache-Control: no-transform, got %q", got)
		}

		// Empty handler (neither WriteHeader nor Write called)
		hEmpty := compress.Middleware()(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
		reqEmpty := httptest.NewRequest(http.MethodGet, "/empty", nil)
		reqEmpty.Header.Set("Accept-Encoding", "br")
		recEmpty := httptest.NewRecorder()
		hEmpty.ServeHTTP(recEmpty, reqEmpty)
		if recEmpty.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", recEmpty.Code)
		}

		// HEAD request skipped
		reqHead := httptest.NewRequest(http.MethodHead, "/head", nil)
		reqHead.Header.Set("Accept-Encoding", "br")
		recHead := httptest.NewRecorder()
		hEmpty.ServeHTTP(recHead, reqHead)
		if recHead.Header().Get("Vary") != "Accept-Encoding" {
			t.Fatalf("expected Vary on HEAD")
		}
	})
}

func TestMiddleware_DecompressRequest(t *testing.T) {
	cfg := compress.Config{
		DecompressRequest:    true,
		MaxDecompressedBytes: 128,
	}

	var gotBody string
	var gotEncoding, gotLength string
	h := compress.Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEncoding = r.Header.Get("Content-Encoding")
		gotLength = r.Header.Get("Content-Length")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))

	// 1. Valid Brotli request body
	t.Run("valid_brotli_request", func(t *testing.T) {
		var buf bytes.Buffer
		bw := brotli.NewWriter(&buf)
		_, _ = bw.Write([]byte(`{"hello":"brotli"}`))
		_ = bw.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest", &buf)
		req.Header.Set("Content-Encoding", "br")
		req.Header.Set("Content-Length", "99")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if gotBody != `{"hello":"brotli"}` {
			t.Fatalf("gotBody = %q", gotBody)
		}
		if gotEncoding != "" || gotLength != "" {
			t.Fatalf("expected stripped headers, got encoding=%q length=%q", gotEncoding, gotLength)
		}
	})

	// 2. Valid Gzip request body
	t.Run("valid_gzip_request", func(t *testing.T) {
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		_, _ = gw.Write([]byte(`{"hello":"gzip"}`))
		_ = gw.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest", &buf)
		req.Header.Set("Content-Encoding", "gzip")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK || gotBody != `{"hello":"gzip"}` {
			t.Fatalf("status=%d body=%q", rec.Code, gotBody)
		}
	})

	// 3. Corrupt Gzip header -> 400 Bad Request
	t.Run("corrupt_gzip_header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader("not-a-gzip-stream"))
		req.Header.Set("Content-Encoding", "gzip")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
	})

	// 4. Corrupt Brotli stream -> 400 Bad Request
	t.Run("corrupt_brotli_stream", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewReader([]byte{0xff, 0xff, 0xff}))
		req.Header.Set("Content-Encoding", "br")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
	})

	// 5. Decompressed size exceeds MaxDecompressedBytes -> 413 Request Entity Too Large
	t.Run("zip_bomb_exceeds_limit", func(t *testing.T) {
		var buf bytes.Buffer
		bw := brotli.NewWriter(&buf)
		_, _ = bw.Write([]byte(strings.Repeat("A", 512))) // 512 > 128 limit
		_ = bw.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest", &buf)
		req.Header.Set("Content-Encoding", "br")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want 413", rec.Code)
		}
	})
}
