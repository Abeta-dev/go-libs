// SPDX-License-Identifier: MIT

package compress_test

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/abeta-dev/go-libs/compress"
	"github.com/andybalholm/brotli"
)

func brotliBytes(t *testing.T, raw []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	bw := brotli.NewWriter(&buf)
	if _, err := bw.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := bw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func gzipBytes(t *testing.T, raw []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPrecompressedFS_SidecarsAndContentTypes(t *testing.T) {
	wasmRaw := []byte("\x00asm\x01\x00\x00\x00" + strings.Repeat("wasm-bytecode", 20))
	jsRaw := []byte("console.log('hello kmp wasm');")
	cssRaw := []byte("body { color: #111; }")
	jsonRaw := []byte(`{"ok":true}`)
	svgRaw := []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)
	txtRaw := []byte("plain text fallback content")
	binRaw := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	indexHTML := []byte("<!doctype html><html><head><title>App</title></head><body>root</body></html>")

	fsys := fstest.MapFS{
		"index.html":    {Data: indexHTML},
		"index.html.br": {Data: brotliBytes(t, indexHTML)},
		"app.wasm":      {Data: wasmRaw},
		"app.wasm.br":   {Data: brotliBytes(t, wasmRaw)},
		"app.wasm.gz":   {Data: gzipBytes(t, wasmRaw)},
		"bundle.js":     {Data: jsRaw},
		"bundle.js.gz":  {Data: gzipBytes(t, jsRaw)},
		"styles.css":    {Data: cssRaw},
		"manifest.json": {Data: jsonRaw},
		"logo.svg":      {Data: svgRaw},
		"readme.txt":    {Data: txtRaw},
		"blob.custom":   {Data: binRaw},
	}

	h := compress.PrecompressedFS(
		fsys,
		nil,
		compress.WithSPAFallback("index.html"),
		compress.WithFallbackExcludePrefixes("/api/", "  "),
		compress.WithImmutableCacheControl("public, max-age=31536000, immutable"),
		compress.WithHTMLCacheControl("private, no-cache"),
	)

	// 1. Brotli preferred for .wasm when br & gzip accepted
	t.Run("prefers_brotli_sidecar_for_wasm", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/app.wasm", nil)
		req.Header.Set("Accept-Encoding", "gzip, br")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if got := rec.Header().Get("Content-Encoding"); got != compress.EncodingBrotli {
			t.Fatalf("Content-Encoding = %q, want br", got)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/wasm" {
			t.Fatalf("Content-Type = %q, want application/wasm", got)
		}
		if got := rec.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
			t.Fatalf("Cache-Control = %q", got)
		}
		decoded, err := io.ReadAll(brotli.NewReader(rec.Body))
		if err != nil || !bytes.Equal(decoded, wasmRaw) {
			t.Fatalf("brotli decode failed: err=%v", err)
		}

		// ETag revalidation -> 304 Not Modified
		etag := rec.Header().Get("ETag")
		if etag == "" || !strings.Contains(etag, "-br") {
			t.Fatalf("expected encoding-scoped ETag, got %q", etag)
		}
		req304 := httptest.NewRequest(http.MethodGet, "/app.wasm", nil)
		req304.Header.Set("Accept-Encoding", "gzip, br")
		req304.Header.Set("If-None-Match", "W/"+etag)
		rec304 := httptest.NewRecorder()
		h.ServeHTTP(rec304, req304)
		if rec304.Code != http.StatusNotModified || rec304.Body.Len() != 0 {
			t.Fatalf("expected 304 with 0-byte body, got status=%d len=%d", rec304.Code, rec304.Body.Len())
		}
	})

	// 2. Gzip higher q-value selects .gz sidecar over .br
	t.Run("gzip_higher_q_selects_gz", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/app.wasm", nil)
		req.Header.Set("Accept-Encoding", "br;q=0.4, gzip;q=0.9")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if got := rec.Header().Get("Content-Encoding"); got != compress.EncodingGzip {
			t.Fatalf("Content-Encoding = %q, want gzip", got)
		}
		gz, err := gzip.NewReader(rec.Body)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := io.ReadAll(gz)
		_ = gz.Close()
		if err != nil || !bytes.Equal(decoded, wasmRaw) {
			t.Fatalf("gzip decode failed: err=%v", err)
		}
	})

	// 3. Falls back from br to gz when only .gz exists
	t.Run("falls_back_to_gz_when_br_missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/bundle.js", nil)
		req.Header.Set("Accept-Encoding", "br, gzip")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if got := rec.Header().Get("Content-Encoding"); got != compress.EncodingGzip {
			t.Fatalf("Content-Encoding = %q, want gzip", got)
		}
		if got := rec.Header().Get("Content-Type"); got != "text/javascript; charset=utf-8" {
			t.Fatalf("Content-Type = %q", got)
		}
	})

	// 4. Falls back from gz to br when gzip preferred but only .br exists
	t.Run("falls_back_to_br_when_gz_missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip;q=1.0, br;q=0.5")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if got := rec.Header().Get("Content-Encoding"); got != compress.EncodingBrotli {
			t.Fatalf("Content-Encoding = %q, want br", got)
		}
		if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
			t.Fatalf("Content-Type = %q", got)
		}
		if got := rec.Header().Get("Cache-Control"); got != "private, no-cache" {
			t.Fatalf("Cache-Control = %q", got)
		}
	})

	// 5. Uncompressed fallback + HEAD + MIME detection
	t.Run("uncompressed_and_mime_types", func(t *testing.T) {
		cases := []struct {
			path     string
			wantMIME string
		}{
			{"/styles.css", "text/css; charset=utf-8"},
			{"/manifest.json", "application/json; charset=utf-8"},
			{"/logo.svg", "image/svg+xml"},
			{"/readme.txt", "text/plain"},
			{"/blob.custom", "application/octet-stream"},
		}
		for _, tc := range cases {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s: status = %d, want 200", tc.path, rec.Code)
			}
			if !strings.HasPrefix(rec.Header().Get("Content-Type"), tc.wantMIME) {
				t.Fatalf("%s: Content-Type = %q, want prefix %q", tc.path, rec.Header().Get("Content-Type"), tc.wantMIME)
			}
		}

		// HEAD returns headers with 0-byte body
		reqHead := httptest.NewRequest(http.MethodHead, "/styles.css", nil)
		recHead := httptest.NewRecorder()
		h.ServeHTTP(recHead, reqHead)
		if recHead.Code != http.StatusOK || recHead.Body.Len() != 0 {
			t.Fatalf("HEAD status=%d bodyLen=%d", recHead.Code, recHead.Body.Len())
		}
	})

	// 6. SPA fallback vs excluded API prefix vs missing asset extension
	t.Run("spa_fallback_and_exclusions", func(t *testing.T) {
		reqSPA := httptest.NewRequest(http.MethodGet, "/people/profile/42", nil)
		recSPA := httptest.NewRecorder()
		h.ServeHTTP(recSPA, reqSPA)
		if recSPA.Code != http.StatusOK || !strings.Contains(recSPA.Body.String(), "<body>root</body>") {
			t.Fatalf("expected SPA fallback index.html, got status=%d body=%q", recSPA.Code, recSPA.Body.String())
		}

		reqAPI := httptest.NewRequest(http.MethodGet, "/api/v1/missing", nil)
		recAPI := httptest.NewRecorder()
		h.ServeHTTP(recAPI, reqAPI)
		if recAPI.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for excluded /api/ prefix, got %d", recAPI.Code)
		}

		reqAsset := httptest.NewRequest(http.MethodGet, "/missing.js", nil)
		recAsset := httptest.NewRecorder()
		h.ServeHTTP(recAsset, reqAsset)
		if recAsset.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for missing asset with extension, got %d", recAsset.Code)
		}
	})

	// 7. Method not allowed & traversal protection
	t.Run("method_and_traversal_guards", func(t *testing.T) {
		reqPost := httptest.NewRequest(http.MethodPost, "/", nil)
		recPost := httptest.NewRecorder()
		h.ServeHTTP(recPost, reqPost)
		if recPost.Code != http.StatusMethodNotAllowed || recPost.Header().Get("Allow") != "GET, HEAD" {
			t.Fatalf("expected 405 Method Not Allowed, got %d Allow=%q", recPost.Code, recPost.Header().Get("Allow"))
		}

		for _, badPath := range []string{"/../etc/passwd", "/a\\b", "/a\x00b"} {
			req := &http.Request{Method: http.MethodGet, URL: httptest.NewRequest(http.MethodGet, "/", nil).URL}
			req.URL.Path = badPath
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("badPath %q: expected 404, got %d", badPath, rec.Code)
			}
		}
	})
}

func TestPrecompressedFS_CSRFMetaInjectionAndOptions(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html":    {Data: []byte(`<html><HEAD><title>KMP</title></HEAD><body>ok</body></html>`)},
		"index.html.br": {Data: brotliBytes(t, []byte(`<html><head></head><body>stale</body></html>`))},
		"nohead.html":   {Data: []byte(`<body>minimal</body>`)},
	}

	t.Run("static_csrf_token_bypasses_sidecar_and_escapes", func(t *testing.T) {
		h := compress.PrecompressedFS(
			fsys,
			compress.WithCSRFMetaToken(`tok"en<1>&2`),
			compress.WithCSRFMetaName("x-csrf"),
		)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "br")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if got := rec.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("expected uncompressed HTML when CSRF token injected, got %q", got)
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("Cache-Control = %q, want no-store", got)
		}
		wantMeta := `<meta name="x-csrf" content="tok&#34;en&lt;1&gt;&amp;2">`
		if !strings.Contains(rec.Body.String(), wantMeta+"</HEAD>") {
			t.Fatalf("missing escaped CSRF meta tag in %q", rec.Body.String())
		}

		// HTML without </head> prepends meta tag
		reqNoHead := httptest.NewRequest(http.MethodGet, "/nohead.html", nil)
		recNoHead := httptest.NewRecorder()
		h.ServeHTTP(recNoHead, reqNoHead)
		if !strings.HasPrefix(recNoHead.Body.String(), wantMeta) {
			t.Fatalf("expected prepended meta tag, got %q", recNoHead.Body.String())
		}
	})

	t.Run("dynamic_csrf_provider_and_custom_html_cache_control_and_head", func(t *testing.T) {
		h := compress.PrecompressedFS(
			fsys,
			compress.WithCSRFMetaTokenProvider(func(_ *http.Request) string { return "dynamic-boot-token" }),
			compress.WithHTMLCacheControl("private, no-cache"),
		)
		req := httptest.NewRequest(http.MethodHead, "/", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
			t.Fatalf("HEAD status=%d bodyLen=%d", rec.Code, rec.Body.Len())
		}
		if got := rec.Header().Get("Cache-Control"); got != "private, no-cache" {
			t.Fatalf("Cache-Control = %q", got)
		}
	})

	t.Run("path_prefix_and_nil_fs_and_dir_fs", func(t *testing.T) {
		// Nil fs returns 404
		nilRec := httptest.NewRecorder()
		compress.PrecompressedFS(nil).ServeHTTP(nilRec, httptest.NewRequest(http.MethodGet, "/", nil))
		if nilRec.Code != http.StatusNotFound {
			t.Fatalf("nil fs status = %d, want 404", nilRec.Code)
		}

		// PathPrefix matching and mismatch
		hPrefix := compress.PrecompressedFS(fsys, compress.WithPathPrefix("/static"))
		recMismatch := httptest.NewRecorder()
		hPrefix.ServeHTTP(recMismatch, httptest.NewRequest(http.MethodGet, "/other/index.html", nil))
		if recMismatch.Code != http.StatusNotFound {
			t.Fatalf("prefix mismatch status = %d, want 404", recMismatch.Code)
		}
		recMatch := httptest.NewRecorder()
		hPrefix.ServeHTTP(recMatch, httptest.NewRequest(http.MethodGet, "/static", nil))
		if recMatch.Code != http.StatusOK {
			t.Fatalf("prefix match status = %d, want 200", recMatch.Code)
		}

		// PrecompressedFileServer from real temp dir
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>disk</h1>"), 0o600); err != nil {
			t.Fatal(err)
		}
		diskHandler := compress.PrecompressedFileServer(dir)
		recDisk := httptest.NewRecorder()
		diskHandler.ServeHTTP(recDisk, httptest.NewRequest(http.MethodGet, "/", nil))
		if recDisk.Code != http.StatusOK || recDisk.Body.String() != "<h1>disk</h1>" {
			t.Fatalf("disk handler failed: status=%d body=%q", recDisk.Code, recDisk.Body.String())
		}
	})
}

type staticInfoFS struct {
	base fs.FS
}

func (s staticInfoFS) Open(name string) (fs.File, error) {
	return s.base.Open(name)
}

func (s staticInfoFS) Stat(name string) (fs.FileInfo, error) {
	return fs.Stat(s.base, name)
}

func (s staticInfoFS) ReadFile(_ string) ([]byte, error) {
	return nil, errors.New("read boom")
}

func TestPrecompressedFS_ReadErrorBranches(t *testing.T) {
	base := fstest.MapFS{
		"index.html": {Data: []byte("<html></html>")},
		"app.js":     {Data: []byte("ok")},
	}
	broken := staticInfoFS{base: base}

	t.Run("read_error_on_injected_html", func(t *testing.T) {
		h := compress.PrecompressedFS(broken, compress.WithCSRFMetaToken("tok"))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/index.html", nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 on broken HTML read, got %d", rec.Code)
		}
	})

	t.Run("read_error_on_static_asset", func(t *testing.T) {
		h := compress.PrecompressedFS(broken)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/app.js", nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 on broken static read, got %d", rec.Code)
		}
	})

	t.Run("invalid_fs_path_dot", func(t *testing.T) {
		// Force clean path with dot fallback
		h := compress.PrecompressedFS(base, compress.WithSPAFallback("."))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 when spaFallback resolves to '.', got %d", rec.Code)
		}
	})

	t.Run("wildcard_and_mismatch_if_none_match", func(t *testing.T) {
		h := compress.PrecompressedFS(base)
		req := httptest.NewRequest(http.MethodGet, "/app.js", nil)
		req.Header.Set("If-None-Match", "*")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotModified {
			t.Fatalf("expected 304 on If-None-Match: *, got %d", rec.Code)
		}

		reqMiss := httptest.NewRequest(http.MethodGet, "/app.js", nil)
		reqMiss.Header.Set("If-None-Match", `"does-not-match"`)
		recMiss := httptest.NewRecorder()
		h.ServeHTTP(recMiss, reqMiss)
		if recMiss.Code != http.StatusOK {
			t.Fatalf("expected 200 on non-matching If-None-Match, got %d", recMiss.Code)
		}
	})
}
