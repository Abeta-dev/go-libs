// SPDX-License-Identifier: MIT

package ginmw_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/abeta-dev/go-libs/compress"
	"github.com/abeta-dev/go-libs/ginmw"
	"github.com/andybalholm/brotli"
	"github.com/gin-gonic/gin"
)

func TestCompress_GinMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(ginmw.Compress(compress.Config{
		MinSize:              64,
		DecompressRequest:    true,
		MaxDecompressedBytes: 256,
	}))

	largeJSON := strings.Repeat("compressible-value-", 20)
	r.GET("/json", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": largeJSON})
	})
	r.GET("/string", func(c *gin.Context) {
		if u, ok := c.Writer.(interface{ Unwrap() http.ResponseWriter }); !ok || u.Unwrap() == nil {
			t.Errorf("expected ginCompressWriter to implement Unwrap() http.ResponseWriter")
		}
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.Status(http.StatusOK)
		_, _ = c.Writer.WriteString(largeJSON)
		c.Writer.Flush()
	})
	r.POST("/echo", func(c *gin.Context) {
		b, _ := io.ReadAll(c.Request.Body)
		c.Data(http.StatusOK, "text/plain", b)
	})

	// 1. Brotli JSON compression
	t.Run("brotli_json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/json", nil)
		req.Header.Set("Accept-Encoding", "gzip, br")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if got := rec.Header().Get("Content-Encoding"); got != compress.EncodingBrotli {
			t.Fatalf("Content-Encoding = %q, want br", got)
		}
		raw, err := io.ReadAll(brotli.NewReader(rec.Body))
		if err != nil {
			t.Fatalf("brotli decode failed: %v", err)
		}
		if !strings.Contains(string(raw), largeJSON) {
			t.Fatalf("decompressed body missing expected payload")
		}
	})

	// 2. Gzip WriteString + Flush compression
	t.Run("gzip_write_string_and_flush", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/string", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if got := rec.Header().Get("Content-Encoding"); got != compress.EncodingGzip {
			t.Fatalf("Content-Encoding = %q, want gzip", got)
		}
		gz, err := gzip.NewReader(rec.Body)
		if err != nil {
			t.Fatalf("gzip.NewReader failed: %v", err)
		}
		raw, err := io.ReadAll(gz)
		_ = gz.Close()
		if err != nil || string(raw) != largeJSON {
			t.Fatalf("gzip decode mismatch: err=%v got=%q", err, string(raw))
		}
	})

	// 3. Request decompression and corrupt payload abort (400 / 413)
	t.Run("decompress_request_and_abort_on_error", func(t *testing.T) {
		var buf bytes.Buffer
		bw := brotli.NewWriter(&buf)
		_, _ = bw.Write([]byte(strings.Repeat("ok-", 30)))
		_ = bw.Close()

		req := httptest.NewRequest(http.MethodPost, "/echo", &buf)
		req.Header.Set("Content-Encoding", "br")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.String() != strings.Repeat("ok-", 30) {
			t.Fatalf("echo failed: status=%d body=%q", rec.Code, rec.Body.String())
		}

		// Corrupt stream aborts Gin pipeline with 400
		reqBad := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader("corrupt"))
		reqBad.Header.Set("Content-Encoding", "gzip")
		recBad := httptest.NewRecorder()
		r.ServeHTTP(recBad, reqBad)
		if recBad.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", recBad.Code)
		}
	})
}
